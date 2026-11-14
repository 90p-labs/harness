package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/pflag"

	paneldomain "github.com/90p-labs/harness/internal/panel/domain"
	planmodule "github.com/90p-labs/harness/internal/plan"
	runmodule "github.com/90p-labs/harness/internal/run"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

// panelRepos serves every repo of the registry. On each List it reads the registry, so a repo
// that registers later shows up on the next poll, and it loads the runs of each repo from that
// repo's own state, plan files and branch head.
//
// The panel only reads the state of a repo that is not its own: it opens the file without
// migrating it, and it never creates a folder for it. A repo that fails to load gets one line
// under its key in Problems, and the other repos are not affected.
type panelRepos struct {
	registry paneldomain.Registry
	// flags are the serve flags. They apply only to the serve repo (serveKey), which only a
	// hand-run `harness panel serve` has. A spawned panel has serveKey "", so every repo is
	// file-only.
	flags    *pflag.FlagSet
	serveKey string
	log      io.Writer
	now      func() time.Time
	// open opens the state store of a repo. It must not migrate it.
	open func(ctx context.Context, cfg config.Config, commonDir string) (*dbal.DB, error)

	mu     sync.Mutex
	closed bool
	repos  map[string]*loadedRepo
	failed map[string]string
}

var errPanelReposClosed = errors.New("panel repos are closed")

// loadedRepo is the cached state of one repo: its store, and the readers built on it.
type loadedRepo struct {
	worktree string
	db       *dbal.DB
	runs     panelRuns
	pages    panelPages

	// refs counts the calls that read the store, and retired marks a store that left the cache.
	// A retired store closes when its last reader releases it. Both are guarded by panelRepos.mu.
	refs    int
	retired bool
}

func newPanelRepos(registry paneldomain.Registry, flags *pflag.FlagSet, serveKey string, log io.Writer) *panelRepos {
	if log == nil {
		log = io.Discard
	}
	return &panelRepos{
		registry: registry, flags: flags, serveKey: serveKey, log: log, now: time.Now,
		open:   openStateReadOnly,
		repos:  map[string]*loadedRepo{},
		failed: map[string]string{},
	}
}

// openStateReadOnly opens the state of a repo with no migration. The driver still applies
// the WAL pragma of the DSN. It runs after a stat of the file, because the driver creates
// the folder of a file that is not there.
func openStateReadOnly(ctx context.Context, cfg config.Config, commonDir string) (*dbal.DB, error) {
	return dbal.Open(ctx, cfg.State.DSN, commonDir)
}

// List reads the registry, closes the store of a repo that left it, and lists the runs of each repo.
func (r *panelRepos) List(ctx context.Context) (paneldomain.Listing, error) {
	registered, err := r.registry.List(ctx)
	if err != nil {
		return paneldomain.Listing{}, err
	}
	listing := paneldomain.Listing{Repos: registered, Problems: map[string]string{}}
	loaded := make([]*loadedRepo, len(registered))
	loadErrs := make([]error, len(registered))

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return paneldomain.Listing{}, errPanelReposClosed
	}
	r.prune(registered)
	r.mu.Unlock()
	defer func() {
		for _, l := range loaded {
			if l != nil {
				r.release(l)
			}
		}
	}()

	for i, repo := range registered {
		loaded[i], loadErrs[i] = r.load(ctx, repo)
		if errors.Is(loadErrs[i], errPanelReposClosed) {
			return paneldomain.Listing{}, errPanelReposClosed
		}
	}
	for i, repo := range registered {
		err := loadErrs[i]
		var part paneldomain.Listing
		if err == nil {
			part, err = loaded[i].runs.List(ctx)
		}
		if err != nil {
			r.fail(repo, err)
			listing.Problems[repo.Key] = firstLine(err)
			continue
		}
		r.recover(repo)
		listing.Cards = append(listing.Cards, part.Cards...)
	}
	if len(listing.Problems) == 0 {
		listing.Problems = nil
	}
	return listing, nil
}

// Get reads one run from the repo of its ref.
func (r *panelRepos) Get(ctx context.Context, ref paneldomain.RunRef) (paneldomain.Card, error) {
	l, err := r.loaded(ctx, ref.Repo)
	if err != nil {
		return paneldomain.Card{}, err
	}
	defer r.release(l)
	return l.runs.Get(ctx, ref)
}

func (r *panelRepos) Page(ctx context.Context, card paneldomain.Card, served paneldomain.Served) ([]byte, error) {
	l, err := r.loaded(ctx, card.Repo)
	if err != nil {
		return nil, err
	}
	defer r.release(l)
	return l.pages.Page(ctx, card, served)
}

func (r *panelRepos) Region(ctx context.Context, card paneldomain.Card, name string, served paneldomain.Served) ([]byte, error) {
	l, err := r.loaded(ctx, card.Repo)
	if err != nil {
		return nil, err
	}
	defer r.release(l)
	return l.pages.Region(ctx, card, name, served)
}

func (r *panelRepos) HasRegion(name string) bool { return panelPages{}.HasRegion(name) }

// Close closes the store of every repo, once its readers are done. Calls after Close return an
// error and open nothing. A second Close does nothing.
func (r *panelRepos) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	var errs []error
	for key, l := range r.repos {
		errs = append(errs, r.retire(l))
		delete(r.repos, key)
	}
	return errors.Join(errs...)
}

// retire marks a store as out of the cache and closes it when no call reads it. The caller holds mu.
func (r *panelRepos) retire(l *loadedRepo) error {
	l.retired = true
	if l.refs == 0 {
		return l.db.Close()
	}
	return nil
}

// release ends one read of l, and closes a retired store that no call reads any more.
func (r *panelRepos) release(l *loadedRepo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l.refs--
	if l.retired && l.refs == 0 {
		_ = l.db.Close()
	}
}

// loaded returns the readers of the repo with key, held for the caller, who must release them.
// A key that the last List loaded is served from the cache with no registry read. Any other key
// reads the registry once, for a direct link before any board list.
// A repo that is not registered, or that fails to load, is not found.
func (r *panelRepos) loaded(ctx context.Context, key string) (*loadedRepo, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, errPanelReposClosed
	}
	if l := r.repos[key]; l != nil {
		l.refs++
		r.mu.Unlock()
		return l, nil
	}
	r.mu.Unlock()
	registered, err := r.registry.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, repo := range registered {
		if repo.Key != key {
			continue
		}
		l, err := r.load(ctx, repo)
		if err != nil {
			if errors.Is(err, errPanelReposClosed) {
				return nil, err
			}
			r.fail(repo, err)
			return nil, fmt.Errorf("%w: %s: %w", paneldomain.ErrNotFound, repo.Worktree, err)
		}
		return l, nil
	}
	return nil, fmt.Errorf("%w: repo %s is not registered", paneldomain.ErrNotFound, key)
}

// prune closes the store of each cached repo that is not in the registry. The caller holds mu.
func (r *panelRepos) prune(registered []paneldomain.Repo) {
	keep := make(map[string]bool, len(registered))
	for _, repo := range registered {
		keep[repo.Key] = true
	}
	for key, l := range r.repos {
		if !keep[key] {
			_ = r.retire(l)
			delete(r.repos, key)
			delete(r.failed, key)
		}
	}
}

// load returns the cached readers of repo, held for the caller, and builds them when the repo is new
// or its worktree moved. The caller does not hold mu and releases the result. The slow part, the
// build, runs with no lock, so a slow repo does not block the calls on the others.
func (r *panelRepos) load(ctx context.Context, repo paneldomain.Repo) (*loadedRepo, error) {
	cur, err := r.lookup(repo)
	if cur != nil || err != nil {
		return cur, err
	}
	built, err := r.build(ctx, repo)
	if err != nil {
		return nil, err
	}
	return r.install(repo, built)
}

// lookup returns the cached readers of repo with one more ref, or nil when the repo needs a build.
// It retires a cached store whose worktree moved.
func (r *panelRepos) lookup(repo paneldomain.Repo) (*loadedRepo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, errPanelReposClosed
	}
	cur := r.repos[repo.Key]
	if cur == nil {
		return nil, nil
	}
	if cur.worktree == repo.Worktree {
		cur.refs++
		return cur, nil
	}
	_ = r.retire(cur)
	delete(r.repos, repo.Key)
	return nil, nil
}

// install caches a built repo with its one ref for the caller. After Close it closes the store and
// fails. When another call cached the same worktree first, it closes its own store and returns the cached one.
func (r *panelRepos) install(repo paneldomain.Repo, built *loadedRepo) (*loadedRepo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		_ = built.db.Close()
		return nil, errPanelReposClosed
	}
	if cur := r.repos[repo.Key]; cur != nil {
		if cur.worktree == repo.Worktree {
			_ = built.db.Close()
			cur.refs++
			return cur, nil
		}
		_ = r.retire(cur)
	}
	r.repos[repo.Key] = built
	return built, nil
}

// build loads the environment and opens the store of repo, with refs 1. It holds no lock and caches nothing.
func (r *panelRepos) build(ctx context.Context, repo paneldomain.Repo) (*loadedRepo, error) {
	// Another repo reads its .harness.yaml only: no HARNESS_* variable and no serve flag. The same set flows
	// into the proof adapter and the live run of the repo, which load the environment again.
	flags := config.FileOnly()
	if repo.Key == r.serveKey {
		flags = r.flags
	}
	env, err := environmentAt(ctx, flags, repo.Worktree)
	if err != nil {
		return nil, err
	}
	if !sameDir(env.commonDir, repo.CommonDir) {
		return nil, fmt.Errorf("worktree %s belongs to another repo", repo.Worktree)
	}
	if !dbal.FileExists(env.cfg.State.DSN, env.commonDir) {
		return nil, fmt.Errorf("state.db not found for %s", repo.Worktree)
	}
	db, err := r.open(ctx, env.cfg, env.commonDir)
	if err != nil {
		return nil, err
	}
	env.proofs.shared = func(context.Context) (*dbal.DB, error) { return db, nil }
	now := r.now
	statuses := runmodule.NewStatus(db, now)
	l := &loadedRepo{
		worktree: repo.Worktree,
		db:       db,
		refs:     1,
		runs: newPanelRuns(statuses, env.key.Repo).forRepo(repo).
			withLive(env.key, env.proofs, now),
		pages: newPanelPages(statuses, env.key, planmodule.NewPages(env.proofs, env.defaultBranch)),
	}
	return l, nil
}

// fail logs the failure of a repo once, and again only when its message changes.
func (r *panelRepos) fail(repo paneldomain.Repo, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failed[repo.Key] == err.Error() {
		return
	}
	r.failed[repo.Key] = err.Error()
	_, _ = fmt.Fprintf(r.log, "panel: repo failed %s: %v\n", repo.Worktree, err)
}

func (r *panelRepos) recover(repo paneldomain.Repo) {
	r.mu.Lock()
	delete(r.failed, repo.Key)
	r.mu.Unlock()
}

// sameDir reports whether a and b are the same folder once symlinks resolve, as /var and /private/var on macOS.
// A path that does not resolve is compared as it is.
func sameDir(a, b string) bool {
	resolve := func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return filepath.Clean(p)
	}
	return resolve(a) == resolve(b)
}

func firstLine(err error) string {
	line, _, _ := strings.Cut(err.Error(), "\n")
	return line
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"

	panelmodule "github.com/90p-labs/harness/internal/panel"
	paneldomain "github.com/90p-labs/harness/internal/panel/domain"
	panelprocess "github.com/90p-labs/harness/internal/panel/infrastructure/process"
	panelregistry "github.com/90p-labs/harness/internal/panel/infrastructure/registry"
	panelcli "github.com/90p-labs/harness/internal/panel/ui/cli"
	planmodule "github.com/90p-labs/harness/internal/plan"
	"github.com/90p-labs/harness/internal/plan/application/query/viewpage"
	plandomain "github.com/90p-labs/harness/internal/plan/domain"
	planview "github.com/90p-labs/harness/internal/plan/infrastructure/view"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

const sketchSuffix = ".excalidraw"

// legacyStateDir is the folder under the git common dir that held panel.json,
// panel.lock and panel.log before the panel went user-wide.
const legacyStateDir = ".harness"

// panelEnv is what a panel command needs: the process adapter in the user
// folder, the config, and the repo of the working folder when it is in git.
type panelEnv struct {
	proc *panelprocess.Process
	cfg  config.Config
	dir  string
	// repo is nil outside a git repository.
	repo *paneldomain.Repo
	// legacy is the old state folder of that repo, empty when repo is nil.
	legacy string
}

// panelSetup reads git and the config, and builds the process adapter in the
// user folder. It registers nothing and opens no run state, so status and the
// help of a command have no side effect. A working folder outside git is fine.
func panelSetup(ctx context.Context, flags *pflag.FlagSet) (panelEnv, error) {
	dir, err := panelprocess.UserDir()
	if err != nil {
		return panelEnv{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return panelEnv{}, err
	}
	env := panelEnv{dir: dir}
	worktree := cwd
	commonDir, err := git.CommonDir(ctx, cwd)
	switch {
	case err == nil:
		_, top, err := git.RepoKey(ctx, cwd)
		if err != nil {
			return panelEnv{}, err
		}
		worktree = top
		env.legacy = filepath.Join(commonDir, legacyStateDir)
		env.repo = &paneldomain.Repo{
			Key: paneldomain.RepoKey(commonDir), Name: filepath.Base(top), CommonDir: commonDir, Worktree: top,
		}
	case !errors.Is(err, git.ErrNotRepository):
		return panelEnv{}, err
	}
	if env.cfg, err = config.Load(flags, worktree); err != nil {
		return panelEnv{}, err
	}
	if env.repo != nil {
		env.repo.StateDB = existingStateDB(env.cfg.State.DSN, commonDir)
	}
	exe, err := os.Executable()
	if err != nil {
		return panelEnv{}, err
	}
	env.proc = panelmodule.NewProcess(panelprocess.Config{
		Dir:     dir,
		Command: []string{exe, "panel", "serve"},
		WorkDir: dir,
	})
	return env, nil
}

// existingStateDB returns the file of a sqlite state DSN when it exists, and
// "" otherwise: the registry drops a repo whose recorded state file is gone,
// so a file that is not there yet must not be recorded.
func existingStateDB(dsn, commonDir string) string {
	scheme, path, _ := strings.Cut(dsn, ":")
	if scheme != "sqlite" || path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(commonDir, path)
	}
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// register records the repo of the working folder in the registry of the user
// folder. A working folder outside git registers nothing.
func (e panelEnv) register(ctx context.Context) error {
	if e.repo == nil {
		return nil
	}
	repo := *e.repo
	repo.UsedAt = time.Now().UTC()
	return panelregistry.New(e.dir, os.Stderr).Register(ctx, repo)
}

// stopLegacy stops a panel of the old layout in <common dir>/.harness and
// removes its panel.json, panel.lock and panel.log. It does nothing when the
// repo has none of them, and it never touches the state database there.
func (e panelEnv) stopLegacy(ctx context.Context) error {
	if e.legacy == "" {
		return nil
	}
	names := []string{"panel.json", "panel.lock", "panel.log"}
	found := false
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(e.legacy, name)); err == nil {
			found = true
		}
	}
	if !found {
		return nil
	}
	if err := panelprocess.New(panelprocess.Config{Dir: e.legacy}).Stop(ctx); err != nil {
		return fmt.Errorf("stop the panel of %s: %w", e.legacy, err)
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(e.legacy, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// lazyLifecycle builds the process adapter when a command runs, so the help of
// a command needs no setup. Start and stop also end a legacy panel, and start
// registers the repo.
func lazyLifecycle(flags *pflag.FlagSet) panelcli.Lifecycle {
	with := func(ctx context.Context) (panelEnv, panelcli.Lifecycle, error) {
		env, err := panelSetup(ctx, flags)
		if err != nil {
			return panelEnv{}, panelcli.Lifecycle{}, err
		}
		return env, panelmodule.NewLifecycle(env.proc, env.cfg.Panel.Port, version), nil
	}
	// prepared holds the lifecycle that Prepare built in this process, so Start
	// reuses it. Status builds its own lifecycle on each call.
	var (
		mu       sync.Mutex
		prepared *panelcli.Lifecycle
	)
	prepare := func(ctx context.Context) (panelcli.Lifecycle, error) {
		mu.Lock()
		defer mu.Unlock()
		if prepared != nil {
			return *prepared, nil
		}
		env, life, err := with(ctx)
		if err != nil {
			return panelcli.Lifecycle{}, err
		}
		if err := env.stopLegacy(ctx); err != nil {
			return panelcli.Lifecycle{}, err
		}
		if err := env.register(ctx); err != nil {
			return panelcli.Lifecycle{}, err
		}
		prepared = &life
		return life, nil
	}
	return panelcli.Lifecycle{
		Version: version,
		// The start command runs Prepare before its version check, so a start
		// that reuses a panel of another version still registers the repo.
		Prepare: func(ctx context.Context) error {
			_, err := prepare(ctx)
			return err
		},
		// The host open and the autostart call Start alone, so Start prepares
		// the repo itself when Prepare did not run.
		Start: func(ctx context.Context, port int) error {
			life, err := prepare(ctx)
			if err != nil {
				return err
			}
			return life.Start(ctx, port)
		},
		Stop: func(ctx context.Context) error {
			env, life, err := with(ctx)
			if err != nil {
				return err
			}
			if err := env.stopLegacy(ctx); err != nil {
				return err
			}
			return life.Stop(ctx)
		},
		Status: func(ctx context.Context) (paneldomain.Instance, error) {
			_, life, err := with(ctx)
			if err != nil {
				return paneldomain.Instance{}, err
			}
			return life.Status(ctx)
		},
	}
}

// emptyRuns and emptyPages serve a panel that was spawned outside a git
// repository: it has no repo to read until the multi-repo adapter lands.
type emptyRuns struct{}

func (emptyRuns) List(context.Context) (paneldomain.Listing, error) {
	return paneldomain.Listing{}, nil
}
func (emptyRuns) Get(context.Context, paneldomain.RunRef) (paneldomain.Card, error) {
	return paneldomain.Card{}, paneldomain.ErrNotFound
}

type emptyPages struct{}

func (emptyPages) Page(context.Context, paneldomain.Card, paneldomain.Served) ([]byte, error) {
	return nil, paneldomain.ErrNotFound
}

func (emptyPages) Region(context.Context, paneldomain.Card, string, paneldomain.Served) ([]byte, error) {
	return nil, paneldomain.ErrNotFound
}
func (emptyPages) HasRegion(string) bool { return false }

func newPanelCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := panelcli.NewCommand(func(ctx context.Context, port int, ready func(url string)) (err error) {
		panel, err := panelSetup(ctx, flags)
		if err != nil {
			return err
		}
		if err := panel.register(ctx); err != nil {
			return err
		}
		if panel.repo == nil { // spawned in the user folder: no repo to read yet
			return panelmodule.NewServe(version, emptyRuns{}, emptyPages{}, panel.proc)(ctx, port, ready)
		}
		env, err := newEnvironment(ctx, flags)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, env.state.Close()) }()
		handlers, _, err := openRun(ctx, env)
		if err != nil {
			return err
		}
		runs := newPanelRuns(handlers.Status, env.key.Repo).forRepo(*panel.repo).withLive(env.key, env.proofs, time.Now)
		pages := newPanelPages(handlers.Status, env.key, planmodule.NewPages(env.proofs, env.defaultBranch))
		return panelmodule.NewServe(version, runs, pages, panel.proc)(ctx, port, ready)
	}, lazyLifecycle(flags))
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

// panelRuns lists the runs of the repo as cards. It reads the state and the run folders, and writes nothing.
type panelRuns struct {
	status status.Handler
	// info is the panel repo, the only lane of the board until the multi-repo adapter lands.
	info     paneldomain.Repo
	repo     string
	branches *branchCache
	live     *liveCache
}

func newPanelRuns(statuses status.Handler, repo string) panelRuns {
	return panelRuns{status: statuses, repo: repo, branches: &branchCache{read: os.ReadFile}}
}

// forRepo names the panel repo, whose key goes on each card.
func (r panelRuns) forRepo(info paneldomain.Repo) panelRuns {
	r.info = info
	return r
}

// withLive adds the branch head and the proofs of the active run to the run part that Get returns.
// key is the panel worktree, whose active run is the only one the proof reader reads.
func (r panelRuns) withLive(key domain.RunKey, proofs plandomain.ProofReader, now func() time.Time) panelRuns {
	r.live = &liveCache{key: key, proofs: proofs, head: git.Head, now: now}
	return r
}

// liveTTL is how long the head and the proofs of a run stay cached, so a burst of region requests spawns one git call.
const liveTTL = time.Second

// liveKeep is how long an entry that went stale stays, as the last good value while a read fails.
const liveKeep = time.Minute

// liveCache keeps the head and the proofs of each run for liveTTL.
type liveCache struct {
	key    domain.RunKey
	proofs plandomain.ProofReader
	head   func(ctx context.Context, dir string) (string, error)
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]liveEntry
	flights map[string]*liveFlight
}

type liveEntry struct {
	at     time.Time
	head   string
	proofs []string
}

// liveFlight is the read of one run that is in progress. Later callers of the run wait on done.
type liveFlight struct {
	done  chan struct{}
	parts []string
}

// parts returns the head of the worktree, then the sorted proofs when the run is the active run.
// A failed read is not cached: the last good value stays in its place, and the next call reads again.
// A missing worktree has an empty head, which is cached like a good read.
// One caller per run reads at a time, and the others wait for its result.
// The lock is not held across the git and status calls.
func (c *liveCache) parts(ctx context.Context, statuses status.Handler, s status.Status) []string {
	c.mu.Lock()
	now := c.now()
	for id, e := range c.entries {
		if now.Sub(e.at) >= liveKeep {
			delete(c.entries, id)
		}
	}
	prev, had := c.entries[s.ID]
	if had && now.Sub(prev.at) < liveTTL {
		c.mu.Unlock()
		return append([]string{prev.head}, prev.proofs...)
	}
	if f, running := c.flights[s.ID]; running {
		c.mu.Unlock()
		select {
		case <-f.done:
			return f.parts
		case <-ctx.Done():
			return append([]string{prev.head}, prev.proofs...)
		}
	}
	f := &liveFlight{done: make(chan struct{})}
	if c.flights == nil {
		c.flights = map[string]*liveFlight{}
	}
	c.flights[s.ID] = f
	c.mu.Unlock()
	f.parts = append([]string{prev.head}, prev.proofs...)
	defer func() {
		// Deferred so that waiters are released even when the read panics.
		c.mu.Lock()
		delete(c.flights, s.ID)
		c.mu.Unlock()
		close(f.done)
	}()

	entry, ok := c.read(ctx, statuses, s, now, prev)
	f.parts = append([]string{entry.head}, entry.proofs...)
	if ok {
		c.mu.Lock()
		if c.entries == nil {
			c.entries = map[string]liveEntry{}
		}
		c.entries[s.ID] = entry
		c.mu.Unlock()
	}
	return f.parts
}

// read reads the head and the proofs of a run. ok is false when a read failed, and the entry then keeps prev for it.
func (c *liveCache) read(ctx context.Context, statuses status.Handler, s status.Status, now time.Time, prev liveEntry) (entry liveEntry, ok bool) {
	entry = liveEntry{at: now, head: prev.head, proofs: prev.proofs}
	ok = true
	head, err := c.head(ctx, s.Worktree)
	switch {
	case err == nil:
		entry.head = head
	case errors.Is(err, fs.ErrNotExist):
		entry.head = ""
	default:
		ok = false
	}
	if proofs, err := c.proofParts(ctx, statuses, s.ID); err == nil {
		entry.proofs = proofs
	} else {
		ok = false
	}
	return entry, ok
}

// proofParts lists the passed criteria of the active run. Any other run has none.
// A failed status read returns its error, and a worktree with no active run has no proofs.
func (c *liveCache) proofParts(ctx context.Context, statuses status.Handler, id string) ([]string, error) {
	active, err := statuses.Handle(ctx, status.Query{Key: c.key})
	if errors.Is(err, domain.ErrRunNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if active.ID != id {
		return nil, nil
	}
	passed, err := c.proofs.Passed(ctx)
	if err != nil {
		// The error text joins the parts, so a later recovery changes the version.
		return []string{"proof error", err.Error()}, nil
	}
	nums := make([]int, 0, len(passed))
	for n := range passed {
		nums = append(nums, n)
	}
	slices.Sort(nums)
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa(n) + "=" + strconv.FormatBool(passed[n])
	}
	return parts, nil
}

// branchCache keeps the branch of each plan under the plan part it was read at.
// The poll shares it across requests, so an unchanged plan is not read nor parsed again.
type branchCache struct {
	mu      sync.Mutex
	entries map[string]branchEntry
	read    func(path string) ([]byte, error)
}

type branchEntry struct {
	plan   string
	branch string
}

// branch returns the branch of the plan at path, which has the plan part plan.
func (c *branchCache) branch(path string, plan string) string {
	if c == nil {
		return readBranch(os.ReadFile, path)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[path]; ok && e.plan == plan {
		return e.branch
	}
	branch := readBranch(c.read, path)
	if c.entries == nil {
		c.entries = map[string]branchEntry{}
	}
	c.entries[path] = branchEntry{plan: plan, branch: branch}
	return branch
}

func readBranch(read func(string) ([]byte, error), path string) string {
	text, err := read(path)
	if err != nil {
		return ""
	}
	doc, problems := plandomain.Parse(string(text))
	if len(problems) != 0 {
		return ""
	}
	return doc.RunPR().Branch
}

// List returns the listing of the one repo this panel reads, with its key on each card.
func (r panelRuns) List(ctx context.Context) (paneldomain.Listing, error) {
	phases := make([]domain.Phase, len(shareddomain.Phases))
	for i, p := range shareddomain.Phases {
		phases[i] = domain.Phase(p)
	}
	statuses, err := r.status.ByPhase(ctx, r.repo, phases)
	if err != nil {
		return paneldomain.Listing{}, err
	}
	cards := make([]paneldomain.Card, len(statuses))
	for i, s := range statuses {
		cards[i] = buildCard(s, r.branches)
		cards[i].Repo = r.info.Key
	}
	return paneldomain.Listing{Repos: []paneldomain.Repo{r.info}, Cards: cards}, nil
}

// Get reads one run. A ref of another repo is not found.
func (r panelRuns) Get(ctx context.Context, ref paneldomain.RunRef) (paneldomain.Card, error) {
	if ref.Repo != r.info.Key {
		return paneldomain.Card{}, paneldomain.ErrNotFound
	}
	id := ref.ID
	s, err := r.status.Handle(ctx, status.Query{Key: domain.RunKey{Repo: r.repo}, RunID: id})
	if errors.Is(err, domain.ErrRunNotFound) {
		return paneldomain.Card{}, paneldomain.ErrNotFound
	}
	if err != nil {
		return paneldomain.Card{}, err
	}
	if s.Repo != r.repo {
		return paneldomain.Card{}, paneldomain.ErrNotFound
	}
	card := buildCard(s, r.branches)
	card.Repo = r.info.Key
	if r.live != nil {
		parts := append([]string{strconv.Itoa(s.Version)}, r.live.parts(ctx, r.status, s)...)
		card.Version.Run = paneldomain.Fingerprint(parts...)
	}
	return card, nil
}

func buildCard(s status.Status, branches *branchCache) paneldomain.Card {
	path := planPath(s.Worktree, s.ID)
	card := paneldomain.Card{
		ID: s.ID, Title: s.Prompt, Phase: string(s.Phase), BlockedFrom: string(s.BlockedFrom),
		PRURL: s.PRURL, Worktree: s.Worktree,
		Version: paneldomain.Version{
			Plan: planPart(path),
			Run:  paneldomain.Fingerprint(strconv.Itoa(s.Version)),
		},
	}
	card.Branch = branches.branch(path, card.Version.Plan)
	return card
}

// planPart hashes the mtime and size of plan.md and the mtimes of the sketches beside it.
func planPart(path string) string {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return paneldomain.Fingerprint("no plan")
	}
	if err != nil {
		// Any other stat error is not "no plan", so its text joins the fingerprint.
		return paneldomain.Fingerprint("stat error", err.Error())
	}
	parts := []string{strconv.FormatInt(info.ModTime().UnixNano(), 10), strconv.FormatInt(info.Size(), 10)}
	// A walk error joins the parts, so a later recovery changes the version.
	walkErr := filepath.WalkDir(filepath.Dir(path), func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			parts = append(parts, "walk error", err.Error())
			return nil
		}
		if entry.IsDir() || filepath.Ext(p) != sketchSuffix {
			return nil
		}
		sketch, err := entry.Info()
		if err != nil {
			parts = append(parts, "sketch error", p, err.Error())
			return nil
		}
		parts = append(parts, p, strconv.FormatInt(sketch.ModTime().UnixNano(), 10))
		return nil
	})
	if walkErr != nil {
		parts = append(parts, "walk error", walkErr.Error())
	}
	return paneldomain.Fingerprint(parts...)
}

// panelPages renders the plan page of a card from its own worktree.
type panelPages struct {
	status status.Handler
	key    domain.RunKey
	view   viewpage.Handler
}

const ruleWorktreeGone plandomain.RuleID = "worktree-gone"

func newPanelPages(statuses status.Handler, key domain.RunKey, view viewpage.Handler) panelPages {
	return panelPages{status: statuses, key: key, view: view}
}

// goneView renders the page of a card whose worktree folder was deleted. The page has no plan to read
// and shows one problem row, in place of the read error that the plan check would return.
func goneView(worktree string) viewpage.Handler {
	renderer := planview.NewRenderer()
	return viewpage.NewHandler(viewpage.Dependencies{
		Check: func(context.Context, string, int) (plandomain.Report, error) {
			problem := plandomain.Problem{
				Rule:    ruleWorktreeGone,
				Message: fmt.Sprintf("the worktree %s was deleted, so the plan cannot be read", worktree),
			}
			return plandomain.Report{Problems: []plandomain.Problem{problem}}, nil
		},
		DefaultBranch: func(context.Context) (string, error) { return "", nil },
		Renderer:      renderer,
		Regions:       renderer,
		Tracer:        otel.Tracer("github.com/90p-labs/harness/internal/plan"),
	})
}

func (p panelPages) Page(ctx context.Context, card paneldomain.Card, served paneldomain.Served) ([]byte, error) {
	return p.render(ctx, card, "", served)
}

func (p panelPages) Region(ctx context.Context, card paneldomain.Card, name string, served paneldomain.Served) ([]byte, error) {
	return p.render(ctx, card, name, served)
}

// HasRegion reports whether name is in the region registry of the plan page.
func (p panelPages) HasRegion(name string) bool {
	return slices.ContainsFunc(planview.Regions(), func(r planview.Region) bool { return r.Name == name })
}

func (p panelPages) render(ctx context.Context, card paneldomain.Card, region string, served paneldomain.Served) ([]byte, error) {
	view := p.view
	gone := false
	if _, err := os.Stat(planPath(card.Worktree, card.ID)); errors.Is(err, fs.ErrNotExist) {
		if _, err := os.Stat(card.Worktree); !errors.Is(err, fs.ErrNotExist) {
			return nil, paneldomain.ErrNoPlan
		}
		view = goneView(card.Worktree)
		gone = true
	}
	current, err := p.status.Handle(ctx, status.Query{Key: p.key, RunID: card.ID})
	if err != nil {
		return nil, err
	}
	active, err := p.isActive(ctx, card.ID)
	if err != nil {
		return nil, err
	}
	out, _, err := view.Handle(ctx, viewpage.Query{
		RunID: card.ID, Phase: card.Phase, LiveRound: current.LiveRound,
		Proofs:   active && !gone, // the gone view has no proof reader, and a deleted worktree has no proofs
		Worktree: card.Worktree,
		Served:   &plandomain.ViewServed{Version: served.Version, Base: served.Base, Home: "/", Events: served.Events},
		Region:   region,
	})
	if errors.Is(err, plandomain.ErrUnknownRegion) || errors.Is(err, plandomain.ErrNoRegions) {
		return nil, fmt.Errorf("%w: %w", paneldomain.ErrNotFound, err)
	}
	return out, err
}

// isActive reports whether the run is the active run of the panel worktree, the only one the proof reader reads.
// A worktree with no active run is not an error.
func (p panelPages) isActive(ctx context.Context, id string) (bool, error) {
	active, err := p.status.Handle(ctx, status.Query{Key: p.key})
	if errors.Is(err, domain.ErrRunNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return active.ID == id, nil
}

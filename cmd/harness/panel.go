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
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"

	panelmodule "github.com/90p-labs/harness/internal/panel"
	paneldomain "github.com/90p-labs/harness/internal/panel/domain"
	panelprocess "github.com/90p-labs/harness/internal/panel/infrastructure/process"
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

// panelStateDir is the folder of panel.json, panel.lock and panel.log.
const panelStateDir = ".harness"

// panelLifecycle wires the process adapter for the repo of the working folder,
// and returns the configured panel port. It reads only git and the config, so
// start, stop and status open no run state.
func panelLifecycle(ctx context.Context, flags *pflag.FlagSet) (*panelprocess.Process, int, error) {
	proc, cfg, err := panelSetup(ctx, flags)
	if err != nil {
		return nil, 0, err
	}
	return proc, cfg.Panel.Port, nil
}

// panelProcess wires the process adapter for the serve path, which takes its
// port from the flag.
func panelProcess(ctx context.Context, flags *pflag.FlagSet) (*panelprocess.Process, error) {
	proc, _, err := panelSetup(ctx, flags)
	return proc, err
}

func panelSetup(ctx context.Context, flags *pflag.FlagSet) (*panelprocess.Process, config.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, config.Config{}, err
	}
	repo, worktree, err := git.RepoKey(ctx, cwd)
	if err != nil {
		return nil, config.Config{}, err
	}
	commonDir, err := git.CommonDir(ctx, cwd)
	if err != nil {
		return nil, config.Config{}, err
	}
	cfg, err := config.Load(flags, worktree)
	if err != nil {
		return nil, config.Config{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, config.Config{}, err
	}
	return panelmodule.NewProcess(panelprocess.Config{
		Dir:     filepath.Join(commonDir, panelStateDir),
		Command: []string{exe, "panel", "serve"},
		WorkDir: worktree,
		Repo:    repo,
	}), cfg, nil
}

// lazyLifecycle builds the process adapter when a command runs, so the help of
// a command needs no repo.
func lazyLifecycle(flags *pflag.FlagSet) panelcli.Lifecycle {
	with := func(ctx context.Context) (panelcli.Lifecycle, error) {
		proc, port, err := panelLifecycle(ctx, flags)
		if err != nil {
			return panelcli.Lifecycle{}, err
		}
		return panelmodule.NewLifecycle(proc, port), nil
	}
	return panelcli.Lifecycle{
		Start: func(ctx context.Context, port int) error {
			life, err := with(ctx)
			if err != nil {
				return err
			}
			return life.Start(ctx, port)
		},
		Stop: func(ctx context.Context) error {
			life, err := with(ctx)
			if err != nil {
				return err
			}
			return life.Stop(ctx)
		},
		Status: func(ctx context.Context) (paneldomain.Instance, error) {
			life, err := with(ctx)
			if err != nil {
				return paneldomain.Instance{}, err
			}
			return life.Status(ctx)
		},
	}
}

func newPanelCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := panelcli.NewCommand(func(ctx context.Context, port int, ready func(url string)) (err error) {
		proc, err := panelProcess(ctx, flags)
		if err != nil {
			return err
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
		runs := newPanelRuns(handlers.Status, env.key.Repo).withLive(env.key, env.proofs, time.Now)
		pages := newPanelPages(handlers.Status, env.key, planmodule.NewPages(env.proofs, env.defaultBranch))
		return panelmodule.NewServe(version, env.key.Repo, runs, pages, proc)(ctx, port, ready)
	}, lazyLifecycle(flags))
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

// panelRuns lists the runs of the repo as cards. It reads the state and the run folders, and writes nothing.
type panelRuns struct {
	status   status.Handler
	repo     string
	branches *branchCache
	live     *liveCache
}

func newPanelRuns(statuses status.Handler, repo string) panelRuns {
	return panelRuns{status: statuses, repo: repo, branches: &branchCache{read: os.ReadFile}}
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

func (r panelRuns) List(ctx context.Context) ([]paneldomain.Card, error) {
	phases := make([]domain.Phase, len(shareddomain.Phases))
	for i, p := range shareddomain.Phases {
		phases[i] = domain.Phase(p)
	}
	statuses, err := r.status.ByPhase(ctx, r.repo, phases)
	if err != nil {
		return nil, err
	}
	cards := make([]paneldomain.Card, len(statuses))
	for i, s := range statuses {
		cards[i] = buildCard(s, r.branches)
	}
	return cards, nil
}

func (r panelRuns) Get(ctx context.Context, id string) (paneldomain.Card, error) {
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
	if r.live != nil {
		parts := append([]string{strconv.Itoa(s.Version)}, r.live.parts(ctx, r.status, s)...)
		card.Version.Run = paneldomain.Fingerprint(parts...)
	}
	return card, nil
}

func panelCard(s status.Status) paneldomain.Card {
	return buildCard(s, nil)
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

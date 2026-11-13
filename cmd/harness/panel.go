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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"

	panelmodule "github.com/90p-labs/harness/internal/panel"
	paneldomain "github.com/90p-labs/harness/internal/panel/domain"
	panelcli "github.com/90p-labs/harness/internal/panel/ui/cli"
	planmodule "github.com/90p-labs/harness/internal/plan"
	"github.com/90p-labs/harness/internal/plan/application/query/viewpage"
	plandomain "github.com/90p-labs/harness/internal/plan/domain"
	planview "github.com/90p-labs/harness/internal/plan/infrastructure/view"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const sketchSuffix = ".excalidraw"

func newPanelCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := panelcli.NewCommand(func(ctx context.Context, port int, ready func(url string)) (err error) {
		env, err := newEnvironment(ctx, flags)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, env.state.Close()) }()
		handlers, _, err := openRun(ctx, env)
		if err != nil {
			return err
		}
		runs := newPanelRuns(handlers.Status, env.key.Repo)
		pages := newPanelPages(handlers.Status, env.key, planmodule.NewPages(env.proofs, env.defaultBranch))
		return panelmodule.NewServe(version, env.key.Repo, runs, pages)(ctx, port, ready)
	})
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

// panelRuns lists the runs of the repo as cards. It reads the state and the run folders, and writes nothing.
type panelRuns struct {
	status   status.Handler
	repo     string
	branches *branchCache
}

func newPanelRuns(statuses status.Handler, repo string) panelRuns {
	return panelRuns{status: statuses, repo: repo, branches: &branchCache{read: os.ReadFile}}
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
	return buildCard(s, r.branches), nil
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
		Served:   &plandomain.ViewServed{Version: served.Version, Base: served.Base, Home: "/"},
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

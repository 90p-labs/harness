package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	gatemodule "github.com/90p-labs/harness/internal/gate"
	"github.com/90p-labs/harness/internal/gate/application/command/initgates"
	"github.com/90p-labs/harness/internal/gate/application/command/verify"
	"github.com/90p-labs/harness/internal/gate/application/command/verifycommits"
	"github.com/90p-labs/harness/internal/gate/application/query/evidencestatus"
	gatedomain "github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/gate/infrastructure/toolchain"
	gatecli "github.com/90p-labs/harness/internal/gate/ui/cli"
	"github.com/90p-labs/harness/internal/guard/application/guardplugin"
	guarddomain "github.com/90p-labs/harness/internal/guard/domain"
	lintdomain "github.com/90p-labs/harness/internal/lint/domain"
	plandomain "github.com/90p-labs/harness/internal/plan/domain"
	plancli "github.com/90p-labs/harness/internal/plan/ui/cli"
	runmodule "github.com/90p-labs/harness/internal/run"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

const proofQuote = "`"

var (
	errPlanProblems = errors.New("the plan has problems, run harness plan check")
	errNoCriterion  = errors.New("the plan has no criterion with a proof")
)

func newVerifyCommand(flags *pflag.FlagSet) *cobra.Command {
	return gatecli.NewVerifyCommand(func(ctx context.Context) (gatecli.Handlers, func() error, error) {
		return connectGates(ctx, flags)
	})
}

func newGatesCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := gatecli.NewGatesCommand(func(ctx context.Context) (gatecli.Handlers, func() error, error) {
		return connectGates(ctx, flags)
	})
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

func connectGates(ctx context.Context, flags *pflag.FlagSet) (gatecli.Handlers, func() error, error) {
	env, err := newEnvironment(ctx, flags)
	if err != nil {
		return gatecli.Handlers{}, nil, err
	}
	gates, err := gatemodule.DecodeGates(env.cfg.Gates)
	if err != nil {
		return gatecli.Handlers{}, nil, err
	}
	setup, err := gatemodule.DecodeSetup(env.cfg.Setup)
	if err != nil {
		return gatecli.Handlers{}, nil, err
	}
	db, err := openState(ctx, env.cfg, env.commonDir)
	if err != nil {
		return gatecli.Handlers{}, nil, err
	}
	verifier := gatemodule.NewVerify(db, env.bus, criterionSource{flags: flags}, time.Now)
	commits := gatemodule.NewVerifyCommits(db, env.bus, time.Now)
	initializer := gatemodule.NewInit(toolchain.Lint{Packs: env.cfg.Lint.Packs, Baseline: lintdomain.BaselinePath}, env.bus)
	return gatecli.Handlers{
		Verify: func(ctx context.Context) (verify.Result, error) {
			return verifier.Handle(ctx, verify.Command{
				Repo: env.key.Repo, Dir: env.key.Worktree, Setup: setup, Gates: gates, Parallel: env.cfg.Verify.Parallel,
			})
		},
		VerifyCriterion: func(ctx context.Context, number int) (verify.Result, error) {
			criteria, err := activeCriteria(ctx, flags)
			if err != nil {
				return verify.Result{}, err
			}
			i := slices.IndexFunc(criteria, func(c criterionGate) bool { return c.number == number })
			if i < 0 {
				return verify.Result{}, fmt.Errorf("%w: %d", errNoCriterion, number)
			}
			return verifier.HandleConfigured(ctx, verify.Command{
				Repo: env.key.Repo, Dir: env.key.Worktree, Setup: setup, Gates: []gatedomain.Gate{criteria[i].gate}, Parallel: 1,
			})
		},
		VerifyCommits: func(ctx context.Context, base string) (verifycommits.Result, error) {
			return commits.Handle(ctx, verifycommits.Command{
				Repo: env.key.Repo, Dir: env.key.Worktree, Base: base, Setup: setup, Gates: gates, Parallel: env.cfg.Verify.Parallel,
			})
		},
		Init: func(ctx context.Context) (initgates.Result, error) {
			return initializer.Handle(ctx, initgates.Command{Dir: env.key.Worktree})
		},
	}, db.Close, nil
}

type criterionGate struct {
	number int
	gate   gatedomain.Gate
}

func activeCriteria(ctx context.Context, flags *pflag.FlagSet) ([]criterionGate, error) {
	query, err := activePlan(ctx, flags)
	if errors.Is(err, plancli.ErrNoPlanFile) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	text, err := os.ReadFile(query.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	doc, problems := plandomain.Parse(string(text))
	if len(problems) > 0 {
		return nil, fmt.Errorf("%w: %s", errPlanProblems, problems[0])
	}
	var criteria []criterionGate
	for _, c := range doc.Criteria {
		run := strings.Trim(strings.TrimSpace(c.Proof), proofQuote)
		if run == "" {
			continue
		}
		gate, err := gatedomain.NewGate(gatedomain.CriterionGateID(c.Number), run, nil, 0, "")
		if err != nil {
			return nil, err
		}
		criteria = append(criteria, criterionGate{number: c.Number, gate: gate})
	}
	return criteria, nil
}

type criterionSource struct{ flags *pflag.FlagSet }

func (s criterionSource) Gates(ctx context.Context) ([]gatedomain.Gate, error) {
	criteria, err := activeCriteria(ctx, s.flags)
	gates := make([]gatedomain.Gate, len(criteria))
	for i, c := range criteria {
		gates[i] = c.gate
	}
	return gates, err
}

type proofAdapter struct {
	flags     *pflag.FlagSet
	cfg       config.Config
	commonDir string
	key       domain.RunKey
}

func (p proofAdapter) Passed(ctx context.Context) (passed map[int]bool, err error) {
	criteria, err := activeCriteria(ctx, p.flags)
	if err != nil {
		return nil, err
	}
	gates := make([]gatedomain.Gate, len(criteria))
	for i, c := range criteria {
		gates[i] = c.gate
	}
	db, err := openState(ctx, p.cfg, p.commonDir)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	statuses, err := gatemodule.NewEvidenceStatus(db).Handle(ctx, evidencestatus.Query{Repo: p.key.Repo, Dir: p.key.Worktree, Gates: gates})
	if err != nil {
		return nil, err
	}
	return proofResults(criteria, statuses), nil
}

// proofResults leaves out a criterion with no evidence for this tree, so the plan shows it as not run.
func proofResults(criteria []criterionGate, statuses map[string]gatedomain.Result) map[int]bool {
	passed := make(map[int]bool, len(criteria))
	for _, c := range criteria {
		if status, ok := statuses[c.gate.ID]; ok {
			passed[c.number] = status == gatedomain.ResultPass
		}
	}
	return passed
}

type commitCheckAdapter struct {
	cfg config.Config
}

func (a commitCheckAdapter) Passed(ctx context.Context, dir string) (checked bool, err error) {
	if dir, err = filepath.Abs(dir); err != nil {
		return false, err
	}
	head, err := git.Head(ctx, dir)
	if err != nil {
		return false, err
	}
	repo, _, err := git.RepoKey(ctx, dir)
	if err != nil {
		return false, err
	}
	commonDir, err := git.CommonDir(ctx, dir)
	if err != nil {
		return false, err
	}
	db, err := openState(ctx, a.cfg, commonDir)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	return gatemodule.NewCommitChecks(db).Passed(ctx, repo, head)
}

type reviewRecordAdapter struct {
	cfg config.Config
}

func (a reviewRecordAdapter) Reviews(ctx context.Context, dir string, scope guarddomain.ReviewScope) (reviews []guarddomain.Review, err error) {
	head, repo, db, err := a.open(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	query := gatedomain.ReviewScope{Repo: repo, HeadSHA: head, PatchID: scope.PatchID, RunID: scope.RunID}
	if scope.RunID != "" {
		if query.RunSince, err = runmodule.NewReworkLog(db, time.Now).LastReworkAt(ctx, scope.RunID); err != nil {
			return nil, err
		}
	}
	records, err := gatemodule.NewReviewRecords(db).Matching(ctx, query)
	for _, record := range records {
		reviews = append(reviews, guarddomain.Review{Model: record.Model, PatchID: record.PatchID, RunID: record.RunID, FilesRead: record.FilesRead})
	}
	return reviews, err
}

func (a reviewRecordAdapter) ReworkReasons(ctx context.Context, dir, runID string) (reasons []string, err error) {
	_, _, db, err := a.open(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	return runmodule.NewReworkLog(db, time.Now).ReworkReasons(ctx, runID)
}

func (a reviewRecordAdapter) Save(ctx context.Context, dir string, review guarddomain.Review) (err error) {
	head, repo, db, err := a.open(ctx, dir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	return gatemodule.NewReviewRecords(db).Save(ctx, gatedomain.ReviewRecord{
		Repo: repo, HeadSHA: head, Model: review.Model, PatchID: review.PatchID, FilesRead: review.FilesRead, RunID: review.RunID, At: time.Now(),
	})
}

func (a reviewRecordAdapter) open(ctx context.Context, dir string) (head, repo string, db *dbal.DB, err error) {
	if dir, err = filepath.Abs(dir); err != nil {
		return "", "", nil, err
	}
	if head, err = git.Head(ctx, dir); err != nil {
		return "", "", nil, err
	}
	if repo, _, err = git.RepoKey(ctx, dir); err != nil {
		return "", "", nil, err
	}
	commonDir, err := git.CommonDir(ctx, dir)
	if err != nil {
		return "", "", nil, err
	}
	db, err = openState(ctx, a.cfg, commonDir)
	return head, repo, db, err
}

// panelSource gives the guard and `harness review panel` the facts of the final review panel.
func panelSource(cfg config.Config) guardplugin.PanelSource {
	adapter := reviewRecordAdapter{cfg: cfg}
	return guardplugin.PanelSource{
		LastRework:    adapter.LastRework,
		ReviewedByAll: adapter.ReviewedByAll,
		FixupLines:    git.DiffLines,
		LightReview:   func(ctx context.Context, dir, runID string) (bool, error) { return lightReview(ctx, cfg, dir, runID) },
		Limit:         cfg.Run.PanelFixupLines,
	}
}

// lightReview holds only when the plan, the approved copy and the changed files all give a light review.
// A read, parse or git error is an error, so the caller keeps the full panel.
func lightReview(ctx context.Context, cfg config.Config, dir, runID string) (bool, error) {
	base := cfg.Git.DefaultBranch
	if base == "" {
		var err error
		if base, err = git.DefaultBranch(ctx, dir); err != nil {
			return false, err
		}
	}
	baseRef, err := git.BranchRef(ctx, dir, base)
	if err != nil {
		return false, err
	}
	changed, err := git.ChangedFilesNoRenames(ctx, dir, baseRef)
	if err != nil {
		return false, err
	}
	for _, path := range []string{planPath(dir, runID), keptPlanPath(dir, runID)} {
		var text []byte
		text, err = os.ReadFile(path)
		if err != nil {
			return false, err
		}
		if path != planPath(dir, runID) {
			var approved bool
			approved, err = approvedCopy(ctx, cfg, dir, runID, text)
			if err != nil || !approved {
				return false, err
			}
		}
		doc, problems := plandomain.Parse(string(text))
		if len(problems) > 0 {
			return false, fmt.Errorf("%w: %s", errPlanProblems, problems[0])
		}
		if !plandomain.LightReview(doc, changed) {
			return false, nil
		}
	}
	return true, nil
}

// approvedCopy holds when the latest approval of the run is the sha256 of the approved copy.
// An edit of approved-plan.md after the approval breaks it.
func approvedCopy(ctx context.Context, cfg config.Config, dir, runID string, kept []byte) (matches bool, err error) {
	_, _, db, err := reviewRecordAdapter{cfg: cfg}.open(ctx, dir)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	hash, ok, err := runmodule.NewApprovals(db).Latest(ctx, runID)
	if err != nil || !ok {
		return false, err
	}
	sum := sha256.Sum256(kept)
	return hash == hex.EncodeToString(sum[:]), nil
}

func (a reviewRecordAdapter) LastRework(ctx context.Context, dir, runID string) (last guarddomain.ReworkEvent, err error) {
	_, _, db, err := a.open(ctx, dir)
	if err != nil {
		return last, err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	event, reason, err := runmodule.NewReworkLog(db, time.Now).LastReworkEvent(ctx, runID)
	return guarddomain.ReworkEvent{Name: string(event), Reason: reason}, err
}

func (a reviewRecordAdapter) ReviewedByAll(ctx context.Context, dir, runID string, models []string) (head string, err error) {
	_, repo, db, err := a.open(ctx, dir)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	return gatemodule.NewReviewRecords(db).LastReviewedByAll(ctx, repo, runID, models)
}

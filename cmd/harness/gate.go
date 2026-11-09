package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	gatemodule "github.com/jorge07/harness/internal/gate"
	"github.com/jorge07/harness/internal/gate/application/command/initgates"
	"github.com/jorge07/harness/internal/gate/application/command/verify"
	"github.com/jorge07/harness/internal/gate/application/command/verifycommits"
	"github.com/jorge07/harness/internal/gate/application/query/evidencestatus"
	gatedomain "github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/gate/infrastructure/toolchain"
	gatecli "github.com/jorge07/harness/internal/gate/ui/cli"
	lintdomain "github.com/jorge07/harness/internal/lint/domain"
	plandomain "github.com/jorge07/harness/internal/plan/domain"
	plancli "github.com/jorge07/harness/internal/plan/ui/cli"
	"github.com/jorge07/harness/internal/run/application/query/status"
	"github.com/jorge07/harness/internal/run/domain"
	runcli "github.com/jorge07/harness/internal/run/ui/cli"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/git"
)

const proofQuote = "`"

var errPlanProblems = errors.New("the plan has problems, run harness plan check")

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
	handlers, db, err := openRun(ctx, env)
	if err != nil {
		return gatecli.Handlers{}, nil, err
	}
	verifier := gatemodule.NewVerify(db, env.bus, criterionSource{flags: flags}, time.Now)
	commits := gatemodule.NewVerifyCommits(db, env.bus, time.Now)
	initializer := gatemodule.NewInit(toolchain.Lint{Packs: env.cfg.Lint.Packs, Baseline: lintdomain.BaselinePath}, env.bus)
	return gatecli.Handlers{
		Verify: func(ctx context.Context) (verify.Result, error) {
			runID, err := activeRunID(ctx, handlers, env.key)
			if err != nil {
				return verify.Result{}, err
			}
			return verifier.Handle(ctx, verify.Command{
				RunID: runID, Repo: env.key.Repo, Dir: env.key.Worktree, Setup: setup, Gates: gates, Parallel: env.cfg.Verify.Parallel,
			})
		},
		VerifyCommits: func(ctx context.Context, base string) (verifycommits.Result, error) {
			runID, err := activeRunID(ctx, handlers, env.key)
			if err != nil {
				return verifycommits.Result{}, err
			}
			return commits.Handle(ctx, verifycommits.Command{
				RunID: runID, Repo: env.key.Repo, Dir: env.key.Worktree, Base: base, Setup: setup, Gates: gates, Parallel: env.cfg.Verify.Parallel,
			})
		},
		Init: func(ctx context.Context) (initgates.Result, error) {
			return initializer.Handle(ctx, initgates.Command{Dir: env.key.Worktree})
		},
	}, db.Close, nil
}

func activeRunID(ctx context.Context, handlers runcli.Handlers, key domain.RunKey) (string, error) {
	current, err := handlers.Status.Handle(ctx, status.Query{Key: key})
	if errors.Is(err, domain.ErrRunNotFound) {
		return "", nil
	}
	return current.ID, err
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
	passed = make(map[int]bool, len(criteria))
	for _, c := range criteria {
		passed[c.number] = statuses[c.gate.ID] == gatedomain.ResultPass
	}
	return passed, nil
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

func (a reviewRecordAdapter) Models(ctx context.Context, dir string) (models []string, err error) {
	head, repo, records, closeState, err := a.open(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, closeState()) }()
	return records.Models(ctx, repo, head)
}

func (a reviewRecordAdapter) Save(ctx context.Context, dir, model string) (err error) {
	head, repo, records, closeState, err := a.open(ctx, dir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeState()) }()
	return records.Save(ctx, gatedomain.ReviewRecord{Repo: repo, HeadSHA: head, Model: model, At: time.Now()})
}

func (a reviewRecordAdapter) open(ctx context.Context, dir string) (head, repo string, records gatedomain.ReviewRecordRepository, closeState func() error, err error) {
	if dir, err = filepath.Abs(dir); err != nil {
		return "", "", nil, nil, err
	}
	if head, err = git.Head(ctx, dir); err != nil {
		return "", "", nil, nil, err
	}
	if repo, _, err = git.RepoKey(ctx, dir); err != nil {
		return "", "", nil, nil, err
	}
	commonDir, err := git.CommonDir(ctx, dir)
	if err != nil {
		return "", "", nil, nil, err
	}
	db, err := openState(ctx, a.cfg, commonDir)
	if err != nil {
		return "", "", nil, nil, err
	}
	return head, repo, gatemodule.NewReviewRecords(db), db.Close, nil
}

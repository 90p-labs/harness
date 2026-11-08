package verifycommits

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jorge07/harness/internal/gate/application/command/verify"
	"github.com/jorge07/harness/internal/gate/domain"
)

type Command struct {
	Repo     string
	Dir      string
	Base     string
	Gates    []domain.Gate
	Parallel int
}

type Result struct {
	Commits []domain.CommitResult
}

type Handler struct {
	commits domain.Commits
	verify  verify.Handler
	checks  domain.CommitCheckRepository
	now     func() time.Time
}

func NewHandler(commits domain.Commits, verifier verify.Handler, checks domain.CommitCheckRepository, now func() time.Time) Handler {
	return Handler{commits: commits, verify: verifier, checks: checks, now: now}
}

func (h Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	if len(cmd.Gates) == 0 {
		return Result{}, domain.ErrNoGates
	}
	head, err := h.commits.Head(ctx, cmd.Dir)
	if err != nil {
		return Result{}, err
	}
	listed, err := h.commits.List(ctx, cmd.Dir, cmd.Base)
	if err != nil {
		return Result{}, err
	}
	if len(listed) == 0 {
		return Result{}, fmt.Errorf("%w: %s..HEAD", domain.ErrNoCommits, cmd.Base)
	}
	results := make([]domain.CommitResult, 0, len(listed))
	for _, commit := range listed {
		passed, err := h.passes(ctx, cmd, commit.SHA)
		if err != nil {
			return Result{}, err
		}
		results = append(results, domain.CommitResult{SHA: commit.SHA, Type: domain.CommitType(commit.Subject), Passed: passed})
	}
	judged := domain.JudgeCommits(results)
	check := domain.CommitCheck{Repo: cmd.Repo, HeadSHA: head, Base: cmd.Base, Passed: judged == nil, At: h.now()}
	if err := h.checks.Save(ctx, check); err != nil {
		return Result{}, err
	}
	return Result{Commits: results}, judged
}

func (h Handler) passes(ctx context.Context, cmd Command, sha string) (passed bool, err error) {
	path, err := h.commits.AddWorktree(ctx, cmd.Dir, sha)
	if err != nil {
		return false, err
	}
	defer func() {
		err = errors.Join(err, h.commits.RemoveWorktree(context.WithoutCancel(ctx), cmd.Dir, path))
	}()
	result, err := h.verify.HandleConfigured(ctx, verify.Command{Repo: cmd.Repo, Dir: path, Gates: cmd.Gates, Parallel: cmd.Parallel})
	return !result.Failed, err
}

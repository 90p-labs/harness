package reviewplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	Name     = "plan.review"
	nextStep = "next: fix the problems, commit, then run harness run review again"

	selfCheckNextStep = "next: run the self-check worker, write the file with a Head: <sha> line, then run harness run review again"
)

var selfCheckHead = regexp.MustCompile(`(?m)^Head:[ \t]*[0-9a-f]{7,40}[ \t]*\r?$`)

type Plugin struct {
	worktree      string
	branches      domain.BranchReader
	proofs        domain.ProofReader
	defaultBranch func(ctx context.Context) (string, error)
}

var _ shareddomain.Plugin = Plugin{}

func New(worktree string, branches domain.BranchReader, proofs domain.ProofReader, defaultBranch func(ctx context.Context) (string, error)) Plugin {
	return Plugin{worktree: worktree, branches: branches, proofs: proofs, defaultBranch: defaultBranch}
}

func (Plugin) Name() string { return Name }

func (Plugin) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventRunTransitionBefore}
}

func (p Plugin) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	var payload shareddomain.TransitionPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return shareddomain.Verdict{}, fmt.Errorf("decode transition payload: %w", err)
	}
	if payload.Event != shareddomain.TransitionSubmitForReview {
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
	}
	if reason := p.selfCheckProblem(envelope.Run.ID); reason != "" {
		return shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: reason + "\n" + selfCheckNextStep}, nil
	}
	text, err := os.ReadFile(filepath.Join(p.worktree, filepath.FromSlash(shareddomain.RunPlanFile(envelope.Run.ID))))
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	doc, problems := domain.Parse(string(text))
	if len(problems) > 0 {
		return deny(problems), nil
	}
	base, err := p.base(ctx, doc)
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	branch, err := p.branches.Read(ctx, p.worktree, base, domain.TestFiles(doc))
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	passed, err := p.proofs.Passed(ctx)
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	if problems = domain.VerifyTrace(doc, branch, passed); len(problems) > 0 {
		return deny(problems), nil
	}
	return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
}

func deny(problems []domain.Problem) shareddomain.Verdict {
	lines := make([]string, 0, len(problems)+1)
	for _, problem := range problems {
		lines = append(lines, problem.String())
	}
	return shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: strings.Join(append(lines, nextStep), "\n")}
}

func (p Plugin) base(ctx context.Context, doc domain.Document) (string, error) {
	if base := doc.Base(); base != "" {
		return base, nil
	}
	return p.defaultBranch(ctx)
}

func (p Plugin) selfCheckProblem(runID string) string {
	file := shareddomain.RunSelfCheckFile(runID)
	text, err := os.ReadFile(filepath.Join(p.worktree, filepath.FromSlash(file)))
	if errors.Is(err, fs.ErrNotExist) {
		return file + ": the self-check file is missing"
	}
	if err != nil {
		return file + ": the self-check file cannot be read: " + err.Error()
	}
	if !selfCheckHead.Match(text) {
		return file + ": the self-check file has no Head: <sha> line"
	}
	return ""
}

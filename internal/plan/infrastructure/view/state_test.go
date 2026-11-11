package view

import (
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const statePlan = "# Plan: add the ping query\n" +
	"\n## Goal\n\nThe service answers a ping with the word pong.\n" +
	"\n## Acceptance criteria\n\n" +
	"| # | Criterion | Proof |\n|---|---|---|\n" +
	"| 1 | The domain gives the word pong. | `go test ./internal/ping/domain -run TestPong -count=1` |\n" +
	"\n## Tasks\n\n" +
	"### 1. Ping domain\n\n- Side: backend\n" +
	"- Owned files: `internal/ping/domain/pong.go`\n" +
	"- Test first: `pong_test.go`: `TestPong` fails, because `Pong` does not exist.\n" +
	"- DoD: criterion 1.\n- Depends on: none\n" +
	"\n### 2. Ping query\n\n- Side: backend\n" +
	"- Owned files: `internal/ping/module.go`\n" +
	"- Test first: `handler_test.go`: `TestPingQuery` fails, because the handler does not exist.\n" +
	"- DoD: criterion 1.\n- Depends on: task 1\n" +
	"\n### 3. Ping docs\n\n- Side: other\n" +
	"- Owned files: `docs/ping.md`\n" +
	"- Test first: none.\n" +
	"- DoD: criterion 1.\n- Depends on: task 1\n" +
	"\n## Tech specs\n\n" +
	"### Commits\n\n" +
	"1. `feat: add the pong domain` (task 1)\n" +
	"2. `feat: add the ping query` (task 2)\n" +
	"3. `feat: add the ping module` (task 2)\n"

func stateInput(t *testing.T, phase string, subjects ...string) domain.ViewInput {
	t.Helper()
	doc, problems := domain.Parse(statePlan)
	if len(problems) != 0 {
		t.Fatalf("parse problems: %v", problems)
	}
	in := domain.ViewInput{Doc: doc, Links: domain.Trace(doc), Run: domain.ViewRun{Phase: phase}}
	if subjects != nil {
		in.Branch = &domain.Branch{}
		for i, s := range subjects {
			in.Branch.Commits = append(in.Branch.Commits, domain.BranchCommit{SHA: string(rune('a' + i)), Subject: s})
		}
	}
	return in
}

func taskStates(m model) map[string]string {
	got := map[string]string{}
	for _, w := range m.Waves {
		for _, task := range w.Tasks {
			got[task.Key] = task.State
		}
	}
	return got
}

func TestPageTaskState(t *testing.T) {
	// Task 1 has its only commit, task 2 has one of two, task 3 has none planned.
	in := stateInput(t, shareddomain.PhaseWorking, "feat: add the pong domain", "feat: add the ping query")

	m := newBuilder(in).model()

	want := map[string]string{"t1": taskCommitted, "t2": taskOpen, "t3": taskOpen}
	got := taskStates(m)
	for key, state := range want {
		if got[key] != state {
			t.Errorf("task %s state = %q, want %q", key, got[key], state)
		}
	}
	for key, state := range want {
		if m.Data.Tasks[key] != state {
			t.Errorf("data task %s = %q, want %q", key, m.Data.Tasks[key], state)
		}
	}
	if m.Data.Phase != shareddomain.PhaseWorking {
		t.Errorf("data phase = %q, want %q", m.Data.Phase, shareddomain.PhaseWorking)
	}
	if m.Status != "Building" {
		t.Errorf("status = %q, want Building", m.Status)
	}
}

func TestPageTaskStateNoBranch(t *testing.T) {
	m := newBuilder(stateInput(t, shareddomain.PhaseAwaitingApproval)).model()

	for key, state := range taskStates(m) {
		if state != taskOpen {
			t.Errorf("task %s state = %q, want open without a branch", key, state)
		}
	}
	if m.Status != "Waiting for approval" {
		t.Errorf("status = %q, want Waiting for approval", m.Status)
	}
}

func TestPageStatusLabels(t *testing.T) {
	want := map[string]string{
		shareddomain.PhasePlanning:         "Planning",
		shareddomain.PhaseAwaitingApproval: "Waiting for approval",
		shareddomain.PhaseWorking:          "Building",
		shareddomain.PhaseReviewing:        "In review",
		shareddomain.PhaseShipping:         "Shipping",
		shareddomain.PhaseReleasing:        "Releasing",
		shareddomain.PhaseObserving:        "Observing",
		shareddomain.PhaseBlocked:          "Blocked",
		shareddomain.PhaseDone:             "Done",
		shareddomain.PhaseAbandoned:        "Abandoned",
	}
	for _, phase := range shareddomain.Phases {
		label, ok := want[phase]
		if !ok {
			t.Errorf("phase %s has no expected status label", phase)
			continue
		}
		if got := phaseLabel(phase); got != label {
			t.Errorf("phase %s label = %q, want %q", phase, got, label)
		}
	}
	if got := phaseLabel(""); got != "" {
		t.Errorf("empty phase label = %q, want empty", got)
	}
}

func TestPageNoteBlocksOpenByDefault(t *testing.T) {
	doc, problems := domain.Parse(statePlan +
		"\n## Review risks\n\n- A risk.\n" +
		"\n## Decisions\n\n- A decision.\n" +
		"\n## Open questions\n\n- A question?\n")
	if len(problems) != 0 {
		t.Fatalf("parse problems: %v", problems)
	}
	in := domain.ViewInput{Doc: doc, Links: domain.Trace(doc)}

	open := map[string]bool{}
	for _, block := range newBuilder(in).notes() {
		open[block.Title] = block.Open
	}
	if !open[domain.HeadingReviewRisks] || !open[domain.HeadingOpenQuestions] {
		t.Errorf("open = %v, want Review risks and Open questions open", open)
	}
	if open[domain.HeadingDecisions] {
		t.Error("the Decisions block opens by default")
	}
}

func TestPageFilesCountOnceAcrossTrees(t *testing.T) {
	doc, problems := domain.Parse(statePlan +
		"\n### Final code tree\n\n```tree\nping/\n├── shared.go     new\n└── code.go     new\n```\n" +
		"\n### Final test tree\n\n```tree\nping/\n├── shared.go     new\n└── code_test.go     new\n```\n")
	if len(problems) != 0 {
		t.Logf("parse problems: %v", problems)
	}
	in := domain.ViewInput{Doc: doc, Links: domain.Trace(doc)}

	m := newBuilder(in).model()

	if m.Counts.Files != 3 {
		t.Errorf("files = %d, want 3 with the shared path counted once", m.Counts.Files)
	}
	if m.Done.NewFiles != 3 {
		t.Errorf("new files = %d, want 3", m.Done.NewFiles)
	}
}

func TestPagePropertiesCounts(t *testing.T) {
	in := stateInput(t, shareddomain.PhaseWorking, "feat: add the pong domain", "feat: add the ping query")
	in.Proofs = map[int]bool{1: true}

	m := newBuilder(in).model()

	if m.Done.Criteria != 1 || m.Done.Tasks != 1 {
		t.Errorf("done = %+v, want 1 criterion proved and 1 task committed", m.Done)
	}
	if m.Counts.Criteria != 1 || m.Counts.Tasks != 3 {
		t.Errorf("counts = %+v, want 1 criterion and 3 tasks", m.Counts)
	}

	none := newBuilder(stateInput(t, shareddomain.PhaseAwaitingApproval)).model()
	if none.Done.Criteria != 0 || none.Done.Tasks != 0 {
		t.Errorf("done without a branch = %+v, want zeros", none.Done)
	}
}

package verifycommits_test

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/gate/application/command/verify"
	"github.com/90p-labs/harness/internal/gate/application/command/verifycommits"
	"github.com/90p-labs/harness/internal/gate/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	repo    = "github.com/90p-labs/harness"
	workDir = "/work/harness"
	baseRef = "main"
	headSHA = "c3c3c3"
	baseSHA = "0a83514a6119"

	testGate   = "go-test"
	originBase = "origin/main"
	failTail   = "FAIL: TestWidget expected 2 got 3"

	spanVerifyCommits = "gate.verify_commits"
	spanVerifyCommit  = "gate.verify_commit"
	spanVerify        = "gate.verify"
	attrRunID         = "harness.run.id"
	attrWorktree      = "harness.worktree"
	attrCommit        = "harness.commit"
)

var fixedNow = time.Date(2026, 11, 8, 5, 0, 0, 0, time.UTC)

var errInjected = errors.New("injected")

type fakeCommits struct {
	base       domain.Base
	list       []domain.Commit
	resolved   []string
	listed     []string
	added      []string
	removed    []string
	headErr    error
	resolveErr error
	listErr    error
	addErr     error
	removeErr  error
}

func (f *fakeCommits) Head(context.Context, string) (string, error) { return headSHA, f.headErr }

func (f *fakeCommits) ResolveBase(_ context.Context, _, base string) (domain.Base, error) {
	f.resolved = append(f.resolved, base)
	return f.base, f.resolveErr
}

func (f *fakeCommits) List(_ context.Context, _, base string) ([]domain.Commit, error) {
	f.listed = append(f.listed, base)
	return f.list, f.listErr
}

func (f *fakeCommits) AddWorktree(_ context.Context, _, sha string) (string, error) {
	f.added = append(f.added, sha)
	return "/tmp/wt-" + sha, f.addErr
}

func (f *fakeCommits) RemoveWorktree(ctx context.Context, _, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.removed = append(f.removed, path)
	return f.removeErr
}

type fakeHasher struct{}

func (fakeHasher) InputHash(_ context.Context, dir string, _ domain.Gate) (string, error) {
	return "tree@" + dir, nil
}

type fakeRunner struct {
	mu       sync.Mutex
	passDirs map[string]bool
	err      error
	tail     string
	ranDirs  []string
}

func (f *fakeRunner) Run(_ context.Context, dir string, g domain.Gate) (domain.Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ranDirs = append(f.ranDirs, dir+"#"+g.ID)
	if f.err != nil {
		return domain.Outcome{}, f.err
	}
	if f.passDirs[dir] {
		return domain.Outcome{Result: domain.ResultPass}, nil
	}
	return domain.Outcome{Result: domain.ResultFail, Tail: f.tail}, nil
}

type fakeEvidence struct{}

func (fakeEvidence) Find(context.Context, string, string, string) (domain.Evidence, bool, error) {
	return domain.Evidence{}, false, nil
}

func (fakeEvidence) Save(context.Context, domain.Evidence) error { return nil }

type noSetups struct{}

func (noSetups) LastPass(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}

func (noSetups) SavePass(context.Context, string, string, string, time.Time) error { return nil }

type allowAll struct{}

func (allowAll) Dispatch(context.Context, shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}}, nil
}

type fakeChecks struct {
	saved   []domain.CommitCheck
	saveErr error
}

func (f *fakeChecks) Passed(context.Context, string, string) (bool, error) { return false, nil }

func (f *fakeChecks) Save(_ context.Context, c domain.CommitCheck) error {
	f.saved = append(f.saved, c)
	return f.saveErr
}

type fakeSource struct{ gates []domain.Gate }

func (f fakeSource) Gates(context.Context) ([]domain.Gate, error) { return f.gates, nil }

type fixture struct {
	commits *fakeCommits
	runner  *fakeRunner
	checks  *fakeChecks
	spans   *tracetest.SpanRecorder
	handler verifycommits.Handler
}

type denyAll struct{ reason string }

func (d denyAll) Dispatch(context.Context, shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: d.reason}}, nil
}

func newFixture(source domain.GateSource, passDirs ...string) *fixture {
	return newFixtureWith(source, allowAll{}, passDirs...)
}

func newFixtureWith(source domain.GateSource, dispatcher shareddomain.Dispatcher, passDirs ...string) *fixture {
	f := &fixture{
		commits: &fakeCommits{base: domain.Base{Ref: baseRef, SHA: baseSHA}, list: []domain.Commit{
			{SHA: "a1a1a1", Subject: "test: reproduce the crash"},
			{SHA: "b2b2b2", Subject: "fix(gate): stop the crash"},
			{SHA: headSHA, Subject: "chore: tidy"},
		}},
		runner: &fakeRunner{passDirs: map[string]bool{}},
		checks: &fakeChecks{},
	}
	for _, dir := range passDirs {
		f.runner.passDirs[dir] = true
	}
	f.spans = tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(f.spans)).Tracer("test")
	readFile := func(string) ([]byte, error) { return nil, fs.ErrNotExist }
	exists := func(string) (bool, error) { return false, nil }
	inner := verify.NewHandler(
		fakeHasher{}, f.runner, fakeEvidence{}, noSetups{}, dispatcher, source,
		readFile, exists, tracer, noop.NewMeterProvider().Meter("test"), func() time.Time { return fixedNow },
	)
	f.handler = verifycommits.NewHandler(f.commits, inner, f.checks, func() time.Time { return fixedNow }, tracer)
	return f
}

func command() verifycommits.Command {
	return verifycommits.Command{Repo: repo, Dir: workDir, Base: baseRef, Gates: []domain.Gate{{ID: testGate, Run: "go test"}}}
}

func TestVerifyCommits(t *testing.T) {
	f := newFixture(nil, "/tmp/wt-b2b2b2", "/tmp/wt-"+headSHA)

	result, err := f.handler.Handle(context.Background(), command())

	if err != nil {
		t.Fatal(err)
	}
	want := []domain.CommitResult{
		{SHA: "a1a1a1", Type: domain.CommitTypeTest, Passed: false, Failed: []domain.GateFailure{{Gate: testGate, Status: domain.StatusFail}}},
		{SHA: "b2b2b2", Type: domain.CommitTypeFix, Passed: true},
		{SHA: headSHA, Type: "chore", Passed: true},
	}
	if !reflect.DeepEqual(result.Commits, want) {
		t.Fatalf("commits = %+v, want %+v", result.Commits, want)
	}
	if !slices.Equal(f.commits.added, []string{"a1a1a1", "b2b2b2", headSHA}) {
		t.Fatalf("added = %v", f.commits.added)
	}
	if !slices.Equal(f.commits.removed, []string{"/tmp/wt-a1a1a1", "/tmp/wt-b2b2b2", "/tmp/wt-" + headSHA}) {
		t.Fatalf("removed = %v", f.commits.removed)
	}
	wantCheck := domain.CommitCheck{Repo: repo, HeadSHA: headSHA, Base: baseRef, Passed: true, At: fixedNow}
	if !slices.Equal(f.checks.saved, []domain.CommitCheck{wantCheck}) {
		t.Fatalf("saved = %+v, want %+v", f.checks.saved, wantCheck)
	}
}

func TestVerifyCommitsResolvesTheBase(t *testing.T) {
	f := newFixture(nil, "/tmp/wt-b2b2b2", "/tmp/wt-"+headSHA)
	f.commits.base = domain.Base{Ref: originBase, SHA: baseSHA}

	result, err := f.handler.Handle(context.Background(), command())

	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.commits.resolved, []string{baseRef}) || !slices.Equal(f.commits.listed, []string{originBase}) {
		t.Fatalf("resolved = %v, listed = %v, want %s resolved and %s listed", f.commits.resolved, f.commits.listed, baseRef, originBase)
	}
	if result.Base != f.commits.base {
		t.Fatalf("base = %+v, want %+v", result.Base, f.commits.base)
	}
	if len(f.checks.saved) != 1 || f.checks.saved[0].Base != originBase {
		t.Fatalf("saved = %+v, want the check base %s", f.checks.saved, originBase)
	}

	empty := newFixture(nil)
	empty.commits.base = f.commits.base
	empty.commits.list = nil
	_, err = empty.handler.Handle(context.Background(), command())
	if !errors.Is(err, domain.ErrNoCommits) || !strings.Contains(err.Error(), originBase+"..HEAD") {
		t.Fatalf("error = %v, want ErrNoCommits naming %s..HEAD", err, originBase)
	}
}

func TestVerifyCommitsKeepsTheFailedGates(t *testing.T) {
	f := newFixture(nil, "/tmp/wt-b2b2b2", "/tmp/wt-"+headSHA)
	f.runner.tail = failTail

	result, err := f.handler.Handle(context.Background(), command())

	if err != nil {
		t.Fatal(err)
	}
	want := []domain.GateFailure{{Gate: testGate, Status: domain.StatusFail, Tail: failTail}}
	if !reflect.DeepEqual(result.Commits[0].Failed, want) {
		t.Fatalf("failed = %+v, want %+v", result.Commits[0].Failed, want)
	}
	if result.Commits[1].Failed != nil || result.Commits[2].Failed != nil {
		t.Fatalf("passing commits = %+v, want no failed gate", result.Commits[1:])
	}

	broken := newFixture(nil)
	broken.runner.tail = failTail
	broken.commits.list = []domain.Commit{{SHA: "b2b2b2", Subject: "fix(gate): stop the crash"}}
	result, err = broken.handler.Handle(context.Background(), command())
	if !errors.Is(err, domain.ErrCommitRule) || !strings.Contains(err.Error(), "failed "+testGate) {
		t.Fatalf("error = %v, want the rule error naming %s", err, testGate)
	}
	if result.Base != broken.commits.base || len(result.Commits) != 1 || !reflect.DeepEqual(result.Commits[0].Failed, want) {
		t.Fatalf("result = %+v, want the base and the failed gates also when the rule breaks", result)
	}
}

func TestVerifyCommitsKeepsAVetoedGate(t *testing.T) {
	const reason = "a rule vetoed the gate"
	f := newFixtureWith(nil, denyAll{reason: reason})
	f.commits.list = []domain.Commit{{SHA: "b2b2b2", Subject: "fix(gate): stop the crash"}}

	result, err := f.handler.Handle(context.Background(), command())

	if !errors.Is(err, domain.ErrCommitRule) {
		t.Fatalf("error = %v, want the commit rule error", err)
	}
	want := []domain.GateFailure{{Gate: testGate, Status: domain.StatusVetoed, Tail: reason}}
	if len(result.Commits) != 1 || result.Commits[0].Passed || !reflect.DeepEqual(result.Commits[0].Failed, want) {
		t.Fatalf("commits = %+v, want one failed commit with %+v", result.Commits, want)
	}
}

func TestHandleSharedSchedulerSkips(t *testing.T) {
	const lintGate = "lint"
	f := newFixture(nil, "/tmp/wt-b2b2b2", "/tmp/wt-"+headSHA)
	cmd := command()
	cmd.Gates = []domain.Gate{{ID: lintGate, Run: "lint"}, {ID: testGate, Run: "go test", Needs: []string{lintGate}}}
	cmd.Parallel = 2

	result, err := f.handler.Handle(context.Background(), cmd)

	if err != nil || result.Commits[0].Passed || !result.Commits[1].Passed {
		t.Fatalf("result = %+v, err = %v, want the test commit to fail and the fix to pass", result, err)
	}
	if slices.Contains(f.runner.ranDirs, "/tmp/wt-a1a1a1#"+testGate) {
		t.Fatalf("ran = %q, want %s skipped after the failed lint", f.runner.ranDirs, testGate)
	}
}

func TestHandleSetupInEachWorktree(t *testing.T) {
	const depsStep = "deps"
	shas := []string{"a1a1a1", "b2b2b2", headSHA}
	var dirs []string
	for _, sha := range shas {
		dirs = append(dirs, "/tmp/wt-"+sha)
	}
	f := newFixture(nil, dirs...)
	cmd := command()
	cmd.Setup = []domain.Setup{{Gate: domain.Gate{ID: depsStep, Run: "make install"}, Creates: "vendor/autoload.php"}}

	_, _ = f.handler.Handle(context.Background(), cmd)

	for _, dir := range dirs {
		step, gate := slices.Index(f.runner.ranDirs, dir+"#"+depsStep), slices.Index(f.runner.ranDirs, dir+"#"+testGate)
		if step < 0 || gate < step {
			t.Errorf("ran = %q, want %s before %s in %s", f.runner.ranDirs, depsStep, testGate, dir)
		}
	}
}

func TestVerifyCommitsRuleBroken(t *testing.T) {
	f := newFixture(nil, "/tmp/wt-a1a1a1", "/tmp/wt-b2b2b2", "/tmp/wt-"+headSHA)

	_, err := f.handler.Handle(context.Background(), command())

	if !errors.Is(err, domain.ErrCommitRule) {
		t.Fatalf("error = %v, want ErrCommitRule", err)
	}
	if len(f.checks.saved) != 1 || f.checks.saved[0].Passed {
		t.Fatalf("saved = %+v, want one failed check", f.checks.saved)
	}
}

func TestVerifyCommitsRemovesWorktreesOnError(t *testing.T) {
	f := newFixture(nil)
	f.runner.err = errors.New("boom")

	_, err := f.handler.Handle(context.Background(), command())

	if !errors.Is(err, f.runner.err) {
		t.Fatalf("error = %v, want the runner error", err)
	}
	if len(f.commits.added) != 1 || !slices.Equal(f.commits.removed, []string{"/tmp/wt-a1a1a1"}) {
		t.Fatalf("added = %v, removed = %v, want the first worktree removed", f.commits.added, f.commits.removed)
	}
	if len(f.checks.saved) != 0 {
		t.Fatalf("saved = %+v, want no check", f.checks.saved)
	}
}

func TestVerifyCommitsRemovesWorktreesAfterACancel(t *testing.T) {
	f := newFixture(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.handler.Handle(ctx, command())

	if errors.Is(err, context.Canceled) || len(f.commits.removed) != len(f.commits.added) {
		t.Fatalf("error = %v, added = %v, removed = %v, want each worktree removed", err, f.commits.added, f.commits.removed)
	}
}

func TestVerifyCommitsErrors(t *testing.T) {
	cases := map[string]func(f *fixture){
		"head":            func(f *fixture) { f.commits.headErr = errInjected },
		"resolve base":    func(f *fixture) { f.commits.resolveErr = errInjected },
		"list":            func(f *fixture) { f.commits.listErr = errInjected },
		"add worktree":    func(f *fixture) { f.commits.addErr = errInjected },
		"remove worktree": func(f *fixture) { f.commits.removeErr = errInjected },
		"save check":      func(f *fixture) { f.checks.saveErr = errInjected },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(nil, "/tmp/wt-b2b2b2", "/tmp/wt-"+headSHA)
			setup(f)

			_, err := f.handler.Handle(context.Background(), command())

			if !errors.Is(err, errInjected) {
				t.Fatalf("error = %v, want the injected error", err)
			}
		})
	}
}

func TestVerifyCommitsRefusesAnEmptyCheck(t *testing.T) {
	cases := map[string]struct {
		setup func(f *fixture, cmd *verifycommits.Command)
		want  error
	}{
		"an empty range": {setup: func(f *fixture, _ *verifycommits.Command) { f.commits.list = nil }, want: domain.ErrNoCommits},
		"no gate":        {setup: func(_ *fixture, cmd *verifycommits.Command) { cmd.Gates = nil }, want: domain.ErrNoGates},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(nil, "/tmp/wt-a1a1a1", "/tmp/wt-b2b2b2", "/tmp/wt-"+headSHA)
			cmd := command()
			tc.setup(f, &cmd)

			_, err := f.handler.Handle(context.Background(), cmd)

			if !errors.Is(err, tc.want) || len(f.checks.saved) != 0 || len(f.commits.added) != 0 {
				t.Fatalf("error = %v, saved = %+v, added = %v, want %v and no check", err, f.checks.saved, f.commits.added, tc.want)
			}
		})
	}
}

func TestVerifyCommitsSkipsCriteria(t *testing.T) {
	criterion := domain.Gate{ID: domain.CriterionGateID(1), Run: "go vet"}
	f := newFixture(fakeSource{gates: []domain.Gate{criterion}}, "/tmp/wt-b2b2b2", "/tmp/wt-"+headSHA)

	if _, err := f.handler.Handle(context.Background(), command()); err != nil {
		t.Fatal(err)
	}

	for _, ran := range f.runner.ranDirs {
		if ran[len(ran)-len(criterion.ID):] == criterion.ID {
			t.Fatalf("criterion gate ran: %v", f.runner.ranDirs)
		}
	}
	if len(f.runner.ranDirs) != 3 {
		t.Fatalf("ran = %v, want one YAML gate for each commit", f.runner.ranDirs)
	}
}

func spanAttrs(span sdktrace.ReadOnlySpan) map[string]string {
	attrs := map[string]string{}
	for _, a := range span.Attributes() {
		attrs[string(a.Key)] = a.Value.Emit()
	}
	return attrs
}

func TestVerifyCommitsTrace(t *testing.T) {
	f := newFixture(nil, "/tmp/wt-b2b2b2", "/tmp/wt-"+headSHA)
	cmd := command()

	if _, err := f.handler.Handle(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}

	var roots, commits, verifies []sdktrace.ReadOnlySpan
	ended := f.spans.Ended()
	for _, span := range ended {
		if span.SpanContext().TraceID() != ended[0].SpanContext().TraceID() {
			t.Errorf("span %s has a different trace id", span.Name())
		}
		if !span.Parent().IsValid() {
			roots = append(roots, span)
		}
		switch span.Name() {
		case spanVerifyCommit:
			commits = append(commits, span)
		case spanVerify:
			verifies = append(verifies, span)
		}
	}
	if len(roots) != 1 || roots[0].Name() != spanVerifyCommits {
		t.Fatalf("roots = %d, want one %s", len(roots), spanVerifyCommits)
	}
	root := roots[0]
	attrs := spanAttrs(root)
	for _, key := range []string{attrRunID, attrWorktree} {
		if _, has := attrs[key]; has {
			t.Errorf("root has %s, want none", key)
		}
	}
	if len(commits) != 3 || len(verifies) != 3 {
		t.Fatalf("commit spans = %d, verify spans = %d, want 3 each", len(commits), len(verifies))
	}
	for i, sha := range []string{"a1a1a1", "b2b2b2", headSHA} {
		if commits[i].Parent().SpanID() != root.SpanContext().SpanID() || spanAttrs(commits[i])[attrCommit] != sha {
			t.Errorf("commit span %d: parent or %s wrong, want child of root with %s", i, attrCommit, sha)
		}
	}
	for _, span := range verifies {
		parent := span.Parent().SpanID()
		if !slices.ContainsFunc(commits, func(c sdktrace.ReadOnlySpan) bool { return c.SpanContext().SpanID() == parent }) {
			t.Errorf("verify span is not the child of a %s span", spanVerifyCommit)
		}
		if _, has := spanAttrs(span)[attrRunID]; has {
			t.Errorf("verify span has %s, want none", attrRunID)
		}
		if tree := spanAttrs(span)[attrWorktree]; tree == "" || tree == workDir {
			t.Errorf("verify span %s = %q, want the temp worktree", attrWorktree, tree)
		}
	}
}

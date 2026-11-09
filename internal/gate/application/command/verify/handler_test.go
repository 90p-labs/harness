package verify_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jorge07/harness/internal/gate/application/command/verify"
	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/shared/application/eventbus"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	repo          = "github.com/jorge07/harness"
	workDir       = "/work/harness"
	inputHash     = "tree-1"
	testGate      = "go-test"
	lintGate      = "lint"
	baselinePath  = ".harness-lint/baseline.json"
	spanName      = "gate.run"
	spanVerify    = "gate.verify"
	spanSetup     = "gate.setup"
	attrCount     = "harness.gate.count"
	attrWorktree  = "harness.worktree"
	attrRunID     = "harness.run.id"
	metricName    = "harness.gate.duration"
	attrGate      = "harness.gate"
	attrResult    = "harness.gate.result"
	attrCached    = "harness.gate.cached"
	tailText      = "ok  all packages"
	failTail      = "FAIL"
	criterionText = "go vet ./..."
)

var fixedNow = time.Date(2026, 11, 8, 5, 0, 0, 0, time.UTC)

var errInjected = errors.New("injected")

var statusResult = map[domain.Status]domain.Result{domain.StatusPass: domain.ResultPass, domain.StatusFail: domain.ResultFail}

type idLog struct {
	mu  sync.Mutex
	ids []string
}

func (l *idLog) add(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ids = append(l.ids, id)
}

func (l *idLog) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.ids)
}

type fakeHasher struct {
	hash   string
	err    error
	hashed *idLog
}

func (f fakeHasher) InputHash(_ context.Context, _ string, gate domain.Gate) (string, error) {
	if f.hashed != nil {
		f.hashed.add(gate.ID)
	}
	return f.hash, f.err
}

type ran struct{ dir, gate string }

type fakeRunner struct {
	mu       sync.Mutex
	outcomes map[string]domain.Outcome
	err      error
	ran      []ran
}

func (f *fakeRunner) Run(_ context.Context, dir string, gate domain.Gate) (domain.Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ran = append(f.ran, ran{dir: dir, gate: gate.ID})
	if f.err != nil {
		return domain.Outcome{}, f.err
	}
	return f.outcomes[gate.ID], nil
}

type fakeEvidence struct {
	mu      sync.Mutex
	records map[string]domain.Evidence
	saved   []domain.Evidence
	findErr error
	saveErr error
}

func recordKey(gate string) string { return gate + "@" + inputHash }

func (f *fakeEvidence) Find(_ context.Context, _, gateID, hash string) (domain.Evidence, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.records[gateID+"@"+hash]
	return e, ok, f.findErr
}

func (f *fakeEvidence) Save(_ context.Context, e domain.Evidence) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, e)
	return f.saveErr
}

type fakeDispatcher struct {
	mu        sync.Mutex
	deny      map[string]bool
	errEvent  shareddomain.EventName
	envelopes []shareddomain.Envelope
}

func (f *fakeDispatcher) Dispatch(_ context.Context, e shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envelopes = append(f.envelopes, e)
	if e.Event == f.errEvent {
		return shareddomain.DispatchResult{}, errInjected
	}
	var payload struct {
		Gate string `json:"gate"`
	}
	if err := json.Unmarshal(e.Payload, &payload); err != nil {
		return shareddomain.DispatchResult{}, err
	}
	if e.Event == shareddomain.EventGateBefore && f.deny[payload.Gate] {
		return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "no"}}, nil
	}
	return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}}, nil
}

type fakeSetups struct {
	mu    sync.Mutex
	last  map[string]string
	saved []string
}

func (f *fakeSetups) LastPass(_ context.Context, _, stepID string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	hash, ok := f.last[stepID]
	return hash, ok, nil
}

func (f *fakeSetups) SavePass(_ context.Context, _, stepID, hash string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, stepID+"@"+hash)
	f.last[stepID] = hash
	return nil
}

type fakeSource struct {
	gates []domain.Gate
	err   error
}

func (f fakeSource) Gates(context.Context) ([]domain.Gate, error) { return f.gates, f.err }

type fixture struct {
	runner     *fakeRunner
	evidence   *fakeEvidence
	dispatcher *fakeDispatcher
	spans      *tracetest.SpanRecorder
	reader     *sdkmetric.ManualReader
	files      map[string]string
	setups     *fakeSetups
	created    map[string]bool
	handler    verify.Handler
	readErr    error
}

func newFixture(source domain.GateSource) *fixture {
	return newFixtureWith(source, fakeHasher{hash: inputHash}, nil)
}

func newFixtureWith(source domain.GateSource, hasher fakeHasher, runner domain.Runner) *fixture {
	f := &fixture{
		runner:     &fakeRunner{outcomes: map[string]domain.Outcome{}},
		evidence:   &fakeEvidence{records: map[string]domain.Evidence{}},
		dispatcher: &fakeDispatcher{deny: map[string]bool{}},
		spans:      tracetest.NewSpanRecorder(),
		reader:     sdkmetric.NewManualReader(),
		files:      map[string]string{},
		setups:     &fakeSetups{last: map[string]string{}},
		created:    map[string]bool{},
	}
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(f.spans)).Tracer("test")
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(f.reader)).Meter("test")
	readFile := func(path string) ([]byte, error) {
		if f.readErr != nil {
			return nil, f.readErr
		}
		content, ok := f.files[path]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return []byte(content), nil
	}
	if runner == nil {
		runner = f.runner
	}
	exists := func(path string) (bool, error) { return f.created[path], nil }
	f.handler = verify.NewHandler(
		hasher, runner, f.evidence, f.setups, f.dispatcher, source,
		readFile, exists, tracer, meter, func() time.Time { return fixedNow },
	)
	return f
}

func gate(id string) domain.Gate {
	return domain.Gate{ID: id, Run: "run " + id, Timeout: time.Minute}
}

func command(gates ...domain.Gate) verify.Command {
	return verify.Command{Repo: repo, Dir: workDir, Gates: gates}
}

func passOutcome() domain.Outcome {
	return domain.Outcome{Result: domain.ResultPass, Duration: 2 * time.Second, Tail: tailText}
}

func failOutcome() domain.Outcome {
	return domain.Outcome{Result: domain.ResultFail, Duration: time.Second, Tail: failTail}
}

func TestVerifyRunsGates(t *testing.T) {
	f := newFixture(nil)
	f.runner.outcomes[testGate] = passOutcome()
	f.runner.outcomes[lintGate] = failOutcome()

	result, err := f.handler.Handle(context.Background(), command(gate(testGate), gate(lintGate)))

	if err != nil {
		t.Fatal(err)
	}
	want := []verify.Line{
		{Gate: testGate, Status: domain.StatusPass, Duration: 2 * time.Second, Tail: tailText},
		{Gate: lintGate, Status: domain.StatusFail, Duration: time.Second, Tail: failTail},
	}
	if !slices.Equal(result.Lines, want) || !result.Failed {
		t.Fatalf("result = %+v, want lines %+v and failed", result, want)
	}
	if !slices.Equal(f.runner.ran, []ran{{workDir, testGate}, {workDir, lintGate}}) {
		t.Fatalf("ran = %+v", f.runner.ran)
	}
	wantSaved := []domain.Evidence{
		{Repo: repo, GateID: testGate, InputHash: inputHash, Result: domain.ResultPass, Duration: 2 * time.Second, Tail: tailText, At: fixedNow},
		{Repo: repo, GateID: lintGate, InputHash: inputHash, Result: domain.ResultFail, Duration: time.Second, Tail: failTail, At: fixedNow},
	}
	if !slices.Equal(f.evidence.saved, wantSaved) {
		t.Fatalf("saved = %+v, want %+v", f.evidence.saved, wantSaved)
	}
}

func TestVerifyReusesEvidence(t *testing.T) {
	f := newFixture(nil)
	f.evidence.records[recordKey(testGate)] = domain.Evidence{
		GateID: testGate, InputHash: inputHash, Result: domain.ResultPass, Duration: 3 * time.Second, Tail: tailText,
	}

	result, err := f.handler.Handle(context.Background(), command(gate(testGate)))

	if err != nil {
		t.Fatal(err)
	}
	want := []verify.Line{{Gate: testGate, Status: domain.StatusCached, Duration: 3 * time.Second, Tail: tailText}}
	if !slices.Equal(result.Lines, want) || result.Failed {
		t.Fatalf("result = %+v, want %+v", result, want)
	}
	if len(f.runner.ran) != 0 || len(f.dispatcher.envelopes) != 0 || len(f.evidence.saved) != 0 {
		t.Fatalf("a cached gate ran %d, dispatched %d, saved %d", len(f.runner.ran), len(f.dispatcher.envelopes), len(f.evidence.saved))
	}
}

func TestVerifyRerunsFailure(t *testing.T) {
	f := newFixture(nil)
	f.evidence.records[recordKey(testGate)] = domain.Evidence{GateID: testGate, InputHash: inputHash, Result: domain.ResultFail}
	f.runner.outcomes[testGate] = passOutcome()

	result, err := f.handler.Handle(context.Background(), command(gate(testGate)))

	if err != nil {
		t.Fatal(err)
	}
	if result.Lines[0].Status != domain.StatusPass || len(f.runner.ran) != 1 {
		t.Fatalf("result = %+v, ran = %d, want a rerun that passes", result, len(f.runner.ran))
	}
}

func TestVerifyGateEvents(t *testing.T) {
	f := newFixture(nil)
	f.runner.outcomes[testGate] = passOutcome()
	f.dispatcher.deny[lintGate] = true

	result, err := f.handler.Handle(context.Background(), command(gate(testGate), gate(lintGate)))

	if err != nil {
		t.Fatal(err)
	}
	events := make([]shareddomain.EventName, 0, len(f.dispatcher.envelopes))
	for _, e := range f.dispatcher.envelopes {
		events = append(events, e.Event)
	}
	wantEvents := []shareddomain.EventName{shareddomain.EventGateBefore, shareddomain.EventGateAfter, shareddomain.EventGateBefore}
	if !slices.Equal(events, wantEvents) {
		t.Fatalf("events = %v, want %v", events, wantEvents)
	}
	var before, after map[string]any
	if err := json.Unmarshal(f.dispatcher.envelopes[0].Payload, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(f.dispatcher.envelopes[1].Payload, &after); err != nil {
		t.Fatal(err)
	}
	if before["gate"] != testGate || before["input_hash"] != inputHash || len(before) != 3 {
		t.Fatalf("before payload = %v", before)
	}
	if after["gate"] != testGate || after["result"] != string(domain.ResultPass) || after["duration_ms"] != float64(2000) || after["input_hash"] != inputHash {
		t.Fatalf("after payload = %v", after)
	}
	if f.dispatcher.envelopes[0].Cwd != workDir || f.dispatcher.envelopes[0].Version != shareddomain.EnvelopeVersion {
		t.Fatalf("envelope = %+v", f.dispatcher.envelopes[0])
	}
	if result.Lines[1].Status != domain.StatusVetoed || !result.Failed {
		t.Fatalf("result = %+v, want vetoed and failed", result)
	}
	if len(f.runner.ran) != 1 || len(f.evidence.saved) != 1 {
		t.Fatalf("vetoed gate ran or saved: ran %d, saved %d", len(f.runner.ran), len(f.evidence.saved))
	}
}

func TestVerifyTelemetry(t *testing.T) {
	f := newFixture(nil)
	f.evidence.records[recordKey(lintGate)] = domain.Evidence{GateID: lintGate, InputHash: inputHash, Result: domain.ResultPass}
	f.runner.outcomes[testGate] = passOutcome()

	if _, err := f.handler.Handle(context.Background(), command(gate(testGate), gate(lintGate))); err != nil {
		t.Fatal(err)
	}

	var spans []sdktrace.ReadOnlySpan
	for _, span := range f.spans.Ended() {
		if span.Name() == spanName {
			spans = append(spans, span)
		}
	}
	if len(spans) != 2 {
		t.Fatalf("spans = %d, want 2", len(spans))
	}
	wantCached := map[string]string{testGate: "false", lintGate: "true"}
	for _, span := range spans {
		attrs := map[string]string{}
		for _, a := range span.Attributes() {
			attrs[string(a.Key)] = a.Value.Emit()
		}
		if span.Name() != spanName || attrs[attrResult] != string(domain.ResultPass) || attrs[attrCached] != wantCached[attrs[attrGate]] {
			t.Fatalf("span %s attributes = %v", span.Name(), attrs)
		}
	}
	var collected metricdata.ResourceMetrics
	if err := f.reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	var points []metricdata.HistogramDataPoint[float64]
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == metricName {
				points = append(points, m.Data.(metricdata.Histogram[float64]).DataPoints...)
			}
		}
	}
	if len(points) != 1 || points[0].Count != 1 {
		t.Fatalf("points = %d, want one point for the gate that ran", len(points))
	}
	want := attribute.NewSet(attribute.String(attrGate, testGate), attribute.String(attrResult, string(domain.ResultPass)))
	if !points[0].Attributes.Equals(&want) {
		t.Fatalf("attributes = %v, want %v", points[0].Attributes, want)
	}
	if !slices.Equal(points[0].Bounds, eventbus.DurationBuckets()) || points[0].Sum != 2 {
		t.Fatalf("bounds = %v, sum = %v", points[0].Bounds, points[0].Sum)
	}
}

func TestVerifyCriterionGates(t *testing.T) {
	criterion := domain.Gate{ID: domain.CriterionGateID(1), Run: criterionText, Timeout: time.Minute}
	f := newFixture(fakeSource{gates: []domain.Gate{criterion}})
	f.runner.outcomes[testGate] = passOutcome()
	f.runner.outcomes[criterion.ID] = failOutcome()

	result, err := f.handler.Handle(context.Background(), command(gate(testGate)))

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != 2 || result.Lines[0].Gate != testGate || result.Lines[1].Gate != criterion.ID || !result.Failed {
		t.Fatalf("result = %+v, want the YAML gate then the criterion gate, failed", result)
	}
}

func TestVerifyConfiguredGatesSkipCriteria(t *testing.T) {
	criterion := domain.Gate{ID: domain.CriterionGateID(1), Run: criterionText, Timeout: time.Minute}
	f := newFixture(fakeSource{gates: []domain.Gate{criterion}})
	f.runner.outcomes[testGate] = passOutcome()

	result, err := f.handler.HandleConfigured(context.Background(), command(gate(testGate)))

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != 1 || len(f.runner.ran) != 1 {
		t.Fatalf("result = %+v, ran = %+v, want the YAML gate only", result, f.runner.ran)
	}
}

func TestVerifyBaseline(t *testing.T) {
	known := shareddomain.Finding{Rule: "no-any", Severity: shareddomain.SeverityError, File: "a.ts", Line: 3, Message: "any is banned"}
	fresh := shareddomain.Finding{Rule: "no-eval", Severity: shareddomain.SeverityError, File: "b.ts", Line: 9, Message: "eval is banned"}
	warning := shareddomain.Finding{Rule: "style", Severity: shareddomain.SeverityWarn, File: "c.ts", Line: 1, Message: "style"}
	report := func(findings ...shareddomain.Finding) string {
		raw, _ := json.Marshal(shareddomain.Report{Version: shareddomain.ReportVersion, Findings: findings})
		return string(raw)
	}
	cases := []struct {
		name       string
		exit       domain.Result
		stdout     string
		baseline   string
		wantStatus domain.Status
		timedOut   bool
	}{
		{name: "a timed out gate with known findings fails", exit: domain.ResultFail, stdout: report(known), baseline: report(known), wantStatus: domain.StatusFail, timedOut: true},
		{"known finding passes", domain.ResultFail, report(known, warning), report(known), domain.StatusPass, false},
		{"a clean report with exit 0 passes", domain.ResultPass, report(warning), report(), domain.StatusPass, false},
		{"new error fails", domain.ResultFail, report(known, fresh), report(known), domain.StatusFail, false},
		{"missing baseline file is empty", domain.ResultFail, report(known), "", domain.StatusFail, false},
		{"a failed command with no error finding fails", domain.ResultFail, report(warning), report(), domain.StatusFail, false},
		{"a non-report stdout fails", domain.ResultPass, "not json", report(), domain.StatusFail, false},
		{"an unknown report version fails", domain.ResultPass, `{"version":2,"findings":[]}`, report(), domain.StatusFail, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(nil)
			g := gate(lintGate)
			g.Baseline = baselinePath
			if tc.baseline != "" {
				f.files[workDir+"/"+baselinePath] = tc.baseline
			}
			f.runner.outcomes[lintGate] = domain.Outcome{Result: tc.exit, Duration: time.Second, Stdout: tc.stdout, TimedOut: tc.timedOut}

			result, err := f.handler.Handle(context.Background(), command(g))

			if err != nil {
				t.Fatal(err)
			}
			if result.Lines[0].Status != tc.wantStatus || result.Failed != (tc.wantStatus == domain.StatusFail) {
				t.Fatalf("result = %+v, want %s", result, tc.wantStatus)
			}
			if saved := f.evidence.saved; len(saved) != 1 || saved[0].Result != statusResult[tc.wantStatus] {
				t.Fatalf("saved = %+v, want one %s record", saved, tc.wantStatus)
			}
		})
	}
}

func TestFailedGateWithNoReport(t *testing.T) {
	cases := []struct {
		name       string
		exit       domain.Result
		wantReport bool
	}{
		{"a failed gate with an empty stdout keeps its tail only", domain.ResultFail, false},
		{"a passing gate with an empty stdout has no report", domain.ResultPass, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(nil)
			g := gate(lintGate)
			g.Baseline = baselinePath
			f.runner.outcomes[lintGate] = domain.Outcome{Result: tc.exit, Duration: time.Second, Stdout: " \n", Tail: "tool crashed"}

			result, err := f.handler.Handle(context.Background(), command(g))

			if err != nil {
				t.Fatal(err)
			}
			line := result.Lines[0]
			if line.Status != domain.StatusFail || !strings.Contains(line.Tail, "tool crashed") {
				t.Fatalf("line = %+v, want a failure that keeps its tail", line)
			}
			if strings.Contains(line.Tail, "findings report") != tc.wantReport {
				t.Errorf("tail = %q, want findings report present = %v", line.Tail, tc.wantReport)
			}
		})
	}
}

func TestVerifyErrors(t *testing.T) {
	baselined := gate(lintGate)
	baselined.Baseline = baselinePath
	cases := []struct {
		name  string
		setup func(f *fixture)
		hash  fakeHasher
		src   domain.GateSource
		gate  domain.Gate
	}{
		{name: "hasher", hash: fakeHasher{err: errInjected}, gate: gate(testGate)},
		{name: "evidence find", setup: func(f *fixture) { f.evidence.findErr = errInjected }, gate: gate(testGate)},
		{name: "dispatch before", setup: func(f *fixture) { f.dispatcher.errEvent = shareddomain.EventGateBefore }, gate: gate(testGate)},
		{name: "dispatch after", setup: func(f *fixture) { f.dispatcher.errEvent = shareddomain.EventGateAfter }, gate: gate(testGate)},
		{name: "runner", setup: func(f *fixture) { f.runner.err = errInjected }, gate: gate(testGate)},
		{name: "evidence save", setup: func(f *fixture) { f.evidence.saveErr = errInjected }, gate: gate(testGate)},
		{name: "criteria source", src: fakeSource{err: errInjected}, gate: gate(testGate)},
		{name: "baseline read", setup: func(f *fixture) {
			f.readErr = errInjected
			f.runner.outcomes[lintGate] = domain.Outcome{Stdout: `{"version":1}`}
		}, gate: baselined},
		{name: "baseline parse", setup: func(f *fixture) {
			f.files[workDir+"/"+baselinePath] = "not json"
			f.runner.outcomes[lintGate] = domain.Outcome{Stdout: `{"version":1}`}
		}, gate: baselined},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hasher := tc.hash
			if hasher.hash == "" {
				hasher.hash = inputHash
			}
			f := newFixtureWith(tc.src, hasher, nil)
			if tc.setup != nil {
				tc.setup(f)
			}

			_, err := f.handler.Handle(context.Background(), command(tc.gate))

			if err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

const (
	fmtGate       = "fmt"
	typecheckGate = "typecheck"
	e2eGate       = "e2e"
	docsGate      = "docs"
	startEvent    = "start:"
	endEvent      = "end:"
)

type step struct {
	outcome domain.Outcome
	err     error
}

var passStep = step{outcome: passOutcome()}

type gatedRunner struct {
	started   chan string
	cancelled chan string
	release   map[string]chan step
	events    idLog
	mu        sync.Mutex
	inFlight  int
	peak      int
}

func newGatedRunner(ids ...string) *gatedRunner {
	g := &gatedRunner{started: make(chan string), cancelled: make(chan string), release: map[string]chan step{}}
	for _, id := range ids {
		g.release[id] = make(chan step)
	}
	return g
}

func (g *gatedRunner) Run(ctx context.Context, _ string, gate domain.Gate) (domain.Outcome, error) {
	g.mu.Lock()
	g.inFlight++
	g.peak = max(g.peak, g.inFlight)
	g.mu.Unlock()
	g.events.add(startEvent + gate.ID)
	g.started <- gate.ID
	var s step
	select {
	case s = <-g.release[gate.ID]:
	case <-ctx.Done():
		g.cancelled <- gate.ID
		s = <-g.release[gate.ID]
	}
	g.events.add(endEvent + gate.ID)
	g.mu.Lock()
	g.inFlight--
	g.mu.Unlock()
	return s.outcome, s.err
}

func (g *gatedRunner) peakInFlight() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.peak
}

type handled struct {
	result verify.Result
	err    error
}

func handleAsync(f *fixture, cmd verify.Command) <-chan handled {
	done := make(chan handled, 1)
	go func() {
		result, err := f.handler.Handle(context.Background(), cmd)
		done <- handled{result: result, err: err}
	}()
	return done
}

func receive(ch <-chan string, n int) []string {
	got := make([]string, n)
	for i := range got {
		got[i] = <-ch
	}
	slices.Sort(got)
	return got
}

func needing(id string, needs ...string) domain.Gate {
	g := gate(id)
	g.Needs = needs
	return g
}

func parallel(n int, gates ...domain.Gate) verify.Command {
	cmd := command(gates...)
	cmd.Parallel = n
	return cmd
}

func gatesOf(f *fixture) (ran, saved, dispatched []string) {
	for _, r := range f.runner.ran {
		ran = append(ran, r.gate)
	}
	for _, e := range f.evidence.saved {
		saved = append(saved, e.GateID)
	}
	for _, e := range f.dispatcher.envelopes {
		var payload verify.GateBeforePayload
		_ = json.Unmarshal(e.Payload, &payload)
		dispatched = append(dispatched, payload.Gate)
	}
	return ran, saved, dispatched
}

func TestHandleSchedules(t *testing.T) {
	t.Run("parallel up to the limit, then the needs", func(t *testing.T) {
		runner := newGatedRunner(fmtGate, lintGate, typecheckGate, testGate)
		f := newFixtureWith(nil, fakeHasher{hash: inputHash}, runner)

		done := handleAsync(f, parallel(2, gate(fmtGate), gate(lintGate), gate(typecheckGate), needing(testGate, fmtGate, lintGate, typecheckGate)))

		if got := receive(runner.started, 2); !slices.Equal(got, []string{fmtGate, lintGate}) {
			t.Fatalf("first starts = %q, want fmt and lint together", got)
		}
		runner.release[fmtGate] <- passStep
		if got := receive(runner.started, 1); got[0] != typecheckGate {
			t.Fatalf("start after fmt = %q, want typecheck", got)
		}
		runner.release[lintGate] <- passStep
		runner.release[typecheckGate] <- passStep
		if got := receive(runner.started, 1); got[0] != testGate {
			t.Fatalf("start after the needs = %q, want %s", got, testGate)
		}
		runner.release[testGate] <- passStep
		h := <-done

		if h.err != nil || h.result.Failed {
			t.Fatalf("result = %+v, err = %v", h.result, h.err)
		}
		if peak := runner.peakInFlight(); peak != 2 {
			t.Errorf("peak in flight = %d, want 2", peak)
		}
		events := runner.events.list()
		testStart := slices.Index(events, startEvent+testGate)
		for _, need := range []string{fmtGate, lintGate, typecheckGate} {
			if end := slices.Index(events, endEvent+need); end < 0 || end > testStart {
				t.Errorf("events = %q, want %s to end before %s starts", events, need, testGate)
			}
		}
	})

	t.Run("no parallel key and no needs keep the gates order", func(t *testing.T) {
		ids := []string{lintGate, fmtGate, testGate}
		runner := newGatedRunner(ids...)
		f := newFixtureWith(nil, fakeHasher{hash: inputHash}, runner)

		done := handleAsync(f, command(gate(lintGate), gate(fmtGate), gate(testGate)))

		for _, id := range ids {
			if got := <-runner.started; got != id {
				t.Fatalf("start = %s, want %s", got, id)
			}
			runner.release[id] <- passStep
		}
		if h := <-done; h.err != nil || runner.peakInFlight() != 1 {
			t.Fatalf("err = %v, peak = %d, want one gate at a time", h.err, runner.peakInFlight())
		}
	})

	t.Run("a cached need lets the gate start", func(t *testing.T) {
		f := newFixture(nil)
		f.evidence.records[recordKey(lintGate)] = domain.Evidence{GateID: lintGate, InputHash: inputHash, Result: domain.ResultPass}
		f.runner.outcomes[testGate] = passOutcome()

		result, err := f.handler.Handle(context.Background(), parallel(2, gate(lintGate), needing(testGate, lintGate)))

		if err != nil || result.Lines[0].Status != domain.StatusCached || result.Lines[1].Status != domain.StatusPass {
			t.Fatalf("result = %+v, err = %v, want CACHED then PASS", result, err)
		}
	})
}

func TestHandleSkips(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *fixture)
	}{
		{name: "failed need", setup: func(f *fixture) { f.runner.outcomes[lintGate] = failOutcome() }},
		{name: "vetoed need", setup: func(f *fixture) {
			f.runner.outcomes[lintGate] = passOutcome()
			f.dispatcher.deny[lintGate] = true
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hashed := &idLog{}
			f := newFixtureWith(nil, fakeHasher{hash: inputHash, hashed: hashed}, nil)
			f.runner.outcomes[fmtGate] = passOutcome()
			f.runner.outcomes[docsGate] = passOutcome()
			tc.setup(f)

			result, err := f.handler.Handle(context.Background(), parallel(3,
				gate(fmtGate), gate(lintGate), needing(testGate, fmtGate, lintGate), needing(e2eGate, testGate), gate(docsGate),
			))

			if err != nil {
				t.Fatal(err)
			}
			wantSkips := []verify.Line{
				{Gate: testGate, Status: domain.StatusSkipped, Need: lintGate},
				{Gate: e2eGate, Status: domain.StatusSkipped, Need: testGate},
			}
			if !slices.Equal(result.Lines[2:4], wantSkips) || result.Lines[4].Status != domain.StatusPass || !result.Failed {
				t.Fatalf("lines = %+v, want the skips %+v, docs PASS and failed", result.Lines, wantSkips)
			}
			ran, saved, dispatched := gatesOf(f)
			for _, skipped := range []string{testGate, e2eGate} {
				for name, ids := range map[string][]string{"ran": ran, "saved": saved, "dispatched": dispatched, "hashed": hashed.list()} {
					if slices.Contains(ids, skipped) {
						t.Errorf("%s %q holds the skipped gate %s", name, ids, skipped)
					}
				}
			}
		})
	}
}

func TestHandleKeepsOrder(t *testing.T) {
	t.Run("a gate listed before its need", func(t *testing.T) {
		f := newFixture(nil)
		f.runner.outcomes[testGate] = domain.Outcome{Result: domain.ResultPass, Tail: "test tail"}
		f.runner.outcomes[lintGate] = domain.Outcome{Result: domain.ResultPass, Tail: "lint tail"}

		result, err := f.handler.Handle(context.Background(), parallel(2, needing(testGate, lintGate), gate(lintGate)))

		if err != nil {
			t.Fatal(err)
		}
		want := []verify.Line{
			{Gate: testGate, Status: domain.StatusPass, Tail: "test tail"},
			{Gate: lintGate, Status: domain.StatusPass, Tail: "lint tail"},
		}
		if ran, _, _ := gatesOf(f); !slices.Equal(result.Lines, want) || !slices.Equal(ran, []string{lintGate, testGate}) {
			t.Fatalf("lines = %+v, ran = %q, want %+v after lint then test", result.Lines, ran, want)
		}
	})

	t.Run("the second gate ends first", func(t *testing.T) {
		runner := newGatedRunner(fmtGate, lintGate)
		f := newFixtureWith(nil, fakeHasher{hash: inputHash}, runner)

		done := handleAsync(f, parallel(2, gate(fmtGate), gate(lintGate)))
		receive(runner.started, 2)
		runner.release[lintGate] <- step{outcome: domain.Outcome{Result: domain.ResultPass, Tail: "lint tail"}}
		runner.release[fmtGate] <- step{outcome: domain.Outcome{Result: domain.ResultFail, Tail: "fmt tail"}}
		h := <-done

		want := []verify.Line{
			{Gate: fmtGate, Status: domain.StatusFail, Tail: "fmt tail"},
			{Gate: lintGate, Status: domain.StatusPass, Tail: "lint tail"},
		}
		if h.err != nil || !slices.Equal(h.result.Lines, want) {
			t.Fatalf("lines = %+v, err = %v, want %+v", h.result.Lines, h.err, want)
		}
	})
}

func TestHandleStopsOnError(t *testing.T) {
	runner := newGatedRunner(fmtGate, lintGate, testGate)
	f := newFixtureWith(nil, fakeHasher{hash: inputHash}, runner)

	done := handleAsync(f, parallel(2, gate(fmtGate), gate(lintGate), gate(testGate)))
	receive(runner.started, 2)
	runner.release[fmtGate] <- step{err: errInjected}
	if got := <-runner.cancelled; got != lintGate {
		t.Fatalf("cancelled = %s, want %s", got, lintGate)
	}
	select {
	case h := <-done:
		t.Fatalf("returned %+v before the running gate ended", h)
	default:
	}
	runner.release[lintGate] <- step{outcome: failOutcome()}
	h := <-done

	if !errors.Is(h.err, errInjected) {
		t.Fatalf("err = %v, want the first error", h.err)
	}
	if events := runner.events.list(); slices.Contains(events, startEvent+testGate) {
		t.Errorf("events = %q, want no start after the error", events)
	}
	if _, saved, _ := gatesOf(f); len(saved) != 0 {
		t.Errorf("saved = %q, want no evidence from a cancelled run", saved)
	}
	for _, e := range f.dispatcher.envelopes {
		if e.Event == shareddomain.EventGateAfter {
			t.Errorf("dispatched %s after the cancel", e.Event)
		}
	}
}

func TestHandleSharedSchedulerCriteria(t *testing.T) {
	criterion := domain.CriterionGateID(1)
	runner := newGatedRunner(testGate, criterion)
	f := newFixtureWith(fakeSource{gates: []domain.Gate{gate(criterion)}}, fakeHasher{hash: inputHash}, runner)

	done := handleAsync(f, parallel(2, gate(testGate)))

	want := []string{criterion, testGate}
	slices.Sort(want)
	if got := receive(runner.started, 2); !slices.Equal(got, want) {
		t.Fatalf("starts = %q, want the criterion gate beside the configured gate", got)
	}
	runner.release[testGate] <- passStep
	runner.release[criterion] <- passStep
	if h := <-done; h.err != nil || len(h.result.Lines) != 2 {
		t.Fatalf("result = %+v, err = %v", h.result, h.err)
	}
}

const (
	depsStep   = "deps"
	toolsStep  = "tools"
	vendorFile = "vendor/autoload.php"
)

func setupStep(id, creates string) domain.Setup {
	return domain.Setup{Gate: gate(id), Creates: creates}
}

func withSetup(cmd verify.Command, steps ...domain.Setup) verify.Command {
	cmd.Setup = steps
	return cmd
}

func TestHandleSetup(t *testing.T) {
	t.Run("runs the steps in file order before the gates and records the hash", func(t *testing.T) {
		f := newFixture(nil)
		for _, id := range []string{depsStep, toolsStep, testGate} {
			f.runner.outcomes[id] = passOutcome()
		}

		result, err := f.handler.Handle(context.Background(), withSetup(command(gate(testGate)), setupStep(depsStep, ""), setupStep(toolsStep, "")))

		if err != nil {
			t.Fatal(err)
		}
		if want := []ran{{workDir, depsStep}, {workDir, toolsStep}, {workDir, testGate}}; !slices.Equal(f.runner.ran, want) {
			t.Errorf("ran = %+v, want %+v", f.runner.ran, want)
		}
		if want := []verify.SetupLine{{Step: depsStep, Duration: 2 * time.Second}, {Step: toolsStep, Duration: 2 * time.Second}}; !slices.Equal(result.Setup, want) {
			t.Errorf("setup lines = %+v, want %+v", result.Setup, want)
		}
		if want := []string{depsStep + "@" + inputHash, toolsStep + "@" + inputHash}; !slices.Equal(f.setups.saved, want) {
			t.Errorf("saved setup = %q, want %q", f.setups.saved, want)
		}
		if len(f.evidence.saved) != 1 || len(f.dispatcher.envelopes) != 2 || len(result.Lines) != 1 {
			t.Errorf("evidence = %+v, events = %d, lines = %+v, want only the gate in them", f.evidence.saved, len(f.dispatcher.envelopes), result.Lines)
		}
	})

	cases := map[string]struct {
		last    string
		created bool
		creates string
		runs    bool
	}{
		"skips a step with the same hash and its creates path": {last: inputHash, created: true, creates: vendorFile},
		"skips a step with the same hash and no creates path":  {last: inputHash},
		"runs a step whose creates path is missing":            {last: inputHash, creates: vendorFile, runs: true},
		"runs a step whose hash changed":                       {last: "tree-0", created: true, creates: vendorFile, runs: true},
		"runs a step with no pass":                             {runs: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(nil)
			f.runner.outcomes[depsStep] = passOutcome()
			f.runner.outcomes[testGate] = passOutcome()
			if c.last != "" {
				f.setups.last[depsStep] = c.last
			}
			f.created[filepath.Join(workDir, vendorFile)] = c.created

			result, err := f.handler.Handle(context.Background(), withSetup(command(gate(testGate)), setupStep(depsStep, c.creates)))

			if err != nil {
				t.Fatal(err)
			}
			if ran := slices.Contains(f.runner.ran, ran{workDir, depsStep}); ran != c.runs || len(result.Setup) == 0 == c.runs {
				t.Errorf("step ran = %v, setup lines = %+v, want ran %v", ran, result.Setup, c.runs)
			}
		})
	}
}

func TestHandleSetupFails(t *testing.T) {
	f := newFixture(nil)
	f.runner.outcomes[depsStep] = failOutcome()
	f.runner.outcomes[testGate] = passOutcome()

	_, err := f.handler.Handle(context.Background(), withSetup(command(gate(testGate)), setupStep(depsStep, ""), setupStep(toolsStep, "")))

	if !errors.Is(err, domain.ErrSetupFailed) || !strings.Contains(err.Error(), depsStep) || !strings.Contains(err.Error(), failTail) {
		t.Fatalf("err = %v, want a setup failure that names %s and holds its tail", err, depsStep)
	}
	if want := []ran{{workDir, depsStep}}; !slices.Equal(f.runner.ran, want) {
		t.Errorf("ran = %+v, want only the failed step", f.runner.ran)
	}
	if len(f.setups.saved) != 0 || len(f.evidence.saved) != 0 || len(f.dispatcher.envelopes) != 0 {
		t.Errorf("setup saved = %q, evidence = %+v, events = %d, want nothing", f.setups.saved, f.evidence.saved, len(f.dispatcher.envelopes))
	}
}

func spanAttrs(span sdktrace.ReadOnlySpan) map[string]string {
	attrs := map[string]string{}
	for _, a := range span.Attributes() {
		attrs[string(a.Key)] = a.Value.Emit()
	}
	return attrs
}

func TestVerifyTrace(t *testing.T) {
	f := newFixture(nil)
	f.runner.outcomes[depsStep] = passOutcome()
	f.runner.outcomes[testGate] = passOutcome()
	f.runner.outcomes[lintGate] = passOutcome()

	_, err := f.handler.Handle(context.Background(), withSetup(command(gate(testGate), gate(lintGate)), setupStep(depsStep, "")))

	if err != nil {
		t.Fatal(err)
	}
	var roots []sdktrace.ReadOnlySpan
	names := map[string]int{}
	for _, span := range f.spans.Ended() {
		names[span.Name()]++
		if !span.Parent().IsValid() {
			roots = append(roots, span)
		}
	}
	if len(roots) != 1 || roots[0].Name() != spanVerify {
		t.Fatalf("roots = %d, want one %s", len(roots), spanVerify)
	}
	root := roots[0]
	for _, span := range f.spans.Ended() {
		if span == root {
			continue
		}
		if span.SpanContext().TraceID() != root.SpanContext().TraceID() || span.Parent().SpanID() != root.SpanContext().SpanID() {
			t.Errorf("span %s is not a direct child of %s", span.Name(), spanVerify)
		}
	}
	if names[spanSetup] != 1 || names[spanName] != 2 || names[spanVerify] != 1 {
		t.Fatalf("span names = %v, want one setup, two runs, one verify", names)
	}
}

func TestVerifyRootAttributes(t *testing.T) {
	cases := []struct {
		name       string
		outcome    domain.Outcome
		wantStatus codes.Code
	}{
		{"a passing call", passOutcome(), codes.Unset},
		{"a failing gate", failOutcome(), codes.Error},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(nil)
			f.runner.outcomes[testGate] = tc.outcome
			cmd := command(gate(testGate), gate(lintGate))
			f.runner.outcomes[lintGate] = tc.outcome

			if _, err := f.handler.Handle(context.Background(), cmd); err != nil {
				t.Fatal(err)
			}

			var root sdktrace.ReadOnlySpan
			for _, span := range f.spans.Ended() {
				if span.Name() == spanVerify {
					root = span
				}
			}
			if root == nil {
				t.Fatalf("no %s span", spanVerify)
			}
			attrs := spanAttrs(root)
			if id, has := attrs[attrRunID]; has {
				t.Errorf("%s = %q, want none: the span processor sets it", attrRunID, id)
			}
			if attrs[attrCount] != "2" || attrs[attrWorktree] != workDir {
				t.Errorf("attributes = %v, want count 2 and worktree %s", attrs, workDir)
			}
			if root.Status().Code != tc.wantStatus {
				t.Errorf("status = %v, want %v", root.Status().Code, tc.wantStatus)
			}
		})
	}
}

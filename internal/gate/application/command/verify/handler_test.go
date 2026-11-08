package verify_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"slices"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
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
	baselinePath  = ".harness-baseline/lint.json"
	spanName      = "gate.run"
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

type fakeHasher struct {
	hash string
	err  error
}

func (f fakeHasher) InputHash(context.Context, string, domain.Gate) (string, error) {
	return f.hash, f.err
}

type ran struct{ dir, gate string }

type fakeRunner struct {
	outcomes map[string]domain.Outcome
	err      error
	ran      []ran
}

func (f *fakeRunner) Run(_ context.Context, dir string, gate domain.Gate) (domain.Outcome, error) {
	f.ran = append(f.ran, ran{dir: dir, gate: gate.ID})
	if f.err != nil {
		return domain.Outcome{}, f.err
	}
	return f.outcomes[gate.ID], nil
}

type fakeEvidence struct {
	records map[string]domain.Evidence
	saved   []domain.Evidence
	findErr error
	saveErr error
}

func recordKey(gate string) string { return gate + "@" + inputHash }

func (f *fakeEvidence) Find(_ context.Context, _, gateID, hash string) (domain.Evidence, bool, error) {
	e, ok := f.records[gateID+"@"+hash]
	return e, ok, f.findErr
}

func (f *fakeEvidence) Save(_ context.Context, e domain.Evidence) error {
	f.saved = append(f.saved, e)
	return f.saveErr
}

type fakeDispatcher struct {
	deny      map[string]bool
	errEvent  shareddomain.EventName
	envelopes []shareddomain.Envelope
}

func (f *fakeDispatcher) Dispatch(_ context.Context, e shareddomain.Envelope) (shareddomain.DispatchResult, error) {
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
	handler    verify.Handler
	readErr    error
}

func newFixture(source domain.GateSource) *fixture {
	return newFixtureWith(source, fakeHasher{hash: inputHash})
}

func newFixtureWith(source domain.GateSource, hasher fakeHasher) *fixture {
	f := &fixture{
		runner:     &fakeRunner{outcomes: map[string]domain.Outcome{}},
		evidence:   &fakeEvidence{records: map[string]domain.Evidence{}},
		dispatcher: &fakeDispatcher{deny: map[string]bool{}},
		spans:      tracetest.NewSpanRecorder(),
		reader:     sdkmetric.NewManualReader(),
		files:      map[string]string{},
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
	f.handler = verify.NewHandler(
		hasher, f.runner, f.evidence, f.dispatcher, source,
		readFile, tracer, meter, func() time.Time { return fixedNow },
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

	spans := f.spans.Ended()
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
			f := newFixtureWith(tc.src, hasher)
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

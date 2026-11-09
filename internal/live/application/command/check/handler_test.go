package check_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/live/application/command/check"
	"github.com/90p-labs/harness/internal/live/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	repo         = "/work/repo"
	environment  = "production"
	rollback     = "kubectl rollout undo deploy/api"
	prURL        = "https://github.com/acme/api/pull/7"
	otherPRURL   = "https://github.com/acme/api/pull/8"
	mergeSHA     = "abc1234"
	spanName     = "live.check"
	metricName   = "harness.live.checks"
	attrRunID    = "harness.run.id"
	attrWorktree = "harness.worktree"
	attrPhase    = "harness.phase"
	attrOutcome  = "harness.live.outcome"
)

var (
	now        = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	mergedAt   = time.Date(2026, 11, 8, 9, 50, 0, 0, time.UTC)
	deployedAt = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

	errGitHub = errors.New("gh failed")
	errBus    = errors.New("bus failed")
)

var (
	errorRate = domain.Threshold{
		Signal: "error rate", Source: "prometheus", Query: "api:http_errors:ratio_rate5m",
		Breach: shareddomain.Breach{Op: shareddomain.OperatorGreater, Value: 0.01}, Window: 30 * time.Minute,
	}
	latency = domain.Threshold{
		Signal: "latency", Source: "grafana", Query: "api:latency:p99",
		Breach: shareddomain.Breach{Op: shareddomain.OperatorGreater, Value: 250}, Window: time.Hour,
	}
)

const (
	errorRatePayload = `{"signal":"error rate","source":"prometheus","query":"api:http_errors:ratio_rate5m","breach":{"op":">","value":0.01},"window":{"from":"2026-11-08T10:00:00Z","to":"2026-11-08T10:30:00Z"},"sha":"abc1234","environment":"production"}`
	latencyPayload   = `{"signal":"latency","source":"grafana","query":"api:latency:p99","breach":{"op":">","value":250},"window":{"from":"2026-11-08T10:00:00Z","to":"2026-11-08T11:00:00Z"},"sha":"abc1234","environment":"production"}`
)

type applyCall struct {
	RunID  string
	Event  domain.Event
	Reason string
}

type archiveCall struct {
	RunID string
	Round int
}

type fakeRuns struct {
	runs       []domain.LiveRun
	liveErr    error
	signals    []domain.Threshold
	signalsErr error
	applyErr   error
	applied    []applyCall
}

func (f *fakeRuns) Live(context.Context, string) ([]domain.LiveRun, error) {
	return f.runs, f.liveErr
}

func (f *fakeRuns) Apply(_ context.Context, run domain.LiveRun, event domain.Event, reason string) error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.applied = append(f.applied, applyCall{RunID: run.ID, Event: event, Reason: reason})
	return nil
}

func (f *fakeRuns) Signals(context.Context, domain.LiveRun) ([]domain.Threshold, error) {
	return f.signals, f.signalsErr
}

type fakeReleases struct {
	stored  map[string]domain.Release
	saved   []domain.Release
	getErr  error
	saveErr error
}

func (f *fakeReleases) Get(_ context.Context, runID string) (domain.Release, error) {
	if f.getErr != nil {
		return domain.Release{}, f.getErr
	}
	release, ok := f.stored[runID]
	if !ok {
		return domain.Release{}, domain.ErrReleaseNotFound
	}
	return release, nil
}

func (f *fakeReleases) Save(_ context.Context, release domain.Release) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, release)
	return nil
}

type fakeGitHub struct {
	t          *testing.T
	forbid     bool
	merge      domain.Merge
	mergeErr   map[string]error
	deployment domain.Deployment
	deployErr  error
	deployArgs []string
}

func (f *fakeGitHub) Merge(_ context.Context, url string) (domain.Merge, error) {
	if f.forbid {
		f.t.Fatal("github called with no live run")
	}
	return f.merge, f.mergeErr[url]
}

func (f *fakeGitHub) Deployment(_ context.Context, url, sha, env string) (domain.Deployment, error) {
	f.deployArgs = []string{url, sha, env}
	return f.deployment, f.deployErr
}

type fakeFolder struct {
	archived []archiveCall
	err      error
}

func (f *fakeFolder) Archive(run domain.LiveRun, round int) error {
	if f.err != nil {
		return f.err
	}
	f.archived = append(f.archived, archiveCall{RunID: run.ID, Round: round})
	return nil
}

type fakeBus struct {
	results   []shareddomain.DispatchResult
	err       error
	envelopes []shareddomain.Envelope
}

func (f *fakeBus) Dispatch(_ context.Context, envelope shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	f.envelopes = append(f.envelopes, envelope)
	if f.err != nil {
		return shareddomain.DispatchResult{}, f.err
	}
	if len(f.results) == 0 {
		return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}}, nil
	}
	result := f.results[0]
	f.results = f.results[1:]
	return result, nil
}

type fixture struct {
	runs     *fakeRuns
	releases *fakeReleases
	github   *fakeGitHub
	folder   *fakeFolder
	bus      *fakeBus
	spans    *tracetest.SpanRecorder
	reader   *sdkmetric.ManualReader
	handler  check.Handler
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		runs:     &fakeRuns{},
		releases: &fakeReleases{stored: map[string]domain.Release{}},
		github:   &fakeGitHub{t: t, mergeErr: map[string]error{}},
		folder:   &fakeFolder{},
		bus:      &fakeBus{},
		spans:    tracetest.NewSpanRecorder(),
		reader:   sdkmetric.NewManualReader(),
	}
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(f.spans)).Tracer("test")
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(f.reader)).Meter("test")
	f.handler = check.NewHandler(f.runs, f.releases, f.github, f.folder, f.bus, environment, rollback, tracer, meter)
	return f
}

func (f *fixture) handle(t *testing.T) ([]check.Result, error) {
	t.Helper()
	return f.handler.Handle(context.Background(), check.Command{Repo: repo, Now: now})
}

func observing(id string, round int) domain.LiveRun {
	return domain.LiveRun{ID: id, Worktree: "/work/" + id, Phase: domain.PhaseObserving, LiveRound: round, PRURL: prURL, Version: 4}
}

func releasing(id string) domain.LiveRun {
	return domain.LiveRun{ID: id, Worktree: "/work/" + id, Phase: domain.PhaseReleasing, PRURL: prURL, Version: 2}
}

func deployedRelease(id string) domain.Release {
	return domain.Release{
		RunID: id, PRURL: prURL, MergeSHA: mergeSHA, MergedAt: mergedAt, DeployedAt: deployedAt, Environment: environment,
	}
}

func deny(reason string) shareddomain.DispatchResult {
	return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: reason}}
}

func TestLiveCheckDispatch(t *testing.T) {
	t.Run("observing deny regresses and archives", func(t *testing.T) {
		f := newFixture(t)
		f.runs.runs = []domain.LiveRun{observing("run-a", 2)}
		f.runs.signals = []domain.Threshold{errorRate, latency}
		f.releases.stored["run-a"] = deployedRelease("run-a")
		f.bus.results = []shareddomain.DispatchResult{
			{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}},
			deny("grafana: latency above 250\n\ngrafana: p99 spike"),
		}

		results, err := f.handle(t)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 1 || results[0].RunID != "run-a" || results[0].Phase != domain.PhaseObserving || results[0].Outcome != domain.OutcomeRegression {
			t.Fatalf("results = %+v", results)
		}
		if len(f.bus.envelopes) != 2 {
			t.Fatalf("envelopes = %d, want 2", len(f.bus.envelopes))
		}
		first := f.bus.envelopes[0]
		wantRun := shareddomain.RunRef{ID: "run-a", Phase: string(domain.PhaseObserving)}
		if first.Event != shareddomain.EventSignalsCheck || first.Version != shareddomain.EnvelopeVersion || first.Cwd != "/work/run-a" || first.Run != wantRun {
			t.Fatalf("envelope = %+v", first)
		}
		if string(first.Payload) != errorRatePayload || string(f.bus.envelopes[1].Payload) != latencyPayload {
			t.Fatalf("payloads = %s and %s", first.Payload, f.bus.envelopes[1].Payload)
		}
		if len(f.runs.applied) != 1 || f.runs.applied[0].Event != domain.EventRegress || f.runs.applied[0].RunID != "run-a" {
			t.Fatalf("applied = %+v", f.runs.applied)
		}
		text := f.runs.applied[0].Reason
		for _, want := range []string{
			"live round 3 of run run-a", "- grafana: latency above 250, window 1h", "- grafana: p99 spike, window 1h", "Rollback: " + rollback,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("findings missing %q:\n%s", want, text)
			}
		}
		if len(f.folder.archived) != 1 || f.folder.archived[0] != (archiveCall{RunID: "run-a", Round: 3}) {
			t.Fatalf("archived = %+v", f.folder.archived)
		}
	})

	t.Run("releasing deployed saves the deploy time and applies deploy", func(t *testing.T) {
		f := newFixture(t)
		f.runs.runs = []domain.LiveRun{releasing("run-a")}
		f.github.merge = domain.Merge{State: domain.MergeMerged, SHA: mergeSHA, At: mergedAt}
		f.github.deployment = domain.Deployment{State: domain.DeploySuccess, At: deployedAt}

		results, err := f.handle(t)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 1 || results[0].Outcome != domain.OutcomeDeployed {
			t.Fatalf("results = %+v", results)
		}
		if len(f.releases.saved) != 1 || f.releases.saved[0] != deployedRelease("run-a") {
			t.Fatalf("saved = %+v", f.releases.saved)
		}
		if len(f.runs.applied) != 1 || f.runs.applied[0] != (applyCall{RunID: "run-a", Event: domain.EventDeploy}) {
			t.Fatalf("applied = %+v", f.runs.applied)
		}
		if strings.Join(f.github.deployArgs, " ") != strings.Join([]string{prURL, mergeSHA, environment}, " ") {
			t.Fatalf("deployment args = %v", f.github.deployArgs)
		}
	})
}

func TestLiveCheckOutcomes(t *testing.T) {
	tests := []struct {
		name        string
		run         domain.LiveRun
		stored      map[string]domain.Release
		merge       domain.Merge
		deployment  domain.Deployment
		bus         []shareddomain.DispatchResult
		wantOutcome domain.Outcome
		wantApplied []applyCall
		wantSaved   int
		wantArchive int
	}{
		{
			name: "open PR is pending", run: releasing("run-a"),
			merge: domain.Merge{State: domain.MergeOpen}, wantOutcome: domain.OutcomePending,
		},
		{
			name: "merged PR with a pending deploy saves the merge", run: releasing("run-a"),
			merge:       domain.Merge{State: domain.MergeMerged, SHA: mergeSHA, At: mergedAt},
			deployment:  domain.Deployment{State: domain.DeployPending},
			wantOutcome: domain.OutcomePending, wantSaved: 1,
		},
		{
			name: "closed PR regresses", run: releasing("run-a"),
			merge:       domain.Merge{State: domain.MergeClosed},
			wantOutcome: domain.OutcomeRegression, wantApplied: []applyCall{{RunID: "run-a", Event: domain.EventRegress}}, wantArchive: 1,
		},
		{
			name: "observing past the window with no breach closes", run: observing("run-a", 0),
			stored:      map[string]domain.Release{"run-a": deployedRelease("run-a")},
			wantOutcome: domain.OutcomeHealthy, wantApplied: []applyCall{{RunID: "run-a", Event: domain.EventClose}},
		},
		{
			name: "plugin failures past two windows block", run: observing("run-a", 0),
			stored: map[string]domain.Release{"run-a": deployedRelease("run-a")},
			bus: []shareddomain.DispatchResult{
				{Failures: []shareddomain.DispatchFailure{{Plugin: "prometheus", Err: errBus}}},
			},
			wantOutcome: domain.OutcomeUnavailable,
			wantApplied: []applyCall{{RunID: "run-a", Event: domain.EventBlock, Reason: domain.ReasonSignalsUnavailable}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.runs.runs = []domain.LiveRun{tt.run}
			f.runs.signals = []domain.Threshold{errorRate}
			for id, release := range tt.stored {
				f.releases.stored[id] = release
			}
			f.github.merge, f.github.deployment, f.bus.results = tt.merge, tt.deployment, tt.bus

			results, err := f.handle(t)
			if err != nil {
				t.Fatal(err)
			}

			if results[0].Outcome != tt.wantOutcome {
				t.Fatalf("outcome = %s, want %s", results[0].Outcome, tt.wantOutcome)
			}
			if len(f.runs.applied) != len(tt.wantApplied) {
				t.Fatalf("applied = %+v, want %+v", f.runs.applied, tt.wantApplied)
			}
			for i, want := range tt.wantApplied {
				got := f.runs.applied[i]
				if got.RunID != want.RunID || got.Event != want.Event || (want.Reason != "" && got.Reason != want.Reason) {
					t.Fatalf("applied[%d] = %+v, want %+v", i, got, want)
				}
			}
			if len(f.releases.saved) != tt.wantSaved || len(f.folder.archived) != tt.wantArchive {
				t.Fatalf("saved = %d, archived = %d", len(f.releases.saved), len(f.folder.archived))
			}
		})
	}
}

func TestLiveCheckStartsNewReleaseForNewPR(t *testing.T) {
	f := newFixture(t)
	f.runs.runs = []domain.LiveRun{releasing("run-a")}
	f.releases.stored["run-a"] = domain.Release{RunID: "run-a", PRURL: otherPRURL, MergedAt: mergedAt, DeployedAt: deployedAt}
	f.github.merge = domain.Merge{State: domain.MergeMerged, SHA: mergeSHA, At: mergedAt}
	f.github.deployment = domain.Deployment{State: domain.DeployPending}

	if _, err := f.handle(t); err != nil {
		t.Fatal(err)
	}

	want := domain.Release{RunID: "run-a", PRURL: prURL, MergeSHA: mergeSHA, MergedAt: mergedAt, Environment: environment}
	if len(f.releases.saved) != 1 || f.releases.saved[0] != want {
		t.Fatalf("saved = %+v, want %+v", f.releases.saved, want)
	}
}

func TestLiveCheckStaleRun(t *testing.T) {
	f := newFixture(t)
	f.runs.runs = []domain.LiveRun{observing("run-a", 0)}
	f.runs.signals = []domain.Threshold{errorRate}
	f.releases.stored["run-a"] = deployedRelease("run-a")
	f.bus.results = []shareddomain.DispatchResult{deny("prometheus: error rate above")}
	f.runs.applyErr = domain.ErrStaleRun

	results, err := f.handle(t)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 || results[0].Outcome != domain.OutcomeSkipped || results[0].Err != nil {
		t.Fatalf("results = %+v", results)
	}
	if len(f.folder.archived) != 0 {
		t.Fatalf("archived = %+v, want none", f.folder.archived)
	}
}

func TestLiveCheckIsolatesFailures(t *testing.T) {
	f := newFixture(t)
	first, second := releasing("run-a"), releasing("run-b")
	second.PRURL = otherPRURL
	f.runs.runs = []domain.LiveRun{first, second}
	f.github.mergeErr[prURL] = errGitHub
	f.github.merge = domain.Merge{State: domain.MergeOpen}

	results, err := f.handle(t)

	if !errors.Is(err, errGitHub) || !strings.Contains(err.Error(), "run-a") {
		t.Fatalf("err = %v, want it to name run-a and wrap the gh error", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Outcome != domain.OutcomeError || !errors.Is(results[0].Err, errGitHub) {
		t.Fatalf("first = %+v", results[0])
	}
	if results[1].RunID != "run-b" || results[1].Outcome != domain.OutcomePending || results[1].Err != nil {
		t.Fatalf("second = %+v", results[1])
	}
}

func TestLiveCheckSignalsUnreadable(t *testing.T) {
	f := newFixture(t)
	f.runs.runs = []domain.LiveRun{observing("run-a", 0)}
	f.runs.signalsErr = errors.New("open plan.md: no such file")
	f.releases.stored["run-a"] = deployedRelease("run-a")

	results, err := f.handle(t)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 || results[0].Outcome != domain.OutcomeUnavailable || results[0].Err != nil {
		t.Fatalf("results = %+v", results)
	}
	want := []applyCall{{RunID: "run-a", Event: domain.EventBlock, Reason: "signals unavailable: open plan.md: no such file"}}
	if !reflect.DeepEqual(f.runs.applied, want) {
		t.Fatalf("applied = %+v, want %+v", f.runs.applied, want)
	}
}

func TestLiveCheckRunErrors(t *testing.T) {
	t.Run("observing without a release", func(t *testing.T) {
		f := newFixture(t)
		f.runs.runs = []domain.LiveRun{observing("run-a", 0)}

		results, err := f.handle(t)

		if !errors.Is(err, domain.ErrReleaseNotFound) || results[0].Outcome != domain.OutcomeError {
			t.Fatalf("results = %+v, err = %v", results, err)
		}
	})
	t.Run("dispatch error", func(t *testing.T) {
		f := newFixture(t)
		f.runs.runs = []domain.LiveRun{observing("run-a", 0)}
		f.runs.signals = []domain.Threshold{errorRate}
		f.releases.stored["run-a"] = deployedRelease("run-a")
		f.bus.err = errBus

		results, err := f.handle(t)

		if !errors.Is(err, errBus) || results[0].Outcome != domain.OutcomeError {
			t.Fatalf("results = %+v, err = %v", results, err)
		}
	})
	t.Run("archive error", func(t *testing.T) {
		f := newFixture(t)
		f.runs.runs = []domain.LiveRun{observing("run-a", 0)}
		f.runs.signals = []domain.Threshold{errorRate}
		f.releases.stored["run-a"] = deployedRelease("run-a")
		f.bus.results = []shareddomain.DispatchResult{deny("prometheus: error rate above")}
		f.folder.err = domain.ErrArchiveExists

		results, err := f.handle(t)

		if !errors.Is(err, domain.ErrArchiveExists) || results[0].Outcome != domain.OutcomeError {
			t.Fatalf("results = %+v, err = %v", results, err)
		}
	})
	t.Run("store, signals and deployment errors", func(t *testing.T) {
		errStore := errors.New("store failed")
		tests := []struct {
			name  string
			run   domain.LiveRun
			setup func(f *fixture)
		}{
			{"release read", releasing("run-a"), func(f *fixture) { f.releases.getErr = errStore }},
			{"deployment read", releasing("run-a"), func(f *fixture) {
				f.github.merge = domain.Merge{State: domain.MergeMerged, SHA: mergeSHA}
				f.github.deployErr = errStore
			}},
			{"release save", releasing("run-a"), func(f *fixture) {
				f.github.merge = domain.Merge{State: domain.MergeMerged, SHA: mergeSHA}
				f.releases.saveErr = errStore
			}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				f := newFixture(t)
				f.runs.runs = []domain.LiveRun{tt.run}
				tt.setup(f)

				results, err := f.handle(t)

				if !errors.Is(err, errStore) || results[0].Outcome != domain.OutcomeError {
					t.Fatalf("results = %+v, err = %v", results, err)
				}
			})
		}
	})
	t.Run("runs query error stops the pass", func(t *testing.T) {
		f := newFixture(t)
		f.runs.liveErr = errGitHub

		results, err := f.handle(t)

		if !errors.Is(err, errGitHub) || len(results) != 0 {
			t.Fatalf("results = %+v, err = %v", results, err)
		}
	})
}

func TestLiveCheckTelemetry(t *testing.T) {
	f := newFixture(t)
	failing := releasing("run-b")
	failing.PRURL = otherPRURL
	f.runs.runs = []domain.LiveRun{observing("run-a", 0), failing}
	f.releases.stored["run-a"] = deployedRelease("run-a")
	f.github.mergeErr[otherPRURL] = errGitHub

	if _, err := f.handle(t); err == nil {
		t.Fatal("want the run error")
	}

	spans := f.spans.Ended()
	if len(spans) != 2 {
		t.Fatalf("spans = %d, want 2", len(spans))
	}
	wantOutcome := map[string]string{"run-a": string(domain.OutcomeHealthy), "run-b": string(domain.OutcomeError)}
	wantPhase := map[string]string{"run-a": string(domain.PhaseObserving), "run-b": string(domain.PhaseReleasing)}
	for _, span := range spans {
		attrs := map[string]string{}
		for _, a := range span.Attributes() {
			attrs[string(a.Key)] = a.Value.Emit()
		}
		id := attrs[attrRunID]
		if span.Name() != spanName || attrs[attrOutcome] != wantOutcome[id] || attrs[attrPhase] != wantPhase[id] {
			t.Fatalf("span %s attributes = %v", span.Name(), attrs)
		}
		if hasErrorEvent := len(span.Events()) > 0; hasErrorEvent != (id == "run-b") {
			t.Fatalf("span %s events = %v", id, span.Events())
		}
	}

	var collected metricdata.ResourceMetrics
	if err := f.reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metricName {
				continue
			}
			for _, point := range m.Data.(metricdata.Sum[int64]).DataPoints {
				value, _ := point.Attributes.Value(attribute.Key(attrOutcome))
				counts[value.AsString()] += point.Value
			}
		}
	}
	if len(counts) != 2 || counts[string(domain.OutcomeHealthy)] != 1 || counts[string(domain.OutcomeError)] != 1 {
		t.Fatalf("counts = %v", counts)
	}
}

func TestCheckSpanNamesTheRun(t *testing.T) {
	f := newFixture(t)
	runs := []domain.LiveRun{observing("run-a", 0), releasing("run-b")}
	f.runs.runs = runs
	f.releases.stored["run-a"] = deployedRelease("run-a")
	f.releases.stored["run-b"] = deployedRelease("run-b")

	if _, err := f.handle(t); err != nil {
		t.Fatal(err)
	}

	spans := f.spans.Ended()
	if len(spans) != len(runs) {
		t.Fatalf("spans = %d, want %d", len(spans), len(runs))
	}
	for i, span := range spans {
		attrs := map[string]string{}
		for _, a := range span.Attributes() {
			attrs[string(a.Key)] = a.Value.Emit()
		}
		if attrs[attrRunID] != runs[i].ID || attrs[attrWorktree] != runs[i].Worktree {
			t.Errorf("span %d attributes = %v, want run %s in %s", i, attrs, runs[i].ID, runs[i].Worktree)
		}
	}
}

func TestLiveCheckNoRuns(t *testing.T) {
	f := newFixture(t)
	f.github.forbid = true

	results, err := f.handle(t)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 0 || len(f.bus.envelopes) != 0 || len(f.spans.Ended()) != 0 {
		t.Fatalf("results = %+v, envelopes = %d", results, len(f.bus.envelopes))
	}
}

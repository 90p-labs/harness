package lint_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jorge07/harness/internal/lint/application/command/lint"
	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	binaryVersion  = "v1.2.0"
	spanRun        = "lint.run"
	spanEngine     = "lint.engine"
	eventFindings  = "lint.findings"
	attrEngine     = "harness.lint.engine"
	attrRules      = "harness.lint.rules"
	attrFindings   = "harness.lint.findings"
	attrCount      = "harness.lint.count"
	eventException = "exception"
	metricName     = "harness.lint.findings"
	attrRule       = "harness.lint.rule"
	attrSeverity   = "harness.lint.severity"
	lintDir        = ".harness-lint"
	versionFile    = "VERSION"
)

var errEngine = errors.New("engine failed")

type fakeEngine struct {
	name     domain.EngineName
	findings []shareddomain.Finding
	err      error
	got      []domain.Rule
	ran      bool
}

type fakeEngines []*fakeEngine

func (f fakeEngines) Check(_ context.Context, _ string, engine domain.EngineName, rules []domain.Rule) ([]shareddomain.Finding, error) {
	for _, e := range f {
		if e.name == engine {
			e.ran = true
			e.got = rules
			return e.findings, e.err
		}
	}
	return nil, nil
}

func (fakeEngines) Prepare(context.Context, string, domain.EngineName, []domain.Rule) error {
	return nil
}

func rule(id string, engine domain.EngineName) domain.Rule {
	return domain.Rule{ID: id, Engine: engine, Severity: shareddomain.SeverityError}
}

func finding(ruleID, file string, line int, severity shareddomain.Severity) shareddomain.Finding {
	return shareddomain.Finding{Rule: ruleID, Severity: severity, File: file, Line: line, Message: ruleID}
}

func lookup(name domain.PackName) (domain.Pack, error) {
	switch name {
	case domain.PackDDD:
		return domain.Pack{Name: name, Rules: []domain.Rule{
			rule("ddd/layout", domain.EngineHarness),
			rule("ddd/grit", domain.EngineBiomeGrit),
			rule("next/biome", domain.EngineBiomeRule),
		}}, nil
	case domain.PackDDDES:
		return domain.Pack{Name: name, Requires: []domain.PackName{domain.PackDDD}}, nil
	}
	return domain.Pack{}, domain.ErrUnknownPack
}

func repoWithVersion(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, lintDir), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, lintDir, versionFile), []byte(version), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

type fixture struct {
	spans   *tracetest.SpanRecorder
	reader  *sdkmetric.ManualReader
	handler lint.Handler
}

func newFixture(packs []domain.PackName, engines ...*fakeEngine) fixture {
	f := fixture{spans: tracetest.NewSpanRecorder(), reader: sdkmetric.NewManualReader()}
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(f.spans)).Tracer("test")
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(f.reader)).Meter("test")
	f.handler = lint.NewHandler(packs, lookup, fakeEngines(engines), binaryVersion, tracer, meter)
	return f
}

func TestLintTelemetry(t *testing.T) {
	harness := &fakeEngine{name: domain.EngineHarness, findings: []shareddomain.Finding{
		finding("ddd/layout", "b.ts", 4, shareddomain.SeverityError),
		finding("ddd/layout", "a.ts", 9, shareddomain.SeverityError),
	}}
	biome := &fakeEngine{name: domain.EngineBiomeGrit, findings: []shareddomain.Finding{
		finding("next/biome", "a.ts", 2, shareddomain.SeverityWarn),
		finding("ddd/grit", "a.ts", 2, shareddomain.SeverityError),
	}}
	idle := &fakeEngine{name: domain.EngineOxlint}
	f := newFixture([]domain.PackName{domain.PackDDD}, harness, biome, idle)

	report, err := f.handler.Handle(context.Background(), lint.Command{Dir: repoWithVersion(t, binaryVersion)})

	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"ddd/grit@a.ts:2", "next/biome@a.ts:2", "ddd/layout@a.ts:9", "ddd/layout@b.ts:4"}
	var gotOrder []string
	for _, x := range report.Findings {
		gotOrder = append(gotOrder, fmt.Sprintf("%s@%s:%d", x.Rule, x.File, x.Line))
	}
	if report.Version != shareddomain.ReportVersion || !slices.Equal(gotOrder, wantOrder) {
		t.Fatalf("report = %+v, order %v", report, gotOrder)
	}
	if len(biome.got) != 2 || len(harness.got) != 1 || biome.got[1].ID != "next/biome" {
		t.Fatalf("routing: biome %v, harness %v", biome.got, harness.got)
	}
	if idle.ran {
		t.Fatal("an engine with no rule must not run")
	}

	ended := f.spans.Ended()
	if len(ended) != 3 {
		t.Fatalf("spans = %v, want one %s and two %s", ended, spanRun, spanEngine)
	}
	root := ended[2]
	if root.Name() != spanRun {
		t.Fatalf("last span = %s, want %s", root.Name(), spanRun)
	}
	for _, s := range ended[:2] {
		if s.Name() != spanEngine || s.Parent().SpanID() != root.SpanContext().SpanID() {
			t.Fatalf("span %s must be a %s child of %s", s.Name(), spanEngine, spanRun)
		}
	}

	var collected metricdata.ResourceMetrics
	if err := f.reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	counts := map[attribute.Set]int64{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metricName {
				continue
			}
			for _, p := range m.Data.(metricdata.Sum[int64]).DataPoints {
				counts[p.Attributes] = p.Value
			}
		}
	}
	want := map[attribute.Set]int64{
		attribute.NewSet(attribute.String(attrRule, "ddd/layout"), attribute.String(attrSeverity, "error")): 2,
		attribute.NewSet(attribute.String(attrRule, "next/biome"), attribute.String(attrSeverity, "warn")):  1,
		attribute.NewSet(attribute.String(attrRule, "ddd/grit"), attribute.String(attrSeverity, "error")):   1,
	}
	if len(counts) != len(want) {
		t.Fatalf("counts = %v", counts)
	}
	for set, n := range want {
		if counts[set] != n {
			t.Fatalf("counts = %v, want %v", counts, want)
		}
	}
}

func TestLintStaleFiles(t *testing.T) {
	engine := &fakeEngine{name: domain.EngineHarness}
	f := newFixture([]domain.PackName{domain.PackDDD}, engine)

	missing := t.TempDir()
	for name, dir := range map[string]string{"missing": missing, "old": repoWithVersion(t, "v0.9.0")} {
		if _, err := f.handler.Handle(context.Background(), lint.Command{Dir: dir}); !errors.Is(err, domain.ErrStaleLintFiles) {
			t.Fatalf("%s: err = %v, want ErrStaleLintFiles", name, err)
		}
	}
	if engine.ran {
		t.Fatal("no engine runs on stale files")
	}
}

func TestLintPacks(t *testing.T) {
	t.Run("no pack gives an empty report without the version file", func(t *testing.T) {
		f := newFixture(nil, &fakeEngine{name: domain.EngineHarness})

		report, err := f.handler.Handle(context.Background(), lint.Command{Dir: t.TempDir()})

		if err != nil || report.Version != shareddomain.ReportVersion || report.Findings == nil || len(report.Findings) != 0 {
			t.Fatalf("report = %+v, err = %v", report, err)
		}
	})

	t.Run("a missing required pack is an error", func(t *testing.T) {
		f := newFixture([]domain.PackName{domain.PackDDDES}, &fakeEngine{name: domain.EngineHarness})

		_, err := f.handler.Handle(context.Background(), lint.Command{Dir: repoWithVersion(t, binaryVersion)})

		if !errors.Is(err, domain.ErrMissingPack) {
			t.Fatalf("err = %v, want ErrMissingPack", err)
		}
	})

	t.Run("an engine error stops the run", func(t *testing.T) {
		f := newFixture([]domain.PackName{domain.PackDDD}, &fakeEngine{name: domain.EngineHarness, err: errEngine})

		_, err := f.handler.Handle(context.Background(), lint.Command{Dir: repoWithVersion(t, binaryVersion)})

		if !errors.Is(err, errEngine) {
			t.Fatalf("err = %v, want the engine error", err)
		}
	})
}

func engineSpans(f fixture) map[string]sdktrace.ReadOnlySpan {
	out := map[string]sdktrace.ReadOnlySpan{}
	for _, s := range f.spans.Ended() {
		if s.Name() != spanEngine {
			continue
		}
		for _, a := range s.Attributes() {
			if string(a.Key) == attrEngine {
				out[a.Value.AsString()] = s
			}
		}
	}
	return out
}

func attrMap(attrs []attribute.KeyValue) map[string]attribute.Value {
	out := map[string]attribute.Value{}
	for _, a := range attrs {
		out[string(a.Key)] = a.Value
	}
	return out
}

func TestEngineSpanAttributes(t *testing.T) {
	harness := &fakeEngine{name: domain.EngineHarness, findings: []shareddomain.Finding{
		finding("ddd/layout", "a.ts", 1, shareddomain.SeverityError),
		finding("ddd/layout", "b.ts", 2, shareddomain.SeverityError),
	}}
	grit := &fakeEngine{name: domain.EngineBiomeGrit}
	f := newFixture([]domain.PackName{domain.PackDDD}, harness, grit)

	if _, err := f.handler.Handle(context.Background(), lint.Command{Dir: repoWithVersion(t, binaryVersion)}); err != nil {
		t.Fatal(err)
	}

	spans := engineSpans(f)
	if len(spans) != 2 {
		t.Fatalf("engine spans = %v", spans)
	}
	got := attrMap(spans[string(domain.EngineHarness)].Attributes())
	if got[attrRules].AsInt64() != 1 || got[attrFindings].AsInt64() != 2 {
		t.Fatalf("harness attributes = %v", got)
	}
	got = attrMap(spans[string(domain.EngineBiomeGrit)].Attributes())
	if got[attrRules].AsInt64() != 2 || got[attrFindings].AsInt64() != 0 {
		t.Fatalf("grit attributes = %v", got)
	}
}

func TestEngineSpanError(t *testing.T) {
	f := newFixture([]domain.PackName{domain.PackDDD}, &fakeEngine{name: domain.EngineHarness, err: errEngine})

	_, err := f.handler.Handle(context.Background(), lint.Command{Dir: repoWithVersion(t, binaryVersion)})

	if !errors.Is(err, errEngine) {
		t.Fatalf("err = %v, want the engine error", err)
	}
	span := engineSpans(f)[string(domain.EngineHarness)]
	if span == nil {
		t.Fatal("the failed engine span must end")
	}
	if span.Status().Code != codes.Error {
		t.Fatalf("status = %v, want Error", span.Status())
	}
	if len(span.Events()) != 1 || span.Events()[0].Name != eventException {
		t.Fatalf("events = %v, want one recorded error", span.Events())
	}
}

func TestEngineSpanEvents(t *testing.T) {
	harness := &fakeEngine{name: domain.EngineHarness, findings: []shareddomain.Finding{
		finding("ddd/layout", "b.ts", 4, shareddomain.SeverityError),
		finding("ddd/layout", "a.ts", 9, shareddomain.SeverityError),
		finding("ddd/layout", "c.ts", 1, shareddomain.SeverityWarn),
	}}
	f := newFixture([]domain.PackName{domain.PackDDD}, harness)

	if _, err := f.handler.Handle(context.Background(), lint.Command{Dir: repoWithVersion(t, binaryVersion)}); err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, e := range engineSpans(f)[string(domain.EngineHarness)].Events() {
		if e.Name != eventFindings {
			t.Fatalf("event = %s, want %s", e.Name, eventFindings)
		}
		a := attrMap(e.Attributes)
		got = append(got, fmt.Sprintf("%s/%s=%d", a[attrRule].AsString(), a[attrSeverity].AsString(), a[attrCount].AsInt64()))
	}
	want := []string{"ddd/layout/error=2", "ddd/layout/warn=1"}
	if !slices.Equal(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

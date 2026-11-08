package lintplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/jorge07/harness/internal/lint/application/lintplugin"
	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	repoDir      = "/repo"
	execEngine   = "todo-lint"
	execRuleID   = "team/no-todo"
	execScope    = "src/**/*.ts"
	emptyScope   = "lib/**/*.ts"
	scopedFile   = "src/a.ts"
	layoutRuleID = "ddd/layout"
)

var errPlugin = errors.New("plugin crashed")

type fakeEngine struct {
	name     domain.EngineName
	findings []shareddomain.Finding
	err      error
	ran      []domain.Rule
	prepared []domain.Rule
	dir      string
}

func (f *fakeEngine) Name() domain.EngineName { return f.name }
func (f *fakeEngine) Prepare(dir string, rules []domain.Rule) error {
	f.dir, f.prepared = dir, rules
	return f.err
}
func (f *fakeEngine) Run(_ context.Context, dir string, rules []domain.Rule) ([]shareddomain.Finding, error) {
	f.dir, f.ran = dir, rules
	return f.findings, f.err
}

type execPlugin struct {
	name    string
	events  []shareddomain.EventName
	verdict shareddomain.Verdict
	err     error
	got     *domain.CheckPayload
	raw     *json.RawMessage
}

func (p execPlugin) Name() string                         { return p.name }
func (p execPlugin) Subscribes() []shareddomain.EventName { return p.events }
func (p execPlugin) Handle(_ context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	if p.raw != nil {
		*p.raw = envelope.Payload
	}
	if p.got != nil {
		if err := json.Unmarshal(envelope.Payload, p.got); err != nil {
			return shareddomain.Verdict{}, err
		}
	}
	return p.verdict, p.err
}

func checkOnly() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventLintCheck}
}

func listFiles(_ context.Context, _ string, globs ...string) ([]string, error) {
	switch {
	case len(globs) == 0:
		return []string{scopedFile, "README.md"}, nil
	case slices.Equal(globs, []string{execScope}):
		return []string{scopedFile}, nil
	case slices.Equal(globs, []string{emptyScope}):
		return nil, nil
	}
	return nil, errors.New("unexpected globs")
}

func newEngines(defaults []domain.Engine, disabled []string, execs ...shareddomain.Plugin) (lintplugin.Engines, error) {
	return lintplugin.NewEngines(defaults, disabled, execs, listFiles, tracenoop.NewTracerProvider(), metricnoop.NewMeterProvider())
}

func execRule() domain.Rule {
	return domain.Rule{ID: execRuleID, Engine: execEngine, Severity: shareddomain.SeverityError, Scope: execScope, Message: "A TODO is left."}
}

func execFinding() shareddomain.Finding {
	return shareddomain.Finding{Rule: execRuleID, Severity: shareddomain.SeverityError, File: scopedFile, Line: 2, Message: "A TODO is left."}
}

func TestEnginesDispatch(t *testing.T) {
	layout := domain.Rule{ID: layoutRuleID, Engine: domain.EngineHarness, Severity: shareddomain.SeverityError}
	layoutFinding := shareddomain.Finding{Rule: layoutRuleID, Severity: shareddomain.SeverityError, File: "src/x.ts", Line: 1, Message: "layout"}
	harness := &fakeEngine{name: domain.EngineHarness, findings: []shareddomain.Finding{layoutFinding}}
	oxlint := &fakeEngine{name: domain.EngineOxlint}
	var payload domain.CheckPayload
	todo := execPlugin{name: execEngine, events: checkOnly(), got: &payload, verdict: shareddomain.Verdict{
		Kind: shareddomain.VerdictAllow, Findings: []shareddomain.Finding{execFinding()},
	}}
	engines, err := newEngines([]domain.Engine{harness, oxlint}, []string{string(domain.EngineOxlint)}, todo)
	if err != nil {
		t.Fatalf("NewEngines: %v", err)
	}
	ctx := context.Background()

	if err := engines.Prepare(ctx, repoDir, domain.EngineHarness, []domain.Rule{layout}); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if harness.dir != repoDir || !slices.Equal(harness.prepared, []domain.Rule{layout}) {
		t.Fatalf("prepare reached %q with %+v, want %q with the layout rule", harness.dir, harness.prepared, repoDir)
	}

	found, err := engines.Check(ctx, repoDir, domain.EngineHarness, []domain.Rule{layout})
	if err != nil || !slices.Equal(found, []shareddomain.Finding{layoutFinding}) || !slices.Equal(harness.ran, []domain.Rule{layout}) {
		t.Fatalf("default check = %+v, %v, ran %+v", found, err, harness.ran)
	}

	found, err = engines.Check(ctx, repoDir, domain.EngineOxlint, []domain.Rule{{ID: "next/x", Engine: domain.EngineOxlint, Scope: execScope}})
	if err != nil || len(found) != 0 || oxlint.ran != nil {
		t.Fatalf("disabled check = %+v, %v, ran %+v, want no run", found, err, oxlint.ran)
	}

	found, err = engines.Check(ctx, repoDir, execEngine, []domain.Rule{execRule()})
	if err != nil || !slices.Equal(found, []shareddomain.Finding{execFinding()}) {
		t.Fatalf("exec check = %+v, %v", found, err)
	}
	if !slices.Equal(payload.Rules, []domain.Rule{execRule()}) || !slices.Equal(payload.Files, []string{scopedFile}) {
		t.Fatalf("exec payload = %+v, want the rule and %s", payload, scopedFile)
	}
	if err := engines.Prepare(ctx, repoDir, execEngine, []domain.Rule{execRule()}); err != nil {
		t.Fatalf("prepare of a plugin without lint.prepare: %v", err)
	}
}

func TestExecPluginReplacesDisabledDefault(t *testing.T) {
	oxlint := &fakeEngine{name: domain.EngineOxlint}
	replacement := execPlugin{name: string(domain.EngineOxlint), events: checkOnly(), verdict: shareddomain.Verdict{
		Kind: shareddomain.VerdictAllow, Findings: []shareddomain.Finding{execFinding()},
	}}
	engines, err := newEngines([]domain.Engine{oxlint}, []string{string(domain.EngineOxlint)}, replacement)
	if err != nil {
		t.Fatalf("NewEngines: %v", err)
	}

	found, err := engines.Check(context.Background(), repoDir, domain.EngineOxlint, []domain.Rule{execRule()})

	if err != nil || !slices.Equal(found, []shareddomain.Finding{execFinding()}) {
		t.Fatalf("check = %+v, %v, want the finding of the exec plugin", found, err)
	}
	if oxlint.ran != nil {
		t.Fatal("the disabled default ran instead of the exec plugin")
	}
}

func TestNewEnginesRefuses(t *testing.T) {
	harness := func() []domain.Engine { return []domain.Engine{&fakeEngine{name: domain.EngineHarness}} }
	cases := []struct {
		name     string
		disabled []string
		execs    []shareddomain.Plugin
		want     error
	}{
		{name: "an unknown disabled name", disabled: []string{"eslint"}, want: domain.ErrUnknownEngine},
		{name: "an exec plugin with an enabled default name", execs: []shareddomain.Plugin{execPlugin{name: string(domain.EngineHarness), events: checkOnly()}}, want: domain.ErrEngineTaken},
		{name: "an exec plugin named biome-rule", execs: []shareddomain.Plugin{execPlugin{name: string(domain.EngineBiomeRule), events: checkOnly()}}, want: domain.ErrEngineTaken},
		{name: "an exec plugin on lint.prepare only", execs: []shareddomain.Plugin{execPlugin{name: execEngine, events: []shareddomain.EventName{shareddomain.EventLintPrepare}}}, want: domain.ErrPrepareWithoutCheck},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newEngines(harness(), tc.disabled, tc.execs...)

			if !errors.Is(err, tc.want) {
				t.Fatalf("NewEngines = %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("a rule engine with no plugin", func(t *testing.T) {
		engines, err := newEngines(harness(), nil)
		if err != nil {
			t.Fatal(err)
		}
		typo := domain.Rule{ID: execRuleID, Engine: "todo-lnt"}

		err = engines.Cover(map[domain.EngineName][]domain.Rule{typo.Engine: {typo}})

		if !errors.Is(err, domain.ErrUnknownEngine) {
			t.Fatalf("Cover = %v, want ErrUnknownEngine", err)
		}
	})
}

func TestEnginesCheckRefuses(t *testing.T) {
	withFinding := func(change func(*shareddomain.Finding)) shareddomain.Verdict {
		f := execFinding()
		change(&f)
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Findings: []shareddomain.Finding{f}}
	}
	cases := []struct {
		name    string
		verdict shareddomain.Verdict
		err     error
		want    error
	}{
		{name: "a plugin failure", err: errPlugin, want: errPlugin},
		{name: "a deny", verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "no config"}, want: domain.ErrEngineDenied},
		{name: "a finding of another rule", verdict: withFinding(func(f *shareddomain.Finding) { f.Rule = "team/typo" }), want: domain.ErrBadFinding},
		{name: "a finding with an absolute file", verdict: withFinding(func(f *shareddomain.Finding) { f.File = repoDir + "/" + scopedFile }), want: domain.ErrBadFinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engines, err := newEngines(nil, nil, execPlugin{name: execEngine, events: checkOnly(), verdict: tc.verdict, err: tc.err})
			if err != nil {
				t.Fatal(err)
			}

			_, err = engines.Check(context.Background(), repoDir, execEngine, []domain.Rule{execRule()})

			if !errors.Is(err, tc.want) {
				t.Fatalf("Check = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestEnginesCheckTakesRuleSeverity(t *testing.T) {
	sent := execFinding()
	sent.Severity = shareddomain.SeverityInfo
	engines, err := newEngines(nil, nil, execPlugin{name: execEngine, events: checkOnly(), verdict: shareddomain.Verdict{
		Kind: shareddomain.VerdictAllow, Findings: []shareddomain.Finding{sent},
	}})
	if err != nil {
		t.Fatal(err)
	}

	found, err := engines.Check(context.Background(), repoDir, execEngine, []domain.Rule{execRule()})

	if err != nil || !slices.Equal(found, []shareddomain.Finding{execFinding()}) {
		t.Fatalf("check = %+v, %v, want the severity of the rule", found, err)
	}
}

func TestEnginesCheckSendsEmptyFiles(t *testing.T) {
	var raw json.RawMessage
	engines, err := newEngines(nil, nil, execPlugin{name: execEngine, events: checkOnly(), raw: &raw, verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}})
	if err != nil {
		t.Fatal(err)
	}
	rule := execRule()
	rule.Scope = emptyScope

	if _, err := engines.Check(context.Background(), repoDir, execEngine, []domain.Rule{rule}); err != nil {
		t.Fatal(err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil || string(payload["files"]) != "[]" {
		t.Fatalf("payload = %s, want an empty files list", raw)
	}
}

func TestEnginesCheckDefaultEngineError(t *testing.T) {
	harness := &fakeEngine{name: domain.EngineHarness, err: errPlugin}
	engines, err := newEngines([]domain.Engine{harness}, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engines.Check(context.Background(), repoDir, domain.EngineHarness, []domain.Rule{{ID: layoutRuleID, Engine: domain.EngineHarness}})

	if !errors.Is(err, errPlugin) {
		t.Fatalf("Check = %v, want the engine error", err)
	}
}

package guardplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/90p-labs/harness/internal/guard"
	"github.com/90p-labs/harness/internal/guard/application/guardplugin"
	"github.com/90p-labs/harness/internal/guard/domain"
	"github.com/90p-labs/harness/internal/shared/application/eventbus"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	pluginPrefix         = "guard."
	pluginReviewRecorder = pluginPrefix + "review-recorder"
	forcePush            = "git push --force origin feat/x"
	metricDenies         = "harness.guard.denies"
	attrRule             = "harness.rule"
	toolBash             = "Bash"
	toolAgent            = "Agent"
	branchDevelop        = "develop"
	emDashText           = "fix — it\n"
	modelSonnet          = "sonnet"
	modelGPT             = "gpt-6-astra"
	roleWorker           = "worker"
	pushToDevelop        = "git push origin HEAD:develop"
	pushToMain           = "git push origin HEAD:main"
	stopReason           = "the run is in phase working and is not finished"
	stopShellTasksText   = "wait for agents"
	missingBodyMsg       = "missing.md"
	subDir               = "sub"
)

type fakeStop struct {
	deny         bool
	reason       string
	err          error
	agentRunning bool
	calls        int
}

func (f *fakeStop) DecideStop(_ context.Context, agentRunning bool) (bool, string, error) {
	f.calls++
	f.agentRunning = agentRunning
	return f.deny, f.reason, f.err
}

func deps(stop domain.StopDecider) guardplugin.Deps {
	return guardplugin.Deps{
		ReadFile:      os.ReadFile,
		DefaultBranch: func(context.Context, string) (string, error) { return "main", nil },
		Stop:          stop,
		CommitChecked: func(context.Context, string) (bool, error) { return true, nil },
		PatchID:       func(context.Context, string, string) (string, error) { return "", nil },
		ReworkReasons: func(context.Context, string, string) ([]string, error) { return nil, nil },
		Models:        map[shareddomain.Host]map[string][]string{},
		Meter:         metricnoop.NewMeterProvider().Meter("test"),
	}
}

func build(t *testing.T, disabled []string, d guardplugin.Deps) []shareddomain.Plugin {
	t.Helper()
	plugins, err := guardplugin.Plugins(disabled, d)
	if err != nil {
		t.Fatalf("plugins: %v", err)
	}
	return plugins
}

func newBus(plugins []shareddomain.Plugin) eventbus.Bus {
	return eventbus.NewBus(plugins, tracenoop.NewTracerProvider(), metricnoop.NewMeterProvider())
}

func toolEnvelope(t *testing.T, host shareddomain.Host, cwd, tool string, input map[string]string) shareddomain.Envelope {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	kind := shareddomain.ToolKindShell
	if tool == toolAgent {
		kind = shareddomain.ToolKindAgent
	}
	payload, err := json.Marshal(shareddomain.ToolPayload{Tool: tool, Kind: kind, Input: raw})
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{Event: shareddomain.EventToolBefore, Host: host, Cwd: cwd, Payload: payload}
}

func bash(t *testing.T, cwd, command string) shareddomain.Envelope {
	t.Helper()
	return toolEnvelope(t, shareddomain.HostClaude, cwd, toolBash, map[string]string{"command": command})
}

func stopEnvelope(t *testing.T, background int) shareddomain.Envelope {
	t.Helper()
	payload, err := json.Marshal(shareddomain.StopPayload{BackgroundAgents: background})
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{Event: shareddomain.EventTurnStopping, Host: shareddomain.HostClaude, Payload: payload}
}

func dispatch(t *testing.T, bus eventbus.Bus, env shareddomain.Envelope) shareddomain.Verdict {
	t.Helper()
	result, err := bus.Dispatch(context.Background(), env)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	return result.Verdict
}

func TestGuardPluginNames(t *testing.T) {
	plugins := build(t, nil, deps(&fakeStop{}))

	var want []string
	for _, rule := range domain.Rules() {
		want = append(want, pluginPrefix+rule.ID)
	}
	want = append(want, pluginPrefix+domain.RuleStopHoldsTurn, pluginReviewRecorder, pluginPrefix+domain.RuleReviewModels, pluginPrefix+domain.RuleReviewFindings)
	events := map[string]shareddomain.EventName{
		pluginPrefix + domain.RuleStopHoldsTurn:  shareddomain.EventTurnStopping,
		pluginReviewRecorder:                     shareddomain.EventSubagentStopped,
		pluginPrefix + domain.RuleReviewModels:   shareddomain.EventRunTransitionBefore,
		pluginPrefix + domain.RuleReviewFindings: shareddomain.EventRunTransitionBefore,
	}
	var got []string
	for _, p := range plugins {
		got = append(got, p.Name())
		event, ok := events[p.Name()]
		if !ok {
			event = shareddomain.EventToolBefore
		}
		if !slices.Equal(p.Subscribes(), []shareddomain.EventName{event}) {
			t.Errorf("%s subscribes to %v, want only %s", p.Name(), p.Subscribes(), event)
		}
	}
	if len(want) != 17 || !slices.Equal(got, want) {
		t.Fatalf("names = %v, want the 17 %v", got, want)
	}
}

func TestGuardDisabled(t *testing.T) {
	plugins := build(t, []string{domain.RuleGitNoForce, domain.RuleStopHoldsTurn}, deps(&fakeStop{}))

	if len(plugins) != 15 {
		t.Fatalf("plugins = %d, want 15", len(plugins))
	}
	for _, p := range plugins {
		if p.Name() == pluginPrefix+domain.RuleGitNoForce || p.Name() == pluginPrefix+domain.RuleStopHoldsTurn {
			t.Errorf("%s built, want it disabled", p.Name())
		}
	}
	if v := dispatch(t, newBus(plugins), bash(t, t.TempDir(), forcePush)); v.Kind != shareddomain.VerdictAllow {
		t.Errorf("verdict = %+v, want allow with the rule disabled", v)
	}

	_, err := guardplugin.Plugins([]string{domain.RuleGitNoForce, "no-such-rule"}, deps(&fakeStop{}))
	if err == nil || !strings.Contains(err.Error(), "no-such-rule") {
		t.Fatalf("error = %v, want one naming no-such-rule", err)
	}
}

func TestCommitsCheckedRule(t *testing.T) {
	var asked int
	var askedDir string
	checked, checkErr := false, error(nil)
	d := deps(&fakeStop{})
	d.CommitChecked = func(_ context.Context, dir string) (bool, error) {
		asked++
		askedDir = dir
		return checked, checkErr
	}
	bus := newBus(build(t, nil, d))
	denyPrefix := pluginPrefix + domain.RuleCommitsChecked + ": "

	for _, command := range []string{"git push origin feat/x", "gh pr create --draft --title t"} {
		cwd := t.TempDir()
		v := dispatch(t, bus, bash(t, cwd, command))
		if v.Kind != shareddomain.VerdictDeny || !strings.HasPrefix(v.Reason, denyPrefix) || !strings.Contains(v.Reason, "harness verify --commits") {
			t.Errorf("%q without a check = %+v, want a deny from commits-checked", command, v)
		}
		if askedDir != cwd {
			t.Errorf("%q asked the check for %q, want the tool call dir %q", command, askedDir, cwd)
		}
	}

	checkErr = errors.New("no commit yet")
	v := dispatch(t, bus, bash(t, t.TempDir(), "git push origin feat/x"))
	if v.Kind != shareddomain.VerdictDeny || !strings.Contains(v.Reason, "commit, then run harness verify --commits") {
		t.Errorf("push without a HEAD = %+v, want the commit next step", v)
	}

	checked, checkErr = true, nil
	for _, command := range []string{"git push origin feat/x", "gh pr create --draft --title t"} {
		if v := dispatch(t, bus, bash(t, t.TempDir(), command)); v.Kind != shareddomain.VerdictAllow {
			t.Errorf("%q with a check = %+v, want allow", command, v)
		}
	}

	asked = 0
	dispatch(t, bus, bash(t, t.TempDir(), "git status"))
	if asked != 0 {
		t.Errorf("git status asked the commit check %d times, want 0", asked)
	}
}

func TestAgentModelRouting(t *testing.T) {
	d := deps(&fakeStop{})
	d.Models = map[shareddomain.Host]map[string][]string{
		shareddomain.HostClaude: {roleWorker: {modelSonnet}},
		shareddomain.HostCodex:  {roleWorker: {modelGPT}},
	}
	bus := newBus(build(t, nil, d))
	call := func(host shareddomain.Host, model string) shareddomain.Verdict {
		return dispatch(t, bus, toolEnvelope(t, host, t.TempDir(), toolAgent, map[string]string{"subagent_type": roleWorker, "model": model}))
	}

	if v := call(shareddomain.HostClaude, modelSonnet); v.Kind != shareddomain.VerdictAllow {
		t.Errorf("claude with sonnet = %+v, want allow", v)
	}
	v := call(shareddomain.HostCodex, modelSonnet)
	if v.Kind != shareddomain.VerdictDeny || !strings.HasPrefix(v.Reason, pluginPrefix+domain.RuleAgentModelRouting+": ") || !strings.Contains(v.Reason, modelGPT) {
		t.Errorf("codex with sonnet = %+v, want a deny naming %s", v, modelGPT)
	}
	if v := call(shareddomain.HostCodex, modelGPT); v.Kind != shareddomain.VerdictAllow {
		t.Errorf("codex with gpt-6-astra = %+v, want allow", v)
	}
}

func TestEmptyModelByHost(t *testing.T) {
	d := deps(&fakeStop{})
	d.Models = map[shareddomain.Host]map[string][]string{
		shareddomain.HostClaude: {roleWorker: {modelSonnet}},
		shareddomain.HostCodex:  {roleWorker: {modelGPT}},
	}
	bus := newBus(build(t, nil, d))
	call := func(host shareddomain.Host) shareddomain.Verdict {
		return dispatch(t, bus, toolEnvelope(t, host, t.TempDir(), toolAgent, map[string]string{"subagent_type": roleWorker}))
	}

	if v := call(shareddomain.HostCodex); v.Kind != shareddomain.VerdictAllow {
		t.Errorf("codex with no model = %+v, want allow", v)
	}
	if v := call(shareddomain.HostClaude); v.Kind != shareddomain.VerdictDeny {
		t.Errorf("claude with no model = %+v, want deny", v)
	}
}

func TestGuardReadsTextFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("dash.txt", emDashText)
	write("clean.txt", "fix it\n")
	if err := os.Mkdir(filepath.Join(dir, subDir), 0o700); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(subDir, "note.txt"), emDashText)
	bus := newBus(build(t, nil, deps(&fakeStop{})))

	for _, tc := range []struct {
		name, command, wantRule string
	}{
		{"short flag", "git commit -F dash.txt", domain.RuleWritingStyle},
		{"long flag", "git commit --file dash.txt", domain.RuleWritingStyle},
		{"equals form", "git commit --file=dash.txt", domain.RuleWritingStyle},
		{"pr body file", "gh pr comment 3 --body-file dash.txt", domain.RuleWritingStyle},
		{"missing file", "gh pr create --draft --body-file " + missingBodyMsg, domain.RulePRBodyShape},
		{"missing message file", "git commit -F " + missingBodyMsg, domain.RuleWritingStyle},
		{"file after a cd", "cd " + subDir + " && gh pr comment 3 --body-file note.txt", domain.RuleWritingStyle},
		{"file of another command", "git commit -F clean.txt && gh pr edit 3 --add-label ci", ""},
		{"clean file", "git commit -F clean.txt", ""},
		{"stdin", "git commit -F -", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := dispatch(t, bus, bash(t, dir, tc.command))

			if tc.wantRule == "" {
				if v.Kind != shareddomain.VerdictAllow {
					t.Fatalf("verdict = %+v, want allow", v)
				}
				return
			}
			if v.Kind != shareddomain.VerdictDeny || !strings.HasPrefix(v.Reason, pluginPrefix+tc.wantRule+": ") {
				t.Fatalf("verdict = %+v, want a deny from %s", v, tc.wantRule)
			}
		})
	}
}

func TestStopHoldsTurn(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stop       fakeStop
		background int
		wantDeny   bool
		wantAgent  bool
		wantErr    bool
	}{
		{"blocked with no agent", fakeStop{deny: true, reason: stopReason}, 0, true, false, false},
		{"background agent counts as running", fakeStop{}, 1, false, true, false},
		{"decider error", fakeStop{err: errors.New("db down")}, 0, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stop := tc.stop
			plugin := findPlugin(t, build(t, nil, deps(&stop)), pluginPrefix+domain.RuleStopHoldsTurn)

			v, err := plugin.Handle(context.Background(), stopEnvelope(t, tc.background))

			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if stop.agentRunning != tc.wantAgent || stop.calls != 1 {
				t.Errorf("agentRunning = %v after %d calls, want %v after 1", stop.agentRunning, stop.calls, tc.wantAgent)
			}
			if tc.wantDeny != (v.Kind == shareddomain.VerdictDeny) {
				t.Fatalf("verdict = %+v, want deny %v", v, tc.wantDeny)
			}
			if tc.wantDeny && (!strings.HasPrefix(v.Reason, stopReason) || !strings.HasSuffix(v.Reason, domain.StopHoldsTurnNextStep)) {
				t.Errorf("reason = %q, want the decider reason then the next step", v.Reason)
			}
		})
	}
}

func TestStopHoldsTurnDenyNamesShellTasks(t *testing.T) {
	stop := &fakeStop{deny: true, reason: stopReason}
	plugin := findPlugin(t, build(t, nil, deps(stop)), pluginPrefix+domain.RuleStopHoldsTurn)

	v, err := plugin.Handle(context.Background(), stopEnvelope(t, 0))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if v.Kind != shareddomain.VerdictDeny {
		t.Fatalf("verdict = %+v, want deny", v)
	}
	if !strings.Contains(v.Reason, stopShellTasksText) {
		t.Errorf("reason = %q, want it to contain %q", v.Reason, stopShellTasksText)
	}
}

func findPlugin(t *testing.T, plugins []shareddomain.Plugin, name string) shareddomain.Plugin {
	t.Helper()
	for _, p := range plugins {
		if p.Name() == name {
			return p
		}
	}
	t.Fatalf("no plugin %s", name)
	return nil
}

func TestGuardDenyCounter(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	d := deps(&fakeStop{deny: true, reason: stopReason})
	d.Meter = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)).Meter("test")
	bus := newBus(build(t, nil, d))

	dispatch(t, bus, bash(t, t.TempDir(), forcePush))
	dispatch(t, bus, bash(t, t.TempDir(), forcePush))
	dispatch(t, bus, bash(t, t.TempDir(), "git status"))
	dispatch(t, bus, stopEnvelope(t, 0))

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("collect: %v", err)
	}
	got := map[string]int64{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != metricDenies {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("data = %T, want an int64 sum", m.Data)
			}
			for _, point := range sum.DataPoints {
				rule, _ := point.Attributes.Value(attrRule)
				got[rule.AsString()] += point.Value
			}
		}
	}
	want := map[string]int64{domain.RuleGitNoForce: 2, domain.RuleStopHoldsTurn: 1}
	if len(got) != len(want) || got[domain.RuleGitNoForce] != 2 || got[domain.RuleStopHoldsTurn] != 1 {
		t.Fatalf("denies = %v, want %v", got, want)
	}
}

func TestDenyReasonShape(t *testing.T) {
	bus := newBus(build(t, nil, deps(&fakeStop{})))

	v := dispatch(t, bus, bash(t, t.TempDir(), forcePush))

	prefix := pluginPrefix + domain.RuleGitNoForce + ": "
	if v.Kind != shareddomain.VerdictDeny || !strings.HasPrefix(v.Reason, prefix) {
		t.Fatalf("verdict = %+v, want a deny starting with %q", v, prefix)
	}
	if strings.Count(v.Reason, prefix) != 1 || strings.TrimSpace(strings.TrimPrefix(v.Reason, prefix)) == "" {
		t.Errorf("reason = %q, want the prefix once and a reason after it", v.Reason)
	}
}

func TestDefaultBranchOverride(t *testing.T) {
	denied := func(t *testing.T, plugins []shareddomain.Plugin, command string) bool {
		t.Helper()
		return dispatch(t, newBus(plugins), bash(t, t.TempDir(), command)).Kind == shareddomain.VerdictDeny
	}

	t.Run("deps function", func(t *testing.T) {
		d := deps(&fakeStop{})
		d.DefaultBranch = func(context.Context, string) (string, error) { return branchDevelop, nil }
		plugins := build(t, nil, d)

		if !denied(t, plugins, pushToDevelop) || denied(t, plugins, pushToMain) {
			t.Fatal("want a push to develop denied and a push to main allowed")
		}
	})

	t.Run("configured value skips git", func(t *testing.T) {
		plugins, err := guard.NewPlugins(nil, branchDevelop, nil, t.TempDir(), &fakeStop{}, func(context.Context, string) (bool, error) { return true, nil }, nil, nil, nil, false)
		if err != nil {
			t.Fatalf("new plugins: %v", err)
		}

		if !denied(t, plugins, pushToDevelop) || denied(t, plugins, pushToMain) {
			t.Fatal("want a push to develop denied and a push to main allowed, with no git repository")
		}
	})
}

type failingMeter struct{ metricnoop.Meter }

func (failingMeter) Int64Counter(string, ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	return nil, errors.New("meter down")
}

func TestGuardCounterError(t *testing.T) {
	d := deps(&fakeStop{})
	d.Meter = failingMeter{}

	_, err := guardplugin.Plugins(nil, d)

	if err == nil || !strings.Contains(err.Error(), metricDenies) {
		t.Fatalf("error = %v, want one naming %s", err, metricDenies)
	}
}

func TestGuardMalformedPayload(t *testing.T) {
	plugins := build(t, nil, deps(&fakeStop{}))
	toolPlugin := findPlugin(t, plugins, pluginPrefix+domain.RuleGitNoForce)
	stopPlugin := findPlugin(t, plugins, pluginPrefix+domain.RuleStopHoldsTurn)
	listCommand := shareddomain.Envelope{
		Event:   shareddomain.EventToolBefore,
		Payload: []byte(`{"tool":"Bash","kind":"shell","input":{"command":["ls"]}}`),
	}

	for _, tc := range []struct {
		name     string
		plugin   shareddomain.Plugin
		envelope shareddomain.Envelope
	}{
		{"tool payload that is not JSON", toolPlugin, shareddomain.Envelope{Event: shareddomain.EventToolBefore, Payload: []byte("{")}},
		{"shell command that is a list", toolPlugin, listCommand},
		{"stop payload that is not JSON", stopPlugin, shareddomain.Envelope{Event: shareddomain.EventTurnStopping, Payload: []byte("{")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.plugin.Handle(context.Background(), tc.envelope); err == nil {
				t.Fatal("want an error, so the veto bus fails closed")
			}
		})
	}
}

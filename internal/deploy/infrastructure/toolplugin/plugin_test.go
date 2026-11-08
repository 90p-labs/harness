package toolplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/deploy/domain"
	"github.com/jorge07/harness/internal/deploy/infrastructure/toolplugin"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const fakeHelm = "#!/bin/sh\necho \"# args: $*\"\necho \"# dir: $(pwd)\"\n"

func envelope(t *testing.T, event shareddomain.EventName, cwd string, payload any) shareddomain.Envelope {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{Version: shareddomain.EnvelopeVersion, Event: event, Cwd: cwd, Payload: raw}
}

func pathWith(t *testing.T, script string) {
	t.Helper()
	bin := t.TempDir()
	if script != "" {
		if err := os.WriteFile(filepath.Join(bin, "helm"), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

func TestSubscriptions(t *testing.T) {
	if got := toolplugin.Helm().Subscribes(); !slices.Equal(got, []shareddomain.EventName{shareddomain.EventGatesDetect, shareddomain.EventChartRender}) {
		t.Errorf("helm subscribes %v", got)
	}
	if got := toolplugin.Promtool().Subscribes(); !slices.Equal(got, []shareddomain.EventName{shareddomain.EventGatesDetect}) {
		t.Errorf("promtool subscribes %v", got)
	}
}

func TestDetectAnswersOneGateOnEachLine(t *testing.T) {
	files := shareddomain.GatesDetectPayload{Files: []string{"deploy/chart/Chart.yaml", "deploy/api.rules.test.yaml"}}
	for _, plugin := range []toolplugin.Plugin{toolplugin.Helm(), toolplugin.Promtool()} {
		verdict, err := plugin.Handle(context.Background(), envelope(t, shareddomain.EventGatesDetect, t.TempDir(), files))
		if err != nil {
			t.Fatalf("%s: %v", plugin.Name(), err)
		}
		var gate shareddomain.ProposedGate
		if err := json.Unmarshal([]byte(verdict.Context), &gate); err != nil || verdict.Kind != shareddomain.VerdictAllow {
			t.Fatalf("%s verdict = %+v, err = %v", plugin.Name(), verdict, err)
		}
		if gate.ID == "" || gate.Run == "" {
			t.Errorf("%s gate = %+v", plugin.Name(), gate)
		}
	}
}

func TestDetectWithoutFilesProposesNothing(t *testing.T) {
	verdict, err := toolplugin.Helm().Handle(context.Background(), envelope(t, shareddomain.EventGatesDetect, t.TempDir(), shareddomain.GatesDetectPayload{Files: []string{"go.mod"}}))
	if err != nil || verdict.Kind != shareddomain.VerdictAllow || verdict.Context != "" {
		t.Errorf("verdict = %+v, err = %v, want an allow with no context", verdict, err)
	}
}

func TestHelmRendersInTheEnvelopeFolder(t *testing.T) {
	pathWith(t, fakeHelm)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	verdict, err := toolplugin.Helm().Handle(context.Background(), envelope(t, shareddomain.EventChartRender, dir, shareddomain.ChartRenderPayload{Chart: "deploy/chart"}))

	if err != nil {
		t.Fatal(err)
	}
	if want := "# args: template deploy/chart\n# dir: " + dir + "\n"; verdict.Context != want {
		t.Errorf("context = %q, want %q", verdict.Context, want)
	}
}

func TestHelmRenderFailures(t *testing.T) {
	tests := map[string]struct {
		script string
		want   error
		text   string
	}{
		"helm missing":    {want: domain.ErrToolMissing, text: "helm"},
		"template failed": {script: "#!/bin/sh\necho 'Chart.yaml file is missing' >&2\nexit 1\n", text: "Chart.yaml file is missing"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			pathWith(t, tt.script)

			_, err := toolplugin.Helm().Handle(context.Background(), envelope(t, shareddomain.EventChartRender, t.TempDir(), shareddomain.ChartRenderPayload{Chart: "deploy/chart"}))

			if err == nil || !strings.Contains(err.Error(), tt.text) || (tt.want != nil && !errors.Is(err, tt.want)) {
				t.Errorf("err = %v, want %v with %q", err, tt.want, tt.text)
			}
		})
	}
}

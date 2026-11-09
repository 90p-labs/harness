package renderchart_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/deploy/application/query/renderchart"
	"github.com/90p-labs/harness/internal/deploy/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

var errHelmFailed = errors.New("helm template failed")

type fakeBus struct {
	result shareddomain.DispatchResult
	err    error
	got    shareddomain.Envelope
}

func (b *fakeBus) Dispatch(_ context.Context, envelope shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	b.got = envelope
	return b.result, b.err
}

func TestRenderReturnsTheManifest(t *testing.T) {
	bus := &fakeBus{result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: "kind: Deployment\n"}}}

	out, err := renderchart.NewHandler(bus).Handle(context.Background(), renderchart.Query{Dir: "/repo", Chart: "deploy/chart"})

	if err != nil || string(out) != "kind: Deployment\n" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	var payload shareddomain.ChartRenderPayload
	if err := json.Unmarshal(bus.got.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if bus.got.Event != shareddomain.EventChartRender || bus.got.Cwd != "/repo" || payload.Chart != "deploy/chart" {
		t.Errorf("envelope = %+v, payload = %+v", bus.got, payload)
	}
}

func TestRenderFailures(t *testing.T) {
	tests := map[string]struct {
		result shareddomain.DispatchResult
		err    error
		want   error
		text   string
	}{
		"the bus fails": {err: errHelmFailed, want: errHelmFailed},
		"a plugin denies": {
			result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "helm: chart is locked", Context: "kind: Deployment\n"}},
			text:   "helm: chart is locked",
		},
		"no plugin answers": {result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}}, want: domain.ErrNoRenderer},
		"a plugin fails": {
			result: shareddomain.DispatchResult{
				Verdict:  shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: "kind: Deployment\n"},
				Failures: []shareddomain.DispatchFailure{{Plugin: "helm", Err: errHelmFailed}},
			},
			want: errHelmFailed,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			out, err := renderchart.NewHandler(&fakeBus{result: tt.result, err: tt.err}).Handle(context.Background(), renderchart.Query{Dir: "/repo", Chart: "deploy/chart"})
			if err == nil || (tt.want != nil && !errors.Is(err, tt.want)) || !strings.Contains(err.Error(), tt.text) {
				t.Errorf("err = %v, want %v with %q", err, tt.want, tt.text)
			}
			if out != nil {
				t.Errorf("out = %q, want none after a failure", out)
			}
		})
	}
}

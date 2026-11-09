package pluginhooks_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/infrastructure/pluginhooks"
	plugindomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	worktree   = "/work/tree"
	denyReason = "freeze window"
	runID      = "run-1"
)

type recorder struct {
	got     []plugindomain.Envelope
	verdict plugindomain.Verdict
}

func (r *recorder) Dispatch(_ context.Context, envelope plugindomain.Envelope) (plugindomain.DispatchResult, error) {
	r.got = append(r.got, envelope)
	return plugindomain.DispatchResult{Verdict: r.verdict}, nil
}

func hooksWith(bus *recorder) pluginhooks.Hooks {
	return pluginhooks.NewHooks(bus, worktree)
}

func decode(t *testing.T, envelope plugindomain.Envelope) plugindomain.TransitionPayload {
	t.Helper()
	var p plugindomain.TransitionPayload
	if err := json.Unmarshal(envelope.Payload, &p); err != nil {
		t.Fatalf("payload: %v", err)
	}
	return p
}

func requireEnvelope(t *testing.T, envelope plugindomain.Envelope, event plugindomain.EventName, phase domain.Phase) {
	t.Helper()
	if envelope.Version != plugindomain.EnvelopeVersion || envelope.Event != event || envelope.Host != "" || envelope.Cwd != worktree {
		t.Fatalf("envelope = %+v", envelope)
	}
	if want := (plugindomain.RunRef{ID: runID, Phase: string(phase)}); envelope.Run != want {
		t.Fatalf("run ref = %+v, want %+v", envelope.Run, want)
	}
}

func TestHooksBeforeDeny(t *testing.T) {
	plugin := &recorder{verdict: plugindomain.Verdict{Kind: plugindomain.VerdictDeny, Reason: denyReason}}
	run := domain.Run{ID: runID, Phase: domain.PhaseWorking}

	err := hooksWith(plugin).Before(context.Background(), run, domain.Command{Event: domain.EventSubmitPlan, Reason: "why"}, domain.PhaseWorking)

	if !errors.Is(err, domain.ErrTransitionVetoed) || !strings.Contains(err.Error(), denyReason) {
		t.Fatalf("err = %v", err)
	}
	if len(plugin.got) != 1 {
		t.Fatalf("dispatched %d envelopes", len(plugin.got))
	}
	requireEnvelope(t, plugin.got[0], plugindomain.EventRunTransitionBefore, domain.PhaseWorking)
	want := plugindomain.TransitionPayload{Event: string(domain.EventSubmitPlan), From: string(domain.PhaseWorking), To: string(domain.PhaseWorking), Reason: "why"}
	if got := decode(t, plugin.got[0]); got != want {
		t.Fatalf("payload = %+v, want %+v", got, want)
	}
}

func TestBeforePayloadCarriesTarget(t *testing.T) {
	plugin := &recorder{verdict: plugindomain.Verdict{Kind: plugindomain.VerdictAllow}}
	run := domain.Run{ID: runID, Phase: domain.PhasePlanning}

	err := hooksWith(plugin).Before(context.Background(), run, domain.Command{Event: domain.EventSubmitPlan, Reason: "why"}, domain.PhaseWorking)

	if err != nil {
		t.Fatal(err)
	}
	want := plugindomain.TransitionPayload{Event: string(domain.EventSubmitPlan), From: string(domain.PhasePlanning), To: string(domain.PhaseWorking), Reason: "why"}
	if got := decode(t, plugin.got[0]); got != want {
		t.Fatalf("payload = %+v, want %+v", got, want)
	}
}

func TestHooksBeforeAllow(t *testing.T) {
	allow := &recorder{verdict: plugindomain.Verdict{Kind: plugindomain.VerdictAllow}}
	if err := hooksWith(allow).Before(context.Background(), domain.Run{}, domain.Command{}, domain.PhaseNone); err != nil {
		t.Fatalf("allow: %v", err)
	}
}

func TestHooksAfterEnvelope(t *testing.T) {
	plugin := &recorder{verdict: plugindomain.Verdict{Kind: plugindomain.VerdictDeny, Reason: denyReason}}
	from := domain.Run{ID: runID}
	to, err := from.Apply(domain.Command{Event: domain.EventStart, Reason: "go"}, domain.Policy{})
	if err != nil {
		t.Fatal(err)
	}

	hooksWith(plugin).After(context.Background(), from, to)

	if len(plugin.got) != 1 {
		t.Fatalf("dispatched %d envelopes", len(plugin.got))
	}
	requireEnvelope(t, plugin.got[0], plugindomain.EventRunTransitionAfter, domain.PhaseNone)
	want := plugindomain.TransitionPayload{Event: string(domain.EventStart), From: string(domain.PhaseNone), To: string(domain.PhasePlanning), Reason: "go"}
	if got := decode(t, plugin.got[0]); got != want {
		t.Fatalf("payload = %+v, want %+v", got, want)
	}
}

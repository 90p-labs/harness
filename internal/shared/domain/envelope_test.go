package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/jorge07/harness/internal/shared/domain"
)

const specEnvelope = `{"version":1,"event":"tool.before","host":"claude","cwd":"/repo","run":{"id":"r1","phase":"working"},"payload":{"tool":"Bash","input":{"command":"git push"}}}`

func TestEnvelopeJSON(t *testing.T) {
	envelope := domain.Envelope{
		Version: domain.EnvelopeVersion,
		Event:   domain.EventToolBefore,
		Host:    domain.HostClaude,
		Cwd:     "/repo",
		Run:     domain.RunRef{ID: "r1", Phase: "working"},
		Payload: json.RawMessage(`{"tool":"Bash","input":{"command":"git push"}}`),
	}

	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(encoded) != specEnvelope {
		t.Fatalf("encoded envelope\n got: %s\nwant: %s", encoded, specEnvelope)
	}

	var decoded domain.Envelope
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Version != envelope.Version || decoded.Event != envelope.Event ||
		decoded.Host != envelope.Host || decoded.Cwd != envelope.Cwd ||
		decoded.Run != envelope.Run || string(decoded.Payload) != string(envelope.Payload) {
		t.Fatalf("round trip mismatch: %+v", decoded)
	}
}

func TestTransitionPhaseNames(t *testing.T) {
	wire := map[string]string{
		domain.PhasePlanning:         "planning",
		domain.PhaseAwaitingApproval: "awaiting_approval",
		domain.PhaseWorking:          "working",
		domain.PhaseReviewing:        "reviewing",
		domain.PhaseShipping:         "shipping",
		domain.PhaseReleasing:        "releasing",
		domain.PhaseObserving:        "observing",
		domain.PhaseBlocked:          "blocked",
		domain.PhaseDone:             "done",
		domain.PhaseAbandoned:        "abandoned",
	}
	for phase, name := range wire {
		encoded, err := json.Marshal(domain.TransitionPayload{To: phase})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("unmarshal %s: %v", encoded, err)
		}
		if decoded["to"] != name {
			t.Errorf("encoded %s, want to %q", encoded, name)
		}
	}
}

func TestVerdictJSON(t *testing.T) {
	var verdict domain.Verdict
	if err := json.Unmarshal([]byte(`{"verdict":"deny","reason":"no push","context":"c"}`), &verdict); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := domain.Verdict{Kind: domain.VerdictDeny, Reason: "no push", Context: "c"}
	if verdict != want {
		t.Fatalf("got %+v, want %+v", verdict, want)
	}
}

func TestVerdictKindKnown(t *testing.T) {
	for _, kind := range []domain.VerdictKind{domain.VerdictAllow, domain.VerdictDeny, domain.VerdictContext} {
		if !kind.Known() {
			t.Errorf("%q is not known", kind)
		}
	}
	for _, kind := range []domain.VerdictKind{"", "block", "Deny"} {
		if kind.Known() {
			t.Errorf("%q is known", kind)
		}
	}
}

func TestKindOf(t *testing.T) {
	cases := map[domain.EventName]domain.Kind{
		domain.EventSessionStarted:      domain.KindContext,
		domain.EventPromptSubmitted:     domain.KindVeto,
		domain.EventToolBefore:          domain.KindVeto,
		domain.EventToolAfter:           domain.KindContext,
		domain.EventTurnStopping:        domain.KindVeto,
		domain.EventSubagentStarted:     domain.KindContext,
		domain.EventSubagentStopped:     domain.KindObserve,
		domain.EventRunTransitionBefore: domain.KindVeto,
		domain.EventRunTransitionAfter:  domain.KindObserve,
		domain.EventPlanSubmitted:       domain.KindCollect,
		domain.EventGateBefore:          domain.KindVeto,
		domain.EventGateAfter:           domain.KindObserve,
		domain.EventSignalsCheck:        domain.KindCollect,
	}
	for name, want := range cases {
		got, ok := domain.KindOf(name)
		if !ok || got != want {
			t.Errorf("KindOf(%q) = %q, %v; want %q", name, got, ok, want)
		}
	}
	if _, ok := domain.KindOf("nope"); ok {
		t.Error("KindOf of an unknown event must report false")
	}
}

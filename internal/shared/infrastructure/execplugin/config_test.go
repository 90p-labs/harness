package execplugin_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/execplugin"
)

func TestFromConfigUnknownEvent(t *testing.T) {
	plugins := []config.Plugin{{Name: "audit", Command: []string{"audit"}, Events: []string{string(domain.EventToolBefore), "nope.event"}}}

	_, err := execplugin.FromConfig(plugins)

	if !errors.Is(err, domain.ErrUnknownEvent) || !strings.Contains(err.Error(), "audit") || !strings.Contains(err.Error(), "nope.event") {
		t.Fatalf("err = %v", err)
	}
}

func TestFromConfigKnownEvents(t *testing.T) {
	plugins := []config.Plugin{{Name: "audit", Command: []string{"audit", "-v"}, Timeout: 3 * time.Second, Events: []string{string(domain.EventToolBefore)}}}

	built, err := execplugin.FromConfig(plugins)

	if err != nil {
		t.Fatal(err)
	}
	if len(built) != 1 || built[0].Name() != "audit" || len(built[0].Subscribes()) != 1 || built[0].Subscribes()[0] != domain.EventToolBefore {
		t.Fatalf("built = %v", built)
	}
}

func TestFromConfigBuildsFilter(t *testing.T) {
	plugins := []config.Plugin{{
		Name: "notify", Command: []string{"notify"}, Events: []string{string(domain.EventRunTransitionAfter)},
		Transition: []string{domain.TransitionApprovePlan}, From: []string{domain.PhaseAwaitingApproval}, To: []string{domain.PhaseWorking},
	}}

	built, err := execplugin.FromConfig(plugins)

	if err != nil {
		t.Fatal(err)
	}
	matcher, ok := built[0].(domain.Matcher)
	if !ok {
		t.Fatal("plugin is not a Matcher")
	}
	hit, _ := json.Marshal(domain.TransitionPayload{Event: domain.TransitionApprovePlan, From: domain.PhaseAwaitingApproval, To: domain.PhaseWorking})
	miss, _ := json.Marshal(domain.TransitionPayload{Event: domain.TransitionApprovePlan, From: domain.PhaseWorking, To: domain.PhaseWorking})
	if !matcher.Matches(domain.Envelope{Payload: hit}) || matcher.Matches(domain.Envelope{Payload: miss}) || matcher.Matches(domain.Envelope{Payload: json.RawMessage("nope")}) {
		t.Fatal("filter did not match as expected")
	}
}

func TestMatchesEmptyFilterSkipsDecoding(t *testing.T) {
	plugin := execplugin.ExecPlugin{}

	if !plugin.Matches(domain.Envelope{Payload: json.RawMessage("nope")}) {
		t.Fatal("empty filter must match")
	}
}

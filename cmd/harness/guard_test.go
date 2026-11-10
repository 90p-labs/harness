package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/metric/noop"

	"github.com/90p-labs/harness/internal/guard/application/guardplugin"
	guarddomain "github.com/90p-labs/harness/internal/guard/domain"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type domainStop struct {
	handoff error
	phase   domain.Phase
}

func (d domainStop) DecideStop(context.Context, bool) (bool, string, error) {
	phase := d.phase
	if phase == "" {
		phase = domain.PhaseReviewing
	}
	run := domain.Run{Phase: phase, Step: 1, HandoffStep: 1}
	verdict, _ := domain.DecideStop(run, domain.Policy{MaxStopBlocks: 2}, false, d.handoff)
	return verdict.Block, verdict.Reason, nil
}

func TestStopDenyReasonSentToHostIsShort(t *testing.T) {
	long := strings.Repeat("x", 500)
	type tc struct {
		phase   domain.Phase
		handoff error
	}
	cases := map[string]tc{
		"handoff missing": {domain.PhaseReviewing, domain.ErrHandoffMissing},
		"handoff nil":     {domain.PhaseReviewing, nil},
	}
	for _, phase := range []domain.Phase{domain.PhasePlanning, domain.PhaseWorking, domain.PhaseReviewing, domain.PhaseShipping} {
		run := domain.Run{Phase: phase, Step: 12345}
		head := "Phase: " + string(phase) + "\nStep: 12345\n"
		sections := "## Next\n- a\n## Open findings\n- a\n## Unreviewed commits\n- a\n## Decisions\n- a\n"
		bodies := map[string]string{
			"over the limit":   strings.Repeat("x", domain.HandoffLimit*100),
			"no phase":         "Step: 1\n",
			"long phase":       "Phase: " + long + "\nStep: 12345\n" + sections,
			"awaiting phase":   "Phase: awaiting_approval\nStep: 12345\n" + sections,
			"releasing phase":  "Phase: releasing\nStep: 12345\n" + sections,
			"no step":          "Phase: " + string(phase) + "\n",
			"long step":        "Phase: " + string(phase) + "\nStep: " + long + "\n" + sections,
			"no section":       head,
			"empty next":       head + "## Next\n## Open findings\n- a\n## Unreviewed commits\n- a\n## Decisions\n- a\n",
			"empty unreviewed": head + "## Next\n- a\n## Open findings\n- a\n## Unreviewed commits\n## Decisions\n- a\n",
		}
		for name, body := range bodies {
			refusal := domain.CheckHandoff(run, []byte(body))
			if refusal == nil {
				t.Fatalf("%s: want a refusal", name)
			}
			cases[string(phase)+" "+name] = tc{phase, refusal}
		}
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			plugins, err := guardplugin.Plugins(nil, guardplugin.Deps{
				ReadFile: os.ReadFile,
				Stop:     domainStop{handoff: c.handoff, phase: c.phase},
				Meter:    noop.NewMeterProvider().Meter("test"),
			})
			if err != nil {
				t.Fatal(err)
			}
			var verdict shareddomain.Verdict
			for _, p := range plugins {
				if p.Name() == "guard."+guarddomain.RuleStopHoldsTurn {
					verdict, err = p.Handle(context.Background(), shareddomain.Envelope{Event: shareddomain.EventTurnStopping, Payload: []byte(`{}`)})
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if len(verdict.Reason) > 200 {
				t.Errorf("host reason has %d characters, want at most 200: %q", len(verdict.Reason), verdict.Reason)
			}
			for _, want := range []string{string(c.phase), "no background agent or shell task", "in the background"} {
				if !strings.Contains(verdict.Reason, want) {
					t.Errorf("reason %q lacks %q", verdict.Reason, want)
				}
			}
		})
	}
}

package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
)

const handoffBody = `# Handoff

Phase: working
Step: 7

## Next
- write the tests

## Open findings
- none

## Unreviewed commits
- abc1234

## Decisions
- none
`

func TestCheckHandoff(t *testing.T) {
	run := domain.Run{Phase: domain.PhaseWorking, Step: 7}
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"ready", handoffBody, ""},
		{"over the limit", handoffBody + strings.Repeat("x", domain.HandoffLimit), fmt.Sprintf("handoff.md has %d bytes, the limit is %d", len(handoffBody)+domain.HandoffLimit, domain.HandoffLimit)},
		{"wrong phase", strings.Replace(handoffBody, "Phase: working", "Phase: reviewing", 1), "phase"},
		{"wrong step", strings.Replace(handoffBody, "Step: 7", "Step: 6", 1), "step"},
		{"missing phase header", strings.Replace(handoffBody, "Phase: working\n", "", 1), "Phase"},
		{"missing step header", strings.Replace(handoffBody, "Step: 7\n", "", 1), "Step"},
		{"missing section", strings.Replace(handoffBody, "## Decisions\n- none\n", "", 1), "## Decisions"},
		{"empty section", strings.Replace(handoffBody, "## Next\n- write the tests\n", "## Next\n", 1), "## Next"},
		{"section line without a dash", strings.Replace(handoffBody, "- abc1234", "abc1234", 1), "## Unreviewed commits"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := domain.CheckHandoff(run, []byte(c.raw))
			if c.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want a cause with %q", err, c.want)
			}
		})
	}
	if err := domain.CheckHandoff(run, nil); !errors.Is(err, domain.ErrHandoffMissing) {
		t.Fatalf("nil content got %v", err)
	}
}

package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestHandoffSections(t *testing.T) {
	want := []string{"## Next", "## Open findings", "## Unreviewed commits", "## Decisions"}
	got := domain.HandoffSections()
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
	got[0] = "changed"
	if again := domain.HandoffSections(); again[0] != want[0] {
		t.Fatalf("HandoffSections returns its own slice: got %q after a caller edit", again[0])
	}
}

func TestCheckHandoffListsEveryProblem(t *testing.T) {
	run := domain.Run{Phase: domain.PhaseWorking, Step: 7}
	const sectionsProblem = `needs ## Next, ## Open findings, ## Unreviewed commits, ## Decisions, each with a "- " line`
	raw := "Phase: reviewing\nStep: 6\n## Next\n- a\n## Open findings\n## Unreviewed commits\n- a\n"
	err := domain.CheckHandoff(run, []byte(raw))
	if err == nil {
		t.Fatal("want a refusal")
	}
	want := "handoff.md: phase reviewing, want working; step 6, want 7; " +
		`needs ## Open findings, ## Decisions, each with a "- " line`
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}

	err = domain.CheckHandoff(run, []byte("# Handoff\n"))
	want = "handoff.md: no Phase line; no Step line; " + sectionsProblem
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}

	multibyte := strings.Repeat("é", 20)
	err = domain.CheckHandoff(run, []byte("Phase: "+multibyte+"\nStep: "+multibyte+"\n"))
	clipped := strings.Repeat("é", 6) + "..."
	for _, part := range []string{"phase " + clipped + ", want working", "step " + clipped + ", want 7", sectionsProblem} {
		if err == nil || !strings.Contains(err.Error(), part) {
			t.Fatalf("got %v, want a problem %q", err, part)
		}
	}

	// One ASCII byte first puts byte 12 inside a rune, so clip steps back to 11.
	offset := "x" + multibyte
	err = domain.CheckHandoff(run, []byte("Phase: "+offset+"\nStep: 7\n"))
	part := "phase x" + strings.Repeat("é", 5) + "..., want working"
	if err == nil || !strings.Contains(err.Error(), part) || !utf8.ValidString(err.Error()) {
		t.Fatalf("got %v, want a valid UTF-8 problem %q", err, part)
	}
}

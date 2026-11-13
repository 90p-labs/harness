package domain_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/90p-labs/harness/internal/panel/domain"
)

func ids(cards []domain.Card) []string {
	out := make([]string, len(cards))
	for i, c := range cards {
		out[i] = c.ID
	}
	return out
}

func column(t *testing.T, b domain.Board, name string) domain.Column {
	t.Helper()
	for _, c := range b.Columns {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no column %q", name)
	return domain.Column{}
}

func TestBoardColumns(t *testing.T) {
	cards := []domain.Card{
		{ID: "r1", Phase: "planning"},
		{ID: "r2", Phase: "awaiting_approval"},
		{ID: "r3", Phase: "working"},
		{ID: "r4", Phase: "reviewing"},
		{ID: "r5", Phase: "shipping"},
		{ID: "r6", Phase: "releasing"},
		{ID: "r7", Phase: "observing"},
		{ID: "r8", Phase: "done"},
		{ID: "r9", Phase: "abandoned"},
		{ID: "r10", Phase: "blocked", BlockedFrom: "reviewing"},
	}
	b := domain.NewBoard(cards)
	want := map[string][]string{
		"Planning":  {"r1"},
		"Approval":  {"r2"},
		"Working":   {"r3"},
		"Reviewing": {"r4", "r10"},
		"Shipping":  {"r5", "r6", "r7"},
		"Done":      {"r9", "r8"},
	}
	if len(b.Columns) != 6 {
		t.Fatalf("columns = %d, want 6", len(b.Columns))
	}
	for name, ids0 := range want {
		got := ids(column(t, b, name).Cards)
		if !slices.Equal(got, ids0) {
			t.Errorf("%s = %v, want %v", name, got, ids0)
		}
	}
}

func TestBoardDoneKeepsTwentyMostRecent(t *testing.T) {
	var cards []domain.Card
	for i := 0; i < 25; i++ {
		cards = append(cards, domain.Card{ID: fmt.Sprintf("2026%02d", i), Phase: "done"})
	}
	done := column(t, domain.NewBoard(cards), "Done").Cards
	if len(done) != 20 || done[0].ID != "202624" || done[19].ID != "202605" {
		t.Errorf("done = %v", ids(done))
	}
}

func TestBoardVersionTracksCards(t *testing.T) {
	a := domain.NewBoard([]domain.Card{{ID: "r1", Phase: "working", Version: domain.Version{Plan: "p", Run: "1"}}})
	same := domain.NewBoard([]domain.Card{{ID: "r1", Phase: "working", Version: domain.Version{Plan: "p", Run: "1"}}})
	diff := domain.NewBoard([]domain.Card{{ID: "r1", Phase: "working", Version: domain.Version{Plan: "p", Run: "2"}}})
	if a.Version != same.Version || a.Version == diff.Version || a.Version == "" {
		t.Errorf("versions %q %q %q", a.Version, same.Version, diff.Version)
	}
}

func TestVersionParse(t *testing.T) {
	v := domain.Version{Plan: "abc", Run: "def"}
	if v.String() != "abc.def" || domain.ParseVersion(v.String()) != v {
		t.Errorf("roundtrip failed: %q", v.String())
	}
	for _, bad := range []string{"", "abc", "a.b.c", "."} {
		if p := domain.ParseVersion(bad); p.Plan != "" || p.Run != "" {
			t.Errorf("ParseVersion(%q) = %+v", bad, p)
		}
	}
	if len(domain.Fingerprint("a", "b")) != 12 {
		t.Error("fingerprint length")
	}
}

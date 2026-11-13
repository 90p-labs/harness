package domain_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

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

func TestColumnKeys(t *testing.T) {
	cards := []domain.Card{
		{ID: "r1", Phase: "awaiting_approval"},
		{ID: "r2", Phase: "blocked", BlockedFrom: "working"},
	}
	b := domain.NewBoard(cards)
	wantKeys := map[string]string{
		"Planning":  "planning",
		"Approval":  "approval",
		"Working":   "working",
		"Reviewing": "reviewing",
		"Shipping":  "shipping",
		"Done":      "done",
	}
	for name, key := range wantKeys {
		if got := column(t, b, name).Key; got != key {
			t.Errorf("%s key = %q, want %q", name, got, key)
		}
	}
	if !column(t, b, "Approval").NeedsYou() {
		t.Error("Approval with a card should need you")
	}
	if column(t, b, "Working").NeedsYou() || column(t, b, "Planning").NeedsYou() {
		t.Error("only the Approval column needs you")
	}
	if column(t, domain.NewBoard(nil), "Approval").NeedsYou() {
		t.Error("empty Approval column should not need you")
	}
	if !(domain.Card{Phase: "blocked"}).Blocked() {
		t.Error("blocked phase should be Blocked")
	}
	if (domain.Card{Phase: "working"}).Blocked() {
		t.Error("working phase should not be Blocked")
	}
}

func TestCardPRNumber(t *testing.T) {
	for url, want := range map[string]string{
		"https://github.com/o/r/pull/129":  "129",
		"https://github.com/o/r/pull/129/": "129",
		"https://example.test/pr/1":        "1",
		"https://example.test/pr/abc":      "",
		"":                                 "",
	} {
		if got := (domain.Card{PRURL: url}).PRNumber(); got != want {
			t.Errorf("PRNumber(%q) = %q, want %q", url, got, want)
		}
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

func TestCardSummary(t *testing.T) {
	id := "20261113-133205-r3-panel-live-of-the-served-control-pane"
	ts := time.Date(2026, 11, 13, 13, 32, 5, 0, time.Local)
	tests := []struct {
		name         string
		card         domain.Card
		now          time.Time
		title, short string
		age          string
	}{
		{"minutes", domain.Card{ID: id, Title: "Live panel\nsecond line"}, ts.Add(12 * time.Minute), "Live panel", "20261113-133205", "12m"},
		{"hours", domain.Card{ID: id, Title: "t"}, ts.Add(3*time.Hour + 59*time.Minute), "t", "20261113-133205", "3h"},
		{"days", domain.Card{ID: id}, ts.Add(50 * time.Hour), "", "20261113-133205", "2d"},
		{"just now", domain.Card{ID: id}, ts.Add(20 * time.Second), "", "20261113-133205", "0m"},
		{"future", domain.Card{ID: id}, ts.Add(-time.Hour), "", "20261113-133205", "0m"},
		{"long", domain.Card{ID: id, Title: strings.Repeat("é", 100)}, ts, strings.Repeat("é", 79) + "…", "20261113-133205", "0m"},
		{"exact 80", domain.Card{ID: id, Title: strings.Repeat("a", 80)}, ts, strings.Repeat("a", 80), "20261113-133205", "0m"},
		{"no timestamp", domain.Card{ID: "r1", Title: "x"}, ts, "x", "", ""},
		{"bad timestamp", domain.Card{ID: "20269999-999999-x"}, ts, "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.card.ShortTitle(); got != tc.title {
				t.Errorf("ShortTitle = %q, want %q", got, tc.title)
			}
			if got := tc.card.ShortID(); got != tc.short {
				t.Errorf("ShortID = %q, want %q", got, tc.short)
			}
			if got := tc.card.Age(tc.now); got != tc.age {
				t.Errorf("Age = %q, want %q", got, tc.age)
			}
		})
	}
}

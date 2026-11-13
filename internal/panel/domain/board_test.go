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

const soloRepo = "aaaaaaaaaaaa"

// solo wraps cards in a listing of one repo.
func solo(cards []domain.Card) domain.Listing {
	stamped := make([]domain.Card, len(cards))
	for i, c := range cards {
		c.Repo = soloRepo
		stamped[i] = c
	}
	return domain.Listing{Repos: []domain.Repo{{Key: soloRepo, Name: "harness"}}, Cards: stamped}
}

// column finds a column of the board.
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
	b := domain.NewBoard(solo(cards), "")
	want := map[string][]string{
		"Planning":  {"r1"},
		"Approval":  {"r2"},
		"Working":   {"r3"},
		"Reviewing": {"r4", "r10"},
		"Shipping":  {"r7", "r6", "r5"},
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
	done := column(t, domain.NewBoard(solo(cards), ""), "Done").Cards
	if len(done) != 20 || done[0].ID != "202624" || done[19].ID != "202605" {
		t.Errorf("done = %v", ids(done))
	}
}

func TestBoardVersionTracksCards(t *testing.T) {
	a := domain.NewBoard(solo([]domain.Card{{ID: "r1", Phase: "working", Version: domain.Version{Plan: "p", Run: "1"}}}), "")
	same := domain.NewBoard(solo([]domain.Card{{ID: "r1", Phase: "working", Version: domain.Version{Plan: "p", Run: "1"}}}), "")
	diff := domain.NewBoard(solo([]domain.Card{{ID: "r1", Phase: "working", Version: domain.Version{Plan: "p", Run: "2"}}}), "")
	if a.Version != same.Version || a.Version == diff.Version || a.Version == "" {
		t.Errorf("versions %q %q %q", a.Version, same.Version, diff.Version)
	}
}

func TestBoardVersionTracksRepoNameAndPath(t *testing.T) {
	build := func(name, path string) string {
		return domain.NewBoard(domain.Listing{Repos: []domain.Repo{{Key: "aaaaaaaaaaaa", Name: name, Worktree: path}}}, "").Version
	}
	base := build("api", "/w/api")
	if base != build("api", "/w/api") {
		t.Error("same repo, different versions")
	}
	if base == build("api", "/w/api-moved") {
		t.Error("a moved worktree keeps the version")
	}
	if base == build("api2", "/w/api") {
		t.Error("a renamed repo keeps the version")
	}
}

func TestColumnKeys(t *testing.T) {
	cards := []domain.Card{
		{ID: "r1", Phase: "awaiting_approval"},
		{ID: "r2", Phase: "blocked", BlockedFrom: "working"},
	}
	b := domain.NewBoard(solo(cards), "")
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

func repoKeys(b domain.Board) []string {
	out := make([]string, len(b.Repos))
	for i, r := range b.Repos {
		out[i] = r.Key
	}
	return out
}

func repoItem(t *testing.T, b domain.Board, key string) domain.RepoItem {
	t.Helper()
	for _, r := range b.Repos {
		if r.Key == key {
			return r
		}
	}
	t.Fatalf("no repo item %q", key)
	return domain.RepoItem{}
}

// shownRepos lists the repo key of every card of every column.
func shownRepos(b domain.Board) map[string]int {
	out := map[string]int{}
	for _, col := range b.Columns {
		for _, c := range col.Cards {
			out[c.Repo]++
		}
	}
	return out
}

func TestBoardRepos(t *testing.T) {
	const api, harness, empty, broken, stray = "aaaaaaaaaaaa", "bbbbbbbbbbbb", "cccccccccccc", "dddddddddddd", "zzzzzzzzzzzz"
	repos := []domain.Repo{
		{Key: api, Name: "api", Worktree: "/w/api"},
		{Key: harness, Name: "harness", Worktree: "/w/harness"},
		{Key: empty, Name: "empty", Worktree: "/w/empty"},
		{Key: broken, Name: "broken", Worktree: "/w/broken"},
	}
	var cards []domain.Card
	for i := 0; i < 25; i++ { // api: 25 done runs, the newest is 20261125-000000
		cards = append(cards, domain.Card{Repo: api, ID: fmt.Sprintf("202611%02d-000000", i+1), Phase: "done"})
	}
	cards = append(cards,
		domain.Card{Repo: harness, ID: "20261113-221148-r5", Phase: "working"},
		domain.Card{Repo: harness, ID: "20261112-101010-old", Phase: "awaiting_approval"},
		domain.Card{Repo: harness, ID: "20261001-000000-d1", Phase: "done"},
		domain.Card{Repo: harness, ID: "20261002-000000-d2", Phase: "done"},
		domain.Card{Repo: harness, ID: "20261003-000000-d3", Phase: "abandoned"},
		domain.Card{Repo: stray, ID: "20261120-000000-stray", Phase: "working"}, // no repo in the listing
	)
	problems := map[string]string{broken: "state.db has no runs table"}
	listing := domain.Listing{Repos: repos, Cards: cards, Problems: problems}

	b := domain.NewBoard(listing, "")

	t.Run("one set of six columns holds the cards of every repo", func(t *testing.T) {
		if len(b.Columns) != 6 {
			t.Fatalf("columns = %d, want 6", len(b.Columns))
		}
		if got := ids(column(t, b, "Working").Cards); !slices.Equal(got, []string{"20261113-221148-r5"}) {
			t.Errorf("working = %v", got)
		}
		if got := ids(column(t, b, "Approval").Cards); !slices.Equal(got, []string{"20261112-101010-old"}) {
			t.Errorf("approval = %v", got)
		}
		if b.Filter != "" {
			t.Errorf("filter = %q", b.Filter)
		}
		// Each repo keeps its own Done limit: api 20 of 25, harness all 3.
		if got := shownRepos(b); got[api] != 20 || got[harness] != 5 || got[stray] != 0 || len(got) != 2 {
			t.Errorf("cards shown by repo = %v, want api 20 and harness 5, no stray", got)
		}
		done := column(t, b, "Done").Cards
		if len(done) != 23 || done[0].ID != "20261125-000000" || done[19].ID != "20261106-000000" || done[20].ID != "20261003-000000-d3" {
			t.Errorf("done = %d cards %v", len(done), ids(done))
		}
	})

	t.Run("cards name their repo", func(t *testing.T) {
		for _, col := range b.Columns {
			for _, c := range col.Cards {
				want := map[string]string{api: "api", harness: "harness"}[c.Repo]
				if c.RepoName != want || want == "" {
					t.Errorf("card %s of %s has RepoName %q, want %q", c.ID, c.Repo, c.RepoName, want)
				}
			}
		}
	})

	t.Run("a column sorts newest first by run id, then by repo key", func(t *testing.T) {
		tied := domain.Listing{Repos: repos[:2], Cards: []domain.Card{
			{Repo: harness, ID: "20261113-same", Phase: "working"},
			{Repo: api, ID: "20261113-same", Phase: "working"},
			{Repo: harness, ID: "20261114-newer", Phase: "working"},
			{Repo: api, ID: "20261101-older", Phase: "working"},
		}}
		var got []string
		for _, c := range column(t, domain.NewBoard(tied, ""), "Working").Cards {
			got = append(got, c.ID+"@"+c.Repo[:1])
		}
		if want := []string{"20261114-newer@b", "20261113-same@a", "20261113-same@b", "20261101-older@a"}; !slices.Equal(got, want) {
			t.Errorf("working = %v, want %v", got, want)
		}
	})

	t.Run("the repo list keeps every repo, newest run first, with unfiltered counts", func(t *testing.T) {
		if want := []string{api, harness, empty, broken}; !slices.Equal(repoKeys(b), want) {
			t.Fatalf("repos = %v, want %v (an empty repo sorts last, in listing order)", repoKeys(b), want)
		}
		a, h := repoItem(t, b, api), repoItem(t, b, harness)
		if a.Name != "api" || a.Path != "/w/api" || a.Count != 20 || a.NeedsYou {
			t.Errorf("api item = %+v", a)
		}
		if h.Name != "harness" || h.Path != "/w/harness" || h.Count != 5 || !h.NeedsYou {
			t.Errorf("harness item = %+v", h)
		}
		if e := repoItem(t, b, empty); e.Count != 0 || e.NeedsYou {
			t.Errorf("empty item = %+v", e)
		}
		if b.Total != 25 || !b.NeedsYou {
			t.Errorf("total = %d needs you = %v, want 25 and true", b.Total, b.NeedsYou)
		}
		newer := append(append([]domain.Card(nil), cards...), domain.Card{Repo: harness, ID: "20261201-000000-new", Phase: "planning"})
		if got := repoKeys(domain.NewBoard(domain.Listing{Repos: repos, Cards: newer}, "")); got[0] != harness {
			t.Errorf("repos = %v, want the repo of the newest run first", got)
		}
	})

	t.Run("problem notices read as repo name and problem", func(t *testing.T) {
		if len(b.Problems) != 1 || b.Problems[0].Line() != "broken: state.db has no runs table" {
			t.Errorf("problems = %+v", b.Problems)
		}
	})

	t.Run("a filter keeps the cards and the problem of its repo only", func(t *testing.T) {
		for _, tc := range []struct {
			filter   string
			cards    map[string]int
			problems int
		}{
			{api, map[string]int{api: 20}, 0},
			{harness, map[string]int{harness: 5}, 0},
			{empty, map[string]int{}, 0},
			{broken, map[string]int{}, 1},
			{stray, map[string]int{}, 0},
		} {
			f := domain.NewBoard(listing, tc.filter)
			if f.Filter != tc.filter {
				t.Errorf("filter %s: Filter = %q", tc.filter, f.Filter)
			}
			got := shownRepos(f)
			if len(got) != len(tc.cards) {
				t.Errorf("filter %s: cards by repo = %v, want %v", tc.filter, got, tc.cards)
			}
			for k, n := range tc.cards {
				if got[k] != n {
					t.Errorf("filter %s: cards by repo = %v, want %v", tc.filter, got, tc.cards)
				}
			}
			if len(f.Problems) != tc.problems || (tc.problems == 1 && f.Problems[0].Repo != broken) {
				t.Errorf("filter %s: problems = %+v, want %d", tc.filter, f.Problems, tc.problems)
			}
			// The repo list, the counts and the marks ignore the filter.
			if !slices.Equal(repoKeys(f), repoKeys(b)) || f.Total != b.Total || f.NeedsYou != b.NeedsYou ||
				repoItem(t, f, api).Count != 20 || repoItem(t, f, harness).Count != 5 || !repoItem(t, f, harness).NeedsYou {
				t.Errorf("filter %s changed the repo list: %+v total %d", tc.filter, f.Repos, f.Total)
			}
			if len(f.Columns) != 6 {
				t.Errorf("filter %s: %d columns", tc.filter, len(f.Columns))
			}
		}
		// The filtered Done keeps the same 20 of its repo as the full board.
		if got := column(t, domain.NewBoard(listing, api), "Done").Cards; len(got) != 20 || got[0].ID != "20261125-000000" {
			t.Errorf("api done = %d cards %v", len(got), ids(got))
		}
	})

	t.Run("the version tracks every part of the board", func(t *testing.T) {
		same := domain.NewBoard(listing, "")
		if b.Version == "" || b.Version != same.Version {
			t.Errorf("same listing, versions %q %q", b.Version, same.Version)
		}
		moved := append([]domain.Card(nil), cards...)
		moved[25].Phase = "reviewing"
		other := append([]domain.Card(nil), cards...)
		other[25].Repo = api // the same run id under another repo
		for name, v := range map[string]string{
			"a run phase change":      domain.NewBoard(domain.Listing{Repos: repos, Cards: moved, Problems: problems}, "").Version,
			"a card of another repo":  domain.NewBoard(domain.Listing{Repos: repos, Cards: other, Problems: problems}, "").Version,
			"no problem line":         domain.NewBoard(domain.Listing{Repos: repos, Cards: cards}, "").Version,
			"a filter":                domain.NewBoard(listing, api).Version,
			"another filter":          domain.NewBoard(listing, harness).Version,
			"a repo list without one": domain.NewBoard(domain.Listing{Repos: repos[:3], Cards: cards, Problems: problems}, "").Version,
		} {
			if v == b.Version {
				t.Errorf("%s left the version", name)
			}
		}
		if domain.NewBoard(listing, api).Version == domain.NewBoard(listing, harness).Version {
			t.Error("two filters share a version")
		}
	})
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

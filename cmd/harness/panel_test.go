package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	paneldomain "github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/panel/infrastructure/registry"
	planmodule "github.com/90p-labs/harness/internal/plan"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

var panelURLLine = regexp.MustCompile(`panel: (http://127\.0\.0\.1:\d+/)`)

func panelGet(t *testing.T, url, host string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func TestPanelServe(t *testing.T) {
	clearHarnessEnv(t)
	home := t.TempDir()
	t.Setenv(envHome, home)
	repoDir := initRepo(t)
	wantKey := repoKeyOf(t, repoDir)
	const prompt = "Add the login page"
	if code, _, stderr := execute(t, commandRun, "start", prompt); code != exitOK {
		t.Fatalf("start: code = %d, stderr = %q", code, stderr)
	}
	previous := version
	version = testBuildVersion
	t.Cleanup(func() { version = previous })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errOut lockedBuffer
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, []string{"panel", "serve", "--port", "0"}, strings.NewReader(""), &out, &errOut)
	}()

	var url string
	deadline := time.Now().Add(10 * time.Second)
	for url == "" {
		if m := panelURLLine.FindStringSubmatch(out.String()); m != nil {
			url = m[1]
			break
		}
		select {
		case code := <-done:
			t.Fatalf("serve ended early: code = %d, stderr = %q", code, errOut.String())
		case <-time.After(20 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("no URL line, stdout = %q, stderr = %q", out.String(), errOut.String())
		}
	}

	if status, body := panelGet(t, url, ""); status != http.StatusOK || !strings.Contains(body, prompt) {
		t.Errorf("board: status = %d, want 200 with the prompt %q", status, prompt)
	}
	if status, body := panelGet(t, url+"healthz", ""); status != http.StatusOK || !strings.Contains(body, testBuildVersion) {
		t.Errorf("healthz: status = %d, body = %q", status, body)
	} else {
		var health map[string]any
		if err := json.Unmarshal([]byte(body), &health); err != nil || health["repo"] != nil {
			t.Errorf("healthz = %q (%v), want no repo field", body, err)
		}
	}
	if repos, err := registry.New(home, nil).List(context.Background()); err != nil || len(repos) != 1 || repos[0].Key != wantKey {
		t.Errorf("serve registered %+v (%v), want the repo %s", repos, err, wantKey)
	}
	if status, _ := panelGet(t, url+"runs/no-such-run", ""); status != http.StatusNotFound {
		t.Errorf("unknown run: status = %d, want 404", status)
	}
	if status, _ := panelGet(t, url, "evil.com"); status != http.StatusForbidden {
		t.Errorf("foreign host: status = %d, want 403", status)
	}

	cancel()
	select {
	case code := <-done:
		if code != exitOK {
			t.Errorf("code = %d, stderr = %q, want %d", code, errOut.String(), exitOK)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not stop on cancel")
	}
}

// fakeRunRepo is a run repository over a map. It counts the list calls.
type fakeRunRepo struct {
	runs      map[string]domain.Run
	lists     int
	listed    []domain.Run
	activeErr error
	active    string
	// activeCalls counts the Active calls, which are the status reads of the active run.
	activeCalls atomic.Int32
}

func (f *fakeRunRepo) Active(context.Context, domain.RunKey) (domain.Run, error) {
	f.activeCalls.Add(1)
	if f.activeErr != nil {
		return domain.Run{}, f.activeErr
	}
	if run, ok := f.runs[f.active]; ok {
		return run, nil
	}
	return domain.Run{}, domain.ErrRunNotFound
}

func (f *fakeRunRepo) Get(_ context.Context, id string) (domain.Run, error) {
	run, ok := f.runs[id]
	if !ok {
		return domain.Run{}, domain.ErrRunNotFound
	}
	return run, nil
}

func (f *fakeRunRepo) ListByPhase(context.Context, string, []domain.Phase) ([]domain.Run, error) {
	f.lists++
	return f.listed, nil
}

func (f *fakeRunRepo) Save(_ context.Context, run domain.Run) (domain.Run, error) { return run, nil }

type noProofs struct{}

func (noProofs) Passed(context.Context) (map[int]bool, error) { return nil, nil }

func TestPanelPagesDeletedWorktreeShowsProblems(t *testing.T) {
	clearHarnessEnv(t)
	const id = "20260101-000000-gone"
	gone := filepath.Join(t.TempDir(), "deleted-worktree")
	empty := t.TempDir()
	key := domain.RunKey{Repo: "repo", Worktree: empty}
	repo := &fakeRunRepo{runs: map[string]domain.Run{id: {ID: id, Key: domain.RunKey{Repo: "repo", Worktree: gone}, Phase: domain.Phase("working")}}}
	pages := newPanelPages(status.NewHandler(repo), key,
		planmodule.NewPages(noProofs{}, func(context.Context) (string, error) { return "", nil }))
	served := paneldomain.Served{Version: "p.r", Base: "/repos/" + testRepoKey + "/runs/" + id + "/regions/"}

	card := paneldomain.Card{ID: id, Phase: "working", Worktree: gone}
	for _, region := range []string{"", "problems"} {
		out, err := pages.render(context.Background(), card, region, served)
		if err != nil {
			t.Fatalf("deleted worktree, region %q: err = %v, want a page with its problems", region, err)
		}
		if !strings.Contains(string(out), "worktree") {
			t.Errorf("deleted worktree, region %q: no problem row in %q", region, out)
		}
	}

	existing := paneldomain.Card{ID: id, Phase: "working", Worktree: empty}
	if _, err := pages.Page(context.Background(), existing, served); !errors.Is(err, paneldomain.ErrNoPlan) {
		t.Errorf("worktree without plan.md: err = %v, want ErrNoPlan", err)
	}
}

func TestPanelPagesDeletedWorktreeOfActiveRun(t *testing.T) {
	clearHarnessEnv(t)
	const id = "20260101-000000-gone-active"
	gone := filepath.Join(t.TempDir(), "deleted-worktree")
	key := domain.RunKey{Repo: "repo", Worktree: t.TempDir()}
	repo := &fakeRunRepo{
		runs:   map[string]domain.Run{id: {ID: id, Key: domain.RunKey{Repo: "repo", Worktree: gone}, Phase: domain.Phase("working")}},
		active: id,
	}
	pages := newPanelPages(status.NewHandler(repo), key,
		planmodule.NewPages(noProofs{}, func(context.Context) (string, error) { return "", nil }))
	served := paneldomain.Served{Version: "p.r", Base: "/repos/" + testRepoKey + "/runs/" + id + "/regions/"}
	card := paneldomain.Card{ID: id, Phase: "working", Worktree: gone}
	for _, region := range []string{"", "problems"} {
		out, err := pages.render(context.Background(), card, region, served)
		if err != nil {
			t.Fatalf("active run, deleted worktree, region %q: err = %v, want a page with its problems", region, err)
		}
		if !strings.Contains(string(out), "worktree") {
			t.Errorf("active run, deleted worktree, region %q: no problem row in %q", region, out)
		}
	}
}

func TestPanelServedHome(t *testing.T) {
	clearHarnessEnv(t)
	raw, err := os.ReadFile("../../internal/plan/domain/testdata/valid.md")
	if err != nil {
		t.Fatal(err)
	}
	const id = "20260101-000000-home"
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(planPath(dir, id)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath(dir, id), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	run := domain.Run{ID: id, Key: domain.RunKey{Repo: "repo", Worktree: dir}, Phase: domain.Phase("working")}
	repo := &fakeRunRepo{runs: map[string]domain.Run{id: run}}
	pages := newPanelPages(status.NewHandler(repo), domain.RunKey{Repo: "repo", Worktree: dir},
		planmodule.NewPages(noProofs{}, func(context.Context) (string, error) { return "", nil }))
	card := paneldomain.Card{ID: id, Phase: "working", Worktree: dir}
	served := paneldomain.Served{Version: "p.r", Base: "/repos/" + testRepoKey + "/runs/" + id + "/regions/"}
	out, err := pages.Page(context.Background(), card, served)
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if !strings.Contains(string(out), `<a href="/">Runs</a>`) {
		t.Errorf("served run page lacks the Runs crumb link to the board at /")
	}
}

// TestPanelPagesServedRunPageCarriesEvents renders a run page through the composition root and wants the SSE wiring.
func TestPanelPagesServedRunPageCarriesEvents(t *testing.T) {
	clearHarnessEnv(t)
	raw, err := os.ReadFile("../../internal/plan/domain/testdata/valid.md")
	if err != nil {
		t.Fatal(err)
	}
	const id = "20260101-000000-events"
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(planPath(dir, id)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath(dir, id), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	run := domain.Run{ID: id, Key: domain.RunKey{Repo: "repo", Worktree: dir}, Phase: domain.Phase("working")}
	repo := &fakeRunRepo{runs: map[string]domain.Run{id: run}}
	pages := newPanelPages(status.NewHandler(repo), domain.RunKey{Repo: "repo", Worktree: dir},
		planmodule.NewPages(noProofs{}, func(context.Context) (string, error) { return "", nil }))
	card := paneldomain.Card{ID: id, Phase: "working", Worktree: dir}
	events := "/events?repo=" + testRepoKey + "&run=" + id
	served := paneldomain.Served{Version: "p.r", Base: "/repos/" + testRepoKey + "/runs/" + id + "/regions/", Events: events}
	out, err := pages.Page(context.Background(), card, served)
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if !strings.Contains(string(out), `sse-connect="`+html.EscapeString(events)+`"`) {
		t.Errorf("served run page lacks sse-connect to %s", events)
	}
}

func TestPlanPartStatErrorChangesVersion(t *testing.T) {
	dir := t.TempDir()
	missing := planPart(filepath.Join(dir, "plan.md"))
	// A plan.md under a regular file fails with ENOTDIR, not ErrNotExist.
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := planPart(filepath.Join(file, "plan.md")); got == missing {
		t.Errorf("stat error fingerprint = %q, want one that differs from the no plan one", got)
	}
}

func TestPanelPagesActiveRunErrorSurfaces(t *testing.T) {
	clearHarnessEnv(t)
	const id = "20260101-000000-active"
	dir := t.TempDir()
	boom := errors.New("state db down")
	repo := &fakeRunRepo{
		runs:      map[string]domain.Run{id: {ID: id, Key: domain.RunKey{Repo: "repo", Worktree: dir}, Phase: domain.Phase("working")}},
		activeErr: boom,
	}
	if err := os.MkdirAll(filepath.Dir(planPath(dir, id)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath(dir, id), []byte("# plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pages := newPanelPages(status.NewHandler(repo), domain.RunKey{Repo: "repo", Worktree: dir},
		planmodule.NewPages(noProofs{}, func(context.Context) (string, error) { return "", nil }))
	card := paneldomain.Card{ID: id, Phase: "working", Worktree: dir}
	served := paneldomain.Served{Version: "p.r", Base: "/repos/" + testRepoKey + "/runs/" + id + "/regions/"}
	if _, err := pages.render(context.Background(), card, "problems", served); !errors.Is(err, boom) {
		t.Errorf("render: err = %v, want the status error", err)
	}
}

func TestPanelCardBranchNeedsCleanParse(t *testing.T) {
	raw, err := os.ReadFile("../../internal/plan/domain/testdata/valid.md")
	if err != nil {
		t.Fatal(err)
	}
	const id = "20260101-000000-branch"
	for name, tc := range map[string]struct {
		text string
		want string
	}{
		"clean":  {string(raw), "feat/ping-query"},
		"broken": {strings.Replace(string(raw), "### 2. Ping query", "### Ping query", 1), ""},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Dir(planPath(dir, id)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(planPath(dir, id), []byte(tc.text), 0o644); err != nil {
				t.Fatal(err)
			}
			card := buildCard(status.Status{ID: id, Worktree: dir, Phase: domain.Phase("working")}, nil)
			if card.Branch != tc.want {
				t.Errorf("branch = %q, want %q", card.Branch, tc.want)
			}
		})
	}
}

func TestPlanPartChangesWhenWalkRecovers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(path, []byte("# plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Log("the folder stays readable for this user, so the walk cannot fail here")
		return
	}
	failing := planPart(path)
	if err := os.Chmod(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if recovered := planPart(path); recovered == failing {
		t.Errorf("version %q did not change after the walk recovered", recovered)
	}
}

func TestPanelRunsGetReadsOneRun(t *testing.T) {
	clearHarnessEnv(t)
	repo := &fakeRunRepo{runs: map[string]domain.Run{
		"a":     {ID: "a", Key: domain.RunKey{Repo: "repo", Worktree: t.TempDir()}, Prompt: "first", Phase: domain.Phase("working")},
		"other": {ID: "other", Key: domain.RunKey{Repo: "elsewhere", Worktree: t.TempDir()}, Phase: domain.Phase("working")},
	}}
	runs := newPanelRuns(status.NewHandler(repo), "repo")

	card, err := runs.Get(context.Background(), "a")
	if err != nil || card.ID != "a" || card.Title != "first" {
		t.Fatalf("Get a = %+v, %v", card, err)
	}
	if repo.lists != 0 {
		t.Errorf("Get listed the runs %d times, want one single read", repo.lists)
	}
	for _, id := range []string{"missing", "other"} {
		if _, err := runs.Get(context.Background(), id); !errors.Is(err, paneldomain.ErrNotFound) {
			t.Errorf("Get %s: err = %v, want ErrNotFound", id, err)
		}
	}
	if repo.lists != 0 {
		t.Errorf("Get listed the runs %d times, want none", repo.lists)
	}
}

func TestPanelRunsReadsPlanOnlyWhenItChanges(t *testing.T) {
	clearHarnessEnv(t)
	raw, err := os.ReadFile("../../internal/plan/domain/testdata/valid.md")
	if err != nil {
		t.Fatal(err)
	}
	const id = "20260101-000000-cache"
	dir := t.TempDir()
	path := planPath(dir, id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	run := domain.Run{ID: id, Key: domain.RunKey{Repo: "repo", Worktree: dir}, Phase: domain.Phase("working")}
	repo := &fakeRunRepo{runs: map[string]domain.Run{id: run}, listed: []domain.Run{run}}
	var reads atomic.Int32
	runs := newPanelRuns(status.NewHandler(repo), "repo")
	runs.branches.read = func(p string) ([]byte, error) {
		reads.Add(1)
		return os.ReadFile(p)
	}
	ctx := context.Background()

	var first paneldomain.Card
	for range 2 {
		cards, err := runs.List(ctx)
		if err != nil || len(cards) != 1 {
			t.Fatalf("List = %+v, %v", cards, err)
		}
		first = cards[0]
	}
	if first.Branch != "feat/ping-query" {
		t.Fatalf("branch = %q, want feat/ping-query", first.Branch)
	}
	if _, err := runs.Get(ctx, id); err != nil {
		t.Fatal(err)
	}
	if got := reads.Load(); got != 1 {
		t.Fatalf("plan read %d times for an unchanged plan, want 1", got)
	}

	changed := strings.Replace(string(raw), "feat/ping-query", "feat/pong-query-longer", 1)
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	cards, err := runs.List(ctx)
	if err != nil || len(cards) != 1 {
		t.Fatalf("List = %+v, %v", cards, err)
	}
	if cards[0].Branch != "feat/pong-query-longer" {
		t.Errorf("branch = %q after the change, want feat/pong-query-longer", cards[0].Branch)
	}
	if got := reads.Load(); got != 2 {
		t.Errorf("plan read %d times after one change, want 2", got)
	}
	if cards[0].Version.Plan == first.Version.Plan {
		t.Errorf("plan part did not change")
	}
}

type stubProofs struct {
	passed map[int]bool
	calls  int
}

func (s *stubProofs) Passed(context.Context) (map[int]bool, error) {
	s.calls++
	return s.passed, nil
}

func emptyCommit(t *testing.T, dir, message string) {
	t.Helper()
	gitIn(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false",
		"commit", "-q", "--allow-empty", "-m", message)
}

func TestPanelRunVersion(t *testing.T) {
	clearHarnessEnv(t)
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	emptyCommit(t, dir, "one")
	const active, other = "20260101-000000-active", "20260101-000001-other"
	key := domain.RunKey{Repo: "repo", Worktree: dir}
	repo := &fakeRunRepo{
		runs: map[string]domain.Run{
			active: {ID: active, Key: key, Phase: domain.Phase("working")},
			other:  {ID: other, Key: key, Phase: domain.Phase("working")},
		},
		listed: []domain.Run{{ID: active, Key: key, Phase: domain.Phase("working")}},
		active: active,
	}
	proofs := &stubProofs{passed: map[int]bool{1: true}}
	now := time.Unix(1000, 0)
	runs := newPanelRuns(status.NewHandler(repo), "repo").withLive(key, proofs, func() time.Time { return now })
	ctx := context.Background()
	runPart := func(id string) string {
		t.Helper()
		card, err := runs.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return card.Version.Run
	}

	base := runPart(active)
	if proofs.calls != 1 {
		t.Fatalf("proof reader called %d times for one Get, want 1", proofs.calls)
	}

	emptyCommit(t, dir, "two")
	if got := runPart(active); got != base {
		t.Errorf("run part changed inside the 1s cache window")
	}
	if proofs.calls != 1 {
		t.Errorf("proof reader called %d times inside the cache window, want 1", proofs.calls)
	}
	now = now.Add(2 * time.Second)
	afterCommit := runPart(active)
	if afterCommit == base {
		t.Errorf("run part did not change with the branch head")
	}

	proofs.passed = map[int]bool{1: true, 2: true}
	now = now.Add(2 * time.Second)
	afterProof := runPart(active)
	if afterProof == afterCommit {
		t.Errorf("run part did not change with the proofs of the active run")
	}

	// A run that is not the active one reads no proofs, so a proof change leaves its part alone.
	calls := proofs.calls
	otherBase := runPart(other)
	proofs.passed = map[int]bool{1: false}
	now = now.Add(2 * time.Second)
	if got := runPart(other); got != otherBase {
		t.Errorf("run part of an inactive run changed with the proofs")
	}
	if proofs.calls != calls {
		t.Errorf("proof reader called for an inactive run")
	}

	// List skips the cache, the head and the proofs.
	calls = proofs.calls
	cards, err := runs.List(ctx)
	if err != nil || len(cards) != 1 {
		t.Fatalf("List = %+v, %v", cards, err)
	}
	if proofs.calls != calls {
		t.Errorf("List read the proofs")
	}
	if cards[0].Version.Run == afterProof {
		t.Errorf("List run part carries the head and the proofs, want the status version only")
	}
}

// newLiveTestRuns builds panelRuns over run a (active) and run b, with a fake head and the stub proofs.
func newLiveTestRuns(t *testing.T, head func(ctx context.Context, dir string) (string, error), now *time.Time) (panelRuns, *fakeRunRepo) {
	t.Helper()
	key := domain.RunKey{Repo: "repo", Worktree: "/wt"}
	repo := &fakeRunRepo{
		runs: map[string]domain.Run{
			"a": {ID: "a", Key: key, Phase: domain.Phase("working")},
			"b": {ID: "b", Key: domain.RunKey{Repo: "repo", Worktree: "/other"}, Phase: domain.Phase("working")},
		},
		active: "a",
	}
	proofs := &stubProofs{passed: map[int]bool{1: true}}
	runs := newPanelRuns(status.NewHandler(repo), "repo").withLive(key, proofs, func() time.Time { return *now })
	runs.live.head = head
	return runs, repo
}

func TestPanelLiveCacheErrors(t *testing.T) {
	ctx := context.Background()
	run := func(t *testing.T, runs panelRuns, id string) string {
		t.Helper()
		card, err := runs.Get(ctx, id)
		if err != nil {
			t.Error(err)
		}
		return card.Version.Run
	}

	t.Run("a failed head is not cached", func(t *testing.T) {
		now := time.Unix(1000, 0)
		fail := true
		runs, _ := newLiveTestRuns(t, func(context.Context, string) (string, error) {
			if fail {
				return "", errors.New("git down")
			}
			return "h1", nil
		}, &now)
		failed := run(t, runs, "a")
		fail = false
		if got := run(t, runs, "a"); got == failed {
			t.Errorf("the failed head stayed cached inside the TTL")
		}
	})

	t.Run("a failed head keeps the last good head", func(t *testing.T) {
		now := time.Unix(1000, 0)
		fail := false
		runs, _ := newLiveTestRuns(t, func(context.Context, string) (string, error) {
			if fail {
				return "", errors.New("git down")
			}
			return "h1", nil
		}, &now)
		good := run(t, runs, "a")
		fail = true
		now = now.Add(2 * time.Second)
		if got := run(t, runs, "a"); got != good {
			t.Errorf("run version flipped when the head read failed")
		}
	})

	t.Run("a failed status keeps the last good proofs", func(t *testing.T) {
		now := time.Unix(1000, 0)
		runs, repo := newLiveTestRuns(t, func(context.Context, string) (string, error) { return "h1", nil }, &now)
		good := run(t, runs, "a")
		repo.activeErr = errors.New("state busy")
		now = now.Add(2 * time.Second)
		if got := run(t, runs, "a"); got != good {
			t.Errorf("run version flipped when the status read failed")
		}
	})

	t.Run("stale entries are pruned", func(t *testing.T) {
		now := time.Unix(1000, 0)
		runs, repo := newLiveTestRuns(t, func(context.Context, string) (string, error) { return "h1", nil }, &now)
		for i := range 20 {
			id := "x" + strconv.Itoa(i)
			repo.runs[id] = domain.Run{ID: id, Key: domain.RunKey{Repo: "repo", Worktree: "/x"}, Phase: domain.Phase("working")}
			run(t, runs, id)
		}
		now = now.Add(time.Hour)
		run(t, runs, "a")
		runs.live.mu.Lock()
		n := len(runs.live.entries)
		runs.live.mu.Unlock()
		if n != 1 {
			t.Errorf("%d entries kept after an hour, want 1", n)
		}
	})

	t.Run("a slow head does not block another run", func(t *testing.T) {
		now := time.Unix(1000, 0)
		release := make(chan struct{})
		runs, _ := newLiveTestRuns(t, func(_ context.Context, dir string) (string, error) {
			if dir == "/wt" {
				<-release
			}
			return "h1", nil
		}, &now)
		var slow sync.WaitGroup
		slow.Add(1)
		go func() { defer slow.Done(); run(t, runs, "a") }()
		time.Sleep(50 * time.Millisecond)
		done := make(chan struct{})
		go func() { run(t, runs, "b"); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("a slow git head of one run held up another run")
		}
		close(release)
		slow.Wait()
	})

	t.Run("a missing worktree gives an empty head that is cached", func(t *testing.T) {
		now := time.Unix(1000, 0)
		var heads int
		missing := filepath.Join(t.TempDir(), "gone")
		runs, _ := newLiveTestRuns(t, func(ctx context.Context, _ string) (string, error) {
			heads++
			return git.Head(ctx, missing)
		}, &now)
		first := run(t, runs, "a")
		if want := paneldomain.Fingerprint("0", "", "1=true"); first != want {
			t.Errorf("run part = %q, want the one of an empty head %q", first, want)
		}
		if got := run(t, runs, "a"); got != first {
			t.Errorf("run part changed inside the TTL")
		}
		if heads != 1 {
			t.Errorf("head read %d times inside the TTL, want 1", heads)
		}
	})

	t.Run("parallel callers after expiry read once", func(t *testing.T) {
		now := time.Unix(1000, 0)
		var heads atomic.Int32
		started := make(chan struct{}, 64)
		release := make(chan struct{})
		runs, repo := newLiveTestRuns(t, func(context.Context, string) (string, error) {
			heads.Add(1)
			started <- struct{}{}
			<-release
			return "h1", nil
		}, &now)
		runs.live.entries = map[string]liveEntry{"a": {at: now, head: "old"}}
		now = now.Add(2 * time.Second)
		const callers = 8
		var wg sync.WaitGroup
		for range callers {
			wg.Add(1)
			go func() { defer wg.Done(); run(t, runs, "a") }()
		}
		<-started
		time.Sleep(50 * time.Millisecond)
		close(release)
		wg.Wait()
		if got := heads.Load(); got != 1 {
			t.Errorf("%d head reads for %d parallel callers, want 1", got, callers)
		}
		// Get looks the run up by id, so every Active call is a live status read.
		if got := repo.activeCalls.Load(); got != 1 {
			t.Errorf("%d status reads of the active run for %d parallel callers, want 1", got, callers)
		}
	})
}

func TestPanelServeHelpNamesPort(t *testing.T) {
	clearHarnessEnv(t)
	code, stdout, _ := execute(t, "panel", "serve", "--help")
	if code != exitOK || !strings.Contains(stdout, "--port") {
		t.Fatalf("code = %d, stdout = %q, want the --port flag", code, stdout)
	}
}

func repoKeyOf(t *testing.T, dir string) string {
	t.Helper()
	commonDir, err := git.CommonDir(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return paneldomain.RepoKey(commonDir)
}

// startLegacyPanel runs a panel as the old layout did: its lock, panel.json
// with a repo field and a healthz in <common dir>/.harness. It returns the
// folder and a channel that closes when the process has exited.
func startLegacyPanel(t *testing.T, repoDir string) (dir string, exited <-chan struct{}) {
	t.Helper()
	dir = filepath.Join(repoDir, ".git", ".harness")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), legacyChildEnv+"="+dir, panelChildEnv+"=")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() { // the child is ours: stop it when a test failed early
		select {
		case <-done:
		default:
			_ = cmd.Process.Kill()
			<-done
		}
	})
	line := make(chan string, 1)
	go func() {
		buf := make([]byte, 16)
		n, _ := stdout.Read(buf)
		line <- string(buf[:n])
	}()
	select {
	case got := <-line:
		if !strings.HasPrefix(got, "ready") {
			t.Fatalf("legacy panel said %q, want ready", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("legacy panel did not start")
	}
	return dir, done
}

func waitExit(t *testing.T, exited <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not exit", what)
	}
}

// testRepoKey is a fixture repo key in the form of the repo routes.
const testRepoKey = "0123456789ab"

func TestPanelUserState(t *testing.T) {
	clearHarnessEnv(t)
	home := filepath.Join(t.TempDir(), "user")
	t.Setenv(envHome, home)
	t.Setenv(panelChildEnv, "1")
	t.Setenv(panelChildVersionEnv, testBuildVersion)
	first := initRepo(t)
	t.Cleanup(func() { execute(t, "panel", "stop") })

	code, started, stderr := execute(t, "panel", "start")
	m := panelURLLine.FindStringSubmatch(started)
	if code != exitOK || m == nil {
		t.Fatalf("start: code = %d, stdout = %q, stderr = %q", code, started, stderr)
	}
	if info, err := os.Stat(home); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("user dir: %v, mode %v, want 0700", err, info)
	}
	for _, name := range []string{"panel.json", "panel.lock", "panel.log"} {
		info, err := os.Stat(filepath.Join(home, name))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("%s: %v, %v, want a 0600 file in the user dir", name, err, info)
		}
	}
	if _, err := os.Stat(filepath.Join(first, ".git", ".harness", "panel.json")); !os.IsNotExist(err) {
		t.Errorf("panel.json in the repo: %v, want none", err)
	}
	raw, err := os.ReadFile(filepath.Join(home, "panel.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil || state["repo"] != nil {
		t.Errorf("panel.json = %s (%v), want no repo field", raw, err)
	}
	repos, err := registry.New(home, nil).List(context.Background())
	if err != nil || len(repos) != 1 || repos[0].Key != repoKeyOf(t, first) {
		t.Fatalf("registry after start = %+v (%v), want the repo %s", repos, err, repoKeyOf(t, first))
	}

	// Another repo sees the same panel and registers beside the first.
	second := initRepo(t)
	if code, stdout, _ := execute(t, "panel", "status"); code != exitOK || !strings.HasPrefix(stdout, "panel: "+m[1]+" pid ") {
		t.Fatalf("status from another repo: code = %d, stdout = %q", code, stdout)
	}
	if code, again, _ := execute(t, "panel", "start"); code != exitOK || again != started {
		t.Fatalf("start from another repo: code = %d, stdout = %q, want %q", code, again, started)
	}
	repos, err = registry.New(home, nil).List(context.Background())
	if err != nil || len(repos) != 2 {
		t.Fatalf("registry after the second start = %+v (%v), want 2 repos", repos, err)
	}
	if got := repoKeyOf(t, second); repos[0].Key != got {
		t.Errorf("newest repo = %s, want %s", repos[0].Key, got)
	}

	// Status and stop from a repo that is not registered leave the registry alone.
	third := initRepo(t)
	if code, _, _ := execute(t, "panel", "status"); code != exitOK {
		t.Fatalf("status from the third repo: code = %d", code)
	}
	repos, err = registry.New(home, nil).List(context.Background())
	if err != nil || len(repos) != 2 {
		t.Fatalf("registry after status = %+v (%v), want 2 repos", repos, err)
	}
	for _, r := range repos {
		if r.Key == repoKeyOf(t, third) {
			t.Errorf("status registered %s", r.Key)
		}
	}
	execute(t, "panel", "stop")
	repos, err = registry.New(home, nil).List(context.Background())
	if err != nil || len(repos) != 2 {
		t.Fatalf("registry after stop = %+v (%v), want 2 repos", repos, err)
	}
	for _, r := range repos {
		if r.Key == repoKeyOf(t, third) {
			t.Errorf("stop registered %s", r.Key)
		}
	}

}

func TestPanelStartOtherVersion(t *testing.T) {
	clearHarnessEnv(t)
	t.Setenv(envHome, t.TempDir())
	t.Setenv(panelChildEnv, "1")
	t.Setenv(panelChildVersionEnv, "0.0.1-old") // execute runs as testBuildVersion
	initRepo(t)
	t.Cleanup(func() { execute(t, "panel", "stop") })

	code, started, stderr := execute(t, "panel", "start")
	m := panelURLLine.FindStringSubmatch(started)
	if code != exitOK || m == nil {
		t.Fatalf("start: code = %d, stdout = %q, stderr = %q", code, started, stderr)
	}
	code, other, _ := execute(t, "panel", "start")
	want := "panel: running version 0.0.1-old, run harness panel stop to switch to " + testBuildVersion + "\npanel: " + m[1] + "\n"
	if code != exitOK || other != want {
		t.Fatalf("second start: code = %d, stdout = %q, want %q", code, other, want)
	}

	// A start that reuses the panel of another version still registers its repo.
	second := initRepo(t)
	if code, out, _ := execute(t, "panel", "start"); code != exitOK || out != want {
		t.Fatalf("start from another repo: code = %d, stdout = %q, want %q", code, out, want)
	}
	repos, err := registry.New(os.Getenv(envHome), nil).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range repos {
		found = found || r.Key == repoKeyOf(t, second)
	}
	if !found {
		t.Errorf("registry = %+v, want the repo %s", repos, repoKeyOf(t, second))
	}
}

func TestPanelLegacyStop(t *testing.T) {
	setup := func(t *testing.T) (repoDir string) {
		clearHarnessEnv(t)
		t.Setenv(envHome, filepath.Join(t.TempDir(), "user"))
		t.Setenv(panelChildEnv, "1")
		repoDir = initRepo(t)
		t.Cleanup(func() { execute(t, "panel", "stop") })
		return repoDir
	}
	legacyFiles := func(t *testing.T, dir string) {
		t.Helper()
		for _, name := range []string{"panel.json", "panel.lock", "panel.log"} {
			if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
				t.Errorf("legacy %s after the stop: %v, want it removed", name, err)
			}
		}
	}

	t.Run("start stops a legacy panel", func(t *testing.T) {
		repoDir := setup(t)
		dir, exited := startLegacyPanel(t, repoDir)
		if err := os.WriteFile(filepath.Join(dir, "panel.log"), []byte("old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "state.db"), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := execute(t, "panel", "start")
		if code != exitOK || panelURLLine.FindStringSubmatch(stdout) == nil {
			t.Fatalf("start: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
		}
		waitExit(t, exited, "the legacy panel")
		legacyFiles(t, dir)
		if _, err := os.Stat(filepath.Join(dir, "state.db")); err != nil {
			t.Errorf("state.db: %v, want it kept", err)
		}
	})

	t.Run("stop stops a legacy panel", func(t *testing.T) {
		repoDir := setup(t)
		dir, exited := startLegacyPanel(t, repoDir)
		if code, stdout, stderr := execute(t, "panel", "stop"); code != exitOK || stdout != "panel: stopped\n" {
			t.Fatalf("stop: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
		}
		waitExit(t, exited, "the legacy panel")
		legacyFiles(t, dir)
	})
}

func TestPanelLifecycle(t *testing.T) {
	clearHarnessEnv(t)
	home := t.TempDir()
	t.Setenv(envHome, home)
	t.Setenv(panelChildEnv, "1")
	t.Setenv(panelChildVersionEnv, testBuildVersion)
	initRepo(t)
	t.Cleanup(func() { execute(t, "panel", "stop") })
	state := filepath.Join(home, "panel.json")

	if code, stdout, _ := execute(t, "panel", "status"); code != exitRuntime || stdout != "panel: stopped\n" {
		t.Fatalf("status before start: code = %d, stdout = %q", code, stdout)
	}

	code, first, stderr := execute(t, "panel", "start")
	m := panelURLLine.FindStringSubmatch(first)
	if code != exitOK || m == nil {
		t.Fatalf("start: code = %d, stdout = %q, stderr = %q", code, first, stderr)
	}
	if status, _ := panelGet(t, m[1]+"healthz", ""); status != http.StatusOK {
		t.Fatalf("healthz: status = %d", status)
	}
	if _, err := os.Stat(state); err != nil {
		t.Fatalf("panel.json: %v", err)
	}

	if code, second, _ := execute(t, "panel", "start"); code != exitOK || second != first {
		t.Fatalf("second start: code = %d, stdout = %q, want %q", code, second, first)
	}
	if code, stdout, _ := execute(t, "panel", "status"); code != exitOK || !strings.HasPrefix(stdout, "panel: "+m[1]+" pid ") {
		t.Fatalf("status: code = %d, stdout = %q", code, stdout)
	}

	if code, stdout, stderr := execute(t, "panel", "stop"); code != exitOK || stdout != "panel: stopped\n" {
		t.Fatalf("stop: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("panel.json after stop: %v", err)
	}
	if code, _, _ := execute(t, "panel", "status"); code != exitRuntime {
		t.Fatalf("status after stop: code = %d, want %d", code, exitRuntime)
	}
}

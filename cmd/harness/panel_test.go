package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	paneldomain "github.com/90p-labs/harness/internal/panel/domain"
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
	repoDir := initRepo(t)
	wantRepo, _, err := git.RepoKey(context.Background(), repoDir)
	if err != nil {
		t.Fatal(err)
	}
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
		var health struct{ Repo string }
		if err := json.Unmarshal([]byte(body), &health); err != nil || health.Repo != wantRepo {
			t.Errorf("healthz repo = %q (%v), want the repo key %q", health.Repo, err, wantRepo)
		}
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
}

func (f *fakeRunRepo) Active(context.Context, domain.RunKey) (domain.Run, error) {
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
	served := paneldomain.Served{Version: "p.r", Base: "/runs/" + id + "/regions/"}

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
	served := paneldomain.Served{Version: "p.r", Base: "/runs/" + id + "/regions/"}
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
	served := paneldomain.Served{Version: "p.r", Base: "/runs/" + id + "/regions/"}
	out, err := pages.Page(context.Background(), card, served)
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if !strings.Contains(string(out), `<a href="/">Runs</a>`) {
		t.Errorf("served run page lacks the Runs crumb link to the board at /")
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
	served := paneldomain.Served{Version: "p.r", Base: "/runs/" + id + "/regions/"}
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
			card := panelCard(status.Status{ID: id, Worktree: dir, Phase: domain.Phase("working")})
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

func TestPanelServeHelpNamesPort(t *testing.T) {
	clearHarnessEnv(t)
	code, stdout, _ := execute(t, "panel", "serve", "--help")
	if code != exitOK || !strings.Contains(stdout, "--port") {
		t.Fatalf("code = %d, stdout = %q, want the --port flag", code, stdout)
	}
}

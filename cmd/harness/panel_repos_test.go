package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/pflag"

	paneldomain "github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/panel/infrastructure/registry"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

// fakeRegistry lists the repos it holds, with no pruning, so a test controls what the adapter sees.
type fakeRegistry struct {
	mu    sync.Mutex
	repos []paneldomain.Repo
}

func (f *fakeRegistry) Register(_ context.Context, repo paneldomain.Repo) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos = append(f.repos, repo)
	return nil
}

func (f *fakeRegistry) List(context.Context) ([]paneldomain.Repo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]paneldomain.Repo(nil), f.repos...), nil
}

func (f *fakeRegistry) set(repos ...paneldomain.Repo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repos = repos
}

// repoInfo is the registry entry of the repo at dir, with the default state file.
func repoInfo(t *testing.T, dir string) paneldomain.Repo {
	t.Helper()
	commonDir, err := git.CommonDir(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return paneldomain.Repo{
		Key: paneldomain.RepoKey(commonDir), Name: filepath.Base(dir), CommonDir: commonDir, Worktree: dir,
		StateDB: filepath.Join(commonDir, ".harness", "state.db"), UsedAt: time.Now().UTC(),
	}
}

// repoWithRun makes a git repo with one started run, and returns its folder and run id.
func repoWithRun(t *testing.T, prompt string) (dir, runID string) {
	t.Helper()
	dir = initRepo(t)
	code, stdout, stderr := execute(t, commandRun, "start", prompt)
	if code != exitOK {
		t.Fatalf("run start in %s: code = %d, stderr = %q", dir, code, stderr)
	}
	return dir, startedRunID(t, stdout)
}

func closed(db *dbal.DB) bool {
	return db == nil || db.PingContext(context.Background()) != nil
}

func TestPanelRepos(t *testing.T) {
	clearHarnessEnv(t)
	t.Setenv(envHome, t.TempDir())
	ctx := context.Background()

	alphaDir, alphaRun := repoWithRun(t, "Add the alpha page")
	betaDir, betaRun := repoWithRun(t, "Add the beta page")
	// A repo whose state file exists but holds no tables: the panel must not migrate it.
	brokenDir := initRepo(t)
	broken := repoInfo(t, brokenDir)
	if err := os.MkdirAll(filepath.Dir(broken.StateDB), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken.StateDB, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// A repo with no state file: the panel must not create its folder.
	goneDir := initRepo(t)
	gone := repoInfo(t, goneDir)
	alpha, beta := repoInfo(t, alphaDir), repoInfo(t, betaDir)

	reg := &fakeRegistry{}
	reg.set(alpha, beta, broken, gone)
	var log lockedBuffer
	repos := newPanelRepos(reg, pflag.NewFlagSet("test", pflag.ContinueOnError), "", &log)
	opens := map[string]int{}
	dbs := map[string]*dbal.DB{}
	open := repos.open
	repos.open = func(ctx context.Context, cfg config.Config, commonDir string) (*dbal.DB, error) {
		opens[commonDir]++
		db, err := open(ctx, cfg, commonDir)
		dbs[commonDir] = db
		return db, err
	}

	var listing paneldomain.Listing
	for range 2 {
		var err error
		if listing, err = repos.List(ctx); err != nil {
			t.Fatalf("List: %v", err)
		}
	}
	if len(listing.Repos) != 4 {
		t.Errorf("repos = %d, want 4 repos, the broken ones too", len(listing.Repos))
	}
	titles := map[string]string{}
	for _, card := range listing.Cards {
		titles[card.Repo] = card.Title
	}
	if len(listing.Cards) != 2 || titles[alpha.Key] != "Add the alpha page" || titles[beta.Key] != "Add the beta page" {
		t.Errorf("cards = %+v, want one run of alpha and one of beta", listing.Cards)
	}
	if len(listing.Problems) != 2 || listing.Problems[broken.Key] == "" || listing.Problems[gone.Key] == "" {
		t.Errorf("problems = %v, want one line each for the broken repo and the repo with no state file", listing.Problems)
	}
	for _, c := range []paneldomain.Repo{alpha, beta, broken} {
		if opens[c.CommonDir] != 1 {
			t.Errorf("opens of %s = %d after two lists, want 1", c.Name, opens[c.CommonDir])
		}
	}
	if opens[gone.CommonDir] != 0 {
		t.Errorf("opens of the repo with no state file = %d, want 0", opens[gone.CommonDir])
	}
	if _, err := os.Stat(filepath.Dir(gone.StateDB)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("state folder of the repo with no state file: %v, want it not created", err)
	}
	if out := log.String(); strings.Count(out, "panel: repo failed "+brokenDir) != 1 || !strings.Contains(out, "panel: repo failed "+goneDir) {
		t.Errorf("log = %q, want one repo failed line for each broken repo, not one for each list", out)
	}
	check, err := dbal.Open(ctx, "sqlite:"+broken.StateDB, "")
	if err != nil {
		t.Fatal(err)
	}
	var tables int
	if err := check.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master`).Scan(&tables); err != nil || tables != 0 {
		t.Errorf("tables of the broken repo = %d (%v), want 0: the panel does not migrate", tables, err)
	}
	_ = check.Close()

	// A run of each repo reads from its own store.
	if card, err := repos.Get(ctx, paneldomain.RunRef{Repo: beta.Key, ID: betaRun}); err != nil || card.Title != "Add the beta page" {
		t.Errorf("Get beta = %+v (%v)", card, err)
	}
	if _, err := repos.Get(ctx, paneldomain.RunRef{Repo: beta.Key, ID: alphaRun}); !errors.Is(err, paneldomain.ErrNotFound) {
		t.Errorf("Get of the alpha run id in beta: %v, want ErrNotFound", err)
	}
	if _, err := repos.Get(ctx, paneldomain.RunRef{Repo: gone.Key, ID: alphaRun}); !errors.Is(err, paneldomain.ErrNotFound) {
		t.Errorf("Get in a broken repo: %v, want ErrNotFound", err)
	}

	// A repo that leaves the registry has its store closed, and the others stay open.
	reg.set(alpha, broken, gone)
	if listing, err = repos.List(ctx); err != nil || len(listing.Repos) != 3 {
		t.Fatalf("List after beta left = %d repos (%v), want 3", len(listing.Repos), err)
	}
	if !closed(dbs[beta.CommonDir]) {
		t.Error("store of the repo that left the registry is open, want it closed")
	}
	if closed(dbs[alpha.CommonDir]) {
		t.Error("store of alpha is closed, want it open")
	}

	if err := repos.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for _, c := range []paneldomain.Repo{alpha, beta, broken} {
		if !closed(dbs[c.CommonDir]) {
			t.Errorf("store of %s is open after Close, want every store closed", c.Name)
		}
	}
	if err := repos.Close(); err != nil {
		t.Errorf("second Close: %v, want nil", err)
	}
}

// countingRegistry counts the reads of the registry it wraps.
type countingRegistry struct {
	*fakeRegistry
	lists atomic.Int64
}

func (c *countingRegistry) List(ctx context.Context) ([]paneldomain.Repo, error) {
	c.lists.Add(1)
	return c.fakeRegistry.List(ctx)
}

func TestPanelReposGetUsesTheLoadedRepos(t *testing.T) {
	clearHarnessEnv(t)
	t.Setenv(envHome, t.TempDir())
	ctx := context.Background()
	dir, runID := repoWithRun(t, "Add the alpha page")
	alpha := repoInfo(t, dir)
	reg := &countingRegistry{fakeRegistry: &fakeRegistry{}}
	reg.set(alpha)
	repos := newPanelRepos(reg, pflag.NewFlagSet("test", pflag.ContinueOnError), "", nil)
	t.Cleanup(func() { _ = repos.Close() })
	ref := paneldomain.RunRef{Repo: alpha.Key, ID: runID}

	// A direct link before any board list reads the registry to find the repo.
	if _, err := repos.Get(ctx, ref); err != nil {
		t.Fatalf("Get before List: %v", err)
	}
	if _, err := repos.List(ctx); err != nil {
		t.Fatal(err)
	}
	before := reg.lists.Load()
	card, err := repos.Get(ctx, ref)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := repos.Page(ctx, card, paneldomain.Served{}); err != nil {
		t.Logf("Page: %v", err)
	}
	_, _ = repos.Region(ctx, card, "nope", paneldomain.Served{})
	if got := reg.lists.Load(); got != before {
		t.Errorf("registry reads = %d after Get, Page and Region of a known key, want %d", got, before)
	}
}

func TestPanelReposAfterClose(t *testing.T) {
	clearHarnessEnv(t)
	t.Setenv(envHome, t.TempDir())
	ctx := context.Background()
	dir, runID := repoWithRun(t, "Add the alpha page")
	alpha := repoInfo(t, dir)
	reg := &fakeRegistry{}
	reg.set(alpha)
	repos := newPanelRepos(reg, pflag.NewFlagSet("test", pflag.ContinueOnError), "", nil)
	opens := 0
	open := repos.open
	repos.open = func(ctx context.Context, cfg config.Config, commonDir string) (*dbal.DB, error) {
		opens++
		return open(ctx, cfg, commonDir)
	}
	if err := repos.Close(); err != nil {
		t.Fatal(err)
	}
	card := paneldomain.Card{Repo: alpha.Key}
	if _, err := repos.Get(ctx, paneldomain.RunRef{Repo: alpha.Key, ID: runID}); err == nil {
		t.Error("Get after Close: nil error")
	}
	if _, err := repos.Page(ctx, card, paneldomain.Served{}); err == nil {
		t.Error("Page after Close: nil error")
	}
	if _, err := repos.Region(ctx, card, "x", paneldomain.Served{}); err == nil {
		t.Error("Region after Close: nil error")
	}
	if _, err := repos.List(ctx); err == nil {
		t.Error("List after Close: nil error")
	}
	if opens != 0 {
		t.Errorf("opens after Close = %d, want 0", opens)
	}
}

// Concurrent Lists while a repo leaves the registry and while another moves its worktree must not
// close a store that a List still reads. Run under -race.
func TestPanelReposConcurrentLists(t *testing.T) {
	clearHarnessEnv(t)
	t.Setenv(envHome, t.TempDir())
	ctx := context.Background()
	alphaDir, _ := repoWithRun(t, "Add the alpha page")
	betaDir, _ := repoWithRun(t, "Add the beta page")
	alpha, beta := repoInfo(t, alphaDir), repoInfo(t, betaDir)
	moved := beta
	moved.Worktree = betaDir + "/." // a distinct path forces a reload
	reg := &fakeRegistry{}
	reg.set(alpha, beta)
	repos := newPanelRepos(reg, pflag.NewFlagSet("test", pflag.ContinueOnError), "", nil)
	t.Cleanup(func() { _ = repos.Close() })

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			switch i % 3 {
			case 0:
				reg.set(alpha)
			case 1:
				reg.set(alpha, moved)
			default:
				reg.set(alpha, beta)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	var readers sync.WaitGroup
	for range 4 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range 25 {
				listing, err := repos.List(ctx)
				if err != nil {
					t.Errorf("List: %v", err)
					return
				}
				if len(listing.Problems) != 0 {
					t.Errorf("problems = %v, want none", listing.Problems)
					return
				}
			}
		}()
	}
	readers.Wait()
	close(stop)
	wg.Wait()
}

// A repo whose store opens slowly must not block the calls on a repo that is already loaded.
func TestPanelReposLoadOutsideLock(t *testing.T) {
	clearHarnessEnv(t)
	t.Setenv(envHome, t.TempDir())
	ctx := context.Background()
	alphaDir, alphaRun := repoWithRun(t, "Add the alpha page")
	betaDir, betaRun := repoWithRun(t, "Add the beta page")
	alpha, beta := repoInfo(t, alphaDir), repoInfo(t, betaDir)
	reg := &fakeRegistry{}
	reg.set(alpha, beta)
	repos := newPanelRepos(reg, pflag.NewFlagSet("test", pflag.ContinueOnError), "", nil)
	t.Cleanup(func() { _ = repos.Close() })

	// Load alpha before beta starts to hang.
	if _, err := repos.Get(ctx, paneldomain.RunRef{Repo: alpha.Key, ID: alphaRun}); err != nil {
		t.Fatalf("Get alpha: %v", err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	open := repos.open
	repos.open = func(ctx context.Context, cfg config.Config, commonDir string) (*dbal.DB, error) {
		if commonDir == beta.CommonDir {
			close(entered)
			<-release
		}
		return open(ctx, cfg, commonDir)
	}

	betaDone := make(chan error, 1)
	go func() {
		_, err := repos.Get(ctx, paneldomain.RunRef{Repo: beta.Key, ID: betaRun})
		betaDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the open of beta did not start")
	}

	within := func(name string, call func() error) {
		t.Helper()
		done := make(chan error, 1)
		go func() { done <- call() }()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("%s while beta opens: %v", name, err)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("%s blocked while the store of beta opens, want it to return", name)
			unblock() // let the stuck calls end, so the test does not hang
			<-done
		}
	}
	within("Get of a loaded repo", func() error {
		_, err := repos.Get(ctx, paneldomain.RunRef{Repo: alpha.Key, ID: alphaRun})
		return err
	})
	unblock()
	if err := <-betaDone; err != nil {
		t.Errorf("Get beta: %v", err)
	}
}

// TestPanelRepos signals: the version of a run in repo B follows the plan and the branch head of repo B.
func TestPanelReposSignalsUseTheirRepo(t *testing.T) {
	clearHarnessEnv(t)
	t.Setenv(envHome, t.TempDir())
	ctx := context.Background()
	alphaDir, alphaRun := repoWithRun(t, "Add the alpha page")
	betaDir, betaRun := repoWithRun(t, "Add the beta page")
	alpha, beta := repoInfo(t, alphaDir), repoInfo(t, betaDir)
	reg := &fakeRegistry{}
	reg.set(alpha, beta)
	repos := newPanelRepos(reg, pflag.NewFlagSet("test", pflag.ContinueOnError), "", nil)
	t.Cleanup(func() { _ = repos.Close() })
	// Each read is two seconds after the last, so the one second cache of the head never hides a change.
	var skew time.Duration
	repos.now = func() time.Time { return time.Now().Add(skew) }

	version := func(repo paneldomain.Repo, id string) string {
		t.Helper()
		skew += 2 * time.Second
		card, err := repos.Get(ctx, paneldomain.RunRef{Repo: repo.Key, ID: id})
		if err != nil {
			t.Fatalf("Get %s: %v", repo.Name, err)
		}
		return card.Version.String()
	}
	alphaBefore, betaBefore := version(alpha, alphaRun), version(beta, betaRun)

	writeRunPlan(t, betaDir, betaRun, "# Plan: beta\n")
	if got := version(beta, betaRun); got == betaBefore {
		t.Error("version of beta did not change when its plan.md changed")
	}
	betaPlan := version(beta, betaRun)
	emptyCommit(t, betaDir, "feat: move the beta head")
	if got := version(beta, betaRun); got == betaPlan {
		t.Error("version of beta did not change when its branch head moved")
	}
	if got := version(alpha, alphaRun); got != alphaBefore {
		t.Error("version of alpha changed with the plan and the head of beta")
	}
}

// TestPanelUserWideIgnoresEnvForOtherRepos covers F48: an absolute HARNESS_STATE_DSN of the serve repo must not reach
// the board entries of another repo.
func TestPanelUserWideIgnoresEnvForOtherRepos(t *testing.T) {
	clearHarnessEnv(t)
	home := t.TempDir()
	t.Setenv(envHome, home)

	base := validPlan(t) // read before initRepo changes the folder
	plan := func(word string) string {
		return strings.Replace(base, "The domain gives the word pong.", "The domain gives the word "+word+".", 1)
	}
	alphaDir, alphaRun := repoWithRun(t, "Add the alpha page")
	writeRunPlan(t, alphaDir, alphaRun, plan("alpha"))
	betaDir, betaRun := repoWithRun(t, "Add the beta page")
	writeRunPlan(t, betaDir, betaRun, plan("beta"))
	alphaKey, betaKey := repoKeyOf(t, alphaDir), repoKeyOf(t, betaDir)

	// The env names the store of alpha with an absolute path, as a shell export would.
	t.Setenv("HARNESS_STATE_DSN", "sqlite:"+filepath.Join(alphaDir, ".git", ".harness", "state.db"))
	t.Chdir(alphaDir)
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
	for deadline := time.Now().Add(10 * time.Second); url == ""; {
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

	t.Chdir(betaDir)
	env, err := panelSetup(context.Background(), pflag.NewFlagSet("test", pflag.ContinueOnError))
	if err != nil {
		t.Fatal(err)
	}
	if err := env.register(context.Background()); err != nil {
		t.Fatal(err)
	}

	status, board := panelGet(t, url, "")
	if status != http.StatusOK {
		t.Fatalf("board: status = %d", status)
	}
	for _, want := range []string{"Add the alpha page", "Add the beta page", "/repos/" + alphaKey + "/runs/" + alphaRun, "/repos/" + betaKey + "/runs/" + betaRun} {
		if !strings.Contains(board, want) {
			t.Errorf("board has no %q", want)
		}
	}
	if strings.Contains(board, `class="board-problems"`) {
		t.Errorf("board shows a problem notice list, want none")
	}

	status, page := panelGet(t, url+"repos/"+betaKey+"/runs/"+betaRun, "")
	if status != http.StatusOK {
		t.Fatalf("page of beta: status = %d, want 200", status)
	}
	if !strings.Contains(page, "The domain gives the word beta.") || strings.Contains(page, "the word alpha.") {
		t.Error("page of beta does not show the criteria of its own plan")
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

func TestPanelUserWide(t *testing.T) {
	clearHarnessEnv(t)
	home := t.TempDir()
	t.Setenv(envHome, home)

	// Each repo has a run and a plan with its own first criterion.
	base := validPlan(t) // read before initRepo changes the folder
	plan := func(word string) string {
		return strings.Replace(base, "The domain gives the word pong.", "The domain gives the word "+word+".", 1)
	}
	alphaDir, alphaRun := repoWithRun(t, "Add the alpha page")
	writeRunPlan(t, alphaDir, alphaRun, plan("alpha"))
	betaDir, betaRun := repoWithRun(t, "Add the beta page")
	writeRunPlan(t, betaDir, betaRun, plan("beta"))
	alphaKey, betaKey := repoKeyOf(t, alphaDir), repoKeyOf(t, betaDir)

	// An entry recorded for beta whose worktree now resolves to alpha must show a problem line and serve no run of alpha.
	t.Run("worktree of another repo", func(t *testing.T) {
		moved := repoInfo(t, betaDir)
		moved.Worktree = alphaDir
		reg := &fakeRegistry{}
		reg.set(moved)
		repos := newPanelRepos(reg, pflag.NewFlagSet("test", pflag.ContinueOnError), "", nil)
		defer repos.Close()
		listing, err := repos.List(context.Background())
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(listing.Cards) != 0 {
			t.Errorf("cards = %+v, want none of the other repo", listing.Cards)
		}
		want := "worktree " + alphaDir + " belongs to another repo"
		if got := listing.Problems[moved.Key]; got != want {
			t.Errorf("problem = %q, want %q", got, want)
		}
	})

	// A third repo whose recorded worktree now resolves to alpha. The served board must list it in the problem notices.
	gammaDir, _ := repoWithRun(t, "Add the gamma page")
	gamma := repoInfo(t, gammaDir)
	gamma.Worktree = alphaDir
	foreign := "worktree " + alphaDir + " belongs to another repo"

	// The panel is served from alpha with a state flag. The flag must stay with alpha.
	t.Chdir(alphaDir)
	previous := version
	version = testBuildVersion
	t.Cleanup(func() { version = previous })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errOut lockedBuffer
	done := make(chan int, 1)
	args := []string{"--state-dsn", "sqlite:" + filepath.Join(alphaDir, ".git", ".harness", "state.db"), "panel", "serve", "--port", "0"}
	go func() { done <- run(ctx, args, strings.NewReader(""), &out, &errOut) }()
	var url string
	for deadline := time.Now().Add(10 * time.Second); url == ""; {
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

	// A start from beta registers beta in the same panel.
	t.Chdir(betaDir)
	env, err := panelSetup(context.Background(), pflag.NewFlagSet("test", pflag.ContinueOnError))
	if err != nil {
		t.Fatal(err)
	}
	if err := env.register(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := registry.New(home, nil).Register(context.Background(), gamma); err != nil {
		t.Fatal(err)
	}
	if listed, err := registry.New(home, nil).List(context.Background()); err != nil || len(listed) != 3 {
		t.Fatalf("registry = %+v (%v), want the three repos", listed, err)
	}

	status, board := panelGet(t, url, "")
	if status != http.StatusOK {
		t.Fatalf("board: status = %d", status)
	}
	for _, want := range []string{"Add the alpha page", "Add the beta page", "/repos/" + alphaKey + "/runs/" + alphaRun, "/repos/" + betaKey + "/runs/" + betaRun} {
		if !strings.Contains(board, want) {
			t.Errorf("board has no %q", want)
		}
	}
	// The notice list names the broken repo and its problem, and the cards of the other repos stay.
	if !strings.Contains(board, `<ul class="board-problems">`) {
		t.Error("board has no board-problems notice list")
	}
	if want := "<li>" + gamma.Name + ": " + foreign + "</li>"; !strings.Contains(board, want) {
		t.Errorf("board has no problem line %q", want)
	}
	if strings.Contains(board, "/repos/"+gamma.Key+"/runs/") {
		t.Error("board links a run of the broken repo")
	}

	for _, tc := range []struct {
		key, id, word, other string
	}{
		{alphaKey, alphaRun, "alpha", "beta"},
		{betaKey, betaRun, "beta", "alpha"},
	} {
		status, page := panelGet(t, url+"repos/"+tc.key+"/runs/"+tc.id, "")
		if status != http.StatusOK {
			t.Errorf("page of %s: status = %d, want 200", tc.word, status)
			continue
		}
		if !strings.Contains(page, "The domain gives the word "+tc.word+".") || strings.Contains(page, "the word "+tc.other+".") {
			t.Errorf("page of %s does not show the criteria of its own plan", tc.word)
		}
	}
	if status, _ := panelGet(t, url+"repos/"+alphaKey+"/runs/"+betaRun, ""); status != http.StatusNotFound {
		t.Errorf("run of beta under the alpha key: status = %d, want 404", status)
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

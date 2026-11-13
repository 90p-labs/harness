package panel

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/panel/infrastructure/process"
)

const liveRun = "20261113-133205-live"

// liveRuns is a Runs port whose run changes phase on demand.
type liveRuns struct {
	mu    sync.Mutex
	phase string
}

func (r *liveRuns) set(phase string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phase = phase
}

func (r *liveRuns) card() domain.Card {
	r.mu.Lock()
	defer r.mu.Unlock()
	return domain.Card{ID: liveRun, Title: "live", Phase: r.phase, Version: domain.Version{Plan: "p1", Run: "run-" + r.phase}}
}

func (r *liveRuns) List(context.Context) ([]domain.Card, error) { return []domain.Card{r.card()}, nil }

func (r *liveRuns) Get(_ context.Context, id string) (domain.Card, error) {
	if id != liveRun {
		return domain.Card{}, domain.ErrNotFound
	}
	return r.card(), nil
}

type livePages struct{}

func (livePages) Page(context.Context, domain.Card, domain.Served) ([]byte, error) {
	return []byte("<html></html>"), nil
}

func (livePages) Region(context.Context, domain.Card, string, domain.Served) ([]byte, error) {
	return []byte("<div></div>"), nil
}

func (livePages) HasRegion(string) bool { return true }

func TestLivePhaseChange(t *testing.T) {
	runs := &liveRuns{phase: "working"}
	watcherStopped := make(chan struct{})
	// The hook is slow on purpose: a serve that does not wait for the watcher
	// returns before it, and the check below sees the open signal.
	watcherDone = func() {
		time.Sleep(100 * time.Millisecond)
		close(watcherStopped)
	}
	defer func() { watcherDone = func() {} }()
	serve := NewServe("9.9.9", "/work/repo", runs, livePages{}, process.New(process.Config{Dir: t.TempDir(), Repo: "/work/repo"}))

	ctx, cancel := context.WithCancel(context.Background())
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- serve(ctx, 0, func(url string) { urls <- url }) }()
	var base string
	select {
	case base = <-urls:
	case err := <-done:
		t.Fatalf("serve returned early: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not listen")
	}

	res, err := http.Get(base + "events?run=" + liveRun)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	lines := make(chan string, 16)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	wait := func(want string, within time.Duration) bool {
		timeout := time.After(within)
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					return false
				}
				if strings.TrimSpace(l) == want {
					return true
				}
			case <-timeout:
				return false
			}
		}
	}
	if !wait("event: run-updated", 2*time.Second) {
		t.Fatal("no signal on connect")
	}
	// The connect signal is read. A second one can only come from the change.
	runs.set("reviewing")
	if !wait("event: run-updated", 2*time.Second) {
		t.Fatal("a phase change did not reach the stream within 2s")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return")
	}
	// Serve has returned, so the watcher must already be stopped: its stop
	// signal is closed with no wait.
	select {
	case <-watcherStopped:
	default:
		t.Fatal("the watcher still runs after serve returned")
	}
}

func TestServePublishes(t *testing.T) {
	const repo = "/work/repo"
	dir := t.TempDir()
	proc := process.New(process.Config{Dir: dir, Repo: repo})
	proc.ClaimWait = 50 * time.Millisecond
	serve := NewServe("9.9.9", repo, &liveRuns{phase: "working"}, livePages{}, proc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	urls := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- serve(ctx, 0, func(url string) { urls <- url }) }()
	var base string
	select {
	case base = <-urls:
	case err := <-done:
		t.Fatalf("serve returned early: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not listen")
	}

	// ready runs after the publish, so the state is there and the panel is running.
	inst, err := proc.Status(context.Background())
	if err != nil {
		t.Fatalf("status after ready: %v", err)
	}
	if inst.PID != os.Getpid() || inst.URL != base || inst.Repo != repo || inst.Version != "9.9.9" {
		t.Fatalf("instance = %+v, want pid %d url %s", inst, os.Getpid(), base)
	}

	// the health answer names the pid
	res, err := http.Get(base + "healthz")
	if err != nil {
		t.Fatal(err)
	}
	var health struct {
		PID  int    `json:"pid"`
		Repo string `json:"repo"`
	}
	err = json.NewDecoder(res.Body).Decode(&health)
	res.Body.Close()
	if err != nil || health.PID != os.Getpid() || health.Repo != repo {
		t.Fatalf("healthz = %+v, err = %v", health, err)
	}

	// the Host guard refuses a foreign Host
	foreign, err := http.NewRequest(http.MethodGet, base+"healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign.Host = "evil.example"
	res, err = http.DefaultClient.Do(foreign)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign Host status = %d, want 403", res.StatusCode)
	}

	// a second serve fails on the held lock and does not touch the state
	second := NewServe("9.9.9", repo, &liveRuns{phase: "working"}, livePages{}, proc)
	err = second(context.Background(), 0, func(string) { t.Error("second serve listened") })
	if !errors.Is(err, process.ErrClaimed) {
		t.Fatalf("second serve = %v, want ErrClaimed", err)
	}
	if _, err := proc.Status(context.Background()); err != nil {
		t.Fatalf("status after the second serve: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return")
	}
	if _, err := os.Stat(filepath.Join(dir, "panel.json")); !os.IsNotExist(err) {
		t.Fatalf("panel.json after exit: %v, want it removed", err)
	}
	if _, err := proc.Status(context.Background()); !errors.Is(err, domain.ErrStopped) {
		t.Fatalf("status after exit = %v, want ErrStopped", err)
	}
}

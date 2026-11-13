package panel

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
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
	serve := NewServe("9.9.9", "/work/repo", runs, livePages{})

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

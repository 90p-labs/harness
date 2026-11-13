package process_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/panel/infrastructure/process"
)

// The child marker does not start with HARNESS_, so a test that blanks the
// HARNESS_ variables keeps it.
const childEnv = "PANEL_TEST_CHILD"

func TestMain(m *testing.M) {
	if kind := os.Getenv(childEnv); kind != "" {
		child(kind)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// child is the spawned panel. Kinds: serve, stubborn (ignores SIGTERM),
// fail (exits 1 with a log line) and lose (exits 1 after a while).
func child(kind string) {
	switch kind {
	case "fail":
		fmt.Fprintln(os.Stderr, "first line")
		fmt.Fprintln(os.Stderr, "boom: cannot listen")
		os.Exit(1)
	case "lose":
		time.Sleep(300 * time.Millisecond)
		fmt.Fprintln(os.Stderr, "panel already runs")
		os.Exit(1)
	}
	repo := os.Getenv("PANEL_TEST_REPO")
	p := process.New(process.Config{Dir: os.Getenv("PANEL_TEST_DIR"), Repo: repo})
	claim, err := p.Claim()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	addr := "127.0.0.1:0"
	for i, arg := range os.Args {
		if arg == "--port" && i+1 < len(os.Args) {
			addr = "127.0.0.1:" + os.Args[i+1]
		}
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"version": "test", "repo": repo, "pid": os.Getpid()})
	})
	go func() { _ = http.Serve(ln, mux) }()
	port := ln.Addr().(*net.TCPAddr).Port
	if err := claim.Publish(domain.Instance{PID: os.Getpid(), Port: port, Repo: repo, Version: "test"}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if kind == "stubborn" {
		signal.Ignore(syscall.SIGTERM)
		select {}
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
	_ = claim.Release()
}

// spawner builds a Process whose command is the test binary in the given kind.
func spawner(t *testing.T, dir, kind string) *process.Process {
	t.Helper()
	p := process.New(process.Config{
		Dir:     dir,
		Repo:    "repo-a",
		WorkDir: t.TempDir(),
		Command: []string{os.Args[0]},
		Env:     []string{childEnv + "=" + kind, "PANEL_TEST_DIR=" + dir, "PANEL_TEST_REPO=repo-a"},
	})
	p.ClaimWait = 150 * time.Millisecond
	p.StartWait = 5 * time.Second
	p.StopWait = 5 * time.Second
	p.Poll = 20 * time.Millisecond
	return p
}

// killPanel kills the pid that panel.json names. Register it after t.TempDir.
func killPanel(t *testing.T, dir string) {
	t.Helper()
	t.Cleanup(func() {
		raw, err := os.ReadFile(filepath.Join(dir, "panel.json"))
		if err != nil {
			return
		}
		var f struct{ PID int }
		if json.Unmarshal(raw, &f) == nil && f.PID > 0 && f.PID != os.Getpid() {
			_ = syscall.Kill(f.PID, syscall.SIGKILL)
		}
	})
}

type health struct {
	Version string `json:"version"`
	Repo    string `json:"repo"`
	PID     int    `json:"pid"`
}

// answer serves /healthz with h and returns its port.
func answer(t *testing.T, h health) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(h)
	}))
	t.Cleanup(srv.Close)
	return srv.Listener.Addr().(*net.TCPAddr).Port
}

func writeFile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "panel.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStatus(t *testing.T) {
	ctx := context.Background()
	ownPID := os.Getpid()

	t.Run("no file is stopped", func(t *testing.T) {
		if _, err := newProcess(t.TempDir()).Status(ctx); !errors.Is(err, domain.ErrStopped) {
			t.Fatalf("err = %v, want ErrStopped", err)
		}
	})

	t.Run("a stale file with a free lock is stopped even if health answers", func(t *testing.T) {
		dir := t.TempDir()
		port := answer(t, health{Repo: "repo-a", PID: ownPID})
		writeFile(t, dir, fmt.Sprintf(`{"pid":%d,"port":%d,"repo":"repo-a"}`, ownPID, port))
		p := newProcess(dir)
		if _, err := p.Status(ctx); !errors.Is(err, domain.ErrStopped) {
			t.Fatalf("err = %v, want ErrStopped", err)
		}
		// the probe must release the lock at once
		claim, err := p.Claim()
		if err != nil {
			t.Fatalf("claim after status probe: %v", err)
		}
		_ = claim.Release()
	})

	t.Run("a held lock with a matching answer is running", func(t *testing.T) {
		dir := t.TempDir()
		port := answer(t, health{Version: "v1", Repo: "repo-a", PID: ownPID})
		claim, err := newProcess(dir).Claim()
		if err != nil {
			t.Fatal(err)
		}
		defer claim.Release()
		// the file URL must never be read
		writeFile(t, dir, fmt.Sprintf(`{"pid":%d,"port":%d,"repo":"repo-a","version":"v1","url":"http://evil.example/"}`, ownPID, port))
		inst, err := newProcess(dir).Status(ctx)
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		want := fmt.Sprintf("http://127.0.0.1:%d/", port)
		if inst.URL != want || inst.PID != ownPID || inst.Port != port || inst.Repo != "repo-a" {
			t.Fatalf("instance = %+v, want url %s pid %d", inst, want, ownPID)
		}
	})

	notRunning := map[string]health{
		"pid mismatch":  {Repo: "repo-a", PID: ownPID + 1},
		"repo mismatch": {Repo: "repo-b", PID: ownPID},
	}
	for name, h := range notRunning {
		t.Run(name+" is stopped", func(t *testing.T) {
			dir := t.TempDir()
			port := answer(t, h)
			claim, err := newProcess(dir).Claim()
			if err != nil {
				t.Fatal(err)
			}
			defer claim.Release()
			if err := claim.Publish(domain.Instance{PID: ownPID, Port: port, Repo: "repo-a"}); err != nil {
				t.Fatal(err)
			}
			if _, err := newProcess(dir).Status(ctx); !errors.Is(err, domain.ErrStopped) {
				t.Fatalf("err = %v, want ErrStopped", err)
			}
		})
	}

	t.Run("no listener on the port is stopped", func(t *testing.T) {
		dir := t.TempDir()
		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		claim, err := newProcess(dir).Claim()
		if err != nil {
			t.Fatal(err)
		}
		defer claim.Release()
		_ = claim.Publish(domain.Instance{PID: ownPID, Port: port, Repo: "repo-a"})
		if _, err := newProcess(dir).Status(ctx); !errors.Is(err, domain.ErrStopped) {
			t.Fatalf("err = %v, want ErrStopped", err)
		}
	})

	t.Run("a garbage file is stopped", func(t *testing.T) {
		dir := t.TempDir()
		claim, err := newProcess(dir).Claim()
		if err != nil {
			t.Fatal(err)
		}
		defer claim.Release()
		writeFile(t, dir, "{not json")
		if _, err := newProcess(dir).Status(ctx); !errors.Is(err, domain.ErrStopped) {
			t.Fatalf("err = %v, want ErrStopped", err)
		}
	})
}

func TestStart(t *testing.T) {
	ctx := context.Background()

	t.Run("spawns a panel, waits for it and reuses it", func(t *testing.T) {
		dir := t.TempDir()
		killPanel(t, dir)
		p := spawner(t, dir, "serve")
		if err := p.Start(ctx, 0); err != nil {
			t.Fatalf("start: %v", err)
		}
		first, err := p.Status(ctx)
		if err != nil {
			t.Fatalf("status after start: %v", err)
		}
		if first.PID == os.Getpid() || first.PID <= 0 {
			t.Fatalf("pid = %d, want the child", first.PID)
		}
		if err := p.Start(ctx, 0); err != nil {
			t.Fatalf("second start: %v", err)
		}
		second, _ := p.Status(ctx)
		if second.PID != first.PID {
			t.Fatalf("second start spawned pid %d, first was %d", second.PID, first.PID)
		}
		for _, name := range []string{"panel.json", "panel.lock", "panel.log"} {
			if got := mode(t, filepath.Join(dir, name)); got != 0o600 {
				t.Errorf("%s mode = %o, want 600", name, got)
			}
		}
	})

	t.Run("two starts at once end with one panel", func(t *testing.T) {
		dir := t.TempDir()
		killPanel(t, dir)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = spawner(t, dir, "serve").Start(ctx, 0)
			}()
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Errorf("start %d: %v", i, err)
			}
		}
		if _, err := spawner(t, dir, "serve").Status(ctx); err != nil {
			t.Fatalf("status: %v", err)
		}
	})

	t.Run("accepts a panel that another start spawned", func(t *testing.T) {
		dir := t.TempDir()
		killPanel(t, dir)
		loser := spawner(t, dir, "lose")
		winner := spawner(t, dir, "serve")
		done := make(chan error, 1)
		time.AfterFunc(100*time.Millisecond, func() { done <- winner.Start(ctx, 0) })
		if err := loser.Start(ctx, 0); err != nil {
			t.Fatalf("losing start: %v", err)
		}
		if err := <-done; err != nil {
			t.Fatalf("winning start: %v", err)
		}
	})

	t.Run("a failed child reports the log tail", func(t *testing.T) {
		dir := t.TempDir()
		killPanel(t, dir)
		p := spawner(t, dir, "fail")
		p.StartWait = 600 * time.Millisecond
		err := p.Start(ctx, 0)
		if err == nil {
			t.Fatal("start succeeded with a failing child")
		}
		if !strings.Contains(err.Error(), "boom: cannot listen") {
			t.Fatalf("error %q does not carry the log tail", err)
		}
	})

	t.Run("a busy fixed port fails fast with the log tail", func(t *testing.T) {
		dir := t.TempDir()
		killPanel(t, dir)
		busy, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer busy.Close()
		port := busy.Addr().(*net.TCPAddr).Port
		p := spawner(t, dir, "serve")
		p.StartWait = 10 * time.Second
		began := time.Now()
		err = p.Start(ctx, port)
		if err == nil {
			t.Fatal("start succeeded on a busy port")
		}
		if took := time.Since(began); took > 3*time.Second {
			t.Fatalf("start took %s, want well under StartWait %s", took, p.StartWait)
		}
		if !strings.Contains(err.Error(), "address already in use") {
			t.Fatalf("error %q does not carry the log tail", err)
		}
	})

	t.Run("an unknown command fails", func(t *testing.T) {
		dir := t.TempDir()
		p := process.New(process.Config{Dir: dir, Repo: "repo-a", WorkDir: dir, Command: []string{filepath.Join(dir, "missing")}})
		if err := p.Start(ctx, 0); err == nil {
			t.Fatal("start succeeded with a missing command")
		}
	})
}

func TestStop(t *testing.T) {
	ctx := context.Background()

	t.Run("signals the verified pid and waits for the free lock", func(t *testing.T) {
		dir := t.TempDir()
		killPanel(t, dir)
		p := spawner(t, dir, "serve")
		if err := p.Start(ctx, 0); err != nil {
			t.Fatal(err)
		}
		inst, err := p.Status(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := p.Stop(ctx); err != nil {
			t.Fatalf("stop: %v", err)
		}
		// the lock is free when Stop returns
		claim, err := p.Claim()
		if err != nil {
			t.Fatalf("lock still held after stop: %v", err)
		}
		_ = claim.Release()
		if _, err := os.Stat(filepath.Join(dir, "panel.json")); !os.IsNotExist(err) {
			t.Errorf("panel.json after stop: %v", err)
		}
		if _, err := p.Status(ctx); !errors.Is(err, domain.ErrStopped) {
			t.Errorf("status after stop = %v, want ErrStopped", err)
		}
		// the pid that Status verified is the one that is gone
		deadline := time.Now().Add(2 * time.Second)
		for syscall.Kill(inst.PID, 0) == nil {
			if time.Now().After(deadline) {
				t.Fatalf("pid %d still runs after stop", inst.PID)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	t.Run("a lock with no panel.json is a panel that is starting", func(t *testing.T) {
		dir := t.TempDir()
		claim, err := newProcess(dir).Claim()
		if err != nil {
			t.Fatal(err)
		}
		defer claim.Release()
		err = newProcess(dir).Stop(ctx)
		if err == nil || !strings.Contains(err.Error(), "panel is starting; try again") {
			t.Fatalf("stop = %v, want the starting error", err)
		}
	})

	t.Run("a stopped panel is nil", func(t *testing.T) {
		if err := newProcess(t.TempDir()).Stop(ctx); err != nil {
			t.Fatalf("stop: %v", err)
		}
	})

	t.Run("removes a stale file with a free lock and sends no signal", func(t *testing.T) {
		dir := t.TempDir()
		// the stale file names this test process: a signal would end it
		writeFile(t, dir, fmt.Sprintf(`{"pid":%d,"port":1,"repo":"repo-a"}`, os.Getpid()))
		if err := newProcess(dir).Stop(ctx); err != nil {
			t.Fatalf("stop: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "panel.json")); !os.IsNotExist(err) {
			t.Errorf("stale panel.json stays: %v", err)
		}
	})

	t.Run("an unverified pid is never signaled", func(t *testing.T) {
		for name, h := range map[string]health{
			"pid mismatch":  {Repo: "repo-a", PID: os.Getpid() + 1},
			"repo mismatch": {Repo: "repo-b", PID: os.Getpid()},
		} {
			dir := t.TempDir()
			port := answer(t, h)
			claim, err := newProcess(dir).Claim()
			if err != nil {
				t.Fatal(err)
			}
			_ = claim.Publish(domain.Instance{PID: os.Getpid(), Port: port, Repo: "repo-a"})
			p := newProcess(dir)
			p.StopWait = 200 * time.Millisecond
			if err := p.Stop(ctx); err == nil {
				t.Errorf("%s: stop succeeded on an unverified pid", name)
			}
			if _, err := os.Stat(filepath.Join(dir, "panel.json")); err != nil {
				t.Errorf("%s: panel.json of a held lock was removed: %v", name, err)
			}
			_ = claim.Release()
		}
	})

	t.Run("a timeout keeps panel.json", func(t *testing.T) {
		dir := t.TempDir()
		killPanel(t, dir)
		p := spawner(t, dir, "stubborn")
		if err := p.Start(ctx, 0); err != nil {
			t.Fatal(err)
		}
		p.StopWait = 300 * time.Millisecond
		if err := p.Stop(ctx); err == nil {
			t.Fatal("stop succeeded on a server that ignores SIGTERM")
		}
		if _, err := os.Stat(filepath.Join(dir, "panel.json")); err != nil {
			t.Errorf("panel.json removed on a timeout: %v", err)
		}
	})
}

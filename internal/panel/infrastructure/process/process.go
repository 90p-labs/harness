package process

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
)

const (
	logTailLines = 10
	logTailBytes = 4096
	healthLimit  = 1 << 16
)

// Config wires a Process.
type Config struct {
	// Dir is the state dir, <git common dir>/.harness.
	Dir string
	// Command is the argv to spawn, for example [exe, "panel", "serve"]. A
	// port above 0 appends "--port <n>".
	Command []string
	// WorkDir is the working directory of the spawned server.
	WorkDir string
	// Repo is the repo identity the health answer must report.
	Repo string
	// Env adds variables to the environment of the spawned server.
	Env []string
	// Client probes /healthz. Nil uses a client with a 2 second timeout.
	Client *http.Client
}

// Process implements domain.Lifecycle with a lock file, a state file and a
// background child.
type Process struct {
	dir     string
	command []string
	workDir string
	repo    string
	env     []string
	client  *http.Client

	// ClaimWait is how long a claim retries a held lock.
	ClaimWait time.Duration
	// StartWait is how long Start polls for a running panel.
	StartWait time.Duration
	// StopWait is how long Stop waits for the lock to free.
	StopWait time.Duration
	// Poll is the pause between two polls.
	Poll time.Duration
}

var _ domain.Lifecycle = (*Process)(nil)

// New returns a Process with the default waits: 1 s to claim, 5 s to start
// and 10 s to stop.
func New(cfg Config) *Process {
	client := cfg.Client
	if client == nil {
		client = &http.Client{
			Timeout:       2 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	return &Process{
		dir:       cfg.Dir,
		command:   cfg.Command,
		workDir:   cfg.WorkDir,
		repo:      cfg.Repo,
		env:       cfg.Env,
		client:    client,
		ClaimWait: time.Second,
		StartWait: 5 * time.Second,
		StopWait:  10 * time.Second,
		Poll:      50 * time.Millisecond,
	}
}

// Status returns the running panel. It is running only when the lock is held
// and /healthz answers the repo and the pid of panel.json. Anything else
// returns domain.ErrStopped.
func (p *Process) Status(ctx context.Context) (domain.Instance, error) {
	raw, err := os.ReadFile(filepath.Join(p.dir, stateFile))
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Instance{}, domain.ErrStopped
		}
		return domain.Instance{}, fmt.Errorf("read panel.json: %w", err)
	}
	var st state
	if err := json.Unmarshal(raw, &st); err != nil || st.PID <= 0 || st.Port <= 0 || st.Port > 65535 {
		return domain.Instance{}, domain.ErrStopped
	}
	free, err := lockFree(p.dir)
	if err != nil {
		return domain.Instance{}, err
	}
	if free {
		return domain.Instance{}, domain.ErrStopped
	}
	if st.Repo != p.repo || !p.healthy(ctx, st) {
		return domain.Instance{}, domain.ErrStopped
	}
	return domain.Instance{
		PID:     st.PID,
		Port:    st.Port,
		URL:     fmt.Sprintf("http://127.0.0.1:%d/", st.Port),
		Repo:    st.Repo,
		Version: st.Version,
	}, nil
}

// healthy reports whether /healthz on the port names the repo and pid of st.
func (p *Process) healthy(ctx context.Context, st state) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", st.Port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var h struct {
		Repo string `json:"repo"`
		PID  int    `json:"pid"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, healthLimit)).Decode(&h); err != nil {
		return false
	}
	return h.Repo == st.Repo && h.PID == st.PID
}

// Start runs the panel in the background. A running panel returns nil with no
// spawn. Otherwise it spawns the command in a new session and polls Status
// until StartWait. A child that exits while the lock is held by another start
// keeps the poll going, since that start may win. A child that exits with the
// lock free ends the poll at once with the log tail.
func (p *Process) Start(ctx context.Context, port int) error {
	if _, err := p.Status(ctx); err == nil {
		return nil
	} else if !errors.Is(err, domain.ErrStopped) {
		return err
	}
	offset, exited, err := p.spawn(port)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(p.StartWait)
	for {
		if _, err := p.Status(ctx); err == nil {
			return nil
		} else if !errors.Is(err, domain.ErrStopped) {
			return err
		}
		select {
		case <-exited:
			free, err := lockFree(p.dir)
			if err != nil {
				return err
			}
			if free {
				return fmt.Errorf("panel exited before it started\n%s", p.logTail(offset))
			}
			exited = nil // another start holds the lock: poll on
		default:
		}
		if !time.Now().Before(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-exited:
		case <-time.After(p.Poll):
		}
	}
	return fmt.Errorf("panel did not start within %s\n%s", p.StartWait, p.logTail(offset))
}

// spawn starts the child and returns the size of panel.log before it, and a
// channel that closes when the child has exited and was reaped.
func (p *Process) spawn(port int) (int64, <-chan struct{}, error) {
	if len(p.command) == 0 {
		return 0, nil, errors.New("no panel command")
	}
	if err := os.MkdirAll(p.dir, dirMode); err != nil {
		return 0, nil, fmt.Errorf("create panel state dir: %w", err)
	}
	log, err := os.OpenFile(filepath.Join(p.dir, logFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, fileMode)
	if err != nil {
		return 0, nil, fmt.Errorf("open panel.log: %w", err)
	}
	defer log.Close()
	var offset int64
	if info, err := log.Stat(); err == nil {
		offset = info.Size()
	}
	argv := append([]string(nil), p.command...)
	if port > 0 {
		argv = append(argv, "--port", strconv.Itoa(port))
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = p.workDir
	cmd.Env = append(os.Environ(), p.env...)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, nil, fmt.Errorf("spawn panel: %w", err)
	}
	exited := make(chan struct{})
	go func() { // reap the child, whatever its exit
		_ = cmd.Wait()
		close(exited)
	}()
	return offset, exited, nil
}

// logTail returns the last lines that the child wrote to panel.log.
func (p *Process) logTail(offset int64) string {
	raw, err := os.ReadFile(filepath.Join(p.dir, logFile))
	if err != nil || int64(len(raw)) < offset {
		return "(no panel.log)"
	}
	raw = raw[offset:]
	if len(raw) > logTailBytes {
		raw = raw[len(raw)-logTailBytes:]
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > logTailLines {
		lines = lines[len(lines)-logTailLines:]
	}
	if len(lines) == 1 && lines[0] == "" {
		return "(panel.log has no output)"
	}
	return strings.Join(lines, "\n")
}

// Stop ends a running panel. It signals only a pid that Status verified, then
// waits StopWait for the lock to free. A stopped panel with a stale
// panel.json removes the file and sends no signal. A lock held by an
// unverified process is an error and changes nothing.
func (p *Process) Stop(ctx context.Context) error {
	inst, err := p.Status(ctx)
	if errors.Is(err, domain.ErrStopped) {
		return p.removeStale()
	}
	if err != nil {
		return err
	}
	if err := syscall.Kill(inst.PID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal panel pid %d: %w", inst.PID, err)
	}
	deadline := time.Now().Add(p.StopWait)
	for {
		free, err := lockFree(p.dir)
		if err != nil {
			return err
		}
		if free {
			return p.removeStale()
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("panel pid %d did not stop within %s", inst.PID, p.StopWait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.Poll):
		}
	}
}

// removeStale removes panel.json when the lock is free. A held lock that
// Status could not verify is an error.
func (p *Process) removeStale() error {
	free, err := lockFree(p.dir)
	if err != nil {
		return err
	}
	if !free {
		if _, err := os.Stat(filepath.Join(p.dir, stateFile)); os.IsNotExist(err) {
			// serve holds the lock and has not published panel.json yet
			return errors.New("panel is starting; try again")
		}
		return errors.New("panel lock is held by a process that is not a verified panel")
	}
	if err := os.Remove(filepath.Join(p.dir, stateFile)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale panel.json: %w", err)
	}
	return nil
}

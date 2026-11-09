package execplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/propagation"

	"github.com/jorge07/harness/internal/shared/domain"
)

const (
	DefaultTimeout = 5 * time.Second
	waitDelay      = 500 * time.Millisecond
	killGrace      = 2 * time.Second
	traceparentEnv = "TRACEPARENT"
	traceparentKey = "traceparent"
)

var errEmptyCommand = errors.New("empty command")

type ExecPlugin struct {
	PluginName string
	Events     []domain.EventName
	Command    []string
	Timeout    time.Duration
	Filter     domain.TransitionFilter
}

func (p ExecPlugin) Name() string { return p.PluginName }

func (p ExecPlugin) Subscribes() []domain.EventName { return p.Events }

func (p ExecPlugin) Matches(envelope domain.Envelope) bool {
	if p.Filter.Empty() {
		return true
	}
	var payload domain.TransitionPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return false
	}
	return p.Filter.Matches(payload)
}

func (p ExecPlugin) Handle(ctx context.Context, envelope domain.Envelope) (domain.Verdict, error) {
	if len(p.Command) == 0 {
		return domain.Verdict{}, p.fail(errEmptyCommand)
	}
	stdin, err := json.Marshal(envelope)
	if err != nil {
		return domain.Verdict{}, p.fail(err)
	}

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout bytes.Buffer
	cmd := exec.CommandContext(callCtx, p.Command[0], p.Command[1:]...)
	cmd.Dir = envelope.Cwd
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return stopGroup(cmd.Process.Pid) }
	cmd.WaitDelay = waitDelay
	cmd.Env = append(os.Environ(), traceparentVar(callCtx))

	if err := cmd.Run(); err != nil && !exitedCleanly(cmd, err) {
		if callCtx.Err() != nil {
			err = fmt.Errorf("%w: %w", callCtx.Err(), err)
		}
		return domain.Verdict{}, p.fail(err)
	}

	if len(bytes.TrimSpace(stdout.Bytes())) == 0 {
		return domain.Verdict{Kind: domain.VerdictAllow}, nil
	}
	var verdict domain.Verdict
	if err := json.Unmarshal(stdout.Bytes(), &verdict); err != nil {
		return domain.Verdict{}, p.fail(err)
	}
	return verdict, nil
}

func (p ExecPlugin) fail(cause error) error {
	return fmt.Errorf("%w: %s: %w", domain.ErrPluginFailed, p.PluginName, cause)
}

func stopGroup(pid int) error {
	// macOS answers EPERM when the group leader is a zombie, and its children still need the SIGKILL.
	if err := syscall.Kill(-pid, syscall.SIGTERM); errors.Is(err, syscall.ESRCH) {
		return err
	}
	time.Sleep(killGrace)
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) && !errors.Is(err, syscall.EPERM) {
		return err
	}
	return nil
}

// A plugin that exits 0 but leaves a child on stdout trips WaitDelay; its verdict is already written.
func exitedCleanly(cmd *exec.Cmd, err error) bool {
	return errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success()
}

func traceparentVar(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	return traceparentEnv + "=" + carrier.Get(traceparentKey)
}

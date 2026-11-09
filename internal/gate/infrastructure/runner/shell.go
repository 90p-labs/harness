package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/telemetry"
)

const (
	TailBytes = 4096
	waitDelay = 2 * time.Second
	shell     = "sh"
	shellFlag = "-c"
)

type Shell struct{}

func (Shell) Run(ctx context.Context, dir string, gate domain.Gate) (domain.Outcome, error) {
	ctx, cancel := context.WithTimeout(ctx, gate.Timeout)
	defer cancel()

	var stdout bytes.Buffer
	tail := &tailBuffer{}
	cmd := exec.CommandContext(ctx, shell, shellFlag, gate.Run)
	cmd.Dir = dir
	if env := telemetry.ChildEnv(ctx); len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = io.MultiWriter(&stdout, tail)
	cmd.Stderr = tail
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = waitDelay

	start := time.Now()
	err := cmd.Run()
	outcome := domain.Outcome{
		Duration: time.Since(start), Tail: tail.String(), Stdout: stdout.String(),
		TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded),
	}
	if cmd.ProcessState == nil {
		return domain.Outcome{}, fmt.Errorf("start gate %s: %w", gate.ID, err)
	}
	outcome.Result = domain.ResultFail
	if err == nil {
		outcome.Result = domain.ResultPass
	}
	return outcome, nil
}

type tailBuffer struct {
	mu   sync.Mutex
	data []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > TailBytes {
		b.data = append([]byte(nil), b.data[len(b.data)-TailBytes:]...)
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

package execplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"go.opentelemetry.io/otel/propagation"

	"github.com/jorge07/harness/internal/plugin/domain"
)

const (
	DefaultTimeout = 5 * time.Second
	waitDelay      = 500 * time.Millisecond
	stderrTail     = 512
	traceparentEnv = "TRACEPARENT"
	traceparentKey = "traceparent"
)

var errEmptyCommand = errors.New("empty command")

type ExecPlugin struct {
	PluginName string
	Events     []domain.EventName
	Command    []string
	Timeout    time.Duration
}

func (p ExecPlugin) Name() string { return p.PluginName }

func (p ExecPlugin) Subscribes() []domain.EventName { return p.Events }

func (p ExecPlugin) Handle(ctx context.Context, envelope domain.Envelope) (domain.Verdict, error) {
	if len(p.Command) == 0 {
		return domain.Verdict{}, p.fail(errEmptyCommand, nil)
	}
	stdin, err := json.Marshal(envelope)
	if err != nil {
		return domain.Verdict{}, p.fail(err, nil)
	}

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdout bytes.Buffer
	var tail tailBuffer
	cmd := exec.CommandContext(callCtx, p.Command[0], p.Command[1:]...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &tail)
	cmd.WaitDelay = waitDelay
	cmd.Env = append(os.Environ(), traceparentVar(callCtx))

	if err := cmd.Run(); err != nil && !exitedCleanly(cmd, err) {
		if callCtx.Err() != nil {
			err = fmt.Errorf("%w: %w", callCtx.Err(), err)
		}
		return domain.Verdict{}, p.fail(err, tail.bytes())
	}

	if len(bytes.TrimSpace(stdout.Bytes())) == 0 {
		return domain.Verdict{Kind: domain.VerdictAllow}, nil
	}
	var verdict domain.Verdict
	if err := json.Unmarshal(stdout.Bytes(), &verdict); err != nil {
		return domain.Verdict{}, p.fail(err, tail.bytes())
	}
	return verdict, nil
}

func (p ExecPlugin) fail(cause error, stderr []byte) error {
	return fmt.Errorf("%w: %s: %w: stderr: %s", domain.ErrPluginFailed, p.PluginName, cause, stderr)
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

type tailBuffer struct {
	data []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.data = append(t.data, p...)
	if len(t.data) > stderrTail {
		t.data = t.data[len(t.data)-stderrTail:]
	}
	return len(p), nil
}

func (t *tailBuffer) bytes() []byte { return t.data }

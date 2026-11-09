package gate_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/90p-labs/harness/internal/gate"
	"github.com/90p-labs/harness/internal/gate/application/command/verify"
	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/gate/infrastructure/persistence"
	"github.com/90p-labs/harness/internal/shared/application/eventbus"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	repoName   = "github.com/90p-labs/harness"
	gateCount  = 8
	gateLimit  = 4
	countRows  = "SELECT COUNT(*) FROM gate_evidence WHERE repo = ?"
	pluginName = "counter"
)

type eventCounter struct {
	mu     sync.Mutex
	counts map[shareddomain.EventName]map[string]int
}

func (c *eventCounter) Name() string { return pluginName }

func (c *eventCounter) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventGateBefore, shareddomain.EventGateAfter}
}

func (c *eventCounter) Handle(_ context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	var payload verify.GateBeforePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return shareddomain.Verdict{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[envelope.Event][payload.Gate]++
	return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
}

func TestVerifyParallelIsRaceFree(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := dbal.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "state.db"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(ctx, persistence.Module, persistence.Migrations); err != nil {
		t.Fatal(err)
	}
	counter := &eventCounter{counts: map[shareddomain.EventName]map[string]int{
		shareddomain.EventGateBefore: {}, shareddomain.EventGateAfter: {},
	}}
	bus := eventbus.NewBus([]shareddomain.Plugin{counter}, tracenoop.NewTracerProvider(), metricnoop.NewMeterProvider())
	gates := make([]domain.Gate, gateCount)
	for i := range gates {
		gates[i] = domain.Gate{ID: fmt.Sprintf("gate-%d", i), Run: "true", Timeout: time.Minute}
	}

	result, err := gate.NewVerify(db, bus, nil, time.Now).Handle(ctx, verify.Command{Repo: repoName, Dir: dir, Gates: gates, Parallel: gateLimit})

	if err != nil || result.Failed {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
	for _, g := range gates {
		for event, perGate := range counter.counts {
			if perGate[g.ID] != 1 {
				t.Errorf("%s for %s = %d, want 1", event, g.ID, perGate[g.ID])
			}
		}
	}
	var rows int
	if err := db.QueryRowContext(ctx, countRows, repoName).Scan(&rows); err != nil || rows != gateCount {
		t.Fatalf("evidence rows = %d, err = %v, want %d", rows, err, gateCount)
	}
}

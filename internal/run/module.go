package run

import (
	"context"
	"time"

	"github.com/90p-labs/harness/internal/run/application/command/decidestop"
	"github.com/90p-labs/harness/internal/run/application/command/recordapproval"
	"github.com/90p-labs/harness/internal/run/application/command/start"
	"github.com/90p-labs/harness/internal/run/application/command/transition"
	"github.com/90p-labs/harness/internal/run/application/command/waitpr"
	"github.com/90p-labs/harness/internal/run/application/query/report"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/infrastructure/findings"
	"github.com/90p-labs/harness/internal/run/infrastructure/handoff"
	"github.com/90p-labs/harness/internal/run/infrastructure/persistence"
	"github.com/90p-labs/harness/internal/run/infrastructure/pluginhooks"
	"github.com/90p-labs/harness/internal/run/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
	"github.com/90p-labs/harness/internal/shared/infrastructure/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/90p-labs/harness/internal/run"

type runTrace struct{ tracer trace.Tracer }

func (r runTrace) Under(ctx context.Context, run domain.Run) context.Context {
	return telemetry.UnderRun(ctx, run.ID)
}

func (r runTrace) End(ctx context.Context, run domain.Run) {
	telemetry.EndRun(ctx, r.tracer, run.ID, run.Key.Worktree, string(run.Phase), run.StartedAt)
}

func policyOf(cfg config.Run) domain.Policy {
	return domain.Policy{
		ApprovalNeeded:  cfg.Approval,
		MaxReviewRounds: cfg.MaxReviewRounds,
		GoLive:          cfg.GoLive,
		MaxLiveRounds:   cfg.MaxLiveRounds,
		MaxStopBlocks:   cfg.MaxStopBlocks,
		StrictMinor:     cfg.StrictMinor,
	}
}

func NewStopDecider(db *dbal.DB, cfg config.Run, now func() time.Time) decidestop.Handler {
	return decidestop.NewHandler(persistence.NewSQLRunRepository(db, now), handoff.File{}, policyOf(cfg), otel.Tracer(tracerName))
}

func NewApprovalRecorder(db *dbal.DB, gate domain.PlanGate, now func() time.Time) recordapproval.Handler {
	return recordapproval.NewHandler(persistence.NewSQLRunRepository(db, now), gate, persistence.NewSQLApprovalRepository(db), now)
}

func NewReworkLog(db *dbal.DB, now func() time.Time) domain.ReworkLog {
	return persistence.NewSQLRunRepository(db, now)
}

func NewStatus(db *dbal.DB, now func() time.Time) status.Handler {
	return status.NewHandler(persistence.NewSQLRunRepository(db, now))
}

func NewModule(db *dbal.DB, bus shareddomain.Dispatcher, gate domain.PlanGate, cfg config.Run, key domain.RunKey, now func() time.Time) cli.Handlers {
	repo := persistence.NewSQLRunRepository(db, now)
	policy := policyOf(cfg)
	tracer := otel.Tracer(tracerName)
	flow := transition.NewFlow(repo, pluginhooks.NewHooks(bus, key.Worktree), gate, persistence.NewSQLApprovalRepository(db), policy, now, tracer, runTrace{tracer: tracer})
	return cli.Handlers{
		Key:        key,
		Start:      start.NewHandler(repo, flow, now),
		Transition: transition.NewHandler(repo, flow, findings.File{}, policy),
		WaitPR:     waitpr.NewHandler(repo, tracer),
		Status:     status.NewHandler(repo),
		Report:     report.NewHandler(repo, repo, now, telemetry.TraceIDOf, tracer),
	}
}

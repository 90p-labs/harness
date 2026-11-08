package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	livemodule "github.com/jorge07/harness/internal/live"
	"github.com/jorge07/harness/internal/live/application/command/check"
	livedomain "github.com/jorge07/harness/internal/live/domain"
	livecli "github.com/jorge07/harness/internal/live/ui/cli"
	plandomain "github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/run/application/command/transition"
	"github.com/jorge07/harness/internal/run/application/query/status"
	"github.com/jorge07/harness/internal/run/domain"
	runcli "github.com/jorge07/harness/internal/run/ui/cli"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

var (
	liveRunPhases = []domain.Phase{domain.PhaseReleasing, domain.PhaseObserving}

	livePhases = map[domain.Phase]livedomain.Phase{
		domain.PhaseReleasing: livedomain.PhaseReleasing,
		domain.PhaseObserving: livedomain.PhaseObserving,
	}

	runEvents = map[livedomain.Event]domain.Event{
		livedomain.EventDeploy:  domain.EventDeploy,
		livedomain.EventRegress: domain.EventRegress,
		livedomain.EventClose:   domain.EventClose,
		livedomain.EventBlock:   domain.EventBlock,
	}
)

type liveRuns struct {
	handlers runcli.Handlers
}

func (r liveRuns) Live(ctx context.Context, repo string) ([]livedomain.LiveRun, error) {
	statuses, err := r.handlers.Status.ByPhase(ctx, repo, liveRunPhases)
	if err != nil {
		return nil, err
	}
	runs := make([]livedomain.LiveRun, len(statuses))
	for i, s := range statuses {
		runs[i] = livedomain.LiveRun{
			ID: s.ID, Worktree: s.Worktree, Phase: livePhases[s.Phase],
			LiveRound: s.LiveRound, PRURL: s.PRURL, Version: s.Version,
		}
	}
	return runs, nil
}

func (r liveRuns) Apply(ctx context.Context, run livedomain.LiveRun, event livedomain.Event, reason string) error {
	err := r.handlers.Transition.Handle(ctx, transition.Command{
		RunID: run.ID, Version: run.Version, Event: runEvents[event], Reason: reason,
	})
	if errors.Is(err, domain.ErrStaleRun) {
		return fmt.Errorf("%w: %w", livedomain.ErrStaleRun, err)
	}
	return err
}

func (liveRuns) Signals(_ context.Context, run livedomain.LiveRun) ([]livedomain.Threshold, error) {
	path := planPath(run.Worktree, run.ID)
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, problems := plandomain.Parse(string(text))
	if len(problems) > 0 {
		lines := make([]string, len(problems))
		for i, p := range problems {
			lines[i] = p.String()
		}
		return nil, fmt.Errorf("%s: %s", path, strings.Join(lines, "; "))
	}
	thresholds := make([]livedomain.Threshold, len(doc.Signals))
	for i, row := range doc.Signals {
		breach, err := shareddomain.ParseBreach(row.Breach)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, row.Line, err)
		}
		window, err := time.ParseDuration(row.Window)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, row.Line, err)
		}
		thresholds[i] = livedomain.Threshold{
			Signal: row.Signal, Source: row.Source, Query: row.Query,
			Breach: breach, Window: window, Panel: row.Panel,
		}
	}
	return thresholds, nil
}

func newLiveCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := livecli.NewCommand(func(ctx context.Context) ([]check.Result, error) {
		env, err := newEnvironment(ctx, flags)
		if err != nil {
			return nil, err
		}
		return runLiveCheck(ctx, env)
	})
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

func runLiveCheck(ctx context.Context, env environment) ([]check.Result, error) {
	handlers, db, err := openRun(ctx, env)
	if err != nil {
		return nil, err
	}
	results, err := livemodule.NewCheck(db, liveRuns{handlers: handlers}, env.bus, env.cfg.Live).
		Handle(ctx, check.Command{Repo: env.key.Repo, Now: time.Now()})
	return results, errors.Join(err, db.Close())
}

type runDispatcher struct {
	next   shareddomain.Dispatcher
	active func(ctx context.Context) (shareddomain.RunRef, error)
}

func (d runDispatcher) Dispatch(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	if ref, err := d.active(ctx); err == nil {
		envelope.Run = ref
	}
	return d.next.Dispatch(ctx, envelope)
}

func activeRunRef(env environment) func(ctx context.Context) (shareddomain.RunRef, error) {
	return func(ctx context.Context) (shareddomain.RunRef, error) {
		handlers, db, err := openRun(ctx, env)
		if err != nil {
			return shareddomain.RunRef{}, err
		}
		defer func() { _ = db.Close() }()
		current, err := handlers.Status.Handle(ctx, status.Query{Key: env.key})
		if err != nil {
			return shareddomain.RunRef{}, err
		}
		return shareddomain.RunRef{ID: current.ID, Phase: string(current.Phase)}, nil
	}
}

package pluginhooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jorge07/harness/internal/run/domain"
	plugindomain "github.com/jorge07/harness/internal/shared/domain"
)

type Hooks struct {
	bus plugindomain.Dispatcher
	cwd string
}

var _ domain.TransitionHooks = Hooks{}

func NewHooks(bus plugindomain.Dispatcher, cwd string) Hooks {
	return Hooks{bus: bus, cwd: cwd}
}

func (h Hooks) Before(ctx context.Context, run domain.Run, cmd domain.Command, to domain.Phase) error {
	result, err := h.dispatch(ctx, plugindomain.EventRunTransitionBefore, run, plugindomain.TransitionPayload{
		Event: string(cmd.Event), From: string(run.Phase), To: string(to), Reason: cmd.Reason,
	})
	if err != nil {
		return fmt.Errorf("dispatch %s: %w", plugindomain.EventRunTransitionBefore, err)
	}
	if result.Verdict.Kind == plugindomain.VerdictDeny {
		return fmt.Errorf("%w: %s", domain.ErrTransitionVetoed, result.Verdict.Reason)
	}
	return nil
}

func (h Hooks) After(ctx context.Context, from domain.Run, to domain.Run) {
	body := plugindomain.TransitionPayload{From: string(from.Phase), To: string(to.Phase)}
	if events := to.Events(); len(events) > 0 {
		last := events[len(events)-1]
		body.Event, body.Reason = string(last.Event), last.Reason
	}
	_, _ = h.dispatch(ctx, plugindomain.EventRunTransitionAfter, from, body)
}

func (h Hooks) dispatch(ctx context.Context, event plugindomain.EventName, run domain.Run, body plugindomain.TransitionPayload) (plugindomain.DispatchResult, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return plugindomain.DispatchResult{}, err
	}
	return h.bus.Dispatch(ctx, plugindomain.Envelope{
		Version: plugindomain.EnvelopeVersion,
		Event:   event,
		Cwd:     h.cwd,
		Run:     plugindomain.RunRef{ID: run.ID, Phase: string(run.Phase)},
		Payload: raw,
	})
}

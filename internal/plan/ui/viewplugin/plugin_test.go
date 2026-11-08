package viewplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jorge07/harness/internal/plan/application/command/renderview"
	"github.com/jorge07/harness/internal/plan/ui/viewplugin"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	worktree      = "/work/tree"
	runID         = "20261108-081430-add-the-login-page"
	otherPhase    = "working"
	previousPhase = "planning"
)

var (
	errRender = errors.New("render failed")
	errRound  = errors.New("no run store")
)

func liveRound(round int, err error) viewplugin.LiveRound {
	return func(context.Context) (int, error) { return round, err }
}

func envelope(t *testing.T, to string) shareddomain.Envelope {
	t.Helper()
	raw, err := json.Marshal(shareddomain.TransitionPayload{Event: shareddomain.TransitionSubmitForReview, From: previousPhase, To: to})
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{
		Event:   shareddomain.EventRunTransitionAfter,
		Run:     shareddomain.RunRef{ID: runID, Phase: to},
		Payload: raw,
	}
}

func TestViewPlugin(t *testing.T) {
	var commands []renderview.Command
	render := func(_ context.Context, c renderview.Command) (string, error) {
		commands = append(commands, c)
		return "", nil
	}
	plugin := viewplugin.New(worktree, render, liveRound(1, nil))

	if !slices.Equal(plugin.Subscribes(), []shareddomain.EventName{shareddomain.EventRunTransitionAfter}) {
		t.Errorf("Subscribes = %v", plugin.Subscribes())
	}

	t.Run("awaiting approval renders the page of the run in its live round", func(t *testing.T) {
		commands = nil

		verdict, err := plugin.Handle(t.Context(), envelope(t, viewplugin.PhaseAwaitingApproval))

		want := renderview.Command{
			Path:      filepath.Join(worktree, ".harness", "runs", runID, "plan.md"),
			RunID:     runID,
			Phase:     viewplugin.PhaseAwaitingApproval,
			LiveRound: 1,
			Proofs:    true,
		}
		if err != nil || verdict.Kind != shareddomain.VerdictAllow {
			t.Fatalf("verdict = %v, err = %v, want allow", verdict, err)
		}
		if len(commands) != 1 || commands[0] != want {
			t.Errorf("commands = %+v, want %+v", commands, want)
		}
	})

	t.Run("a live round read error renders round 0 and reports the error", func(t *testing.T) {
		commands = nil
		noRound := viewplugin.New(worktree, render, liveRound(1, errRound))

		verdict, err := noRound.Handle(t.Context(), envelope(t, viewplugin.PhaseAwaitingApproval))

		if !errors.Is(err, errRound) || verdict.Kind != shareddomain.VerdictAllow {
			t.Fatalf("verdict = %v, err = %v, want allow and the round error", verdict, err)
		}
		if len(commands) != 1 || commands[0].LiveRound != 0 {
			t.Errorf("commands = %+v, want one render in round 0", commands)
		}
	})

	t.Run("another phase renders nothing", func(t *testing.T) {
		commands = nil

		verdict, err := plugin.Handle(t.Context(), envelope(t, otherPhase))

		if err != nil || verdict.Kind != shareddomain.VerdictAllow || len(commands) != 0 {
			t.Errorf("verdict = %v, err = %v, commands = %v", verdict, err, commands)
		}
	})

	t.Run("a render error allows and reports the error", func(t *testing.T) {
		failing := viewplugin.New(worktree, func(context.Context, renderview.Command) (string, error) { return "", errRender }, liveRound(0, nil))

		verdict, err := failing.Handle(t.Context(), envelope(t, viewplugin.PhaseAwaitingApproval))

		if !errors.Is(err, errRender) || verdict.Kind != shareddomain.VerdictAllow {
			t.Errorf("verdict = %v, err = %v, want allow and the render error", verdict, err)
		}
	})

	t.Run("a bad payload reports an error", func(t *testing.T) {
		bad := envelope(t, viewplugin.PhaseAwaitingApproval)
		bad.Payload = []byte("{")

		_, err := plugin.Handle(t.Context(), bad)

		if err == nil {
			t.Error("want a decode error")
		}
	})
}

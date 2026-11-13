package viewplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/90p-labs/harness/internal/plan/application/command/renderview"
	"github.com/90p-labs/harness/internal/plan/ui/viewplugin"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	worktree = "/work/tree"
	runID    = "20261108-081430-add-the-login-page"
)

var (
	errRender = errors.New("render failed")
	errRound  = errors.New("no run store")
)

func liveRound(round int, err error) viewplugin.LiveRound {
	return func(context.Context) (int, error) { return round, err }
}

func noOpen(context.Context, string) error { return nil }

func TestViewPluginOpensPage(t *testing.T) {
	const pagePath = "/work/tree/.harness/runs/r1/plan.html"
	errOpen := errors.New("no browser")
	render := func(context.Context, renderview.Command) (string, error) { return pagePath, nil }

	t.Run("awaiting approval opens the rendered page once", func(t *testing.T) {
		var opened []string
		plugin := viewplugin.New(worktree, render, liveRound(1, nil), func(_ context.Context, page string) error {
			opened = append(opened, page)
			return nil
		})

		verdict, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval))

		if err != nil || verdict.Kind != shareddomain.VerdictAllow {
			t.Fatalf("verdict = %v, err = %v, want allow", verdict, err)
		}
		if !slices.Equal(opened, []string{pagePath}) {
			t.Errorf("opened = %v, want [%s]", opened, pagePath)
		}
	})

	t.Run("another phase opens nothing", func(t *testing.T) {
		var opened []string
		plugin := viewplugin.New(worktree, render, liveRound(1, nil), func(_ context.Context, page string) error {
			opened = append(opened, page)
			return nil
		})

		_, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseWorking))

		if err != nil || len(opened) != 0 {
			t.Errorf("err = %v, opened = %v, want nothing", err, opened)
		}
	})

	t.Run("a render error opens nothing", func(t *testing.T) {
		var opened []string
		failing := func(context.Context, renderview.Command) (string, error) { return "", errRender }
		plugin := viewplugin.New(worktree, failing, liveRound(1, nil), func(_ context.Context, page string) error {
			opened = append(opened, page)
			return nil
		})

		_, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval))

		if !errors.Is(err, errRender) || len(opened) != 0 {
			t.Errorf("err = %v, opened = %v, want the render error and nothing opened", err, opened)
		}
	})

	t.Run("an open error allows and is returned", func(t *testing.T) {
		plugin := viewplugin.New(worktree, render, liveRound(1, nil), func(context.Context, string) error { return errOpen })

		verdict, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval))

		if !errors.Is(err, errOpen) || verdict.Kind != shareddomain.VerdictAllow {
			t.Errorf("verdict = %v, err = %v, want allow and the open error", verdict, err)
		}
	})
}

func envelope(t *testing.T, to string) shareddomain.Envelope {
	t.Helper()
	raw, err := json.Marshal(shareddomain.TransitionPayload{Event: shareddomain.TransitionSubmitForReview, From: shareddomain.PhasePlanning, To: to})
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{
		Event:   shareddomain.EventRunTransitionAfter,
		Run:     shareddomain.RunRef{ID: runID, Phase: to},
		Payload: raw,
	}
}

func TestHostOpen(t *testing.T) {
	const pagePath = "/work/tree/.harness/runs/r1/plan.html"
	render := func(context.Context, renderview.Command) (string, error) { return pagePath, nil }
	errHost := errors.New("panel did not start")
	errBrowser := errors.New("no browser")
	opening := func(pages *[]string) viewplugin.Open {
		return func(_ context.Context, page string) error { *pages = append(*pages, page); return nil }
	}

	t.Run("the host open gets the run id and the browser stays closed", func(t *testing.T) {
		var runs, pages []string
		host := func(_ context.Context, run string) error { runs = append(runs, run); return nil }
		plugin := viewplugin.New(worktree, render, liveRound(1, nil), opening(&pages)).WithHost(host)

		_, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval))

		if err != nil || !slices.Equal(runs, []string{runID}) || len(pages) != 0 {
			t.Errorf("err = %v, runs = %v, pages = %v, want the run id and no browser", err, runs, pages)
		}
	})

	t.Run("a host open error falls back to the browser", func(t *testing.T) {
		var pages []string
		host := func(context.Context, string) error { return errHost }
		plugin := viewplugin.New(worktree, render, liveRound(1, nil), opening(&pages)).WithHost(host)

		verdict, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval))

		if err != nil || verdict.Kind != shareddomain.VerdictAllow || !slices.Equal(pages, []string{pagePath}) {
			t.Errorf("verdict = %v, err = %v, pages = %v, want allow and the browser page", verdict, err, pages)
		}
	})

	t.Run("a nil host open uses the browser", func(t *testing.T) {
		var pages []string
		plugin := viewplugin.New(worktree, render, liveRound(1, nil), opening(&pages)).WithHost(nil)

		if _, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval)); err != nil || !slices.Equal(pages, []string{pagePath}) {
			t.Errorf("err = %v, pages = %v, want the browser page", err, pages)
		}
	})

	t.Run("the fallback browser error is returned", func(t *testing.T) {
		host := func(context.Context, string) error { return errHost }
		plugin := viewplugin.New(worktree, render, liveRound(1, nil), func(context.Context, string) error { return errBrowser }).WithHost(host)

		_, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval))

		if !errors.Is(err, errBrowser) {
			t.Errorf("err = %v, want the browser error", err)
		}
	})

	t.Run("another phase calls no host open", func(t *testing.T) {
		called := false
		host := func(context.Context, string) error { called = true; return nil }
		plugin := viewplugin.New(worktree, render, liveRound(1, nil), noOpen).WithHost(host)

		if _, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseWorking)); err != nil || called {
			t.Errorf("err = %v, called = %v, want nothing", err, called)
		}
	})
}

func TestPluginRendersEveryPhase(t *testing.T) {
	var commands []renderview.Command
	render := func(_ context.Context, c renderview.Command) (string, error) {
		commands = append(commands, c)
		return "", nil
	}
	plugin := viewplugin.New(worktree, render, liveRound(2, nil), noOpen)

	transitions := []string{
		shareddomain.PhaseWorking,
		shareddomain.PhaseReviewing,
		shareddomain.PhaseShipping,
		shareddomain.PhaseDone,
	}
	for _, phase := range transitions {
		commands = nil

		verdict, err := plugin.Handle(t.Context(), envelope(t, phase))

		if err != nil || verdict.Kind != shareddomain.VerdictAllow {
			t.Fatalf("%s: verdict = %v, err = %v, want allow", phase, verdict, err)
		}
		if len(commands) != 1 || commands[0].Phase != phase || commands[0].LiveRound != 2 {
			t.Errorf("%s: commands = %+v, want one render of that phase in round 2", phase, commands)
		}
	}
}

func TestViewPlugin(t *testing.T) {
	var commands []renderview.Command
	render := func(_ context.Context, c renderview.Command) (string, error) {
		commands = append(commands, c)
		return "", nil
	}
	plugin := viewplugin.New(worktree, render, liveRound(1, nil), noOpen)

	if !slices.Equal(plugin.Subscribes(), []shareddomain.EventName{shareddomain.EventRunTransitionAfter}) {
		t.Errorf("Subscribes = %v", plugin.Subscribes())
	}

	t.Run("awaiting approval renders the page of the run in its live round", func(t *testing.T) {
		commands = nil

		verdict, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval))

		want := renderview.Command{
			Path:      filepath.Join(worktree, ".harness", "runs", runID, "plan.md"),
			RunID:     runID,
			Phase:     shareddomain.PhaseAwaitingApproval,
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
		noRound := viewplugin.New(worktree, render, liveRound(1, errRound), noOpen)

		verdict, err := noRound.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval))

		if !errors.Is(err, errRound) || verdict.Kind != shareddomain.VerdictAllow {
			t.Fatalf("verdict = %v, err = %v, want allow and the round error", verdict, err)
		}
		if len(commands) != 1 || commands[0].LiveRound != 0 {
			t.Errorf("commands = %+v, want one render in round 0", commands)
		}
	})

	t.Run("another phase renders the page too", func(t *testing.T) {
		commands = nil

		verdict, err := plugin.Handle(t.Context(), envelope(t, shareddomain.PhaseWorking))

		if err != nil || verdict.Kind != shareddomain.VerdictAllow || len(commands) != 1 || commands[0].Phase != shareddomain.PhaseWorking {
			t.Errorf("verdict = %v, err = %v, commands = %+v, want one render in working", verdict, err, commands)
		}
	})

	t.Run("a render error allows and reports the error", func(t *testing.T) {
		failing := viewplugin.New(worktree, func(context.Context, renderview.Command) (string, error) { return "", errRender }, liveRound(0, nil), noOpen)

		verdict, err := failing.Handle(t.Context(), envelope(t, shareddomain.PhaseAwaitingApproval))

		if !errors.Is(err, errRender) || verdict.Kind != shareddomain.VerdictAllow {
			t.Errorf("verdict = %v, err = %v, want allow and the render error", verdict, err)
		}
	})

	t.Run("a bad payload reports an error", func(t *testing.T) {
		bad := envelope(t, shareddomain.PhaseAwaitingApproval)
		bad.Payload = []byte("{")

		_, err := plugin.Handle(t.Context(), bad)

		if err == nil {
			t.Error("want a decode error")
		}
	})
}

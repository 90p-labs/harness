package marksplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/host/application/marksplugin"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const runID = "20261110-082002-close"

func transition(t *testing.T, to string) shareddomain.Envelope {
	t.Helper()
	raw, err := json.Marshal(shareddomain.TransitionPayload{To: to})
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{
		Event:   shareddomain.EventRunTransitionAfter,
		Run:     shareddomain.RunRef{ID: runID, Phase: to},
		Payload: raw,
	}
}

func TestPluginDeletesTheMarksOfAClosedRun(t *testing.T) {
	cases := []struct {
		to   string
		want []string
	}{
		{shareddomain.PhaseDone, []string{runID}},
		{shareddomain.PhaseAbandoned, []string{runID}},
		{shareddomain.PhaseWorking, nil},
		{shareddomain.PhaseBlocked, nil},
	}
	for _, tc := range cases {
		t.Run(tc.to, func(t *testing.T) {
			var runs []string
			plugin := marksplugin.New(func(_ context.Context, id string) error {
				runs = append(runs, id)
				return nil
			})

			got, err := plugin.Handle(context.Background(), transition(t, tc.to))

			if err != nil || got.Kind != shareddomain.VerdictAllow || !reflect.DeepEqual(runs, tc.want) {
				t.Fatalf("verdict = %+v, err = %v, runs = %v, want allow, nil, %v", got, err, runs, tc.want)
			}
		})
	}
}

func TestPluginFailureNeverDenies(t *testing.T) {
	boom := errors.New("boom")
	plugin := marksplugin.New(func(context.Context, string) error { return boom })

	got, err := plugin.Handle(context.Background(), transition(t, shareddomain.PhaseDone))

	if !errors.Is(err, boom) || got.Kind != shareddomain.VerdictAllow {
		t.Fatalf("verdict = %+v, err = %v, want allow with the error", got, err)
	}
}

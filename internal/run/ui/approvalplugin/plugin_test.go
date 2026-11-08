package approvalplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/jorge07/harness/internal/run/ui/approvalplugin"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const runID = "20261108-170222-add-a-grill-step"

var approve = shareddomain.Answer{Header: shareddomain.ApprovalHeader, Label: shareddomain.ApproveLabel}

func envelope(t *testing.T, phase string, payload shareddomain.ToolPayload) shareddomain.Envelope {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{
		Event:   shareddomain.EventToolAfter,
		Run:     shareddomain.RunRef{ID: runID, Phase: phase},
		Payload: raw,
	}
}

func TestApprovalRecordPlugin(t *testing.T) {
	question := func(answers ...shareddomain.Answer) shareddomain.ToolPayload {
		return shareddomain.ToolPayload{Kind: shareddomain.ToolKindQuestion, Answers: answers}
	}
	recorded := shareddomain.Verdict{Kind: shareddomain.VerdictContext, Context: approvalplugin.RecordedContext}
	allow := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	cases := []struct {
		name     string
		phase    string
		payload  shareddomain.ToolPayload
		want     shareddomain.Verdict
		wantRuns []string
	}{
		{"an Approve answer records the approval", shareddomain.PhaseAwaitingApproval, question(approve), recorded, []string{runID}},
		{"a Change the plan answer records nothing", shareddomain.PhaseAwaitingApproval,
			question(shareddomain.Answer{Header: shareddomain.ApprovalHeader, Label: shareddomain.ChangePlanLabel}), allow, nil},
		{"an Approve label of another question records nothing", shareddomain.PhaseAwaitingApproval,
			question(shareddomain.Answer{Header: "Storage", Label: shareddomain.ApproveLabel}), allow, nil},
		{"another phase records nothing", shareddomain.PhaseWorking, question(approve), allow, nil},
		{"another tool records nothing", shareddomain.PhaseAwaitingApproval,
			shareddomain.ToolPayload{Kind: shareddomain.ToolKindShell, Answers: []shareddomain.Answer{approve}}, allow, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var runs []string
			plugin := approvalplugin.New(func(_ context.Context, id string) error {
				runs = append(runs, id)
				return nil
			})

			got, err := plugin.Handle(context.Background(), envelope(t, tc.phase, tc.payload))

			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(runs, tc.wantRuns) {
				t.Fatalf("verdict = %+v, runs = %v, want %+v, %v", got, runs, tc.want, tc.wantRuns)
			}
		})
	}

	t.Run("a record error returns the error", func(t *testing.T) {
		errStore := errors.New("store is locked")
		plugin := approvalplugin.New(func(context.Context, string) error { return errStore })

		_, err := plugin.Handle(context.Background(), envelope(t, shareddomain.PhaseAwaitingApproval, question(approve)))

		if !errors.Is(err, errStore) {
			t.Fatalf("err = %v, want %v", err, errStore)
		}
	})
}

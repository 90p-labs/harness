package guardplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/guard/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	agentReviewer = "reviewer"
	agentAdvisor  = "advisor"
	agentWorker   = "worker"
	agentExplorer = "explorer"
	modelOpus     = "opus"
	modelHaiku    = "haiku"
	modelSol      = "gpt-6.1-sol"
	modelTerra    = "gpt-6.1-terra"
	modelAstra    = "gpt-6-astra"
	otherEvent    = "submit-plan"
	writeFailure  = "disk full"
)

type reviewStore struct {
	models  []string
	saved   []string
	saveErr error
}

func (s *reviewStore) save(_ context.Context, _, model string) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = append(s.saved, model)
	return nil
}

func (s *reviewStore) read(context.Context, string) ([]string, error) { return s.models, nil }

func reviewPlugins(t *testing.T, disabled []string, store *reviewStore) []shareddomain.Plugin {
	t.Helper()
	d := deps(&fakeStop{})
	d.SaveReview = store.save
	d.ReviewModels = store.read
	return build(t, disabled, d)
}

func agentAfter(t *testing.T, phase, agentType, model string) shareddomain.Envelope {
	t.Helper()
	env := toolEnvelope(t, shareddomain.HostClaude, t.TempDir(), toolAgent, map[string]string{"subagent_type": agentType, "model": model})
	env.Event = shareddomain.EventToolAfter
	env.Run = shareddomain.RunRef{ID: "r1", Phase: phase}
	return env
}

func transitionEnvelope(t *testing.T, event string) shareddomain.Envelope {
	t.Helper()
	payload, err := json.Marshal(shareddomain.TransitionPayload{Event: event})
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{Event: shareddomain.EventRunTransitionBefore, Payload: payload}
}

func TestReviewRecorded(t *testing.T) {
	cases := []struct {
		name  string
		phase string
		agent string
		model string
		want  []string
	}{
		{"reviewer", shareddomain.PhaseReviewing, agentReviewer, modelOpus, []string{modelOpus}},
		{"advisor", shareddomain.PhaseReviewing, agentAdvisor, modelOpus, nil},
		{"worker", shareddomain.PhaseReviewing, agentWorker, modelOpus, nil},
		{"explorer", shareddomain.PhaseReviewing, agentExplorer, modelOpus, nil},
		{"reviewer outside reviewing", shareddomain.PhaseWorking, agentReviewer, modelOpus, nil},
		{"reviewer without model", shareddomain.PhaseReviewing, agentReviewer, "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &reviewStore{}
			v := dispatch(t, newBus(reviewPlugins(t, nil, store)), agentAfter(t, c.phase, c.agent, c.model))
			if !slices.Equal(store.saved, c.want) {
				t.Errorf("saved = %v, want %v", store.saved, c.want)
			}
			if v.Kind == shareddomain.VerdictDeny {
				t.Errorf("verdict = %+v, want no deny", v)
			}
		})
	}

	t.Run("failed write", func(t *testing.T) {
		store := &reviewStore{saveErr: errors.New(writeFailure)}
		v := dispatch(t, newBus(reviewPlugins(t, nil, store)), agentAfter(t, shareddomain.PhaseReviewing, agentReviewer, modelOpus))
		if !strings.Contains(v.Context, writeFailure) {
			t.Errorf("verdict = %+v, want context naming %q", v, writeFailure)
		}
	})
}

func TestReviewModelsShip(t *testing.T) {
	cases := []struct {
		name   string
		models []string
		deny   bool
	}{
		{"none", nil, true},
		{"one", []string{modelOpus}, true},
		{"two", []string{modelOpus, modelSonnet}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bus := newBus(reviewPlugins(t, nil, &reviewStore{models: c.models}))
			v := dispatch(t, bus, transitionEnvelope(t, shareddomain.TransitionApproveReview))
			if (v.Kind == shareddomain.VerdictDeny) != c.deny {
				t.Fatalf("verdict = %+v, want deny=%v", v, c.deny)
			}
			for _, m := range c.models {
				if c.deny && !strings.Contains(v.Reason, m) {
					t.Errorf("reason %q does not list %q", v.Reason, m)
				}
			}
		})
	}

	hostLists := []struct {
		name   string
		models []string
		deny   bool
	}{
		{"claude list incomplete", []string{modelSonnet, modelHaiku}, true},
		{"claude list complete", []string{modelSonnet, modelHaiku, modelOpus}, false},
	}
	for _, c := range hostLists {
		t.Run(c.name, func(t *testing.T) {
			d := deps(&fakeStop{})
			hostStore := &reviewStore{models: c.models}
			d.ReviewModels = hostStore.read
			d.Models = map[shareddomain.Host]map[string][]string{
				shareddomain.HostClaude: {string(shareddomain.RoleReviewer): {modelSonnet, modelHaiku, modelOpus}},
				shareddomain.HostCodex:  {string(shareddomain.RoleReviewer): {modelSol, modelTerra, modelAstra}},
			}
			v := dispatch(t, newBus(build(t, nil, d)), transitionEnvelope(t, shareddomain.TransitionApproveReview))
			if (v.Kind == shareddomain.VerdictDeny) != c.deny {
				t.Fatalf("verdict = %+v, want deny=%v", v, c.deny)
			}
			if c.deny && !strings.Contains(v.Reason, modelOpus) {
				t.Errorf("reason %q does not name %q", v.Reason, modelOpus)
			}
		})
	}

	store := &reviewStore{models: []string{modelOpus}}
	disabled := newBus(reviewPlugins(t, []string{domain.RuleReviewModels}, store))
	if v := dispatch(t, disabled, transitionEnvelope(t, shareddomain.TransitionApproveReview)); v.Kind != shareddomain.VerdictAllow {
		t.Errorf("disabled: verdict = %+v, want allow", v)
	}
	enabled := newBus(reviewPlugins(t, nil, store))
	if v := dispatch(t, enabled, transitionEnvelope(t, otherEvent)); v.Kind != shareddomain.VerdictAllow {
		t.Errorf("other event: verdict = %+v, want allow", v)
	}
	if v := dispatch(t, enabled, transitionEnvelope(t, shareddomain.TransitionApproveReview)); strings.Contains(v.Reason, domain.NextStepReviewerList) {
		t.Errorf("open role list: reason %q asks to widen the role, want the second model step", v.Reason)
	}
}

func TestReviewModelsShipOneListedModel(t *testing.T) {
	d := deps(&fakeStop{})
	store := &reviewStore{models: []string{modelOpus}}
	d.ReviewModels = store.read
	d.Models = map[shareddomain.Host]map[string][]string{
		shareddomain.HostClaude: {string(shareddomain.RoleReviewer): {modelOpus}},
	}
	v := dispatch(t, newBus(build(t, nil, d)), transitionEnvelope(t, shareddomain.TransitionApproveReview))
	if v.Kind != shareddomain.VerdictDeny || !strings.Contains(v.Reason, domain.NextStepReviewerList) {
		t.Fatalf("verdict = %+v, want a deny that asks to widen the role or turn the rule off", v)
	}
}

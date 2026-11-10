package guardplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/guard/application/guardplugin"
	"github.com/90p-labs/harness/internal/guard/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
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
	unread  []string
	saved   []domain.Review
	saveErr error
	scope   domain.ReviewScope
}

func (s *reviewStore) save(_ context.Context, _ string, review domain.Review) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = append(s.saved, review)
	return nil
}

func (s *reviewStore) read(_ context.Context, _ string, scope domain.ReviewScope) ([]domain.Review, error) {
	s.scope = scope
	var reviews []domain.Review
	for _, model := range s.models {
		reviews = append(reviews, domain.Review{Model: model, FilesRead: []string{changedFile}})
	}
	for _, model := range s.unread {
		reviews = append(reviews, domain.Review{Model: model})
	}
	return reviews, nil
}

func reviewPlugins(t *testing.T, disabled []string, store *reviewStore) []shareddomain.Plugin {
	t.Helper()
	d := deps(&fakeStop{})
	d.SaveReview = store.save
	d.Reviews = store.read
	return build(t, disabled, d)
}

func transitionEnvelope(t *testing.T, event string) shareddomain.Envelope {
	t.Helper()
	payload, err := json.Marshal(shareddomain.TransitionPayload{Event: event})
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{Event: shareddomain.EventRunTransitionBefore, Payload: payload}
}

const (
	changedFile    = "a.go"
	otherFile      = "b.go"
	reviewWorktree = "/work/repo"
	reviewPatchID  = "p1"
	transcriptFile = "/t/agent.jsonl"
	haikuModelID   = "claude-haiku-5-5"
)

func transcriptReading(model, file string) string {
	return `{"type":"assistant","message":{"model":"` + model + `","content":[{"type":"tool_use","name":"Read","input":{"file_path":"` + reviewWorktree + `/` + file + `"}}]}}` + "\n"
}

func subagentStopped(t *testing.T, host shareddomain.Host, phase, agentType, transcript string) shareddomain.Envelope {
	t.Helper()
	payload, err := json.Marshal(shareddomain.SubagentPayload{AgentID: "a1", AgentType: agentType, TranscriptPath: transcript})
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{
		Event: shareddomain.EventSubagentStopped, Host: host, Payload: payload,
		Run: shareddomain.RunRef{ID: "r1", Phase: phase},
	}
}

func recorderDeps(store *reviewStore, transcript string) guardplugin.Deps {
	d := deps(&fakeStop{})
	d.SaveReview = store.save
	d.Reviews = store.read
	d.Worktree = reviewWorktree
	d.ReadFile = func(path string) ([]byte, error) {
		if path != transcriptFile {
			return nil, fs.ErrNotExist
		}
		return []byte(transcript), nil
	}
	d.ChangedFiles = func(context.Context, string, string) ([]string, error) { return []string{changedFile, otherFile}, nil }
	d.PatchID = func(context.Context, string, string) (string, error) { return reviewPatchID, nil }
	d.Models = map[shareddomain.Host]map[string][]string{
		shareddomain.HostClaude: {string(shareddomain.RoleReviewer): {modelSonnet, modelHaiku, modelOpus}},
	}
	return d
}

func TestReviewRecorder(t *testing.T) {
	haikuRead := transcriptReading(haikuModelID, changedFile)
	cases := []struct {
		name       string
		phase      string
		agent      string
		path       string
		transcript string
		want       []domain.Review
	}{
		{"reviewer", shareddomain.PhaseReviewing, agentReviewer, transcriptFile, haikuRead,
			[]domain.Review{{Model: modelHaiku, PatchID: reviewPatchID, RunID: "r1", FilesRead: []string{changedFile}}}},
		{"reviewer that read no changed file", shareddomain.PhaseReviewing, agentReviewer, transcriptFile, transcriptReading(haikuModelID, "c.go"),
			[]domain.Review{{Model: modelHaiku, PatchID: reviewPatchID, RunID: "r1"}}},
		{"advisor", shareddomain.PhaseReviewing, agentAdvisor, transcriptFile, haikuRead, nil},
		{"worker", shareddomain.PhaseReviewing, agentWorker, transcriptFile, haikuRead, nil},
		{"reviewer outside reviewing", shareddomain.PhaseWorking, agentReviewer, transcriptFile, haikuRead, nil},
		{"model not listed", shareddomain.PhaseReviewing, agentReviewer, transcriptFile, transcriptReading("claude-fable-5", changedFile), nil},
		{"transcript without a model", shareddomain.PhaseReviewing, agentReviewer, transcriptFile, "not json\n", nil},
		{"no transcript path", shareddomain.PhaseReviewing, agentReviewer, "", haikuRead, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &reviewStore{}
			bus := newBus(build(t, nil, recorderDeps(store, c.transcript)))
			v := dispatch(t, bus, subagentStopped(t, shareddomain.HostClaude, c.phase, c.agent, c.path))
			if !reflect.DeepEqual(store.saved, c.want) {
				t.Errorf("saved = %+v, want %+v", store.saved, c.want)
			}
			if v.Kind == shareddomain.VerdictDeny {
				t.Errorf("verdict = %+v, want no deny", v)
			}
		})
	}

	failures := map[string]func(d *guardplugin.Deps){
		"save": func(d *guardplugin.Deps) { d.SaveReview = (&reviewStore{saveErr: errors.New(writeFailure)}).save },
		"transcript": func(d *guardplugin.Deps) {
			d.ReadFile = func(string) ([]byte, error) { return nil, errors.New(writeFailure) }
		},
		"changed": func(d *guardplugin.Deps) {
			d.ChangedFiles = func(context.Context, string, string) ([]string, error) { return nil, errors.New(writeFailure) }
		},
	}
	for name, fail := range failures {
		t.Run("failed "+name, func(t *testing.T) {
			d := recorderDeps(&reviewStore{}, haikuRead)
			fail(&d)
			var recorder shareddomain.Plugin
			for _, p := range build(t, nil, d) {
				if p.Name() == pluginReviewRecorder {
					recorder = p
				}
			}
			_, err := recorder.Handle(context.Background(), subagentStopped(t, shareddomain.HostClaude, shareddomain.PhaseReviewing, agentReviewer, transcriptFile))
			if err == nil || !strings.Contains(err.Error(), writeFailure) {
				t.Errorf("error = %v, want one that names %q", err, writeFailure)
			}
		})
	}
}

func TestReviewShipScope(t *testing.T) {
	cases := []struct {
		name     string
		strict   bool
		findings bool
		want     domain.ReviewScope
	}{
		{"default rule matches the run with findings", false, true, domain.ReviewScope{PatchID: reviewPatchID, RunID: "r1"}},
		{"default rule drops the run match without findings", false, false, domain.ReviewScope{PatchID: reviewPatchID}},
		{"strict rule drops the run match", true, true, domain.ReviewScope{PatchID: reviewPatchID}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &reviewStore{models: []string{modelOpus, modelSonnet, modelHaiku}}
			d := recorderDeps(store, "")
			d.StrictMinor = c.strict
			d.ReadFile = func(path string) ([]byte, error) {
				if c.findings && path == filepath.Join(reviewWorktree, shareddomain.RunFindingsFile("r1")) {
					return []byte(findingsTable()), nil
				}
				return nil, fs.ErrNotExist
			}
			env := transitionEnvelope(t, shareddomain.TransitionApproveReview)
			env.Run = shareddomain.RunRef{ID: "r1"}
			if v := dispatch(t, newBus(build(t, nil, d)), env); v.Kind == shareddomain.VerdictDeny {
				t.Fatalf("verdict = %+v, want allow", v)
			}
			if store.scope != c.want {
				t.Errorf("scope = %+v, want %+v", store.scope, c.want)
			}
		})
	}
}

func TestReviewShipFindingsReadError(t *testing.T) {
	store := &reviewStore{models: []string{modelOpus, modelSonnet, modelHaiku}}
	d := recorderDeps(store, "")
	d.ReadFile = func(string) ([]byte, error) { return nil, errors.New(findingsRead) }
	env := transitionEnvelope(t, shareddomain.TransitionApproveReview)
	env.Run = shareddomain.RunRef{ID: "r1"}
	if v := dispatch(t, newBus(build(t, []string{domain.RuleReviewFindings}, d)), env); v.Kind == shareddomain.VerdictDeny {
		t.Fatalf("verdict = %+v, want allow", v)
	}
	if want := (domain.ReviewScope{PatchID: reviewPatchID}); store.scope != want {
		t.Errorf("scope = %+v, want %+v", store.scope, want)
	}
}

func TestReviewShipWithoutPatchID(t *testing.T) {
	store := &reviewStore{models: []string{modelOpus, modelSonnet, modelHaiku}}
	d := recorderDeps(store, "")
	d.PatchID = func(context.Context, string, string) (string, error) { return "", errors.New("no merge base") }
	d.ReadFile = func(string) ([]byte, error) { return []byte(findingsTable()), nil }
	env := transitionEnvelope(t, shareddomain.TransitionApproveReview)
	env.Run = shareddomain.RunRef{ID: "r1"}
	if v := dispatch(t, newBus(build(t, nil, d)), env); v.Kind == shareddomain.VerdictDeny {
		t.Fatalf("verdict = %+v, want allow", v)
	}
	if want := (domain.ReviewScope{RunID: "r1"}); store.scope != want {
		t.Errorf("scope = %+v, want %+v", store.scope, want)
	}
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
			d.Reviews = hostStore.read
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
	d.Reviews = store.read
	d.Models = map[shareddomain.Host]map[string][]string{
		shareddomain.HostClaude: {string(shareddomain.RoleReviewer): {modelOpus}},
	}
	v := dispatch(t, newBus(build(t, nil, d)), transitionEnvelope(t, shareddomain.TransitionApproveReview))
	if v.Kind != shareddomain.VerdictDeny || !strings.Contains(v.Reason, domain.NextStepReviewerList) {
		t.Fatalf("verdict = %+v, want a deny that asks to widen the role or turn the rule off", v)
	}
}

func TestReviewShipPanel(t *testing.T) {
	const (
		withModel = "| ID | Severity | Verdict | Status | Model |\n|---|---|---|---|---|\n| F1 | blocker | real | closed | gpt-6.1-sol |\n"
		noModel   = "| ID | Severity | Verdict | Status |\n|---|---|---|---|\n| F1 | blocker | real | closed |\n"
	)
	// Role order: haiku, sonnet, opus. The small panel is haiku and sonnet, plus the model of F1.
	listed := []string{modelHaiku, modelSonnet, modelOpus}
	cases := []struct {
		name     string
		table    string
		last     domain.ReworkEvent
		lines    int
		reviewed []string
		deny     bool
	}{
		{"panel reviewed", strings.Replace(withModel, "gpt-6.1-sol", modelOpus, 1), domain.ReworkEvent{Name: shareddomain.TransitionRequestRework, Reason: "fix F1"}, 10, []string{modelHaiku, modelSonnet, modelOpus}, false},
		{"small panel reviewed", strings.Replace(withModel, "gpt-6.1-sol", modelSonnet, 1), domain.ReworkEvent{Name: shareddomain.TransitionRequestRework, Reason: "fix F1"}, 10, []string{modelHaiku, modelSonnet}, false},
		{"small panel missing the reworked model", strings.Replace(withModel, "gpt-6.1-sol", modelOpus, 1), domain.ReworkEvent{Name: shareddomain.TransitionRequestRework, Reason: "fix F1"}, 10, []string{modelHaiku, modelSonnet}, true},
		{"no rework", withModel, domain.ReworkEvent{}, 10, []string{modelHaiku, modelSonnet}, true},
		{"large fixup", strings.Replace(withModel, "gpt-6.1-sol", modelSonnet, 1), domain.ReworkEvent{Name: shareddomain.TransitionRequestRework, Reason: "fix F1"}, 81, []string{modelHaiku, modelSonnet}, true},
		{"no Model column", noModel, domain.ReworkEvent{Name: shareddomain.TransitionRequestRework, Reason: "fix F1"}, 10, []string{modelHaiku, modelSonnet}, true},
		{"amend after the rework", strings.Replace(withModel, "gpt-6.1-sol", modelSonnet, 1), domain.ReworkEvent{Name: shareddomain.TransitionAmend, Reason: "fix F1"}, 10, []string{modelHaiku, modelSonnet}, true},
		{"regress after the rework", strings.Replace(withModel, "gpt-6.1-sol", modelSonnet, 1), domain.ReworkEvent{Name: shareddomain.TransitionRegress, Reason: "fix F1"}, 10, []string{modelHaiku, modelSonnet}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &reviewStore{models: c.reviewed}
			d := recorderDeps(store, "")
			d.Models = map[shareddomain.Host]map[string][]string{
				shareddomain.HostClaude: {string(shareddomain.RoleReviewer): listed},
			}
			d.ReadFile = func(string) ([]byte, error) { return []byte(c.table), nil }
			d.ReworkReasons = func(context.Context, string, string) ([]string, error) { return []string{"fix F1"}, nil }
			d.Panel = guardplugin.PanelSource{
				LastRework:    func(context.Context, string, string) (domain.ReworkEvent, error) { return c.last, nil },
				ReviewedByAll: func(context.Context, string, string, []string) (string, error) { return "abc123", nil },
				FixupLines:    func(context.Context, string, string) (int, error) { return c.lines, nil },
				Limit:         80,
			}
			env := transitionEnvelope(t, shareddomain.TransitionApproveReview)
			env.Run = shareddomain.RunRef{ID: "r1"}
			v := dispatch(t, newBus(build(t, nil, d)), env)
			if (v.Kind == shareddomain.VerdictDeny) != c.deny {
				t.Fatalf("verdict = %+v, want deny=%v", v, c.deny)
			}
		})
	}
	// The host is chosen before the panel shrinks. A shrunk panel must not hand the check to another host.
	t.Run("two hosts keep the chosen host", func(t *testing.T) {
		store := &reviewStore{models: []string{"fable", modelOpus, "gpt-a", "gpt-b"}}
		d := recorderDeps(store, "")
		d.Models = map[shareddomain.Host]map[string][]string{
			shareddomain.HostClaude: {string(shareddomain.RoleReviewer): {modelHaiku, modelSonnet, modelOpus, "fable"}},
			shareddomain.HostCodex:  {string(shareddomain.RoleReviewer): {"gpt-a", "gpt-b"}},
		}
		d.ReadFile = func(string) ([]byte, error) {
			return []byte(strings.Replace(withModel, "gpt-6.1-sol", modelSonnet, 1)), nil
		}
		d.ReworkReasons = func(context.Context, string, string) ([]string, error) { return []string{"fix F1"}, nil }
		d.Panel = guardplugin.PanelSource{
			LastRework: func(context.Context, string, string) (domain.ReworkEvent, error) {
				return domain.ReworkEvent{Name: shareddomain.TransitionRequestRework, Reason: "fix F1"}, nil
			},
			ReviewedByAll: func(context.Context, string, string, []string) (string, error) { return "abc123", nil },
			FixupLines:    func(context.Context, string, string) (int, error) { return 10, nil },
			Limit:         80,
		}
		env := transitionEnvelope(t, shareddomain.TransitionApproveReview)
		env.Run = shareddomain.RunRef{ID: "r1"}
		if v := dispatch(t, newBus(build(t, nil, d)), env); v.Kind != shareddomain.VerdictDeny {
			t.Fatalf("verdict = %+v, want deny: no model of the claude panel reviewed", v)
		}
	})
	// A missing fact gives the full panel, so the small panel alone is denied.
	fallbacks := []struct {
		name     string
		head     string
		linesErr error
	}{
		{"no HEAD reviewed by every model", "", nil},
		{"unreadable fixup size", "abc123", errors.New("git failed")},
	}
	for _, c := range fallbacks {
		t.Run(c.name, func(t *testing.T) {
			store := &reviewStore{models: []string{modelHaiku, modelSonnet}}
			d := recorderDeps(store, "")
			d.Models = map[shareddomain.Host]map[string][]string{
				shareddomain.HostClaude: {string(shareddomain.RoleReviewer): listed},
			}
			d.ReadFile = func(string) ([]byte, error) {
				return []byte(strings.Replace(withModel, "gpt-6.1-sol", modelSonnet, 1)), nil
			}
			d.ReworkReasons = func(context.Context, string, string) ([]string, error) { return []string{"fix F1"}, nil }
			d.Panel = guardplugin.PanelSource{
				LastRework: func(context.Context, string, string) (domain.ReworkEvent, error) {
					return domain.ReworkEvent{Name: shareddomain.TransitionRequestRework, Reason: "fix F1"}, nil
				},
				ReviewedByAll: func(context.Context, string, string, []string) (string, error) { return c.head, nil },
				FixupLines:    func(context.Context, string, string) (int, error) { return 10, c.linesErr },
				Limit:         80,
			}
			env := transitionEnvelope(t, shareddomain.TransitionApproveReview)
			env.Run = shareddomain.RunRef{ID: "r1"}
			if v := dispatch(t, newBus(build(t, nil, d)), env); v.Kind != shareddomain.VerdictDeny {
				t.Fatalf("verdict = %+v, want deny: the full panel applies", v)
			}
		})
	}
}

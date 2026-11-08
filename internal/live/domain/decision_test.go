package domain_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/jorge07/harness/internal/live/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

var (
	deployedAt = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	window     = 30 * time.Minute
	breach     = domain.SignalBreach{
		Threshold: domain.Threshold{Signal: "error rate", Source: "prom", Window: window},
		Reason:    "0.034",
	}
)

func TestDecideReleasing(t *testing.T) {
	merged := domain.Merge{State: domain.MergeMerged, SHA: "abc1234", At: deployedAt}
	tests := []struct {
		name  string
		facts domain.Facts
		want  domain.Decision
	}{
		{"pr open", domain.Facts{Merge: domain.Merge{State: domain.MergeOpen}},
			domain.Decision{Outcome: domain.OutcomePending, Event: domain.EventNone}},
		{"pr closed", domain.Facts{Merge: domain.Merge{State: domain.MergeClosed}},
			domain.Decision{Outcome: domain.OutcomeRegression, Event: domain.EventRegress, DeployFailure: domain.FailurePRClosed}},
		{"deploy pending", domain.Facts{Merge: merged, Deployment: domain.Deployment{State: domain.DeployPending}},
			domain.Decision{Outcome: domain.OutcomePending, Event: domain.EventNone}},
		{"deploy failed", domain.Facts{Merge: merged, Deployment: domain.Deployment{State: domain.DeployFailed}},
			domain.Decision{Outcome: domain.OutcomeRegression, Event: domain.EventRegress, DeployFailure: domain.FailureDeploy}},
		{"deploy success", domain.Facts{Merge: merged, Deployment: domain.Deployment{State: domain.DeploySuccess}},
			domain.Decision{Outcome: domain.OutcomeDeployed, Event: domain.EventDeploy}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.Decide(domain.PhaseReleasing, tc.facts, deployedAt)
			if got.Outcome != tc.want.Outcome || got.Event != tc.want.Event || got.DeployFailure != tc.want.DeployFailure {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDecideObserving(t *testing.T) {
	tests := []struct {
		name  string
		facts domain.Facts
		now   time.Time
		want  domain.Decision
	}{
		{"breach", domain.Facts{DeployedAt: deployedAt, Window: window, Breaches: []domain.SignalBreach{breach, breach}},
			deployedAt.Add(time.Minute),
			domain.Decision{Outcome: domain.OutcomeRegression, Event: domain.EventRegress, Breaches: []domain.SignalBreach{breach, breach}}},
		{"window running", domain.Facts{DeployedAt: deployedAt, Window: window},
			deployedAt.Add(window - time.Second),
			domain.Decision{Outcome: domain.OutcomePending, Event: domain.EventNone}},
		{"window over", domain.Facts{DeployedAt: deployedAt, Window: window},
			deployedAt.Add(window),
			domain.Decision{Outcome: domain.OutcomeHealthy, Event: domain.EventClose}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.Decide(domain.PhaseObserving, tc.facts, tc.now)
			if got.Outcome != tc.want.Outcome || got.Event != tc.want.Event || len(got.Breaches) != len(tc.want.Breaches) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDecideSignalsUnavailable(t *testing.T) {
	facts := domain.Facts{DeployedAt: deployedAt, Window: window, Failures: 1}
	graceEnd := deployedAt.Add(2 * window)
	tests := []struct {
		name string
		now  time.Time
		want domain.Decision
	}{
		{"before window end", deployedAt.Add(window - time.Minute),
			domain.Decision{Outcome: domain.OutcomePending, Event: domain.EventNone}},
		{"inside grace", graceEnd,
			domain.Decision{Outcome: domain.OutcomePending, Event: domain.EventNone}},
		{"past grace", graceEnd.Add(time.Second),
			domain.Decision{Outcome: domain.OutcomeUnavailable, Event: domain.EventBlock}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.Decide(domain.PhaseObserving, facts, tc.now)
			if got.Outcome != tc.want.Outcome || got.Event != tc.want.Event {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}

	withBreach := facts
	withBreach.Breaches = []domain.SignalBreach{breach}
	got := domain.Decide(domain.PhaseObserving, withBreach, graceEnd.Add(time.Hour))
	if got.Outcome != domain.OutcomeRegression {
		t.Fatalf("a breach must win over failures, got %+v", got)
	}
}

func TestNewSignalCheck(t *testing.T) {
	threshold := domain.Threshold{
		Signal: "error rate", Source: "prometheus", Query: "api:http_errors:ratio_rate5m",
		Breach: shareddomain.Breach{Op: shareddomain.OperatorGreater, Value: 0.01}, Window: window,
	}
	release := domain.Release{MergeSHA: "abc1234", DeployedAt: deployedAt, Environment: "production"}

	got, err := json.Marshal(domain.NewSignalCheck(threshold, release, deployedAt.Add(time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	const spec = `{"signal":"error rate","source":"prometheus","query":"api:http_errors:ratio_rate5m","breach":{"op":">","value":0.01},"window":{"from":"2026-11-08T10:00:00Z","to":"2026-11-08T10:30:00Z"},"sha":"abc1234","environment":"production"}`
	var gotPayload, specPayload map[string]any
	if err := json.Unmarshal(got, &gotPayload); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(spec), &specPayload); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotPayload, specPayload) {
		t.Fatalf("got %s, want %s", got, spec)
	}

	early := domain.NewSignalCheck(threshold, release, deployedAt.Add(10*time.Minute))
	if early.Window.To != "2026-11-08T10:10:00Z" {
		t.Fatalf("window must end at now while the watch runs, got %s", early.Window.To)
	}
}

func TestWatchWindow(t *testing.T) {
	if got := domain.WatchWindow(nil); got != 0 {
		t.Fatalf("no thresholds must give 0, got %s", got)
	}
	got := domain.WatchWindow([]domain.Threshold{{Window: window}, {Window: time.Hour}, {Window: 5 * time.Minute}})
	if got != time.Hour {
		t.Fatalf("got %s, want 1h", got)
	}
}

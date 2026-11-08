package domain

import (
	"time"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type Phase string

const (
	PhaseReleasing Phase = shareddomain.PhaseReleasing
	PhaseObserving Phase = shareddomain.PhaseObserving
)

type Outcome string

const (
	OutcomePending     Outcome = "pending"
	OutcomeDeployed    Outcome = "deployed"
	OutcomeRegression  Outcome = "regression"
	OutcomeHealthy     Outcome = "healthy"
	OutcomeUnavailable Outcome = "unavailable"
	OutcomeSkipped     Outcome = "skipped"
	OutcomeError       Outcome = "error"
)

type Event string

const (
	EventNone    Event = ""
	EventDeploy  Event = "deploy"
	EventRegress Event = "regress"
	EventClose   Event = "close"
	EventBlock   Event = "block"
)

const (
	ReasonSignalsUnavailable = "signals unavailable"
	FailurePRClosed          = "PR closed with no merge"
	FailureDeploy            = "deploy failed"
)

type SignalBreach struct {
	Threshold Threshold
	Reason    string
}

type Facts struct {
	Merge      Merge
	Deployment Deployment
	DeployedAt time.Time
	Window     time.Duration
	Breaches   []SignalBreach
	Failures   int
}

type Decision struct {
	Outcome       Outcome
	Event         Event
	Breaches      []SignalBreach
	DeployFailure string
}

var pending = Decision{Outcome: OutcomePending, Event: EventNone}

func Decide(phase Phase, facts Facts, now time.Time) Decision {
	if phase == PhaseReleasing {
		return decideReleasing(facts)
	}
	return decideObserving(facts, now)
}

func decideReleasing(facts Facts) Decision {
	if facts.Merge.State == MergeClosed {
		return Decision{Outcome: OutcomeRegression, Event: EventRegress, DeployFailure: FailurePRClosed}
	}
	if facts.Merge.State != MergeMerged {
		return pending
	}
	switch facts.Deployment.State {
	case DeployFailed:
		return Decision{Outcome: OutcomeRegression, Event: EventRegress, DeployFailure: FailureDeploy}
	case DeploySuccess:
		return Decision{Outcome: OutcomeDeployed, Event: EventDeploy}
	}
	return pending
}

func decideObserving(facts Facts, now time.Time) Decision {
	if len(facts.Breaches) > 0 {
		return Decision{Outcome: OutcomeRegression, Event: EventRegress, Breaches: facts.Breaches}
	}
	windowEnd := facts.DeployedAt.Add(facts.Window)
	if facts.Failures > 0 {
		if now.After(windowEnd.Add(facts.Window)) {
			return Decision{Outcome: OutcomeUnavailable, Event: EventBlock}
		}
		return pending
	}
	if !now.Before(windowEnd) {
		return Decision{Outcome: OutcomeHealthy, Event: EventClose}
	}
	return pending
}

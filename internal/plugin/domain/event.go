package domain

type EventName string

type Kind string

const (
	KindVeto    Kind = "veto"
	KindCollect Kind = "collect"
	KindContext Kind = "context"
	KindObserve Kind = "observe"
)

const (
	EventSessionStarted      EventName = "session.started"
	EventPromptSubmitted     EventName = "prompt.submitted"
	EventToolBefore          EventName = "tool.before"
	EventToolAfter           EventName = "tool.after"
	EventTurnStopping        EventName = "turn.stopping"
	EventSubagentStarted     EventName = "subagent.started"
	EventSubagentStopped     EventName = "subagent.stopped"
	EventRunTransitionBefore EventName = "run.transition.before"
	EventRunTransitionAfter  EventName = "run.transition.after"
	EventPlanSubmitted       EventName = "plan.submitted"
	EventGateBefore          EventName = "gate.before"
	EventGateAfter           EventName = "gate.after"
	EventSignalsCheck        EventName = "signals.check"
)

var catalogue = map[EventName]Kind{
	EventSessionStarted:      KindContext,
	EventPromptSubmitted:     KindVeto,
	EventToolBefore:          KindVeto,
	EventToolAfter:           KindContext,
	EventTurnStopping:        KindVeto,
	EventSubagentStarted:     KindContext,
	EventSubagentStopped:     KindObserve,
	EventRunTransitionBefore: KindVeto,
	EventRunTransitionAfter:  KindObserve,
	EventPlanSubmitted:       KindCollect,
	EventGateBefore:          KindVeto,
	EventGateAfter:           KindObserve,
	EventSignalsCheck:        KindCollect,
}

func KindOf(name EventName) (Kind, bool) {
	kind, ok := catalogue[name]
	return kind, ok
}

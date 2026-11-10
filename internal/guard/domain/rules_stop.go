package domain

import "context"

const (
	RuleStopHoldsTurn = "stop-holds-turn"

	StopHoldsTurnNextStep = "finish the work or wait for agents"
)

type StopDecider interface {
	DecideStop(ctx context.Context, agentRunning bool) (deny bool, reason string, err error)
}

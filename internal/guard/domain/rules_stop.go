package domain

import "context"

const (
	RuleStopHoldsTurn = "stop-holds-turn"

	StopHoldsTurnNextStep = "finish the phase work, or wait for the background agents and shell tasks, then stop again"
)

type StopDecider interface {
	DecideStop(ctx context.Context, agentRunning bool) (deny bool, reason string, err error)
}

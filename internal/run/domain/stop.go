package domain

import (
	"errors"
	"fmt"
)

type StopVerdict struct {
	Block   bool
	Handoff bool
	Reason  string
}

func DecideStop(run Run, policy Policy, agentRunning bool, handoff error) (StopVerdict, Run) {
	if !run.Phase.HoldsTheTurn() || agentRunning || run.StopBlocks >= policy.MaxStopBlocks {
		return StopVerdict{}, run
	}
	if handoff == nil && run.HandoffStep != run.Step {
		run.HandoffStep = run.Step
		return StopVerdict{Handoff: true}, run
	}
	if run.Phase == PhaseShipping && run.PRURL != "" && handoff == nil {
		return StopVerdict{}, run
	}
	run.StopBlocks++
	run.StopBlocksTotal++
	reason := fmt.Sprintf("phase %s; the hook sees no background agent or shell task. Run long commands in the background", run.Phase)
	switch {
	case errors.Is(handoff, ErrHandoffMissing):
		reason += ". Write handoff.md after a phase change"
	case handoff != nil:
		reason += ". " + handoff.Error()
	}
	return StopVerdict{Block: true, Reason: reason}, run
}

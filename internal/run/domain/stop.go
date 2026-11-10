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
	run.StopBlocks++
	run.StopBlocksTotal++
	reason := fmt.Sprintf("the run is in phase %s and is not finished; the hook sees no background agent or shell task. Run a long command in the background, then stop", run.Phase)
	switch {
	case errors.Is(handoff, ErrHandoffMissing):
		reason += ". After a phase change, write handoff.md, then stop"
	case handoff != nil:
		reason += ". The handoff is refused: " + handoff.Error()
	}
	return StopVerdict{Block: true, Reason: reason}, run
}

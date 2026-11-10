package domain

import "fmt"

type StopVerdict struct {
	Block  bool
	Reason string
}

func DecideStop(run Run, policy Policy, agentRunning bool) (StopVerdict, Run) {
	if !run.Phase.HoldsTheTurn() || agentRunning || run.StopBlocks >= policy.MaxStopBlocks {
		return StopVerdict{}, run
	}
	run.StopBlocks++
	run.StopBlocksTotal++
	return StopVerdict{Block: true, Reason: fmt.Sprintf("the run is in phase %s and is not finished; the hook sees no background agent or shell task. Run a long command in the background, then stop", run.Phase)}, run
}

package domain

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	evidenceReworkFormat = "rework-%d"
	evidenceBlockFormat  = "block-%d"
	evidenceAmends       = "amends"
	evidenceStopBlocks   = "stop-blocks"
	evidenceClassFormat  = "%s: %s"
)

type RunEvidence struct {
	ID   string
	Text string
}

// ListRunEvidence lists the stored signs of what slowed a run. Each rework and each block comes in log order.
// Then come the amend count and the stop block count. A count of 0 gives no item.
func ListRunEvidence(run Run, transitions []Transitioned) []RunEvidence {
	var items []RunEvidence
	var reworks, blocks, amends int
	for _, t := range transitions {
		switch {
		case t.Event.IsRework():
			reworks++
			items = append(items, RunEvidence{ID: fmt.Sprintf(evidenceReworkFormat, reworks), Text: reworkText(t)})
		case t.Event == EventBlock:
			blocks++
			items = append(items, RunEvidence{ID: fmt.Sprintf(evidenceBlockFormat, blocks), Text: oneLine(t.Reason)})
		case t.Event == EventAmend:
			amends++
		}
	}
	if amends > 0 {
		items = append(items, RunEvidence{ID: evidenceAmends, Text: strconv.Itoa(amends)})
	}
	if run.StopBlocksTotal > 0 {
		items = append(items, RunEvidence{ID: evidenceStopBlocks, Text: strconv.Itoa(run.StopBlocksTotal)})
	}
	return items
}

func reworkText(t Transitioned) string {
	reason := oneLine(t.Reason)
	switch {
	case t.Class == "":
		return reason
	case reason == "":
		return string(t.Class)
	}
	return fmt.Sprintf(evidenceClassFormat, t.Class, reason)
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

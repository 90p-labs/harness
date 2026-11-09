package domain

import (
	"slices"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

var AllEvents = []shareddomain.EventName{
	shareddomain.EventSessionStarted,
	shareddomain.EventPromptSubmitted,
	shareddomain.EventToolBefore,
	shareddomain.EventToolAfter,
	shareddomain.EventTurnStopping,
	shareddomain.EventSubagentStarted,
	shareddomain.EventSubagentStopped,
}

func MissingEvents(c Codec) []shareddomain.EventName {
	declared := c.Capabilities()
	missing := []shareddomain.EventName{}
	for _, event := range AllEvents {
		if !slices.Contains(declared, event) {
			missing = append(missing, event)
		}
	}
	return missing
}

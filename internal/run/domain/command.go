package domain

import (
	"time"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type Command struct {
	Event  Event
	Reason string
	By     string
	Track  shareddomain.Track
	PRURL  string
	At     time.Time
}

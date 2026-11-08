package domain

import (
	"time"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type Command struct {
	Event  Event
	Reason string
	By     string
	Track  shareddomain.Track
	PRURL  string
	At     time.Time
}

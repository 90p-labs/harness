package domain

import "time"

type Command struct {
	Event  Event
	Reason string
	At     time.Time
}

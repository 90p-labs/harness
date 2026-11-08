package domain

import "time"

type Transitioned struct {
	RunID  string
	From   Phase
	Event  Event
	To     Phase
	Reason string
	At     time.Time
}

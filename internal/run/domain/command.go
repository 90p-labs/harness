package domain

type Command struct {
	Event  Event
	Reason string
}

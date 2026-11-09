package domain

import "context"

type Plugin interface {
	Name() string
	Subscribes() []EventName
	Handle(ctx context.Context, envelope Envelope) (Verdict, error)
}

type Matcher interface {
	Matches(envelope Envelope) bool
}

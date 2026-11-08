package domain

import "context"

type Dispatcher interface {
	Dispatch(ctx context.Context, envelope Envelope) (DispatchResult, error)
}

type DispatchFailure struct {
	Plugin string
	Err    error
}

type DispatchResult struct {
	Verdict  Verdict
	Failures []DispatchFailure
}

package domain

import "context"

type Tracker interface {
	Create(ctx context.Context, title, body string) (url string, err error)
}

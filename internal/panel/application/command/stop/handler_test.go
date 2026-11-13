package stop

import (
	"context"
	"errors"
	"testing"

	"github.com/90p-labs/harness/internal/panel/domain"
)

type fakeLifecycle struct {
	stops int
	err   error
}

func (f *fakeLifecycle) Start(context.Context, int) error { return nil }
func (f *fakeLifecycle) Stop(context.Context) error {
	f.stops++
	return f.err
}
func (f *fakeLifecycle) Status(context.Context) (domain.Instance, error) {
	return domain.Instance{}, domain.ErrStopped
}

func TestHandle(t *testing.T) {
	f := &fakeLifecycle{}
	if err := NewHandler(f).Handle(context.Background(), Command{}); err != nil || f.stops != 1 {
		t.Fatalf("err = %v, stops = %d, want nil and 1", err, f.stops)
	}
	boom := errors.New("boom")
	if err := NewHandler(&fakeLifecycle{err: boom}).Handle(context.Background(), Command{}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

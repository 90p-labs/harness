package status

import (
	"context"
	"errors"
	"testing"

	"github.com/90p-labs/harness/internal/panel/domain"
)

type fakeLifecycle struct {
	inst domain.Instance
	err  error
}

func (f fakeLifecycle) Start(context.Context, int) error { return nil }
func (f fakeLifecycle) Stop(context.Context) error       { return nil }
func (f fakeLifecycle) Status(context.Context) (domain.Instance, error) {
	return f.inst, f.err
}

func TestHandle(t *testing.T) {
	want := domain.Instance{PID: 7, Port: 4711, URL: "http://127.0.0.1:4711/"}
	got, err := NewHandler(fakeLifecycle{inst: want}).Handle(context.Background(), Query{})
	if err != nil || got != want {
		t.Fatalf("got %+v, %v, want %+v", got, err, want)
	}
	if _, err := NewHandler(fakeLifecycle{err: domain.ErrStopped}).Handle(context.Background(), Query{}); !errors.Is(err, domain.ErrStopped) {
		t.Fatalf("err = %v, want ErrStopped", err)
	}
}

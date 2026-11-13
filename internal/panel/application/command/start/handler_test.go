package start

import (
	"context"
	"errors"
	"testing"

	"github.com/90p-labs/harness/internal/panel/domain"
)

type fakeLifecycle struct {
	ports []int
	err   error
}

func (f *fakeLifecycle) Start(_ context.Context, port int) error {
	f.ports = append(f.ports, port)
	return f.err
}
func (f *fakeLifecycle) Stop(context.Context) error { return nil }
func (f *fakeLifecycle) Status(context.Context) (domain.Instance, error) {
	return domain.Instance{}, domain.ErrStopped
}

func TestHandle(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  int
		want int
	}{
		{"configured port when the command has none", DefaultPort, 7000},
		{"command port wins", 8000, 8000},
		{"explicit 0 picks a free port", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeLifecycle{}
			if err := NewHandler(f, 7000).Handle(context.Background(), Command{Port: tc.cmd}); err != nil {
				t.Fatal(err)
			}
			if len(f.ports) != 1 || f.ports[0] != tc.want {
				t.Fatalf("ports = %v, want [%d]", f.ports, tc.want)
			}
		})
	}

	boom := errors.New("boom")
	if err := NewHandler(&fakeLifecycle{err: boom}, 0).Handle(context.Background(), Command{}); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

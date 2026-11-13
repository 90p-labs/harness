package domain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/90p-labs/harness/internal/panel/domain"
)

type fake struct{}

func (fake) Start(context.Context, int) error { return nil }
func (fake) Stop(context.Context) error       { return nil }
func (fake) Status(context.Context) (domain.Instance, error) {
	return domain.Instance{}, fmt.Errorf("status: %w", domain.ErrStopped)
}

func TestLifecyclePort(t *testing.T) {
	var l domain.Lifecycle = fake{}
	_, err := l.Status(context.Background())
	if !errors.Is(err, domain.ErrStopped) {
		t.Fatalf("Status error = %v, want ErrStopped", err)
	}
}

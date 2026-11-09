package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/host/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestOneHost(t *testing.T) {
	t.Run("one host is the host", func(t *testing.T) {
		host, err := domain.OneHost([]shareddomain.Host{shareddomain.HostCodex})

		if err != nil || host != shareddomain.HostCodex {
			t.Fatalf("host = %q, err = %v, want codex", host, err)
		}
	})

	t.Run("no host refuses", func(t *testing.T) {
		if _, err := domain.OneHost(nil); !errors.Is(err, domain.ErrNoHost) {
			t.Fatalf("err = %v, want ErrNoHost", err)
		}
	})

	t.Run("two hosts refuse with their names", func(t *testing.T) {
		_, err := domain.OneHost([]shareddomain.Host{shareddomain.HostClaude, shareddomain.HostCodex})

		if !errors.Is(err, domain.ErrManyHosts) || !strings.Contains(err.Error(), "claude, codex") {
			t.Fatalf("err = %v, want ErrManyHosts that names claude, codex", err)
		}
	})
}

package execplugin_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/execplugin"
)

func TestFromConfigUnknownEvent(t *testing.T) {
	plugins := []config.Plugin{{Name: "audit", Command: []string{"audit"}, Events: []string{string(domain.EventToolBefore), "nope.event"}}}

	_, err := execplugin.FromConfig(plugins)

	if !errors.Is(err, domain.ErrUnknownEvent) || !strings.Contains(err.Error(), "audit") || !strings.Contains(err.Error(), "nope.event") {
		t.Fatalf("err = %v", err)
	}
}

func TestFromConfigKnownEvents(t *testing.T) {
	plugins := []config.Plugin{{Name: "audit", Command: []string{"audit", "-v"}, Timeout: 3 * time.Second, Events: []string{string(domain.EventToolBefore)}}}

	built, err := execplugin.FromConfig(plugins)

	if err != nil {
		t.Fatal(err)
	}
	if len(built) != 1 || built[0].Name() != "audit" || len(built[0].Subscribes()) != 1 || built[0].Subscribes()[0] != domain.EventToolBefore {
		t.Fatalf("built = %v", built)
	}
}

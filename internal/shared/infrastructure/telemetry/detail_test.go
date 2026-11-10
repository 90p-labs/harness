package telemetry

import (
	"testing"

	"github.com/90p-labs/harness/internal/shared/domain"
)

func TestTraceDetailFromEnv(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  domain.TraceDetail
	}{
		{"", domain.TraceDetailCompact},
		{"compact", domain.TraceDetailCompact},
		{"full", domain.TraceDetailFull},
		{"verbose", domain.TraceDetailCompact},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv(EnvTraceDetail, tc.value)
			if got := TraceDetailFromEnv(); got != tc.want {
				t.Errorf("TraceDetailFromEnv() = %q, want %q", got, tc.want)
			}
		})
	}
}

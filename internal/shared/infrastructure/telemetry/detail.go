package telemetry

import (
	"os"

	"github.com/90p-labs/harness/internal/shared/domain"
)

const EnvTraceDetail = "HARNESS_OTEL_TRACE_DETAIL"

// TraceDetailFromEnv reads EnvTraceDetail; any value but full means compact.
func TraceDetailFromEnv() domain.TraceDetail {
	if domain.TraceDetail(os.Getenv(EnvTraceDetail)) == domain.TraceDetailFull {
		return domain.TraceDetailFull
	}
	return domain.TraceDetailCompact
}

package telemetry

import "testing"

func TestReportTraceIDOfRun(t *testing.T) {
	const runID = "20261110-082002-make-the-run-traces-answer-the-audit-que"
	const want = "3fe697ceb5304820590ed16e64f4e64d"

	if got := TraceIDOf(runID); got != want {
		t.Fatalf("TraceIDOf(%q) = %q, want %q", runID, got, want)
	}
}

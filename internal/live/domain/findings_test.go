package domain_test

import (
	"testing"
	"time"

	"github.com/jorge07/harness/internal/live/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

func TestFindingsText(t *testing.T) {
	release := domain.Release{PRURL: "https://github.com/o/r/pull/12", MergeSHA: "abc1234", Environment: "production"}
	errorRate := domain.SignalBreach{
		Threshold: domain.Threshold{
			Signal: "error rate", Source: "prom", Panel: "https://grafana.example/d/api",
			Breach: shareddomain.Breach{Op: shareddomain.OperatorGreater, Value: 0.01}, Window: 30 * time.Minute,
		},
		Reason: "prom: error rate 0.034 > 0.01",
	}
	latency := domain.SignalBreach{
		Threshold: domain.Threshold{
			Signal: "latency", Source: "prom",
			Breach: shareddomain.Breach{Op: shareddomain.OperatorGreaterEqual, Value: 2}, Window: 90 * time.Minute,
		},
		Reason: "prom: latency 2.5 >= 2",
	}
	const header = "Regression in live round 1 of run r1: PR https://github.com/o/r/pull/12, sha abc1234, production.\n"

	tests := []struct {
		name     string
		findings domain.Findings
		want     string
	}{
		{"breach with rollback",
			domain.Findings{Release: release, RunID: "r1", Round: 1, Breaches: []domain.SignalBreach{errorRate}, Rollback: "helm rollback api"},
			header +
				"- prom: error rate 0.034 > 0.01, window 30m, panel https://grafana.example/d/api\n" +
				"Rollback: helm rollback api\n"},
		{"two breaches, no panel, no rollback",
			domain.Findings{Release: release, RunID: "r1", Round: 1, Breaches: []domain.SignalBreach{errorRate, latency}},
			header +
				"- prom: error rate 0.034 > 0.01, window 30m, panel https://grafana.example/d/api\n" +
				"- prom: latency 2.5 >= 2, window 1h30m\n" +
				"Rollback: not configured, set live.rollback\n"},
		{"deploy failure",
			domain.Findings{Release: release, RunID: "r1", Round: 1, DeployFailure: domain.FailureDeploy, Rollback: "helm rollback api"},
			header + "- deploy failed\nRollback: helm rollback api\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.findings.Text(); got != tc.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

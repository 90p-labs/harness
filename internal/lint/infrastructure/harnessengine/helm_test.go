//go:build linters

package harnessengine

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/fixture"
)

const chartFolder = "deploy/chart/"

func TestObservabilityChartFixtures(t *testing.T) {
	rules := harnessRulesOf(t, domain.PackObservability)
	archives, err := filepath.Glob(filepath.Join("testdata", "chart", "*", archiveGlob))
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, archive := range archives {
		id := filepath.Base(filepath.Dir(archive)) + "/" + strings.TrimSuffix(filepath.Base(archive), ".txtar")
		covered[id] = true
		t.Run(id, func(t *testing.T) {
			findings, err := newTestEngine().Run(context.Background(), fixture.Extract(t, archive), rules)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 {
				t.Fatalf("findings = %+v, want exactly one", findings)
			}
			got := findings[0]
			if got.Rule != id || !strings.HasPrefix(got.File, chartFolder) || got.Line != 1 {
				t.Errorf("finding = %+v, want rule %s in a file under %s at line 1", got, id, chartFolder)
			}
		})
	}
	for id := range chartChecks {
		if !covered[id] {
			t.Errorf("chart rule %s has no fixture archive", id)
		}
	}
}

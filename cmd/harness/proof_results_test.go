package main

import (
	"testing"

	gatedomain "github.com/90p-labs/harness/internal/gate/domain"
)

func TestProofResultsLeaveMissingEvidenceUnrun(t *testing.T) {
	criteria := []criterionGate{
		{number: 1, gate: gatedomain.Gate{ID: "c1"}},
		{number: 2, gate: gatedomain.Gate{ID: "c2"}},
		{number: 3, gate: gatedomain.Gate{ID: "c3"}},
	}
	statuses := map[string]gatedomain.Result{"c1": gatedomain.ResultPass, "c2": gatedomain.ResultFail}

	got := proofResults(criteria, statuses)

	if passed, ran := got[1]; !ran || !passed {
		t.Errorf("criterion 1 = %v, %v, want a pass", passed, ran)
	}
	if passed, ran := got[2]; !ran || passed {
		t.Errorf("criterion 2 = %v, %v, want a fail", passed, ran)
	}
	if _, ran := got[3]; ran {
		t.Error("criterion 3 has no evidence for this tree, so it has not run")
	}
}

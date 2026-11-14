package cli_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	learnDocReference = "skills/harness/references/learning.md"
	learnDocConfig    = "docs/configuration.md"
	learnDocPrefix    = "evidence:"
	learnDocNothing   = "learn: no convention candidates and no evidence"
)

var learnDocNumber = regexp.MustCompile(`-\d+$`)

func TestLearnDocsShowTheOutput(t *testing.T) {
	store := &reportStore{
		run: &domain.Run{ID: learnRunID, Key: reportKey, Phase: domain.PhaseShipping, StopBlocksTotal: 1},
		rows: []domain.Transitioned{
			{Event: domain.EventRequestRework, Class: domain.ReworkImplementation, Reason: "F1 major a.go:3"},
			{Event: domain.EventBlock, Reason: "wait for the token"},
			{Event: domain.EventAmend},
		},
	}
	var forms []string
	for _, line := range runLearnStore(t, store, nil) {
		rest, ok := strings.CutPrefix(line, learnDocPrefix+" ")
		if !ok {
			continue
		}
		id, _, _ := strings.Cut(rest, ":")
		forms = append(forms, "`"+learnDocNumber.ReplaceAllString(id, "-<n>")+"`")
	}
	if want := []string{"`rework-<n>`", "`block-<n>`", "`amends`", "`stop-blocks`"}; strings.Join(forms, " ") != strings.Join(want, " ") {
		t.Fatalf("the command prints the ID forms %q, want %q", forms, want)
	}

	for _, path := range []string{learnDocReference, learnDocConfig} {
		doc := repository.Read(t, path)
		if !strings.Contains(doc, "`"+learnDocPrefix+"`") {
			t.Errorf("%s does not hold the prefix `%s`", path, learnDocPrefix)
		}
		for _, form := range forms {
			if !strings.Contains(doc, form) {
				t.Errorf("%s does not hold the ID form %s", path, form)
			}
		}
	}

	empty := runLearn(t, nil)
	if len(empty) != 3 || empty[1] != learnDocNothing {
		t.Fatalf("the empty run prints %q", empty)
	}
	if !strings.Contains(repository.Read(t, learnDocReference), "`"+empty[1]+"`") {
		t.Errorf("%s does not hold the line %q", learnDocReference, empty[1])
	}
}

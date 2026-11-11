package domain

import (
	"slices"
	"strings"
	"testing"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	testHaiku  = "haiku"
	testOpus   = "opus"
	testSonnet = "sonnet"
	testSol    = "gpt-6.1-sol"
	testTerra  = "gpt-6.1-terra"
	testAstra  = "gpt-6-astra"
)

var (
	claudeList = []string{testHaiku, testOpus, testSonnet}
	codexList  = []string{testAstra, testSol, testTerra}
	bothLists  = map[shareddomain.Host][]string{shareddomain.HostClaude: claudeList, shareddomain.HostCodex: codexList}
	opusOnly   = map[shareddomain.Host][]string{shareddomain.HostClaude: {testOpus}}
)

func TestDenyReviewModels(t *testing.T) {
	claudeName := string(shareddomain.HostClaude)
	codexName := string(shareddomain.HostCodex)
	cases := []struct {
		name   string
		models []string
		listed map[shareddomain.Host][]string
		deny   bool
		want   []string
		absent []string
	}{
		{"claude list misses one", []string{testHaiku, testSonnet}, bothLists, true, []string{testOpus}, nil},
		{"claude list complete", claudeList, bothLists, false, nil, nil},
		{"one record", []string{testOpus}, bothLists, true, []string{testOpus}, nil},
		{"one record names the missing models", []string{testSonnet}, bothLists, true, []string{"misses " + testHaiku + modelSeparator + testOpus + " of " + claudeName, nextStepMissingModels}, nil},
		{"one listed model", []string{testOpus}, opusOnly, true, []string{NextStepReviewerList}, nil},
		{"one listed model, other record", []string{testSonnet}, opusOnly, true, []string{"misses " + testOpus, NextStepReviewerList}, []string{nextStepMissingModels}},
		{"no record names every listed model", nil, bothLists, true, []string{noModel, "misses " + strings.Join(claudeList, modelSeparator) + " of " + claudeName, nextStepMissingModels}, nil},
		{"no lists, two records", []string{testOpus, testSonnet}, nil, false, nil, nil},
		{"no lists, one record", []string{testOpus}, nil, true, []string{testOpus, nextStepReviewModels}, nil},
		{"fuller host list wins", []string{testSol, testTerra}, bothLists, true, []string{testAstra, codexName}, []string{claudeName}},
		{"tie goes to the first host", []string{testSonnet, testAstra}, bothLists, true, []string{claudeName, testHaiku, testOpus}, []string{codexName}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, denied := DenyReviewModels(c.models, nil, c.listed, minReviewModels)
			if denied != c.deny {
				t.Fatalf("denied = %v (%q), want %v", denied, reason, c.deny)
			}
			for _, w := range c.want {
				if !strings.Contains(reason, w) {
					t.Errorf("reason %q does not contain %q", reason, w)
				}
			}
			for _, a := range c.absent {
				if strings.Contains(reason, a) {
					t.Errorf("reason %q contains %q", reason, a)
				}
			}
		})
	}
}

func TestReviewReadNoChangedFile(t *testing.T) {
	reviews := []Review{
		{Model: testSonnet, FilesRead: []string{"a.go"}},
		{Model: testHaiku},
		{Model: testOpus},
		{Model: testOpus, FilesRead: []string{"b.go"}},
		{Model: testHaiku, PatchID: "p1"},
	}

	counted, unread := ReviewedModels(reviews)

	if !slices.Equal(counted, []string{testOpus, testSonnet}) || !slices.Equal(unread, []string{testHaiku}) {
		t.Fatalf("counted = %v, unread = %v, want [opus sonnet] and [haiku]", counted, unread)
	}

	reason, denied := DenyReviewModels([]string{testSonnet}, unread, bothLists, minReviewModels)
	if !denied || !strings.Contains(reason, testHaiku+" read no changed file") {
		t.Errorf("one counted model: denied = %v, reason %q, want a deny that names haiku", denied, reason)
	}
	reason, denied = DenyReviewModels([]string{testOpus, testSonnet}, unread, bothLists, minReviewModels)
	if !denied || !strings.Contains(reason, testHaiku+" read no changed file") {
		t.Errorf("list misses haiku: denied = %v, reason %q, want a deny that names haiku", denied, reason)
	}
	if reason, denied := DenyReviewModels([]string{testOpus, testSonnet}, unread, nil, minReviewModels); denied {
		t.Errorf("two counted models and no list: denied with %q, want allow", reason)
	}
}

func TestDenyReviewModelsFloor(t *testing.T) {
	if ReviewFloor(true) != 1 || ReviewFloor(false) != 2 {
		t.Fatalf("ReviewFloor(true) = %d, ReviewFloor(false) = %d, want 1 and 2", ReviewFloor(true), ReviewFloor(false))
	}

	if reason, denied := DenyReviewModels([]string{testOpus}, nil, opusOnly, 1); denied {
		t.Errorf("floor 1, one listed model, one counted: denied with %q, want allow", reason)
	}

	reason, denied := DenyReviewModels([]string{testOpus}, nil, opusOnly, 2)
	if !denied || !strings.Contains(reason, "needs 2 different models") || !strings.Contains(reason, NextStepReviewerList) {
		t.Errorf("floor 2, one counted: denied = %v, reason %q, want a deny that needs 2 models", denied, reason)
	}

	reason, denied = DenyReviewModels(nil, nil, opusOnly, 1)
	if !denied || !strings.Contains(reason, "needs 1 different model,") {
		t.Errorf("floor 1, no counted: denied = %v, reason %q, want a deny that needs 1 model", denied, reason)
	}
}

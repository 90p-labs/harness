package domain

import (
	"strings"
	"testing"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
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
			reason, denied := DenyReviewModels(c.models, c.listed)
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

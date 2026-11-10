package domain_test

import (
	"testing"

	"github.com/90p-labs/harness/internal/host/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestMarkStepOf(t *testing.T) {
	cases := []struct {
		hook    domain.HookName
		verdict shareddomain.VerdictKind
		want    domain.MarkStep
		ok      bool
	}{
		{domain.HookUserPrompt, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkTurn, Close: true, Open: true}, true},
		{domain.HookUserPrompt, shareddomain.VerdictContext, domain.MarkStep{Kind: domain.MarkTurn, Close: true, Open: true}, true},
		{domain.HookStop, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkTurn, Close: true}, true},
		{domain.HookSubagentStart, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkSubagent, Open: true}, true},
		{domain.HookSubagentStop, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkSubagent, Close: true}, true},
		{domain.HookSessionStart, shareddomain.VerdictAllow, domain.MarkStep{}, false},
		{domain.HookPreToolUse, shareddomain.VerdictAllow, domain.MarkStep{}, false},
		{domain.HookPostToolUse, shareddomain.VerdictAllow, domain.MarkStep{}, false},
		{domain.HookUserPrompt, shareddomain.VerdictDeny, domain.MarkStep{}, false},
		{domain.HookStop, shareddomain.VerdictDeny, domain.MarkStep{}, false},
		{domain.HookSubagentStart, shareddomain.VerdictDeny, domain.MarkStep{}, false},
		{domain.HookSubagentStop, shareddomain.VerdictDeny, domain.MarkStep{}, false},
	}
	for _, c := range cases {
		got, ok := domain.MarkStepOf(c.hook, c.verdict)
		if got != c.want || ok != c.ok {
			t.Errorf("MarkStepOf(%q, %q) = %+v, %v; want %+v, %v", c.hook, c.verdict, got, ok, c.want, c.ok)
		}
	}
}

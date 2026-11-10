package domain_test

import (
	"testing"

	"github.com/90p-labs/harness/internal/host/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestMarkStepOf(t *testing.T) {
	const (
		claude = shareddomain.HostClaude
		codex  = shareddomain.HostCodex
	)
	cases := []struct {
		host    shareddomain.Host
		hook    domain.HookName
		verdict shareddomain.VerdictKind
		want    domain.MarkStep
		ok      bool
	}{
		{claude, domain.HookUserPrompt, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkTurn, Close: true, Open: true}, true},
		{claude, domain.HookUserPrompt, shareddomain.VerdictContext, domain.MarkStep{Kind: domain.MarkTurn, Close: true, Open: true}, true},
		{claude, domain.HookStop, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkTurn, Close: true}, true},
		{claude, domain.HookSubagentStart, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkSubagent, Open: true}, true},
		{claude, domain.HookSubagentStop, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkSubagent, Close: true}, true},
		{claude, domain.HookSessionStart, shareddomain.VerdictAllow, domain.MarkStep{}, false},
		{claude, domain.HookPreToolUse, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkTool, Open: true}, true},
		{claude, domain.HookPreToolUse, shareddomain.VerdictContext, domain.MarkStep{Kind: domain.MarkTool, Open: true}, true},
		{claude, domain.HookPreToolUse, shareddomain.VerdictDeny, domain.MarkStep{}, false},
		{claude, domain.HookPostToolUse, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkTool, Close: true}, true},
		{claude, domain.HookPostToolUseFailure, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkTool, Close: true}, true},
		{claude, domain.HookSessionEnd, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkTool, Close: true, Session: true}, true},
		{claude, domain.HookUserPrompt, shareddomain.VerdictDeny, domain.MarkStep{}, false},
		{claude, domain.HookStop, shareddomain.VerdictDeny, domain.MarkStep{}, false},
		{claude, domain.HookSubagentStart, shareddomain.VerdictDeny, domain.MarkStep{}, false},
		{claude, domain.HookSubagentStop, shareddomain.VerdictDeny, domain.MarkStep{}, false},
		{codex, domain.HookPreToolUse, shareddomain.VerdictAllow, domain.MarkStep{}, false},
		{codex, domain.HookPostToolUse, shareddomain.VerdictAllow, domain.MarkStep{}, false},
		{codex, domain.HookStop, shareddomain.VerdictAllow, domain.MarkStep{Kind: domain.MarkTurn, Close: true}, true},
	}
	for _, c := range cases {
		got, ok := domain.MarkStepOf(c.host, c.hook, c.verdict)
		if got != c.want || ok != c.ok {
			t.Errorf("MarkStepOf(%q, %q, %q) = %+v, %v; want %+v, %v", c.host, c.hook, c.verdict, got, ok, c.want, c.ok)
		}
	}
}

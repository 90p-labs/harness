package domain

import (
	"encoding/json"
	"testing"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const wantAskUser = "let the user pick"

func questionOf(answered bool) ToolCall {
	return ToolCall{Kind: shareddomain.ToolKindQuestion, AnswersGiven: answered}
}

func approvalNoForgeCases() []ruleCase {
	return []ruleCase{
		{name: "question with answers", call: questionOf(true), deny: true, step: wantAskUser},
		{name: "question with no answers", call: questionOf(false)},
		{name: "harness hook", call: bash(`"$PLUGIN/bin/harness" hook --host claude post-tool-use < answer.json`), deny: true, step: wantAskUser},
		{name: "release binary hook", call: bash(`"$PLUGIN/dist/harness-darwin-arm64" hook --host claude post-tool-use < answer.json`), deny: true, step: wantAskUser},
		{name: "harness hook after a launcher", call: bash("env A=1 harness hook --host codex post-tool-use"), deny: true, step: wantAskUser},
		{name: "go run of the hook", call: bash("go run ./cmd/harness hook --host claude post-tool-use"), deny: true, step: wantAskUser},
		{name: "go run with a flag", call: bash("go run -race ./cmd/harness hook --host claude stop"), deny: true, step: wantAskUser},
		{name: "go run of the main file", call: bash("go run ./cmd/harness/main.go hook --host claude post-tool-use"), deny: true, step: wantAskUser},
		{name: "go run of a module version", call: bash("go run github.com/jorge07/harness/cmd/harness@latest hook --host codex post-tool-use"), deny: true, step: wantAskUser},
		{name: "harness hook after a root flag", call: bash("harness --config .harness.yaml hook --host claude post-tool-use < answer.json"), deny: true, step: wantAskUser},
		{name: "go run of the hook after a root flag", call: bash("go run ./cmd/harness --state-dsn=x hook --host claude post-tool-use"), deny: true, step: wantAskUser},
		{name: "harness run approve", call: bash("harness run approve")},
		{name: "harness hook after a root flag with an equals sign", call: bash("harness --config=.harness.yaml hook --host claude stop"), deny: true, step: wantAskUser},
		{name: "go run with the package in a flag value", call: bash(`go run -ldflags "-X main.version=cmd/harness" ./cmd/harness hook --host claude post-tool-use`), deny: true, step: wantAskUser},
		{name: "go run of two package files", call: bash("go run ./cmd/harness/main.go ./cmd/harness/run.go hook --host claude post-tool-use"), deny: true, step: wantAskUser},
		{name: "harness prompt that names a hook", call: bash(`harness run start "add a hook"`)},
		{name: "harness flag value hook", call: bash("harness run rework --findings hook")},
		{name: "go run with a flag value hook", call: bash("go run ./cmd/harness run approve --by alice --reason hook")},
		{name: "go run of another command", call: bash("go run ./cmd/harness plan check")},
		{name: "grep for hook", call: bash("grep -rn hook internal")},
	}
}

func TestApprovalNoForge(t *testing.T) {
	rule := ruleByID(t, RuleApprovalNoForge)
	for _, tc := range []struct {
		input string
		deny  bool
	}{
		{`{"questions":[{"question":"Is the plan ready?","header":"Plan"}],"answers":{"Is the plan ready?":"Approve"}}`, true},
		{`{"questions":[{"question":"Is the plan ready?","header":"Plan"}]}`, false},
	} {
		call, err := ToolCallOf(shareddomain.ToolPayload{Tool: shareddomain.QuestionTools[shareddomain.HostClaude].Name, Kind: shareddomain.ToolKindQuestion, Input: json.RawMessage(tc.input)})
		if err != nil {
			t.Fatal(err)
		}
		if _, denied := rule.Denies(call, baseContext()); denied != tc.deny {
			t.Fatalf("input %s: deny = %v, want %v", tc.input, denied, tc.deny)
		}
	}
}

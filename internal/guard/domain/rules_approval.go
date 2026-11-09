package domain

import (
	"path"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	cmdHarness     = "harness"
	cmdGo          = "go"
	subRun         = "run"
	subHook        = "hook"
	harnessPackage = "cmd/harness"
	releasePrefix  = cmdHarness + "-"
	versionMark    = "@"

	reasonForgedAnswer = "the user gives the answers of a question"
	reasonForgedHook   = "a hook records what the user does, so an agent never runs it"
	nextStepAskUser    = "ask the question with no answers and let the user pick"
)

func denyForgedApproval(call ToolCall, _ RuleContext) (string, bool) {
	if call.Kind == shareddomain.ToolKindQuestion && call.AnswersGiven {
		return deny(reasonForgedAnswer, nextStepAskUser)
	}
	for _, c := range invocations(call.Command) {
		if isHarnessHook(c) || runsHarnessHook(c) {
			return deny(reasonForgedHook, nextStepAskUser)
		}
	}
	return "", false
}

var harnessValuedFlags = []string{"--config", "--state-dsn"}

func isHarnessHook(c Command) bool {
	return len(c.Args) > 1 && (c.Args[0] == cmdHarness || strings.HasPrefix(c.Args[0], releasePrefix)) &&
		isHookSubcommand(c.Args[1:])
}

func runsHarnessHook(c Command) bool {
	if !isCommand(c, cmdGo, subRun) {
		return false
	}
	for i := 2; i < len(c.Args); i++ {
		if isHarnessPackage(c.Args[i]) && isHookSubcommand(c.Args[i+1:]) {
			return true
		}
	}
	return false
}

func isHookSubcommand(args []string) bool {
	rest := skipOptions(args, harnessValuedFlags)
	return len(rest) > 0 && rest[0] == subHook
}

func isHarnessPackage(arg string) bool {
	pkg, _, _ := strings.Cut(path.Clean(arg), versionMark)
	return strings.HasSuffix(pkg, harnessPackage) || strings.HasSuffix(path.Dir(pkg), harnessPackage)
}

package domain

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

const (
	RulePRNoMerge         = "pr-no-merge"
	RulePRDraftOnly       = "pr-draft-only"
	RulePRBodyShape       = "pr-body-shape"
	RuleGitNoForce        = "git-no-force"
	RuleGitNoPushDefault  = "git-no-push-default"
	RuleGitNoVerifySkip   = "git-no-verify-skip"
	RuleSecretsRead       = "secrets-read"
	RuleTestNoFocusSkip   = "test-no-focus-skip"
	RuleShellWriteSource  = "shell-write-source"
	RuleWritingStyle      = "writing-style"
	RuleAgentModelRouting = "agent-model-routing"
)

const (
	cmdGit  = "git"
	cmdGh   = "gh"
	subPush = "push"
	subPR   = "pr"
	subAPI  = "api"

	flagStdin = "-"
	flagMark  = "-"

	cmdEnv     = "env"
	cmdSudo    = "sudo"
	cmdCommand = "command"
	cmdExec    = "exec"
	cmdNohup   = "nohup"
)

var (
	envAssignment    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	gitValuedOptions = []string{"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--config-env", "--super-prefix"}
	launchers        = map[string][]string{
		cmdEnv:     {"-u", "--unset", "-C", "--chdir"},
		cmdSudo:    {"-u", "-g", "-C", "-p", "-h"},
		cmdCommand: nil,
		cmdExec:    nil,
		cmdNohup:   nil,
	}
)

type Rule struct {
	ID     string
	Denies func(call ToolCall, rc RuleContext) (string, bool)
}

type RuleContext struct {
	Cwd           string
	Worktree      string
	DefaultBranch func() (string, error)
	AgentModels   map[string]string
	TextErr       error
}

func Rules() []Rule {
	return []Rule{
		{RulePRNoMerge, denyPRMerge},
		{RulePRDraftOnly, denyPRNotDraft},
		{RulePRBodyShape, denyPRBodyShape},
		{RuleGitNoForce, denyGitForce},
		{RuleGitNoPushDefault, denyPushDefault},
		{RuleGitNoVerifySkip, denyVerifySkip},
		{RuleSecretsRead, denySecretsRead},
		{RuleTestNoFocusSkip, denyFocusSkip},
		{RuleShellWriteSource, denyShellWriteSource},
		{RuleWritingStyle, denyWritingStyle},
		{RuleAgentModelRouting, denyAgentModel},
	}
}

func deny(reason, nextStep string) (string, bool) {
	return reason + ". " + nextStep, true
}

func invocations(line string) []Command {
	commands := Commands(line)
	for i := range commands {
		commands[i].Args = programArgs(commands[i].Args)
	}
	return commands
}

func programArgs(args []string) []string {
	for len(args) > 0 {
		program := filepath.Base(args[0])
		valued, launcher := launchers[program]
		switch {
		case envAssignment.MatchString(args[0]):
			args = args[1:]
		case launcher:
			args = skipOptions(args[1:], valued)
		case program == cmdGit:
			return append([]string{cmdGit}, skipOptions(args[1:], gitValuedOptions)...)
		default:
			return append([]string{program}, args[1:]...)
		}
	}
	return args
}

func skipOptions(args, valued []string) []string {
	for len(args) > 0 && strings.HasPrefix(args[0], flagMark) {
		if slices.Contains(valued, args[0]) && len(args) > 1 {
			args = args[1:]
		}
		args = args[1:]
	}
	return args
}

func isCommand(c Command, words ...string) bool {
	return len(c.Args) >= len(words) && slices.Equal(c.Args[:len(words)], words)
}

func shortCluster(arg string, letter byte) bool {
	if len(arg) < 2 || arg[0] != '-' || arg[1] == '-' {
		return false
	}
	cluster := arg[1:]
	return strings.IndexFunc(cluster, func(r rune) bool { return !unicode.IsLetter(r) }) < 0 &&
		strings.IndexByte(cluster, letter) >= 0
}

func hasFlag(c Command, long string, short byte) bool {
	for _, arg := range c.Args {
		if arg == long || shortCluster(arg, short) {
			return true
		}
	}
	return false
}

func flagValues(c Command, names ...string) []string {
	var values []string
	for i, arg := range c.Args {
		for _, name := range names {
			if arg == name && i+1 < len(c.Args) {
				values = append(values, c.Args[i+1])
			}
			if len(arg) > len(name) && arg[:len(name)] == name && arg[len(name)] == '=' && name[1] == '-' {
				values = append(values, arg[len(name)+1:])
			}
		}
	}
	return values
}

func readsStdin(c Command, names ...string) bool {
	return slices.Contains(flagValues(c, names...), flagStdin)
}

func nonFlags(args []string) []string {
	var out []string
	for _, arg := range args {
		if len(arg) == 0 || arg[0] != '-' {
			out = append(out, arg)
		}
	}
	return out
}

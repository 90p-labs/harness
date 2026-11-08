package domain

import "strings"

const (
	subCommit = "commit"

	flagForce       = "--force"
	flagNoVerify    = "--no-verify"
	letterForce     = 'f'
	letterNoVerify  = 'n'
	forceRefspec    = "+"
	refspecSplit    = ":"
	headsRefPrefix  = "refs/heads/"
	nextStepLease   = "use --force-with-lease, or ask the user"
	nextStepPR      = "push a feature branch and open a pull request"
	nextStepBranch  = "set git.default_branch"
	nextStepHooks   = "let the git hooks run and fix what they report"
	reasonForce     = "a forced push can destroy remote work"
	reasonDefault   = "a push to the default branch skips the pull request"
	reasonNoVerify  = "the flag skips the git hooks"
	reasonBranchErr = "cannot read the default branch: "
)

func pushRefspecs(c Command) []string {
	positional := nonFlags(c.Args[2:])
	if len(positional) < 2 {
		return nil
	}
	return positional[1:]
}

func denyGitForce(call ToolCall, _ RuleContext) (string, bool) {
	for _, c := range invocations(call.Command) {
		if !isCommand(c, cmdGit, subPush) {
			continue
		}
		if hasFlag(c, flagForce, letterForce) {
			return deny(reasonForce, nextStepLease)
		}
		for _, spec := range pushRefspecs(c) {
			if strings.HasPrefix(spec, forceRefspec) {
				return deny(reasonForce, nextStepLease)
			}
		}
	}
	return "", false
}

func denyPushDefault(call ToolCall, rc RuleContext) (string, bool) {
	for _, c := range invocations(call.Command) {
		if !isCommand(c, cmdGit, subPush) {
			continue
		}
		specs := pushRefspecs(c)
		if len(specs) == 0 {
			continue
		}
		branch, err := rc.DefaultBranch()
		if err != nil {
			return deny(reasonBranchErr+err.Error(), nextStepBranch)
		}
		for _, spec := range specs {
			if pushDestination(spec) == branch {
				return deny(reasonDefault, nextStepPR)
			}
		}
	}
	return "", false
}

func pushDestination(spec string) string {
	if _, dest, ok := strings.Cut(spec, refspecSplit); ok {
		spec = dest
	}
	return strings.TrimPrefix(strings.TrimPrefix(spec, forceRefspec), headsRefPrefix)
}

func denyVerifySkip(call ToolCall, _ RuleContext) (string, bool) {
	for _, c := range invocations(call.Command) {
		switch {
		case isCommand(c, cmdGit, subCommit):
			if hasFlag(c, flagNoVerify, letterNoVerify) {
				return deny(reasonNoVerify, nextStepHooks)
			}
		case isCommand(c, cmdGit, subPush):
			if hasFlag(c, flagNoVerify, 0) {
				return deny(reasonNoVerify, nextStepHooks)
			}
		}
	}
	return "", false
}

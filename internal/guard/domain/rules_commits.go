package domain

import (
	"slices"
	"strings"
)

const (
	flagDelete        = "--delete"
	flagDeleteShort   = "-d"
	reasonUnchecked   = "no passed commit check for HEAD"
	nextStepVerify    = "run harness verify --commits <base>. With no gates, run harness gates init first, or add commits-checked to guard.disabled"
	reasonCheckErr    = "cannot read the commit check: "
	nextStepCommitRun = "commit, then run harness verify --commits"
)

func denyUncheckedPublish(call ToolCall, rc RuleContext) (string, bool) {
	for _, c := range invocations(call.Command) {
		if !publishesCommits(c) {
			continue
		}
		checked, err := rc.CommitChecked()
		if err != nil {
			return deny(reasonCheckErr+err.Error(), nextStepCommitRun)
		}
		if !checked {
			return deny(reasonUnchecked, nextStepVerify)
		}
	}
	return "", false
}

func publishesCommits(c Command) bool {
	if isCommand(c, cmdGit, subPush) {
		return !deletesOnly(c)
	}
	return isCommand(c, cmdGh, subPR, subCreate)
}

func deletesOnly(push Command) bool {
	refs := pushRefspecs(push)
	if len(refs) == 0 {
		return false
	}
	// No short cluster: -od is the push option d, not a delete.
	if slices.Contains(push.Args, flagDelete) || slices.Contains(push.Args, flagDeleteShort) {
		return true
	}
	return !slices.ContainsFunc(refs, func(ref string) bool {
		name, isDelete := strings.CutPrefix(strings.TrimPrefix(ref, forceRefspec), refspecSplit)
		return !isDelete || name == ""
	})
}

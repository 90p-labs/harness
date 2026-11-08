package domain

const (
	reasonUnchecked   = "no passed commit check for HEAD"
	nextStepVerify    = "run harness verify --commits <base>. With no gates, run harness gates init first, or add commits-checked to guard.disabled"
	reasonCheckErr    = "cannot read the commit check: "
	nextStepCommitRun = "commit, then run harness verify --commits"
)

func denyUncheckedPublish(call ToolCall, rc RuleContext) (string, bool) {
	for _, c := range invocations(call.Command) {
		if !isCommand(c, cmdGit, subPush) && !isCommand(c, cmdGh, subPR, subCreate) {
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

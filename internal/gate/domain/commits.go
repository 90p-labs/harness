package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	CommitTypeTest = "test"
	CommitTypeFeat = "feat"
	CommitTypeFix  = "fix"
)

type GateFailure struct {
	Gate     string
	Status   Status
	Duration time.Duration
	Tail     string
}

type CommitResult struct {
	SHA    string
	Type   string
	Passed bool
	Failed []GateFailure
}

func CommitType(subject string) string {
	prefix, _, found := strings.Cut(subject, ":")
	if !found {
		return ""
	}
	prefix, _, _ = strings.Cut(prefix, "(")
	return strings.TrimSuffix(prefix, "!")
}

func JudgeCommits(commits []CommitResult) error {
	testsMustFail := hasImplementation(commits)
	for i, c := range commits {
		if c.Type != CommitTypeTest || !testsMustFail {
			if !c.Passed {
				return fmt.Errorf("%w: commit %s (%s) must pass%s", ErrCommitRule, c.SHA, c.Type, failedSuffix(c.Failed))
			}
			continue
		}
		if c.Passed {
			return fmt.Errorf("%w: commit %s (%s) must fail before the code that fixes it", ErrCommitRule, c.SHA, c.Type)
		}
		if !anyPassed(commits[i+1:]) {
			return fmt.Errorf("%w: commit %s (%s) fails and no later commit passes", ErrCommitRule, c.SHA, c.Type)
		}
	}
	return nil
}

func failedSuffix(failed []GateFailure) string {
	if len(failed) == 0 {
		return ""
	}
	ids := make([]string, len(failed))
	for i, f := range failed {
		ids[i] = f.Gate
	}
	return ": failed " + strings.Join(ids, ", ")
}

func hasImplementation(commits []CommitResult) bool {
	for _, c := range commits {
		if c.Type == CommitTypeFeat || c.Type == CommitTypeFix {
			return true
		}
	}
	return false
}

func anyPassed(commits []CommitResult) bool {
	for _, c := range commits {
		if c.Passed {
			return true
		}
	}
	return false
}

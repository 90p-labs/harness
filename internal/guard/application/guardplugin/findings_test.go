package guardplugin_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/guard/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	findingsRunID  = "r1"
	findingsHeader = "| ID | Severity | Verdict | Status |\n|---|---|---|---|\n"
	findingsRead   = "disk unreadable"
)

func findingsTable(rows ...string) string {
	return findingsHeader + strings.Join(rows, "\n") + "\n"
}

func TestReviewFindingsShip(t *testing.T) {
	const worktree = "/work"
	wantPath := filepath.Join(worktree, shareddomain.RunFindingsFile(findingsRunID))
	cases := []struct {
		name   string
		file   string
		err    error
		strict bool
		event  string
		deny   bool
		want   string
	}{
		{"blocker open real", findingsTable("| F1 | blocker | real | open |"), nil, false, shareddomain.TransitionApproveReview, true, "F1"},
		{"major open real", findingsTable("| F2 | major | real | open |"), nil, false, shareddomain.TransitionApproveReview, true, "F2"},
		{"minor open real", findingsTable("| F3 | minor | real | open |"), nil, false, shareddomain.TransitionApproveReview, false, ""},
		{"minor open real strict", findingsTable("| F3 | minor | real | open |"), nil, true, shareddomain.TransitionApproveReview, true, "F3"},
		{"not real", findingsTable("| F4 | blocker | not real | open |"), nil, true, shareddomain.TransitionApproveReview, false, ""},
		{"closed", findingsTable("| F5 | blocker | real | closed |"), nil, true, shareddomain.TransitionApproveReview, false, ""},
		{"missing file", "", fs.ErrNotExist, true, shareddomain.TransitionApproveReview, false, ""},
		{"malformed file", "no table here", nil, false, shareddomain.TransitionApproveReview, true, shareddomain.ErrNoFindingsTable.Error()},
		{"read error", "", errors.New(findingsRead), false, shareddomain.TransitionApproveReview, true, findingsRead},
		{"other transition", findingsTable("| F1 | blocker | real | open |"), nil, true, otherEvent, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := deps(&fakeStop{})
			d.StrictMinor = c.strict
			var readPath string
			d.ReadFile = func(path string) ([]byte, error) {
				readPath = path
				return []byte(c.file), c.err
			}
			d.Worktree = worktree
			d.ReviewModels = (&reviewStore{models: []string{modelOpus, modelSonnet}}).read
			env := transitionEnvelope(t, c.event)
			env.Run = shareddomain.RunRef{ID: findingsRunID}
			v := dispatch(t, newBus(build(t, nil, d)), env)
			if (v.Kind == shareddomain.VerdictDeny) != c.deny {
				t.Fatalf("verdict = %+v, want deny=%v", v, c.deny)
			}
			if c.deny && !strings.Contains(v.Reason, c.want) {
				t.Errorf("reason %q does not contain %q", v.Reason, c.want)
			}
			if c.event == shareddomain.TransitionApproveReview && readPath != wantPath {
				t.Errorf("read %q, want %q", readPath, wantPath)
			}
		})
	}

	t.Run("disabled", func(t *testing.T) {
		d := deps(&fakeStop{})
		d.ReadFile = func(string) ([]byte, error) { return []byte(findingsTable("| F1 | blocker | real | open |")), nil }
		d.ReviewModels = (&reviewStore{models: []string{modelOpus, modelSonnet}}).read
		env := transitionEnvelope(t, shareddomain.TransitionApproveReview)
		env.Run = shareddomain.RunRef{ID: findingsRunID}
		v := dispatch(t, newBus(build(t, []string{domain.RuleReviewFindings}, d)), env)
		if v.Kind != shareddomain.VerdictAllow {
			t.Errorf("verdict = %+v, want allow with the rule disabled", v)
		}
	})
}

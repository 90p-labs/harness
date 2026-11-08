package fileissue_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/report/application/command/fileissue"
	"github.com/jorge07/harness/internal/report/domain"
)

const (
	issueURL = "https://github.com/jorge07/harness/issues/12"
	body     = `## Summary

The verify gate fails for jorge@example.com.

## Reproduction

### Commands

harness verify

### Config

gates: []

### Expected

exit 0

### Actual

exit 1

## Traces

### Host

Codex 0.50.0

### Output

error: no gate is set

## Fix suggestion

internal/gate/ui/cli/verify.go, verifyGates: return nil when no gate is set.

## Workaround

Add one gate.
`
)

type fakeTracker struct {
	calls       int
	title, body string
	err         error
}

func (f *fakeTracker) Create(_ context.Context, title, body string) (string, error) {
	f.calls++
	f.title, f.body = title, body
	return issueURL, f.err
}

var env = domain.Environment{Version: "v0.4.0", Platform: "linux/amd64", Run: domain.RunLine("20261108-fix-verify", "working")}

func TestFileIssue(t *testing.T) {
	t.Run("files the stamped and redacted issue", func(t *testing.T) {
		tracker := &fakeTracker{}
		result, err := fileissue.NewHandler(tracker).Handle(context.Background(), fileissue.Command{
			Draft: domain.Draft{Title: "verify fails for jorge@example.com", Body: body}, Environment: env,
		})
		if err != nil {
			t.Fatal(err)
		}
		if tracker.calls != 1 || result.URL != issueURL {
			t.Fatalf("calls = %d, url = %q, want 1 and %q", tracker.calls, result.URL, issueURL)
		}
		if tracker.title != "verify fails for <redacted>" || tracker.body != result.Issue.Body {
			t.Fatalf("tracker got %q, want the result issue", tracker.title)
		}
		if strings.Contains(tracker.body, "jorge@example.com") || !strings.Contains(tracker.body, "- run: 20261108-fix-verify, phase working") {
			t.Fatalf("body is not redacted and stamped:\n%s", tracker.body)
		}
	})

	t.Run("a dry run returns the issue and files nothing", func(t *testing.T) {
		tracker := &fakeTracker{}
		result, err := fileissue.NewHandler(tracker).Handle(context.Background(), fileissue.Command{
			Draft: domain.Draft{Title: "verify fails", Body: body}, Environment: env, DryRun: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if tracker.calls != 0 || result.URL != "" || !strings.Contains(result.Issue.Body, "- harness: v0.4.0") {
			t.Fatalf("calls = %d, result = %+v, want no call and the stamped issue", tracker.calls, result)
		}
	})

	t.Run("an incomplete draft files nothing in either mode", func(t *testing.T) {
		for _, dryRun := range []bool{false, true} {
			tracker := &fakeTracker{}
			_, err := fileissue.NewHandler(tracker).Handle(context.Background(), fileissue.Command{
				Draft: domain.Draft{Title: "verify fails", Body: "## Summary\n\nshort\n"}, Environment: env, DryRun: dryRun,
			})
			if !errors.Is(err, domain.ErrDraftIncomplete) || tracker.calls != 0 {
				t.Fatalf("dry run %v: err = %v, calls = %d, want ErrDraftIncomplete and no call", dryRun, err, tracker.calls)
			}
		}
	})

	t.Run("a tracker error is returned", func(t *testing.T) {
		boom := errors.New("gh issue create: exit status 1: HTTP 401")
		_, err := fileissue.NewHandler(&fakeTracker{err: boom}).Handle(context.Background(), fileissue.Command{
			Draft: domain.Draft{Title: "verify fails", Body: body}, Environment: env,
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
	})
}

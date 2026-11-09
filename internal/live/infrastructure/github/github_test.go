package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/live/domain"
)

const (
	helperEnv   = "HARNESS_FAKE_GH"
	scenarioEnv = "HARNESS_FAKE_GH_SCENARIO"
	logEnv      = "HARNESS_FAKE_GH_LOG"

	prURL         = "https://github.com/acme/shop/pull/42"
	sha           = "abc1234"
	environment   = "production"
	viewCall      = "pr view " + prURL + " --json state,mergedAt,mergeCommit"
	listCall      = "api repos/acme/shop/deployments?sha=abc1234&environment=production"
	statusesCall7 = "api repos/acme/shop/deployments/7/statuses"
	statusesCall9 = "api repos/acme/shop/deployments/9/statuses"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperEnv) != "1" {
		return
	}
	args := strings.Join(os.Args[indexAfterSeparator(os.Args):], " ")
	if f, err := os.OpenFile(os.Getenv(logEnv), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		fmt.Fprintln(f, args)
		f.Close()
	}
	var scenario map[string]string
	if err := json.Unmarshal([]byte(os.Getenv(scenarioEnv)), &scenario); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	out, ok := scenario[args]
	if !ok {
		fmt.Fprintln(os.Stderr, "  boom: no route  ")
		os.Exit(1)
	}
	fmt.Print(out)
	os.Exit(0)
}

func indexAfterSeparator(args []string) int {
	for i, a := range args {
		if a == "--" {
			return i + 1
		}
	}
	return len(args)
}

func fakeGH(t *testing.T, scenario map[string]string) (*Client, string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "gh")
	script := fmt.Sprintf("#!/bin/sh\nexec %q -test.run=TestHelperProcess -- \"$@\"\n", self)
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "calls.log")
	t.Setenv(helperEnv, "1")
	t.Setenv(scenarioEnv, string(raw))
	t.Setenv(logEnv, logPath)
	return New(bin), logPath
}

func TestGitHubMergeAndDeploy(t *testing.T) {
	merged := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	okAt := time.Date(2026, 11, 8, 11, 30, 0, 0, time.UTC)

	deployments := `[
	{"id":7,"created_at":"2026-11-08T10:05:00Z"},
	{"id":9,"created_at":"2026-11-08T10:40:00Z"},
	{"id":3,"created_at":"2026-11-08T09:00:00Z"}]`
	statusesNewestSuccess := `[
	{"state":"failure","created_at":"2026-11-08T11:00:00Z"},
	{"state":"success","created_at":"2026-11-08T11:30:00Z"},
	{"state":"in_progress","created_at":"2026-11-08T10:50:00Z"}]`
	statusesNewestFailure := `[
	{"state":"success","created_at":"2026-11-08T11:00:00Z"},
	{"state":"error","created_at":"2026-11-08T11:30:00Z"}]`

	t.Run("merged pull request", func(t *testing.T) {
		gh, _ := fakeGH(t, map[string]string{viewCall: `{"state":"MERGED","mergedAt":"2026-11-08T10:00:00Z","mergeCommit":{"oid":"abc1234"}}`})
		got, err := gh.Merge(context.Background(), prURL)
		if err != nil {
			t.Fatal(err)
		}
		want := domain.Merge{State: domain.MergeMerged, SHA: sha, At: merged}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("merged pull request with no merge commit yet is open", func(t *testing.T) {
		gh, _ := fakeGH(t, map[string]string{viewCall: `{"state":"MERGED","mergedAt":"2026-11-08T10:00:00Z","mergeCommit":null}`})
		got, err := gh.Merge(context.Background(), prURL)
		if err != nil {
			t.Fatal(err)
		}
		if got != (domain.Merge{State: domain.MergeOpen}) {
			t.Fatalf("got %+v, want an open merge", got)
		}
	})

	t.Run("a newer inactive status keeps the success", func(t *testing.T) {
		gh, _ := fakeGH(t, map[string]string{
			listCall: `[{"id":7,"created_at":"2026-11-08T10:05:00Z"}]`,
			statusesCall7: `[
	{"state":"inactive","created_at":"2026-11-08T12:00:00Z"},
	{"state":"success","created_at":"2026-11-08T11:30:00Z"}]`,
		})
		got, err := gh.Deployment(context.Background(), prURL, sha, environment)
		if err != nil {
			t.Fatal(err)
		}
		if want := (domain.Deployment{State: domain.DeploySuccess, At: okAt}); got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("open and closed pull requests", func(t *testing.T) {
		for state, want := range map[string]domain.MergeState{"OPEN": domain.MergeOpen, "CLOSED": domain.MergeClosed} {
			gh, _ := fakeGH(t, map[string]string{viewCall: `{"state":"` + state + `","mergedAt":null,"mergeCommit":null}`})
			got, err := gh.Merge(context.Background(), prURL)
			if err != nil {
				t.Fatal(err)
			}
			if got != (domain.Merge{State: want}) {
				t.Fatalf("%s: got %+v", state, got)
			}
		}
	})

	t.Run("newest deployment and newest status win", func(t *testing.T) {
		gh, log := fakeGH(t, map[string]string{
			listCall:      deployments,
			statusesCall9: statusesNewestSuccess,
		})
		got, err := gh.Deployment(context.Background(), prURL, sha, environment)
		if err != nil {
			t.Fatal(err)
		}
		if want := (domain.Deployment{State: domain.DeploySuccess, At: okAt}); got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		calls, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		if want := listCall + "\n" + statusesCall9 + "\n"; string(calls) != want {
			t.Fatalf("calls %q, want %q", calls, want)
		}
	})

	t.Run("newest status error is failed", func(t *testing.T) {
		gh, _ := fakeGH(t, map[string]string{listCall: deployments, statusesCall9: statusesNewestFailure})
		got, err := gh.Deployment(context.Background(), prURL, sha, environment)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != domain.DeployFailed {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("failure status is failed", func(t *testing.T) {
		gh, _ := fakeGH(t, map[string]string{
			listCall:      `[{"id":7,"created_at":"2026-11-08T10:05:00Z"}]`,
			statusesCall7: `[{"state":"failure","created_at":"2026-11-08T11:00:00Z"}]`,
		})
		got, err := gh.Deployment(context.Background(), prURL, sha, environment)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != domain.DeployFailed {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("in progress and missing status are pending", func(t *testing.T) {
		for _, statuses := range []string{`[{"state":"in_progress","created_at":"2026-11-08T11:00:00Z"}]`, `[]`} {
			gh, _ := fakeGH(t, map[string]string{
				listCall:      `[{"id":7,"created_at":"2026-11-08T10:05:00Z"}]`,
				statusesCall7: statuses,
			})
			got, err := gh.Deployment(context.Background(), prURL, sha, environment)
			if err != nil {
				t.Fatal(err)
			}
			if got != (domain.Deployment{State: domain.DeployPending}) {
				t.Fatalf("%s: got %+v", statuses, got)
			}
		}
	})

	t.Run("no deployment is pending", func(t *testing.T) {
		gh, _ := fakeGH(t, map[string]string{listCall: `[]`})
		got, err := gh.Deployment(context.Background(), prURL, sha, environment)
		if err != nil {
			t.Fatal(err)
		}
		if got != (domain.Deployment{State: domain.DeployPending}) {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("gh failure names the command and holds stderr", func(t *testing.T) {
		gh, _ := fakeGH(t, map[string]string{})
		_, err := gh.Merge(context.Background(), prURL)
		if err == nil {
			t.Fatal("expected an error")
		}
		for _, part := range []string{"pr view " + prURL, "boom: no route"} {
			if !strings.Contains(err.Error(), part) {
				t.Fatalf("error %q misses %q", err, part)
			}
		}
		if strings.Contains(err.Error(), "  boom") {
			t.Fatalf("stderr not trimmed: %q", err)
		}
	})

	t.Run("invalid pull request url is an error", func(t *testing.T) {
		gh, _ := fakeGH(t, map[string]string{})
		if _, err := gh.Deployment(context.Background(), "nope", sha, environment); err == nil {
			t.Fatal("expected an error")
		}
	})
}

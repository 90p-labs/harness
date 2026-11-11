package scripts_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	releaseScript  = "scripts/release.sh"
	launcherPath   = "bin/harness"
	claudeManifest = ".claude-plugin/plugin.json"
	codexManifest  = ".codex-plugin/plugin.json"
	manifestStart  = "0.1.0"
	execMode       = 0o755
	fileMode       = 0o644
	exitUsage      = 2
	loggingCurl    = "#!/bin/sh\necho \"$*\" >>%q\nexit 1\n"
)

var manifests = []string{claudeManifest, codexManifest}

type fixture struct {
	dir string
}

func (f fixture) env() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
}

func (f fixture) git(t *testing.T, args ...string) string {
	t.Helper()
	full := append([]string{"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = f.dir
	cmd.Env = f.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{dir: t.TempDir()}
	f.git(t, "init", "-q", "-b", "main")
	for _, file := range manifests {
		target := filepath.Join(f.dir, file)
		if err := os.MkdirAll(filepath.Dir(target), execMode); err != nil {
			t.Fatal(err)
		}
		raw := fmt.Sprintf("{\n  \"name\": \"90p\",\n  \"version\": %q,\n  \"description\": \"x\"\n}\n", manifestStart)
		if err := os.WriteFile(target, []byte(raw), fileMode); err != nil {
			t.Fatal(err)
		}
	}
	f.git(t, "add", ".")
	return f
}

func (f fixture) commit(t *testing.T, message string) {
	t.Helper()
	f.git(t, "commit", "-q", "--allow-empty", "-m", message)
}

func (f fixture) merge(t *testing.T, branchCommit, mergeMessage string) {
	t.Helper()
	f.git(t, "checkout", "-q", "-b", "side")
	f.commit(t, branchCommit)
	f.git(t, "checkout", "-q", "main")
	f.commit(t, "chore: main moves")
	f.git(t, "merge", "-q", "--no-ff", "-m", mergeMessage, "side")
	f.git(t, "branch", "-q", "-D", "side")
}

func (f fixture) run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{repository.Path(t, releaseScript)}, args...)...)
	cmd.Dir = f.dir
	cmd.Env = f.env()
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), out.String(), errOut.String()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, out.String(), errOut.String()
}

func (f fixture) next(t *testing.T) string {
	t.Helper()
	code, stdout, stderr := f.run(t, "next")
	if code != 0 {
		t.Fatalf("next exited %d: %s", code, stderr)
	}
	return strings.TrimSpace(stdout)
}

func (f fixture) read(t *testing.T, file string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.dir, file))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestNextVersion(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		commits []string
		want    string
	}{
		{"fix gives a patch", "v1.2.3", []string{"fix: handle nil"}, "v1.2.4"},
		{"feat gives a minor", "v1.2.3", []string{"feat: add a gate"}, "v1.3.0"},
		{"scoped feat gives a minor", "v1.2.3", []string{"feat(plan): add a gate"}, "v1.3.0"},
		{"scoped fix gives a patch", "v1.2.3", []string{"fix(guard): stop a leak"}, "v1.2.4"},
		{"feat beats fix", "v1.2.3", []string{"fix: a", "feat: b", "fix: c"}, "v1.3.0"},
		{"bang gives a major", "v1.2.3", []string{"fix(guard)!: drop a flag"}, "v2.0.0"},
		{"bang without scope gives a major", "v1.2.3", []string{"feat!: drop a flag"}, "v2.0.0"},
		{"footer gives a major", "v1.2.3", []string{"fix: a\n\nBREAKING CHANGE: the flag is gone"}, "v2.0.0"},
		{"hyphen footer gives a major", "v1.2.3", []string{"feat: a\n\nBREAKING-CHANGE: the flag is gone"}, "v2.0.0"},
		{"footer after other lines gives a major", "v1.2.3", []string{"fix: a\n\nSome text.\n\nBREAKING CHANGE: the flag is gone"}, "v2.0.0"},
		{"phrase in the middle of a line is not breaking", "v1.2.3", []string{"feat: a\n\nThis is not a BREAKING CHANGE: just a note"}, "v1.3.0"},
		{"phrase in an indented line is not breaking", "v1.2.3", []string{"fix: a\n\n  BREAKING CHANGE: quoted"}, "v1.2.4"},
		{"breaking on 0.x gives a minor", "v0.4.1", []string{"fix!: drop a flag"}, "v0.5.0"},
		{"breaking footer on 0.x gives a minor", "v0.4.1", []string{"feat: a\n\nBREAKING CHANGE: x"}, "v0.5.0"},
		{"feat on 0.x gives a minor", "v0.4.1", []string{"feat: a"}, "v0.5.0"},
		{"fix on 0.x gives a patch", "v0.4.1", []string{"fix: a"}, "v0.4.2"},
		{"breaking beats feat", "v1.2.3", []string{"feat: a", "chore!: b", "fix: c"}, "v2.0.0"},
		{"a malformed subject gives no release", "v1.2.3", []string{"feat add a gate", "Fix: a", "feat(: a"}, ""},
		{"a footer on a malformed subject gives no release", "v1.2.3", []string{"update stuff\n\nBREAKING CHANGE: x"}, ""},
		{"an unknown type gives no release", "v1.2.3", []string{"wip: a", "feature: b"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.commit(t, "chore: start")
			f.git(t, "tag", tc.base)
			for _, message := range tc.commits {
				f.commit(t, message)
			}

			if got := f.next(t); got != tc.want {
				t.Fatalf("next = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("a malformed tag is not the base", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "chore: start")
		f.git(t, "tag", "v1.0.0")
		f.commit(t, "fix: a")
		for _, tag := range []string{"v1.2", "latest", "v9.9.9-rc1", "v1.2.3.4", "1.5.0", "v10"} {
			f.git(t, "tag", tag)
		}
		f.commit(t, "fix: b")

		if got := f.next(t); got != "v1.0.1" {
			t.Fatalf("next = %q, want v1.0.1", got)
		}
	})

	t.Run("the highest tag wins by number, not by text", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "chore: start")
		f.git(t, "tag", "v1.9.0")
		f.commit(t, "chore: a")
		f.git(t, "tag", "v1.10.0")
		f.commit(t, "fix: b")

		if got := f.next(t); got != "v1.10.1" {
			t.Fatalf("next = %q, want v1.10.1", got)
		}
	})

	t.Run("a commit before the base tag does not count", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "feat!: old")
		f.git(t, "tag", "v1.0.0")
		f.commit(t, "fix: new")

		if got := f.next(t); got != "v1.0.1" {
			t.Fatalf("next = %q, want v1.0.1", got)
		}
	})

	t.Run("a tag on another branch is not the base", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "chore: start")
		f.git(t, "tag", "v1.0.0")
		f.git(t, "checkout", "-q", "-b", "other")
		f.commit(t, "chore: other")
		f.git(t, "tag", "v5.0.0")
		f.git(t, "checkout", "-q", "main")
		f.commit(t, "fix: a")

		if got := f.next(t); got != "v1.0.1" {
			t.Fatalf("next = %q, want v1.0.1", got)
		}
	})

	t.Run("a merge commit message does not count", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "chore: start")
		f.git(t, "tag", "v1.2.3")
		f.merge(t, "chore: side", "feat!: merged")

		if got := f.next(t); got != "" {
			t.Fatalf("next = %q, want nothing", got)
		}
	})

	t.Run("a commit on a merged branch counts", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "chore: start")
		f.git(t, "tag", "v1.2.3")
		f.merge(t, "feat: side", "Merge branch side")

		if got := f.next(t); got != "v1.3.0" {
			t.Fatalf("next = %q, want v1.3.0", got)
		}
	})
}

func TestNoRelease(t *testing.T) {
	f := newFixture(t)
	f.commit(t, "chore: start")
	f.git(t, "tag", "v1.2.3")
	for _, message := range []string{
		"chore: a", "docs: b", "test: c", "refactor: d", "ci: e", "chore(release): v1.2.3",
		"docs(adr): f", "style: g", "build: h", "perf: i",
	} {
		f.commit(t, message)
	}
	f.merge(t, "docs: side", "Merge pull request #1 from x/y")

	code, stdout, stderr := f.run(t, "next")

	if code != 0 || stdout != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0 and no output", code, stdout, stderr)
	}
}

func TestFirstRelease(t *testing.T) {
	t.Run("no tag uses the manifest version as the base", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "chore: start")
		f.commit(t, "feat: first")
		f.commit(t, "fix: second")

		if got := f.next(t); got != "v0.2.0" {
			t.Fatalf("next = %q, want v0.2.0", got)
		}
	})

	t.Run("no tag and only a fix gives a patch of the manifest version", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "fix: first")

		if got := f.next(t); got != "v0.1.1" {
			t.Fatalf("next = %q, want v0.1.1", got)
		}
	})

	t.Run("no tag and no release commit prints nothing", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "docs: first")

		if got := f.next(t); got != "" {
			t.Fatalf("next = %q, want nothing", got)
		}
	})

	t.Run("after a tag the base is the tag", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "feat: first")
		f.git(t, "tag", "v0.2.0")
		bump(t, f, "v0.2.0")
		f.git(t, "add", ".")
		f.commit(t, "chore(release): v0.2.0")
		f.commit(t, "fix: later")

		if got := f.next(t); got != "v0.2.1" {
			t.Fatalf("next = %q, want v0.2.1", got)
		}
	})

	t.Run("a manifest that disagrees with the tag does not change the base", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "feat: first")
		f.git(t, "tag", "v0.2.0")
		// The manifest still says 0.1.0 and a later feat must bump the tag, not the manifest.
		f.commit(t, "feat: later")

		if got := f.next(t); got != "v0.3.0" {
			t.Fatalf("next = %q, want v0.3.0", got)
		}
	})

	t.Run("a manifest ahead of the tag does not change the base", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "feat: first")
		f.git(t, "tag", "v0.2.0")
		bump(t, f, "v7.0.0")
		f.git(t, "add", ".")
		f.commit(t, "fix: later")

		if got := f.next(t); got != "v0.2.1" {
			t.Fatalf("next = %q, want v0.2.1", got)
		}
	})
}

func bump(t *testing.T, f fixture, tag string) {
	t.Helper()
	if code, _, stderr := f.run(t, "bump", tag); code != 0 {
		t.Fatalf("bump %s exited %d: %s", tag, code, stderr)
	}
}

func TestReleaseNotes(t *testing.T) {
	t.Run("groups the commits in order and leaves out the release commit", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "chore: start")
		f.git(t, "tag", "v1.0.0")
		f.commit(t, "docs: explain the loop")
		f.commit(t, "feat(plan): add a gate")
		f.commit(t, "fix: handle nil")
		f.commit(t, "fix(guard)!: drop a flag")
		f.commit(t, "chore(release): v1.1.0")
		f.commit(t, "feat: add a view\n\nBREAKING CHANGE: the view moved")
		f.commit(t, "refactor: tidy")
		f.commit(t, "feat: add a report")
		f.merge(t, "test: side", "Merge pull request #2 from x/y")
		f.git(t, "tag", "v1.1.0")
		f.commit(t, "feat: after the tag")

		code, stdout, stderr := f.run(t, "notes", "v1.1.0")

		if code != 0 {
			t.Fatalf("code = %d, stderr = %q", code, stderr)
		}
		headings := []string{"## Breaking changes", "## Features", "## Fixes", "## Other changes"}
		last := -1
		for _, heading := range headings {
			at := strings.Index(stdout, heading)
			if at < 0 || at < last {
				t.Fatalf("heading %q missing or out of order in:\n%s", heading, stdout)
			}
			last = at
		}
		section := func(heading string) string {
			_, rest, _ := strings.Cut(stdout, heading)
			body, _, _ := strings.Cut(rest, "\n## ")
			return body
		}
		want := map[string][]string{
			"## Breaking changes": {"fix(guard)!: drop a flag", "feat: add a view"},
			"## Features":         {"feat(plan): add a gate", "feat: add a report"},
			"## Fixes":            {"fix: handle nil"},
			"## Other changes":    {"docs: explain the loop", "refactor: tidy", "chore: main moves", "test: side"},
		}
		for heading, subjects := range want {
			body := section(heading)
			for _, subject := range subjects {
				if !bulletWithSha(body, subject) {
					t.Errorf("%s lacks a bullet %q with a short sha:\n%s", heading, subject, body)
				}
			}
			if n := countBullets(body); n != len(subjects) {
				t.Errorf("%s has %d bullets, want %d:\n%s", heading, n, len(subjects), body)
			}
		}
		for _, absent := range []string{"chore(release)", "after the tag", "chore: start", "Merge pull request"} {
			if strings.Contains(stdout, absent) {
				t.Errorf("notes contain %q:\n%s", absent, stdout)
			}
		}
		if n := strings.Count(stdout, "feat: add a view"); n != 1 {
			t.Errorf("breaking commit appears %d times, want 1", n)
		}
	})

	t.Run("the base is the previous tag, not an older one", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "feat: one")
		f.git(t, "tag", "v1.0.0")
		f.commit(t, "fix: two")
		f.git(t, "tag", "v1.0.1")
		f.commit(t, "fix: three")
		f.git(t, "tag", "v1.0.2")

		_, stdout, _ := f.run(t, "notes", "v1.0.2")

		if !strings.Contains(stdout, "fix: three") || strings.Contains(stdout, "fix: two") || strings.Contains(stdout, "feat: one") {
			t.Fatalf("notes = %q, want only fix: three", stdout)
		}
	})

	t.Run("a tag that does not exist yet reads up to HEAD", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "feat: one")
		f.git(t, "tag", "v1.0.0")
		f.commit(t, "fix: two")

		code, stdout, stderr := f.run(t, "notes", "v1.0.1")

		if code != 0 || !strings.Contains(stdout, "## Fixes") || !strings.Contains(stdout, "fix: two") || strings.Contains(stdout, "feat: one") {
			t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
		}
	})

	t.Run("with no previous tag the notes cover the whole history", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "feat: one")
		f.commit(t, "fix: two")
		f.git(t, "tag", "v0.2.0")

		_, stdout, _ := f.run(t, "notes", "v0.2.0")

		if !strings.Contains(stdout, "feat: one") || !strings.Contains(stdout, "fix: two") {
			t.Fatalf("notes = %q, want both commits", stdout)
		}
	})

	t.Run("an empty section has no heading", func(t *testing.T) {
		f := newFixture(t)
		f.commit(t, "chore: start")
		f.git(t, "tag", "v1.0.0")
		f.commit(t, "fix: two")

		_, stdout, _ := f.run(t, "notes", "v1.0.1")

		for _, heading := range []string{"## Breaking changes", "## Features", "## Other changes"} {
			if strings.Contains(stdout, heading) {
				t.Errorf("notes contain %q:\n%s", heading, stdout)
			}
		}
	})

	for _, tag := range []string{"v1.2", "latest", "1.2.3", "v1.2.3-rc1", "v1.2.3\nx", ""} {
		t.Run(fmt.Sprintf("rejects the tag %q", tag), func(t *testing.T) {
			f := newFixture(t)
			f.commit(t, "feat: one")

			code, stdout, _ := f.run(t, "notes", tag)

			if code == 0 || stdout != "" {
				t.Fatalf("code = %d, stdout = %q, want a failure and no output", code, stdout)
			}
		})
	}
}

func countBullets(body string) int {
	n := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "- ") {
			n++
		}
	}
	return n
}

// bulletWithSha reports whether body has the line "- subject (sha)" with a hex short sha.
func bulletWithSha(body, subject string) bool {
	for _, line := range strings.Split(body, "\n") {
		rest, ok := strings.CutPrefix(line, "- "+subject+" (")
		if !ok {
			continue
		}
		sha, ok := strings.CutSuffix(rest, ")")
		if !ok || len(sha) < 7 {
			continue
		}
		if strings.Trim(sha, "0123456789abcdef") == "" {
			return true
		}
	}
	return false
}

func TestBumpManifests(t *testing.T) {
	t.Run("bump writes both manifests and check passes", func(t *testing.T) {
		f := newFixture(t)
		before := f.read(t, claudeManifest)

		code, _, stderr := f.run(t, "bump", "v1.4.2")
		if code != 0 {
			t.Fatalf("bump exited %d: %s", code, stderr)
		}

		for _, file := range manifests {
			got := f.read(t, file)
			if !strings.Contains(got, "\n  \"version\": \"1.4.2\",\n") {
				t.Errorf("%s = %q, want a one-line version 1.4.2", file, got)
			}
			if strings.Contains(got, manifestStart) {
				t.Errorf("%s still has %s", file, manifestStart)
			}
		}
		if got := f.read(t, claudeManifest); got != strings.Replace(before, manifestStart, "1.4.2", 1) {
			t.Errorf("bump changed more than the version:\n%s", got)
		}
		if code, _, stderr := f.run(t, "check", "v1.4.2"); code != 0 {
			t.Errorf("check exited %d: %s", code, stderr)
		}
		info, err := os.Stat(filepath.Join(f.dir, claudeManifest))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != fileMode {
			t.Errorf("manifest mode = %v, want %o", info.Mode().Perm(), fileMode)
		}
	})

	t.Run("bump fails on a manifest with no version line and names it", func(t *testing.T) {
		for _, file := range manifests {
			f := newFixture(t)
			path := filepath.Join(f.dir, file)
			if err := os.WriteFile(path, []byte("{\n  \"name\": \"harness\"\n}\n"), fileMode); err != nil {
				t.Fatal(err)
			}

			code, _, stderr := f.run(t, "bump", "v1.4.2")

			if code == 0 || !strings.Contains(stderr, file) {
				t.Errorf("bump on %s with no version: code = %d, stderr = %q, want a failure that names it", file, code, stderr)
			}
		}
	})

	t.Run("check fails on a mismatch and names the file", func(t *testing.T) {
		for _, file := range manifests {
			f := newFixture(t)
			bump(t, f, "v1.4.2")
			path := filepath.Join(f.dir, file)
			raw := strings.Replace(f.read(t, file), "1.4.2", "1.4.3", 1)
			if err := os.WriteFile(path, []byte(raw), fileMode); err != nil {
				t.Fatal(err)
			}

			code, _, stderr := f.run(t, "check", "v1.4.2")

			if code == 0 || !strings.Contains(stderr, file) {
				t.Errorf("check on a bad %s: code = %d, stderr = %q, want a failure that names it", file, code, stderr)
			}
		}
	})

	t.Run("check fails on the starting version", func(t *testing.T) {
		f := newFixture(t)

		if code, _, stderr := f.run(t, "check", "v0.2.0"); code == 0 || !strings.Contains(stderr, claudeManifest) {
			t.Fatalf("code = %d, stderr = %q, want a failure that names the manifest", code, stderr)
		}
	})

	t.Run("check fails on a missing manifest", func(t *testing.T) {
		f := newFixture(t)
		bump(t, f, "v1.0.0")
		if err := os.Remove(filepath.Join(f.dir, codexManifest)); err != nil {
			t.Fatal(err)
		}

		if code, _, stderr := f.run(t, "check", "v1.0.0"); code == 0 || !strings.Contains(stderr, codexManifest) {
			t.Fatalf("code = %d, stderr = %q, want a failure that names the manifest", code, stderr)
		}
	})

	for _, command := range []string{"bump", "check"} {
		for _, tag := range []string{"v1.2", "latest", "1.2.3", "v1.2.3-rc1", "v1.2.3\"x", ""} {
			t.Run(fmt.Sprintf("%s rejects the tag %q", command, tag), func(t *testing.T) {
				f := newFixture(t)
				before := f.read(t, claudeManifest)

				code, _, _ := f.run(t, command, tag)

				if code == 0 || f.read(t, claudeManifest) != before {
					t.Fatalf("code = %d, want a failure and no change", code)
				}
			})
		}
	}

	t.Run("an unknown command prints the usage and exits 2", func(t *testing.T) {
		f := newFixture(t)

		for _, args := range [][]string{{"nope"}, {}} {
			if code, _, stderr := f.run(t, args...); code != exitUsage || !strings.Contains(stderr, "usage") {
				t.Errorf("args %v: code = %d, stderr = %q, want %d and the usage", args, code, stderr, exitUsage)
			}
		}
	})

	t.Run("the launcher downloads the bumped version", func(t *testing.T) {
		f := newFixture(t)
		bump(t, f, "v3.1.4")
		launcher, err := os.ReadFile(repository.Path(t, launcherPath))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(f.dir, launcherPath)
		if err := os.MkdirAll(filepath.Dir(target), execMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, launcher, execMode); err != nil {
			t.Fatal(err)
		}
		calls := filepath.Join(t.TempDir(), "curl.log")
		tools := t.TempDir()
		for _, tool := range []string{"sh", "git", "uname", "tr", "mkdir", "find", "rm", "mktemp", "chmod", "mv", "readlink", "cat"} {
			found, err := exec.LookPath(tool)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(found, filepath.Join(tools, tool)); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(tools, "curl"), []byte(fmt.Sprintf(loggingCurl, calls)), execMode); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(target, "version")
		cmd.Dir = f.dir
		cmd.Env = []string{"PATH=" + tools, "HOME=" + t.TempDir()}
		_ = cmd.Run()

		raw, err := os.ReadFile(calls)
		if err != nil {
			t.Fatalf("the launcher did not call curl: %v", err)
		}
		if !strings.Contains(string(raw), "/releases/download/v3.1.4/harness-") {
			t.Fatalf("curl calls = %q, want the asset URL of release v3.1.4", raw)
		}
	})
}

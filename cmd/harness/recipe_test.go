package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/examples"
)

const recipeGo = "go"

func recipeDir(t *testing.T) string {
	t.Helper()
	clearOTLPEnv(t)
	clearHarnessEnv(t)
	dir := t.TempDir()
	t.Chdir(dir)
	return dir
}

func TestRecipeList(t *testing.T) {
	recipeDir(t)
	code, stdout, stderr := execute(t, "recipe", "list")
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	summaries := map[string]string{
		"go":      "Go module with go-arch-lint and the gates in Docker Compose",
		"nextjs":  "Next.js with shadcn, the ddd pack and gates in Docker Compose",
		"python":  "Python with uv, ruff, import-linter and the gates in Docker Compose",
		"symfony": "Symfony with Makefile gates, deptrac and Docker Compose",
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != len(summaries) {
		t.Errorf("list has %d lines, want %d:\n%s", len(lines), len(summaries), stdout)
	}
	for name, summary := range summaries {
		found := slices.ContainsFunc(lines, func(line string) bool {
			rest, ok := strings.CutPrefix(line, name+" ")
			return ok && strings.TrimSpace(rest) == summary
		})
		if !found {
			t.Errorf("list has no line with %s and %q:\n%s", name, summary, stdout)
		}
	}
}

// entries lists every file and folder under root, relative to it.
func entries(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root {
			rel, _ := filepath.Rel(root, path)
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func TestRecipeShow(t *testing.T) {
	recipeDir(t)
	code, stdout, stderr := execute(t, "recipe", "show", "nextjs")
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{".harness.yaml", "compose.yaml", "create-next-app", "next: harness verify"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("show lacks %q:\n%s", want, stdout)
		}
	}
}

func TestRecipeApplyWrites(t *testing.T) {
	dir := recipeDir(t)
	code, stdout, stderr := execute(t, "recipe", "apply", recipeGo)
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"wrote .harness.yaml\n", "wrote compose.yaml\n", "next: "} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".harness.yaml")); err != nil {
		t.Errorf("file not written: %v", err)
	}
}

func TestRecipeApplyKeeps(t *testing.T) {
	dir := recipeDir(t)
	if code, _, stderr := execute(t, "recipe", "apply", recipeGo); code != exitOK {
		t.Fatalf("first apply: code = %d, stderr = %q", code, stderr)
	}
	mine := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(mine, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := execute(t, "recipe", "apply", recipeGo)
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"kept .harness.yaml\n", "differs compose.yaml\n", "next: "} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, stdout)
		}
	}
	if got, _ := os.ReadFile(mine); string(got) != "mine\n" {
		t.Errorf("compose.yaml changed: %q", got)
	}
}

func TestRecipeApplyTopLevel(t *testing.T) {
	dir := recipeDir(t)
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	sub := filepath.Join(dir, "deep", "er")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	code, stdout, stderr := execute(t, "recipe", "apply", recipeGo)
	if code != exitOK {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, ".harness.yaml")); err != nil {
		t.Errorf("file not at the top level: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sub, ".harness.yaml")); err == nil {
		t.Error("file written in the subfolder")
	}
}

func TestRecipeApplyDryRun(t *testing.T) {
	dir := recipeDir(t)
	same, err := examples.FS.ReadFile(recipeGo + "/.harness.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".harness.yaml"), same, 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := execute(t, "recipe", "apply", recipeGo, "--dry-run")
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"would write compose.yaml\n", "would keep .harness.yaml", "next: "} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, stdout)
		}
	}
	if got := entries(t, dir); !slices.Equal(got, []string{".harness.yaml"}) {
		t.Errorf("a dry run changed the folder: %v, want only the existing .harness.yaml", got)
	}
}

func TestRecipeApplyDryRunWritesNothingInAnEmptyFolder(t *testing.T) {
	dir := recipeDir(t)
	code, stdout, stderr := execute(t, "recipe", "apply", recipeGo, "--dry-run")
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"would write lint/architecture/pack.yaml\n", "would write .harness/plugins/go-arch-lint.sh\n"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, stdout)
		}
	}
	if got := entries(t, dir); len(got) != 0 {
		t.Errorf("a dry run left %v in the folder, want nothing", got)
	}
}

// A git error that is not "not a repository" must not turn into a write in the
// current folder. A bare repository fails git rev-parse --show-toplevel that way.
func TestRecipeApplyReturnsOtherGitErrors(t *testing.T) {
	dir := recipeDir(t)
	if out, err := exec.Command("git", "init", "-q", "--bare", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	before := entries(t, dir)

	code, _, stderr := execute(t, "recipe", "apply", recipeGo)

	if code == exitOK {
		t.Fatalf("code = %d, want an error, stderr = %q", code, stderr)
	}
	if !strings.Contains(stderr, "work tree") {
		t.Errorf("stderr = %q, want the git error", stderr)
	}
	if after := entries(t, dir); !slices.Equal(before, after) {
		t.Errorf("apply wrote in the folder: %v, want %v", after, before)
	}
}

func TestRecipeUnknown(t *testing.T) {
	recipeDir(t)
	for _, sub := range []string{"show", "apply"} {
		code, _, stderr := execute(t, "recipe", sub, "nope")
		if code != exitUsage {
			t.Errorf("%s: code = %d, want %d", sub, code, exitUsage)
		}
		for _, name := range []string{"go", "nextjs", "python", "symfony"} {
			if !strings.Contains(stderr, name) {
				t.Errorf("%s: stderr lacks %s: %q", sub, name, stderr)
			}
		}
	}
}

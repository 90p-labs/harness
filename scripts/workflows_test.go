package scripts_test

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
	"gopkg.in/yaml.v3"
)

const (
	autoReleaseFile = ".github/workflows/auto-release.yml"
	releaseFile     = ".github/workflows/release.yml"
	releaseCallPath = "./.github/workflows/release.yml"
	skipGuard       = "startsWith(github.event.head_commit.message, 'chore(release):')"
)

var pinnedUses = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[0-9a-f]{40}$`)

type node = map[string]any

func loadWorkflow(t *testing.T, path string) node {
	t.Helper()
	var doc node
	if err := yaml.Unmarshal([]byte(repository.Read(t, path)), &doc); err != nil {
		t.Fatalf("%s does not parse: %v", path, err)
	}
	return doc
}

func child(t *testing.T, parent any, keys ...string) any {
	t.Helper()
	current := parent
	for _, key := range keys {
		m, ok := current.(node)
		if !ok {
			t.Fatalf("%v: not a mapping before %q", keys, key)
		}
		current, ok = m[key]
		if !ok {
			t.Fatalf("missing key %q in %v", key, keys)
		}
	}
	return current
}

func str(t *testing.T, parent any, keys ...string) string {
	t.Helper()
	s, ok := child(t, parent, keys...).(string)
	if !ok {
		t.Fatalf("%v is not a string", keys)
	}
	return s
}

func steps(t *testing.T, doc node, job string) []node {
	t.Helper()
	raw, ok := child(t, doc, "jobs", job, "steps").([]any)
	if !ok {
		t.Fatalf("job %s has no steps", job)
	}
	out := make([]node, 0, len(raw))
	for _, s := range raw {
		m, ok := s.(node)
		if !ok {
			t.Fatalf("job %s has a step that is not a mapping", job)
		}
		out = append(out, m)
	}
	return out
}

func jobs(t *testing.T, doc node) map[string]node {
	t.Helper()
	raw, ok := child(t, doc, "jobs").(node)
	if !ok {
		t.Fatal("jobs is not a mapping")
	}
	out := map[string]node{}
	for name, j := range raw {
		out[name], _ = j.(node)
	}
	return out
}

func runOf(s node) string {
	r, _ := s["run"].(string)
	return r
}

func stepIndex(list []node, needle string) int {
	for i, s := range list {
		if strings.Contains(runOf(s), needle) {
			return i
		}
	}
	return -1
}

func checkoutStep(t *testing.T, list []node) node {
	t.Helper()
	for _, s := range list {
		if u, _ := s["uses"].(string); strings.HasPrefix(u, "actions/checkout@") {
			return s
		}
	}
	t.Fatal("no checkout step")
	return nil
}

func TestReleaseWorkflows(t *testing.T) {
	auto := loadWorkflow(t, autoReleaseFile)
	release := loadWorkflow(t, releaseFile)
	docs := map[string]node{autoReleaseFile: auto, releaseFile: release}

	t.Run("every step uses a pinned action", func(t *testing.T) {
		for path, doc := range docs {
			for name, job := range jobs(t, doc) {
				if _, ok := job["steps"]; !ok {
					continue
				}
				for _, s := range steps(t, doc, name) {
					if u, ok := s["uses"].(string); ok && !pinnedUses.MatchString(u) {
						t.Errorf("%s job %s: %q is not pinned to a 40 hex sha", path, name, u)
					}
				}
			}
		}
	})

	t.Run("pins are the reviewed commits", func(t *testing.T) {
		want := map[string]bool{
			"actions/checkout@11d5960a326750d5838078e36cf38b85af677262": true,
			"actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff": true,
		}
		for path, doc := range docs {
			for name := range jobs(t, doc) {
				if _, ok := jobs(t, doc)[name]["steps"]; !ok {
					continue
				}
				for _, s := range steps(t, doc, name) {
					if u, ok := s["uses"].(string); ok && !want[u] {
						t.Errorf("%s job %s: unexpected action %q", path, name, u)
					}
				}
			}
		}
	})

	t.Run("no run block interpolates an expression", func(t *testing.T) {
		for path, doc := range docs {
			for name, job := range jobs(t, doc) {
				if _, ok := job["steps"]; !ok {
					continue
				}
				for _, s := range steps(t, doc, name) {
					if strings.Contains(runOf(s), "${{") {
						t.Errorf("%s job %s: run contains ${{: %s", path, name, runOf(s))
					}
				}
			}
		}
	})

	t.Run("top level permissions are empty", func(t *testing.T) {
		for path, doc := range docs {
			got := child(t, doc, "permissions")
			if m, ok := got.(node); !ok || len(m) != 0 {
				t.Errorf("%s: top level permissions = %v, want {}", path, got)
			}
		}
	})

	t.Run("each job has only contents write", func(t *testing.T) {
		want := node{"contents": "write"}
		for path, doc := range docs {
			for name := range jobs(t, doc) {
				got := child(t, doc, "jobs", name, "permissions")
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s job %s: permissions = %v, want %v", path, name, got, want)
				}
			}
		}
	})

	t.Run("auto release triggers only on push to main", func(t *testing.T) {
		on := child(t, auto, "on")
		if m, ok := on.(node); !ok || len(m) != 1 {
			t.Fatalf("on = %v, want only push", on)
		}
		push := child(t, auto, "on", "push")
		if m, ok := push.(node); !ok || len(m) != 1 {
			t.Fatalf("push = %v, want only branches", push)
		}
		if got := child(t, auto, "on", "push", "branches"); !reflect.DeepEqual(got, []any{"main"}) {
			t.Errorf("branches = %v, want [main]", got)
		}
	})

	t.Run("auto release serializes runs", func(t *testing.T) {
		if got := str(t, auto, "concurrency", "group"); got == "" {
			t.Error("concurrency group is empty")
		}
		if got := child(t, auto, "concurrency", "cancel-in-progress"); got != false {
			t.Errorf("cancel-in-progress = %v, want false", got)
		}
	})

	t.Run("tag job skips its own release commit", func(t *testing.T) {
		cond := strings.Join(strings.Fields(str(t, auto, "jobs", "tag", "if")), "")
		want := strings.Join(strings.Fields("!"+skipGuard), "")
		if !strings.Contains(cond, want) {
			t.Errorf("if = %q, want it to contain !%s", cond, skipGuard)
		}
	})

	t.Run("tag job checks out main with full history and tags", func(t *testing.T) {
		with := child(t, checkoutStep(t, steps(t, auto, "tag")), "with")
		if got := child(t, with, "ref"); got != "main" {
			t.Errorf("ref = %v, want main", got)
		}
		if got := child(t, with, "fetch-depth"); got != 0 {
			t.Errorf("fetch-depth = %v, want 0", got)
		}
		if got := child(t, with, "fetch-tags"); got != true {
			t.Errorf("fetch-tags = %v, want true", got)
		}
	})

	t.Run("tag job exposes the tag output", func(t *testing.T) {
		if got := str(t, auto, "jobs", "tag", "outputs", "tag"); got != "${{ steps.next.outputs.tag }}" {
			t.Errorf("output = %q", got)
		}
	})

	t.Run("make dist runs before the atomic push", func(t *testing.T) {
		list := steps(t, auto, "tag")
		dist := stepIndex(list, "make dist")
		push := stepIndex(list, "push --atomic")
		if dist < 0 || push < 0 {
			t.Fatalf("make dist at %d, git push at %d", dist, push)
		}
		if dist > push {
			t.Errorf("make dist (step %d) comes after git push (step %d)", dist, push)
		}
		if !strings.Contains(runOf(list[push]), `push --atomic origin HEAD:main "$TAG"`) {
			t.Errorf("push step is not atomic: %s", runOf(list[push]))
		}
	})

	t.Run("tag job checks the manifests before the push", func(t *testing.T) {
		list := steps(t, auto, "tag")
		bump := stepIndex(list, "release.sh bump")
		check := stepIndex(list, `scripts/release.sh check "$TAG"`)
		push := stepIndex(list, "push --atomic")
		if bump < 0 || check < 0 || push < 0 {
			t.Fatalf("bump at %d, check at %d, push at %d", bump, check, push)
		}
		if bump >= check || check >= push {
			t.Errorf("want bump (%d) before check (%d) before push (%d)", bump, check, push)
		}
	})

	t.Run("checkout does not keep the token", func(t *testing.T) {
		for path, doc := range docs {
			for name, job := range jobs(t, doc) {
				if _, ok := job["steps"]; !ok {
					continue
				}
				with := child(t, checkoutStep(t, steps(t, doc, name)), "with")
				if got := child(t, with, "persist-credentials"); got != false {
					t.Errorf("%s job %s: persist-credentials = %v, want false", path, name, got)
				}
			}
		}
	})

	t.Run("only the push step reads the token", func(t *testing.T) {
		list := steps(t, auto, "tag")
		push := stepIndex(list, "push --atomic")
		for i, s := range list {
			env, _ := s["env"].(node)
			_, has := env["TOKEN"]
			if has != (i == push) {
				t.Errorf("step %d has TOKEN = %v, want it only on the push step %d", i, has, push)
			}
		}
		if push >= 0 {
			if got := child(t, list[push], "env", "TOKEN"); got != "${{ github.token }}" {
				t.Errorf("push TOKEN = %v", got)
			}
		}
	})

	t.Run("later tag steps read the tag from env", func(t *testing.T) {
		list := steps(t, auto, "tag")
		for _, needle := range []string{"release.sh bump", "release.sh check", "git tag", "push --atomic", "make dist"} {
			i := stepIndex(list, needle)
			if i < 0 {
				t.Errorf("no step runs %q", needle)
				continue
			}
			if got := list[i]["if"]; got != "steps.next.outputs.tag != ''" {
				t.Errorf("step %q if = %v", needle, got)
			}
			if got := child(t, list[i], "env", "TAG"); got != "${{ steps.next.outputs.tag }}" {
				t.Errorf("step %q TAG = %v", needle, got)
			}
		}
	})

	t.Run("release job calls the release workflow", func(t *testing.T) {
		if got := str(t, auto, "jobs", "release", "uses"); got != releaseCallPath {
			t.Errorf("uses = %q", got)
		}
		if got := str(t, auto, "jobs", "release", "with", "tag"); got != "${{ needs.tag.outputs.tag }}" {
			t.Errorf("with.tag = %q", got)
		}
		if got := str(t, auto, "jobs", "release", "needs"); got != "tag" {
			t.Errorf("needs = %q", got)
		}
		if got := str(t, auto, "jobs", "release", "if"); !strings.Contains(got, "needs.tag.outputs.tag != ''") {
			t.Errorf("if = %q", got)
		}
	})

	t.Run("release workflow keeps the tag trigger and takes a tag input", func(t *testing.T) {
		if got := child(t, release, "on", "push", "tags"); !reflect.DeepEqual(got, []any{"v*"}) {
			t.Errorf("tags = %v", got)
		}
		in := child(t, release, "on", "workflow_call", "inputs", "tag")
		if got := child(t, in, "required"); got != true {
			t.Errorf("required = %v", got)
		}
		if got := child(t, in, "type"); got != "string" {
			t.Errorf("type = %v", got)
		}
	})

	t.Run("release workflow resolves the tag", func(t *testing.T) {
		env := child(t, release, "jobs", "release", "env")
		if got := str(t, env, "TAG"); !strings.Contains(got, "inputs.tag") || !strings.Contains(got, "github.ref_name") {
			t.Errorf("TAG = %q", got)
		}
		if got := str(t, env, "GH_TOKEN"); got != "${{ github.token }}" {
			t.Errorf("GH_TOKEN = %q", got)
		}
		list := steps(t, release, "release")
		if ref := fmt.Sprint(child(t, checkoutStep(t, list), "with", "ref")); !strings.Contains(ref, "refs/tags/") || !strings.Contains(ref, "TAG") {
			t.Errorf("checkout ref = %q", ref)
		}
	})

	t.Run("release workflow validates, checks, notes and publishes", func(t *testing.T) {
		list := steps(t, release, "release")
		if i := stepIndex(list, `^v[0-9]+\.[0-9]+\.[0-9]+$`); i != 0 {
			t.Errorf("tag validation step at %d, want first", i)
		}
		for _, needle := range []string{
			`scripts/release.sh check "$TAG"`,
			`scripts/release.sh notes "$TAG"`,
			`make dist VERSION="$TAG"`,
			"--verify-tag",
			"--notes-file",
		} {
			if stepIndex(list, needle) < 0 {
				t.Errorf("no step runs %q", needle)
			}
		}
		if stepIndex(list, "--generate-notes") >= 0 {
			t.Error("release still uses --generate-notes")
		}
	})
}

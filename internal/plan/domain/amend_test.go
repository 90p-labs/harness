package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const amendBase = `# Plan: x

## Acceptance criteria

| # | Criterion | Proof |
|---|---|---|
| 1 | It works. | ` + "`go test ./...`" + ` |

## Tasks

### 1. Build

- Owned files: ` + "`a.go`, `a_test.go`, `b.go`" + `
- Test first: ` + "`a_test.go` holds `TestA`" + `

## Decisions

- 2026-11-10: Use a map.

### Final code tree

` + "```tree" + `
harness/
├── a.go   new
└── b.go   new
` + "```" + `

### Final test tree

` + "```tree" + `
harness/
└── a_test.go   new
` + "```" + `
`

func amendEdit(old, replacement string) string {
	return strings.Replace(amendBase, old, replacement, 1)
}

func TestTreeOnlyChange(t *testing.T) {
	cases := []struct {
		name    string
		current string
		want    bool
	}{
		{"the same text", amendBase, true},
		{"a tree line added", amendEdit("└── b.go   new", "├── b.go   new\n└── c.go   new"), true},
		{"a tree line removed", amendEdit("├── a.go   new\n", ""), true},
		{"a tree mark changed", amendEdit("└── b.go   new", "└── b.go   changed"), true},
		{"a test tree line added", amendEdit("└── a_test.go   new", "├── a_test.go   new\n└── b_test.go   new"), true},
		{"a test path added to the owned files", amendEdit("`b.go`", "`b.go`, `b_test.go`"), true},
		{"a test path removed from the owned files", amendEdit("`a_test.go`, ", ""), true},
		{"a testdata path added to the owned files", amendEdit("`b.go`", "`b.go`, `testdata/x.json`"), true},
		{"a criterion edited", amendEdit("It works.", "It works well."), false},
		{"a decision edited", amendEdit("Use a map.", "Use a slice."), false},
		{"a source path added to the owned files", amendEdit("`b.go`", "`b.go`, `c.go`"), false},
		{"a source path removed from the owned files", amendEdit(", `b.go`", ""), false},
		{"a source path written as a go test name", amendEdit("`b.go`", "`b.go`, `x.test.go`"), false},
		{"a folder that looks like a test file", amendEdit("`b.go`", "`b.go`, `foo_test.go/evil.go`"), false},
		{"the test first line changed", amendEdit("holds `TestA`", "holds `TestB`"), false},
		{"a line added outside the fences", amendEdit("## Decisions", "## Decisions\n\nnote"), false},
		{"a decision added", amendEdit("- 2026-11-10: Use a map.", "- 2026-11-10: Use a map.\n- 2026-11-10: New."), false},
		{"a tree heading renamed", amendEdit("### Final test tree", "### Final tests"), false},
		{"a line added between the heading and the fence", amendEdit("### Final code tree\n", "### Final code tree\n\nnote\n"), false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := treeOnlyChange(amendBase, tc.current); got != tc.want {
				t.Fatalf("treeOnlyChange = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("an unclosed fence", func(t *testing.T) {
		broken := strings.TrimSuffix(amendBase, "```\n")
		if treeOnlyChange(broken, broken) {
			t.Fatal("an unclosed fence must not parse")
		}
	})
	t.Run("an empty approved text", func(t *testing.T) {
		if treeOnlyChange("", "") {
			t.Fatal("empty input must be false")
		}
	})
}

func TestIsTestPath(t *testing.T) {
	cases := map[string]bool{
		"a_test.go":                  true,
		"internal/x/a_test.go":       true,
		"test_a.py":                  true,
		"a_test.py":                  true,
		"a.test.ts":                  true,
		"a.spec.tsx":                 true,
		"a.test.js":                  true,
		"a.spec.jsx":                 true,
		"a.test.mjs":                 true,
		"a.spec.cjs":                 true,
		"testdata/a.md":              true,
		"x/testdata/y/a.json":        true,
		"testdata/a.jsonl":           true,
		"testdata/a.txt":             true,
		"testdata/a.yaml":            true,
		"testdata/a.yml":             true,
		"testdata/a.golden":          true,
		"x.test.go":                  false,
		"foo_test.go/evil.go":        false,
		"testdatax/a.md":             false,
		"testdata/x.go":              false,
		"a.spec.rb":                  false,
		"test_a.go":                  false,
		"a.test.ts.bak":              false,
		"a.test.json":                false,
		"a.go":                       false,
		"internal/x/":                false,
		"":                           false,
		"mytestdata/a.md":            false,
		"testdata/":                  false,
		"test_a.py/evil.sh":          false,
		"a_test.go.sh":               false,
		"atest.ts":                   false,
		"testdata/a.md/evil.go":      false,
		"a.test.d.ts":                false,
		"testdata/sub/a.go":          false,
		"testdata/a.MD":              false,
		"x/Testdata/a.md":            false,
		"a_test.go ":                 false,
		"a_test.py.go":               false,
		"test_.go":                   false,
		"test.py":                    false,
		"internal/a/testdata/b.yaml": true,
		"testdata/../README.md":      false,
		"testdata/./a.md":            false,
		"../testdata/a.md":           false,
		"testdata/../a_test.go":      false,
	}
	for path, want := range cases {
		if got := isTestPath(path); got != want {
			t.Errorf("isTestPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestTreeOnlyAmend(t *testing.T) {
	sum := sha256.Sum256([]byte(amendBase))
	hash := hex.EncodeToString(sum[:])
	treeEdit := amendEdit("└── b.go   new", "└── b.go   changed")

	if !TreeOnlyAmend(amendBase, hash, treeEdit) {
		t.Fatal("a tree edit of the approved text must pass")
	}
	if TreeOnlyAmend(amendBase, hash+"0", treeEdit) {
		t.Fatal("a hash mismatch must fail")
	}
	if TreeOnlyAmend(amendBase+"x", hash, treeEdit) {
		t.Fatal("an edited kept copy must fail")
	}
	if TreeOnlyAmend(amendBase, hash, amendEdit("It works.", "It does not.")) {
		t.Fatal("a criterion edit must fail")
	}
}

package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

const amendBase = `# Plan: x

## Goal

Track: full

Fix the thing.

## Acceptance criteria

| # | Criterion | Proof |
|---|---|---|
| 1 | It works. | ` + "`go test ./...`" + ` |

## Tasks

### 1. Build

- Side: backend
- Owned files: ` + "`a.go`, `a_test.go`, `b.go`" + `
- Test first: ` + "`a_test.go` holds `TestA`" + ` and it fails.
- DoD: criteria 1 and 2
- Depends on: none
- Model: sonnet

## Decisions

- 2026-11-10: Use a map.

## Review risks

- Test first: ` + "`TestA`" + ` guards it.

### Commits

1. ` + "`fix(x): y`" + ` (task 1)

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

func TestStructureKept(t *testing.T) {
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
		{"the goal prose edited", amendEdit("Fix the thing.", "Fix another thing.\n\nMore prose."), true},
		{"the track changed", amendEdit("Track: full", "Track: light"), false},
		{"a decision edited", amendEdit("Use a map.", "Use a slice."), true},
		{"a test first prose edited with the same names", amendEdit("holds `TestA` and it fails.", "holds `TestA`, which is red first."), true},
		{"a test first name changed", amendEdit("`TestA` and it", "`TestB` and it"), false},
		{"a test first name added", amendEdit("`TestA` and it", "`TestA`, `TestB` and it"), false},
		{"a dod prose changed with the same numbers", amendEdit("criteria 1 and 2", "criteria 1, 2"), true},
		{"a dod prose that no longer parses", amendEdit("criteria 1 and 2", "the criteria 1, 2"), false},
		{"a dod number changed", amendEdit("criteria 1 and 2", "criteria 1 and 3"), false},
		{"a task renamed", amendEdit("### 1. Build", "### 1. Make"), false},
		{"a model changed", amendEdit("Model: sonnet", "Model: opus"), false},
		{"a dependency changed", amendEdit("Depends on: none", "Depends on: task 2"), false},
		{"a commit changed", amendEdit("`fix(x): y`", "`docs(x): y`"), false},
		{"a heading added inside the goal", amendEdit("Fix the thing.", "Fix the thing.\n\n### Sub"), false},
		{"a heading added inside the decisions", amendEdit("- 2026-11-10: Use a map.", "- 2026-11-10: Use a map.\n\n### Sub"), false},
		{"a hidden heading in an indented fence in the decisions", amendEdit("- 2026-11-10: Use a map.", "- 2026-11-10: Use a map.\n  ```\n## Tasks\n  ```"), false},
		{"a table row hidden in an indented fence in the decisions", amendEdit("- 2026-11-10: Use a map.", "- 2026-11-10: Use a map.\n  ```\n| 2 | New. | x |\n  ```"), false},
		{"a fence in the goal", amendEdit("Fix the thing.", "Fix the thing.\n```\ncode\n```"), false},
		{"a review risk line edited", amendEdit("`TestA` guards it.", "`TestA` guards it well."), false},
		{"a source path added to the owned files", amendEdit("`b.go`", "`b.go`, `c.go`"), false},
		{"a source path removed from the owned files", amendEdit(", `b.go`", ""), false},
		{"a source path written as a go test name", amendEdit("`b.go`", "`b.go`, `x.test.go`"), false},
		{"a folder that looks like a test file", amendEdit("`b.go`", "`b.go`, `foo_test.go/evil.go`"), false},
		{"the test first line changed", amendEdit("holds `TestA`", "holds `TestB`"), false},
		{"a line added outside the fences", amendEdit("## Review risks", "## Review risks\n\nnote"), false},
		{"a decision added", amendEdit("- 2026-11-10: Use a map.", "- 2026-11-10: Use a map.\n- 2026-11-10: New."), true},
		{"a tree heading renamed", amendEdit("### Final test tree", "### Final tests"), false},
		{"a line added between the heading and the fence", amendEdit("### Final code tree\n", "### Final code tree\n\nnote\n"), false},
		{"a dod trailing text negating a number", amendEdit("criteria 1 and 2", "criterion 1, not 2"), false},
		{"a test first backtick pairing shifted", amendEdit("`a_test.go` holds `TestA`", "`a_test.go`TestB`TestA`"), false},
		{"a sketch added in the goal", amendEdit("Fix the thing.", "Fix the thing.\n\n![ui](x.excalidraw)"), false},
		{"a sketch added in the decisions", amendEdit("Use a map.", "Use a map.\n- ![ui](x.excalidraw)"), false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := structureKeptChange(amendBase, tc.current); got != tc.want {
				t.Fatalf("structureKeptChange = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("a mermaid fence under a tree heading is a diagram, not a tree", func(t *testing.T) {
		const treeBlock = "```tree\nharness/\n├── a.go   new\n└── b.go   new\n```"
		diagram := func(body string) string { return "```mermaid\n" + body + "\n```" }
		approved := strings.Replace(amendBase, treeBlock, diagram("sequenceDiagram\n  A->>B: hi"), 1)
		if approved == amendBase {
			t.Fatal("the fixture did not change")
		}
		cases := []struct {
			name, body string
			want       bool
		}{
			{"the same diagram", "sequenceDiagram\n  A->>B: hi", true},
			{"the kind changed", "erDiagram\n  A ||--o{ B : has", false},
			{"only the body changed", "sequenceDiagram\n  A->>B: bye", false},
		}
		for _, tc := range cases {
			current := strings.Replace(amendBase, treeBlock, diagram(tc.body), 1)
			if got := structureKeptChange(approved, current); got != tc.want {
				t.Errorf("%s: structureKeptChange = %v, want %v", tc.name, got, tc.want)
			}
		}
	})

	t.Run("a fence under a tree heading that the parser does not read as a tree keeps its body", func(t *testing.T) {
		const treeBlock = "```tree\nharness/\n├── a.go   new\n└── b.go   new\n```"
		const sameBody = "harness/\n├── a.go   new"
		const changedBody = "harness/\n├── a.go   changed"
		fence := func(info, body string) string { return "```" + info + "\n" + body + "\n```" }
		for _, info := range []string{"", "tree x", "treex"} {
			approved := strings.Replace(amendBase, treeBlock, fence(info, sameBody), 1)
			if approved == amendBase {
				t.Fatal("the fixture did not change")
			}
			same := strings.Replace(amendBase, treeBlock, fence(info, sameBody), 1)
			changed := strings.Replace(amendBase, treeBlock, fence(info, changedBody), 1)
			if !structureKeptChange(approved, same) {
				t.Errorf("info %q: the same body must be kept", info)
			}
			if structureKeptChange(approved, changed) {
				t.Errorf("info %q: a changed body must need approval", info)
			}
		}
	})

	t.Run("a heading with two spaces after the marks is not a tree heading", func(t *testing.T) {
		approved := amendEdit("### Final code tree", "###  Final code tree")
		edited := strings.Replace(approved, "└── b.go   new", "└── b.go   changed", 1)
		if approved == amendBase {
			t.Fatal("the fixture did not change")
		}
		if structureKeptChange(approved, edited) {
			t.Fatal("a tree body under a heading the parser does not read as a tree heading must need approval")
		}
	})

	t.Run("a section heading with two spaces after the marks is not that section", func(t *testing.T) {
		approved := amendEdit("## Decisions", "##  Decisions")
		edited := strings.Replace(approved, "Use a map.", "Use a slice.", 1)
		if approved == amendBase {
			t.Fatal("the fixture did not change")
		}
		if structureKeptChange(approved, edited) {
			t.Fatal("a line under a section the parser does not read as decisions must need approval")
		}
	})

	t.Run("an indented fence and an info-string fence must not hide a task field", func(t *testing.T) {
		const ignoredFences = "```\n```x\n## Decisions\n```\n  ```\n"
		approved := amendEdit("## Decisions", "### 2. Two\n\n"+ignoredFences+"- Model: sonnet\n\n## Decisions")
		edited := strings.Replace(approved, "- Model: sonnet\n\n## Decisions", "- Model: opus\n\n## Decisions", 1)
		if !structureKeptChange(approved, approved) {
			t.Fatal("the same text must be kept")
		}
		if structureKeptChange(approved, edited) {
			t.Fatal("a model change after a fence the parser ignores must need approval")
		}
	})
	t.Run("an indented fence in the decisions holding a heading", func(t *testing.T) {
		approved := amendEdit("- 2026-11-10: Use a map.", "- 2026-11-10: Use a map.\n  ```\n## Tasks\n  ```")
		edited := strings.Replace(approved, "## Tasks\n  ```", "## Review risks\n  ```", 1)
		if structureKeptChange(approved, edited) {
			t.Fatal("an edit inside an indented fence must need approval")
		}
	})
	t.Run("a dod list turned into a range", func(t *testing.T) {
		approved := amendEdit("criteria 1 and 2", "criteria 1 and 9")
		edited := amendEdit("criteria 1 and 2", "criteria 1 to 9")
		if !structureKeptChange(approved, approved) {
			t.Fatal("the same text must be kept")
		}
		if structureKeptChange(approved, edited) {
			t.Fatal("a range that expands to more criteria must need approval")
		}
	})
	t.Run("an unclosed fence", func(t *testing.T) {
		broken := strings.TrimSuffix(amendBase, "```\n")
		if structureKeptChange(broken, broken) {
			t.Fatal("an unclosed fence must not parse")
		}
	})
	t.Run("an empty approved text", func(t *testing.T) {
		if structureKeptChange("", "") {
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

func TestStructureKeptAmend(t *testing.T) {
	sum := sha256.Sum256([]byte(amendBase))
	hash := hex.EncodeToString(sum[:])
	treeEdit := amendEdit("└── b.go   new", "└── b.go   changed")

	if !StructureKeptAmend(amendBase, hash, treeEdit) {
		t.Fatal("a tree edit of the approved text must pass")
	}
	if StructureKeptAmend(amendBase, hash+"0", treeEdit) {
		t.Fatal("a hash mismatch must fail")
	}
	if StructureKeptAmend(amendBase+"x", hash, treeEdit) {
		t.Fatal("an edited kept copy must fail")
	}
	if StructureKeptAmend(amendBase, hash, amendEdit("It works.", "It does not.")) {
		t.Fatal("a criterion edit must fail")
	}
}

func TestAmendSeam(t *testing.T) {
	seamBase := amendEdit("holds `TestA` and it fails.", "holds `TestA` and it fails. Seam: `pkg.A`. Also `pkg.B`.")
	edit := func(old, replacement string) string { return strings.Replace(seamBase, old, replacement, 1) }
	cases := []struct {
		name    string
		current string
		want    bool
	}{
		{"the same text", seamBase, true},
		{"the seam prose changed", edit("and it fails.", "and it is red first."), true},
		{"a changed seam", edit("Seam: `pkg.A`", "Seam: `pkg.C`"), false},
		{"a label moved to another name of the line", edit("Seam: `pkg.A`. Also `pkg.B`", "Also `pkg.A`. Seam: `pkg.B`"), false},
		{"a dropped label", edit(" Seam: `pkg.A`. Also `pkg.B`.", " `pkg.A`. Also `pkg.B`."), false},
		{"an added seam", edit("Also `pkg.B`.", "Seam: `pkg.B`."), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := structureKeptChange(seamBase, tc.current); got != tc.want {
				t.Errorf("structureKeptChange = %v, want %v", got, tc.want)
			}
		})
	}
}

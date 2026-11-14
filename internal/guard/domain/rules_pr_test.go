package domain

import (
	"slices"
	"strings"
	"testing"
)

// shapeTail holds the Evidence and Merge danger sections of a good body.
const shapeTail = "\n\n## Evidence\n\nBefore: x\n\n```\nok\n```\n\nAfter: y\n\n```\nok\n```\n\n## Merge danger\n\n- Door: two-way. Revert.\n- Blast radius: small. One job."

// TestBodyProblemsEmDash checks the em dash problems. They come after the
// section problems, in line order with the other line problems, and an em dash
// in a fence counts too. A mermaid mark problem comes at the closing fence.
func TestBodyProblemsEmDash(t *testing.T) {
	const dash = string(emDashRune)
	const next = ". rewrite the line with commas, colons or sentences"
	const head = "Bottom line.\n\n## Why\n"
	const tail = "\n\n## How\ny" + shapeTail
	cases := map[string]struct {
		body string
		want []string
	}{
		"clean body": {
			head + "x\n\n## What\nz" + tail,
			nil,
		},
		"plain line": {
			head + "x " + dash + " y\n\n## What\nz" + tail,
			[]string{"line 4 has an em dash" + next},
		},
		"fenced code block": {
			head + "x\n\n## What\n```\na " + dash + " b\n```" + tail,
			[]string{"line 8 has an em dash" + next},
		},
		"mermaid fence": {
			head + "x\n\n## What\n```mermaid\nflowchart TD\n  A[a " + dash + " b] --> B\n```" + tail,
			// The mark problem comes at the closing fence, so after the line problem.
			[]string{"line 9 has an em dash" + next, "the mermaid diagram is not marked: no mark: draw the marks with harness diagram. draw it with `harness diagram`"},
		},
		"two lines in line order": {
			head + "x " + dash + " y\n\n## What\n```\na " + dash + " b\n```" + tail,
			[]string{"line 4 has an em dash" + next, "line 8 has an em dash" + next},
		},
		"em dash in an evidence fence": {
			head + "x\n\n## What\nz\n\n## How\ny\n\n## Evidence\n\nBefore: x\n\n```\na " + dash + " b\n```\n\nAfter: y\n\n```\nok\n```\n\n## Merge danger\n\n- Door: two-way. Revert.\n- Blast radius: small. One job.",
			[]string{"line 17 has an em dash" + next},
		},
	}
	for name, tc := range cases {
		if got := BodyProblems(tc.body); !slices.Equal(got, tc.want) {
			t.Errorf("%s: problems = %q, want %q", name, got, tc.want)
		}
	}
}

func TestBodyProblems(t *testing.T) {
	const good = "Bottom line: the guard checks X.\n\n## Why\nBecause.\n\n## What\n- one\n\n## How\nShort." + shapeTail
	long := func(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }

	if got := BodyProblems(good); len(got) != 0 {
		t.Fatalf("a good body gives problems: %q", got)
	}
	if got := BodyProblems(good + "\r\n"); len(got) != 0 {
		t.Fatalf("a good body with a CRLF end gives problems: %q", got)
	}

	body := "Bottom line: x.\n\n## Why\n" + long(52) + "\n\n## What\n" + long(45) + "\n"
	want := []string{
		"the body has no ## How line" + shape,
		"the body has no ## Evidence line" + shape,
		"the body has no ## Merge danger line" + shape,
		`line 4 has 52 words, more than 40 words: "word word word word word word". split the line into lines of 40 words or fewer`,
		`line 7 has 45 words, more than 40 words: "word word word word word word". split the line into lines of 40 words or fewer`,
	}
	if got := BodyProblems(body); !slices.Equal(got, want) {
		t.Fatalf("problems = %q, want %q", got, want)
	}

	const bottom = ". start the body with the bottom line: one or two sentences above "
	const diagram = ". draw it with `harness diagram`"
	cases := map[string]struct {
		body string
		want []string
	}{
		"no why and no how": {
			"Bottom line.\n\n## What\n- one",
			[]string{
				"the body has no ## Why line" + shape,
				"the body has no ## How line" + shape,
				"the body has no ## Evidence line" + shape,
				"the body has no ## Merge danger line" + shape,
			},
		},
		"how before what": {
			"Bottom line.\n\n## Why\nx\n\n## How\ny\n\n## What\nz" + shapeTail,
			[]string{"## How is before ## What" + shape},
		},
		"no bottom line and no why": {
			"## What\n- one\n\n## How\ny" + shapeTail,
			[]string{"the body has no ## Why line" + shape, "the body has no bottom line above ## What" + bottom + "## What"},
		},
		"no bottom line": {
			"## Why\nx\n\n## What\n- one\n\n## How\ny" + shapeTail,
			[]string{"the body has no bottom line above ## Why" + bottom + "## Why"},
		},
		"two unmarked mermaid problems": {
			"Bottom line.\n\n## Why\nx\n\n## What\n```mermaid\nflowchart TD\n  A --> B\n  class A foo\n  class B bar\n```\n\n## How\ny" + shapeTail,
			[]string{"the mermaid diagram is not marked: class foo is not a mark" + diagram, "the mermaid diagram is not marked: class bar is not a mark" + diagram},
		},
		"old shape body": {
			"Bottom line.\n\n## Why\nx\n\n## What\n- one\n\n## How\ny",
			[]string{"the body has no ## Evidence line" + shape, "the body has no ## Merge danger line" + shape},
		},
	}
	for name, tc := range cases {
		if got := BodyProblems(tc.body); !slices.Equal(got, tc.want) {
			t.Errorf("%s: problems = %q, want %q", name, got, tc.want)
		}
	}
}

// shape is the next step of a missing or misplaced section.
const shape = ". write the body with the lines ## Why, ## What, ## How, ## Evidence and ## Merge danger, in this order"

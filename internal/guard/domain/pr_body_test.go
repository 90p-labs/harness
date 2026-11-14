package domain

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

const (
	nextEvidence = "write a Before: line and an After: line under ## Evidence, in this order"
	nextProof    = "put a screenshot, or a fence with the run command and its output, below the line"
	nextDanger   = "write a Door: line and a Blast radius: line under ## Merge danger"
	nextReplace  = "replace it with your text"

	phBottom      = "<bottom line: what the PR changes and what the reviewer must check>"
	phWhy         = "<why: the problem and the reason for the change>"
	phBefore      = "<what the behaviour was, in one line>"
	phAfter       = "<what the behaviour is now, in one line>"
	phProof       = "<proof: a screenshot, or a fence with the run command and its output>"
	phDoor        = "<one-way or two-way>"
	phDoorReason  = "<how to reverse the merge, or why it cannot be reversed>"
	phBlast       = "<small, medium or large>"
	phBlastReason = "<what breaks and for whom if the change is wrong>"

	proofFence  = "```\n$ run\nok\n```"
	goodDanger  = "- Door: two-way. Revert the merge.\n- Blast radius: small. One command breaks.\n"
	goodHead    = "Bottom line: x.\n\n## Why\nBecause.\n\n## What\n- one\n\n## How\nShort.\n\n"
	goodEvidenc = "Before: it failed.\n\n" + proofFence + "\n\nAfter: it passes.\n\n" + proofFence + "\n"
)

const skeletonOne = `<bottom line: what the PR changes and what the reviewer must check>

## Why

<why: the problem and the reason for the change>

## What

- one

## How

The change adds no diagram.

## Evidence

Before: <what the behaviour was, in one line>

<proof: a screenshot, or a fence with the run command and its output>

After: <what the behaviour is now, in one line>

<proof: a screenshot, or a fence with the run command and its output>

## Merge danger

- Door: <one-way or two-way>. <how to reverse the merge, or why it cannot be reversed>
- Blast radius: <small, medium or large>. <what breaks and for whom if the change is wrong>
`

// fillSkeleton replaces each placeholder of the skeleton with text, except the ones named in keep.
func fillSkeleton(skeleton string, keep ...string) string {
	fills := [][2]string{
		{phBottom, "The change adds the guard."},
		{phWhy, "Because it was missing."},
		{phBefore, "It failed."},
		{phAfter, "It passes."},
		{phProof, proofFence},
		{phDoor, "two-way"},
		{phDoorReason, "Revert the merge."},
		{phBlast, "small"},
		{phBlastReason, "One command breaks."},
	}
	for _, f := range fills {
		if !slices.Contains(keep, f[0]) {
			skeleton = strings.ReplaceAll(skeleton, f[0], f[1])
		}
	}
	return skeleton
}

func withEvidence(evidence string) string {
	return goodHead + "## Evidence\n\n" + evidence + "\n## Merge danger\n\n" + goodDanger
}

func withDanger(danger string) string {
	return goodHead + "## Evidence\n\n" + goodEvidenc + "\n## Merge danger\n\n" + danger
}

func checkProblems(t *testing.T, name, body string, want []string) {
	t.Helper()
	if got := BodyProblems(body); !slices.Equal(got, want) {
		t.Errorf("%s: problems = %q, want %q", name, got, want)
	}
}

func TestBodySkeleton(t *testing.T) {
	if got := BodySkeleton([]string{"one"}, nil); got != skeletonOne {
		t.Fatalf("skeleton = %q, want %q", got, skeletonOne)
	}

	fence := "```mermaid\nstateDiagram-v2\n    A --> B: go (new)\n```\n"
	other := "```mermaid\nsequenceDiagram\n    a->>b: call\n```\n"
	two := BodySkeleton([]string{"one", "two"}, []string{fence, other})
	wantHow := "- one\n- two\n\n## How\n\n" + fence + "\n" + other + "\n## Evidence\n"
	if !strings.Contains(two, wantHow) {
		t.Fatalf("skeleton with diagrams = %q, want it to hold %q", two, wantHow)
	}

	cases := map[string]string{
		"no diagram":   skeletonOne,
		"two diagrams": two,
	}
	for name, skeleton := range cases {
		checkProblems(t, name, fillSkeleton(skeleton), nil)
	}

	if got := BodySkeleton(nil, nil); !strings.Contains(got, "## What\n\n\n## How\n") {
		t.Errorf("skeleton with no commit = %q", got)
	}
}

func TestBodyAccessorsReturnCopies(t *testing.T) {
	cases := map[string]struct {
		get  func() []string
		want []string
	}{
		"sections":     {BodySections, []string{"## Why", "## What", "## How", "## Evidence", "## Merge danger"}},
		"labels":       {BodyLabels, []string{"Before:", "After:", "Door:", "Blast radius:"}},
		"door values":  {DoorValues, []string{"one-way", "two-way"}},
		"radius scale": {BlastRadiusValues, []string{"small", "medium", "large"}},
	}
	for name, tc := range cases {
		got := tc.get()
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s = %q, want %q", name, got, tc.want)
		}
		got[0] = "changed"
		if again := tc.get(); !slices.Equal(again, tc.want) {
			t.Errorf("%s changed through its copy: %q", name, again)
		}
	}
}

func TestBodyEvidence(t *testing.T) {
	noBefore := "## Evidence has no Before: line. " + nextEvidence
	noAfter := "## Evidence has no After: line. " + nextEvidence
	beforeNoProof := "the Before: line shows no proof. " + nextProof
	afterNoProof := "the After: line shows no proof. " + nextProof
	noDanger := "the body has no ## Merge danger line" + shape
	image := "![shot](https://example.com/a.png)"
	tag := `<img src="https://example.com/a.png" alt="shot">`

	cases := []struct {
		name     string
		evidence string
		want     []string
	}{
		{"fence proof", goodEvidenc, nil},
		{"image proof below", "Before: x\n\n" + image + "\n\nAfter: y\n\n" + image + "\n", nil},
		{"img tag proof below", "Before: x\n\n" + tag + "\n\nAfter: y\n\n" + tag + "\n", nil},
		{"img tag with single quotes", "Before: x\n<img alt='a' src='https://e.com/a.png'>\nAfter: y\n<img src=https://e.com/b.png>\n", nil},
		{"image on the label line", "Before: x " + image + "\nAfter: y " + image + "\n", nil},
		{"fence with a heading inside", "Before: x\n\n```\n## Notes\n$ run\n```\n\nAfter: y\n\n" + proofFence + "\n", nil},
		{"fence with a label inside", "Before: x\n\n```\nAfter: fake\n```\n\nAfter: y\n\n" + proofFence + "\n", nil},
		{"bold labels", "**Before:** x\n\n" + proofFence + "\n\n**After:** y\n\n" + proofFence + "\n", nil},
		{"star marker", "* Before: x\n\n" + proofFence + "\n\n* After: y\n\n" + proofFence + "\n", nil},
		{"dash marker", "- Before: x\n\n" + proofFence + "\n\n- After: y\n\n" + proofFence + "\n", nil},
		{"labels in another case", "before: x\n\n" + proofFence + "\n\nAFTER: y\n\n" + proofFence + "\n", nil},
		{"duplicate label is plain text", "Before: x\n\n" + proofFence + "\n\nBefore: again\n\nAfter: y\n\n" + proofFence + "\n", nil},
		{"label in a fence only", "```\nBefore: x\nAfter: y\n```\n", []string{noBefore, noAfter}},
		{"empty fence", "Before: x\n\n```\n\n```\n\nAfter: y\n\n" + proofFence + "\n", []string{beforeNoProof}},
		{"image with no target", "Before: x\n\n![shot]()\n\nAfter: y\n\n" + image + "\n", []string{beforeNoProof}},
		{"image with a blank target", "Before: x\n\n![shot]( )\n\nAfter: y\n\n" + image + "\n", []string{beforeNoProof}},
		{"image with an empty angle target", "Before: x\n\n![shot](<>)\n\nAfter: y\n\n" + image + "\n", []string{beforeNoProof}},
		{"image with a blank angle target", "Before: x\n\n![shot](< >)\n\nAfter: y\n\n" + image + "\n", []string{beforeNoProof}},
		{"image with an angle target", "Before: x\n\n![shot](<a.png>)\n\nAfter: y\n\n" + image + "\n", nil},
		{"underscore labels", "__Before:__ x\n\n" + proofFence + "\n\n__After:__ y\n\n" + proofFence + "\n", nil},
		{"underscore labels after a marker", "- __Before:__ x\n\n" + proofFence + "\n\n* __After:__ y\n\n" + proofFence + "\n", nil},
		{"img tag with no src", "Before: x\n\n<img alt=\"a\">\n\nAfter: y\n\n" + tag + "\n", []string{beforeNoProof}},
		{"img tag with an empty src", "Before: x\n\n" + proofFence + "\n\nAfter: y\n\n<img src=\"\">\n", []string{afterNoProof}},
		{"img tag with a blank double-quoted src", "Before: x\n\n<img src=\" \">\n\nAfter: y\n\n" + tag + "\n", []string{beforeNoProof}},
		{"img tag with a blank single-quoted src", "Before: x\n\n<img src=' '>\n\nAfter: y\n\n" + tag + "\n", []string{beforeNoProof}},
		{"img tag with a space inside the src", "Before: x\n\n<img src=\"my shot.png\">\n\nAfter: y\n\n" + tag + "\n", nil},
		{"image inside a fence is a fence with content", "Before: x\n\n```\n" + image + "\n```\n\nAfter: y\n\n" + proofFence + "\n", nil},
		{"image in inline code", "Before: x `" + image + "`\n\nAfter: y\n\n" + image + "\n", []string{beforeNoProof}},
		{"img tag in inline code", "Before: x\n\n`" + tag + "`\n\nAfter: y\n\n" + tag + "\n", []string{beforeNoProof}},
		{"image in a one-line comment", "Before: x\n\n<!-- " + image + " -->\n\nAfter: y\n\n" + image + "\n", []string{beforeNoProof}},
		{"image in a three-line comment", "Before: x\n\n<!--\n" + image + "\n-->\n\nAfter: y\n\n" + image + "\n", []string{beforeNoProof}},
		{"image after a closed comment", "Before: x <!-- note --> " + image + "\n\nAfter: y\n\n" + image + "\n", nil},
		{"fence in a comment", "Before: x\n\n<!--\n" + proofFence + "\n-->\n\nAfter: y\n\n" + image + "\n", []string{beforeNoProof}},
		{"fence after a closed comment", "Before: x\n\n<!-- note -->\n\n" + proofFence + "\n\nAfter: y\n\n" + image + "\n", nil},
		{"open comment hides the rest of the body", "Before: x\n\n<!-- open\n\nAfter: y\n\n" + image + "\n", []string{noDanger, noAfter, beforeNoProof}},
		{"comment over both labels and their proof", "<!--\nBefore: x\n" + proofFence + "\nAfter: y\n" + proofFence + "\n-->\n", []string{noBefore, noAfter}},
		{"comment over before that closes before after", "<!--\nBefore: x\n-->\n\nAfter: y\n\n" + proofFence + "\n", []string{noBefore}},
		{"label in a one-line comment", "<!-- Before: x -->\n\nAfter: y\n\n" + proofFence + "\n", []string{noBefore}},
		{"comment opener in a fence hides nothing", "Before: x\n\n```\n<!--\n$ run\n```\n\nAfter: y\n\n" + proofFence + "\n", nil},
		{"image in a double backtick span", "Before: x ``" + image + "``\n\nAfter: y\n\n" + image + "\n", []string{beforeNoProof}},
		{"image after a backtick with no closing run", "Before: x `" + image + "\n\nAfter: y\n\n" + image + "\n", nil},
		{"image after a run of another length", "Before: x ``" + image + "`\n\nAfter: y\n\n" + image + "\n", nil},
		{"comment opener in inline code", "Before: x `<!--` " + image + "\n\nAfter: y\n\n" + image + "\n", nil},
		{"backtick in a comment", "Before: x <!-- ` --> " + image + "\n\nAfter: y\n\n" + image + "\n", nil},
		{"after above before", "After: y\n\n" + proofFence + "\n\nBefore: x\n\n" + proofFence + "\n", []string{"the After: line is above the Before: line. " + nextEvidence}},
		{"proof only under after", "Before: x\n\nAfter: y\n\n" + proofFence + "\n", []string{beforeNoProof}},
		{"proof only under before", "Before: x\n\n" + proofFence + "\n\nAfter: y\n", []string{afterNoProof}},
		{"no proof at all", "Before: x\n\nAfter: y\n", []string{beforeNoProof, afterNoProof}},
		{"no after line", "Before: x\n\n" + proofFence + "\n", []string{noAfter}},
		{"no before line", "After: y\n\n" + proofFence + "\n", []string{noBefore}},
		{"no label and no proof", "Nothing to show.\n", []string{noBefore, noAfter}},
		// An unclosed fence runs to the end of the body, and it hides the Merge danger heading.
		{"unclosed fence with content", "Before: x\n\n" + proofFence + "\n\nAfter: y\n\n```\n$ run\n", []string{noDanger}},
	}
	for _, tc := range cases {
		checkProblems(t, tc.name, withEvidence(tc.evidence), tc.want)
	}

	emptyTail := goodHead + "## Evidence\n\nBefore: x\n\n" + proofFence + "\n\nAfter: y\n\n```\n"
	checkProblems(t, "unclosed fence at the end with no content", emptyTail, []string{noDanger, afterNoProof})

	crlf := strings.ReplaceAll(withEvidence(goodEvidenc), "\n", "\r\n")
	checkProblems(t, "crlf body", crlf, nil)
	crlfBad := strings.ReplaceAll(withEvidence("Before: x\n\nAfter: y\n"), "\n", "\r\n")
	checkProblems(t, "crlf body with no proof", crlfBad, []string{beforeNoProof, afterNoProof})

	fenced := goodHead + "## Evidence\n\n" + "Before: x\n\n```\n## Merge danger\n" + goodDanger + "```\n\nAfter: y\n\n" + proofFence + "\n"
	checkProblems(t, "heading only in an evidence fence", fenced,
		[]string{"the body has no ## Merge danger line" + shape})

	old := "Bottom line: x.\n\n## Why\nBecause.\n\n## What\n- one\n\n## How\nShort.\n"
	checkProblems(t, "old shape", old, []string{
		"the body has no ## Evidence line" + shape,
		"the body has no ## Merge danger line" + shape,
	})
	misplaced := goodHead + "## Merge danger\n\n" + goodDanger + "\n## Evidence\n\n" + goodEvidenc
	checkProblems(t, "sections out of order", misplaced, []string{"## Merge danger is before ## Evidence" + shape})
}

func TestBodyMergeDanger(t *testing.T) {
	const radius = "- Blast radius: small. One command breaks.\n"
	const door = "- Door: two-way. Revert the merge.\n"
	badDoor := func(value string) string {
		return "the Door: line has the value " + value + ". start it with one-way or two-way"
	}
	badRadius := func(value string) string {
		return "the Blast radius: line has the value " + value + ". start it with small, medium or large"
	}
	noDoor := "## Merge danger has no Door: line. " + nextDanger
	noRadius := "## Merge danger has no Blast radius: line. " + nextDanger
	doorReason := "the Door: line gives no reason. say after the value how to reverse the merge, or why it cannot be reversed"
	radiusReason := "the Blast radius: line gives no reason. say after the value what breaks and for whom if the change is wrong"

	cases := []struct {
		name   string
		danger string
		want   []string
	}{
		{"one-way and small", "- Door: one-way. Data migrates.\n" + radius, nil},
		{"two-way and medium", door + "- Blast radius: medium. One service breaks.\n", nil},
		{"large", door + "- Blast radius: large. Every user breaks.\n", nil},
		{"value in another case", "- Door: Two-Way. Revert.\n- Blast radius: LARGE. All.\n", nil},
		{"value before a comma", "- Door: two-way, because a revert restores it.\n- Blast radius: small; one job.\n", nil},
		{"value before a colon", "- Door: one-way: data.\n" + radius, nil},
		{"bold labels", "- **Door:** two-way. Revert.\n- **Blast radius:** small. One job.\n", nil},
		{"star marker and no marker", "* Door: two-way. Revert.\nBlast radius: small. One job.\n", nil},
		{"labels in another case", "door: two-way. Revert.\nBLAST RADIUS: small. One job.\n", nil},
		{"value outside the list", "- Door: maybe. Unknown.\n" + radius, []string{badDoor(`"maybe"`)}},
		{"radius outside the list", door + "- Blast radius: huge. All.\n", []string{badRadius(`"huge"`)}},
		{"empty door value", "- Door:\n" + radius, []string{badDoor(`""`)}},
		{"empty radius value", door + "- Blast radius:   \n", []string{badRadius(`""`)}},
		{"wrong value gives no reason problem", "- Door: maybe\n" + radius, []string{badDoor(`"maybe"`)}},
		{"door value with no reason", "- Door: two-way\n" + radius, []string{doorReason}},
		{"door value with a dot and no reason", "- Door: two-way.\n" + radius, []string{doorReason}},
		{"radius value with no reason", door + "- Blast radius: small\n", []string{radiusReason}},
		{"both reasons missing", "- Door: one-way\n- Blast radius: large\n", []string{doorReason, radiusReason}},
		{"reason of one word", "- Door: two-way. Revert.\n- Blast radius: small. Nothing.\n", nil},
		{"reason of one digit", "- Door: two-way. 1\n" + radius, nil},
		{"door reason of a dot only", "- Door: two-way .\n" + radius, []string{doorReason}},
		{"radius reason of a dash only", door + "- Blast radius: small -\n", []string{radiusReason}},
		{"reason of marks only", "- Door: two-way. ... --\n" + radius, []string{doorReason}},
		{"value in backticks", "- Door: `two-way` Revert.\n- Blast radius: `small`. One job.\n", nil},
		{"dot inside the backticks", "- Door: `two-way.` Revert.\n- Blast radius: `small.` One job.\n", nil},
		{"dot after the backticks", "- Door: `two-way`. Revert.\n- Blast radius: `small`, one job.\n", nil},
		{"value in backticks with no reason", "- Door: `two-way`.\n" + radius, []string{doorReason}},
		{"wrong value in backticks", "- Door: `maybe`. Unknown.\n" + radius, []string{badDoor(`"maybe"`)}},
		{"no door line", radius, []string{noDoor}},
		{"no radius line", door, []string{noRadius}},
		{"no line at all", "Nothing here.\n", []string{noDoor, noRadius}},
		{"labels in a fence", "```\n" + door + radius + "```\n", []string{noDoor, noRadius}},
		{"duplicate label is plain text", door + "- Door: maybe. Other.\n" + radius, nil},
		{"label with a different word", "- Doors: two-way. Revert.\n" + radius, []string{noDoor}},
	}
	for _, tc := range cases {
		checkProblems(t, tc.name, withDanger(tc.danger), tc.want)
	}

	noSection := goodHead + "## Evidence\n\n" + goodEvidenc
	checkProblems(t, "no merge danger section", noSection, []string{"the body has no ## Merge danger line" + shape})

	hidden := noSection + "\n<!--\n## Merge danger\n\n" + goodDanger + "-->\n"
	checkProblems(t, "section inside a comment", hidden, []string{"the body has no ## Merge danger line" + shape})
	checkProblems(t, "door line inside a comment", withDanger("<!--\n"+door+"-->\n"+radius), []string{noDoor})
	checkProblems(t, "comment after the reason", withDanger("- Door: two-way. Revert. <!-- note -->\n"+radius), nil)
}

func TestBodyCommentsHideOldSections(t *testing.T) {
	body := "Bottom line: x.\n\n<!--\n## Why\n-->\nBecause.\n\n## What\n- one\n\n## How\nShort.\n\n" +
		"## Evidence\n\n" + goodEvidenc + "\n## Merge danger\n\n" + goodDanger
	checkProblems(t, "why in a comment", body, []string{"the body has no ## Why line" + shape})

	heading := goodHead + "## Evidence <!-- note -->\n\n" + goodEvidenc + "\n## Merge danger\n\n" + goodDanger
	checkProblems(t, "heading with a comment", heading, nil)
}

func TestBodyPlaceholders(t *testing.T) {
	skeleton := BodySkeleton([]string{"one"}, nil)
	kept := func(n int, text string) string {
		return fmt.Sprintf("line %d keeps the placeholder %s. %s", n, text, nextReplace)
	}
	want := []string{
		"the Before: line shows no proof. " + nextProof,
		"the After: line shows no proof. " + nextProof,
		kept(1, phBottom),
		kept(5, phWhy),
		kept(17, phBefore),
		kept(19, phProof),
		kept(21, phAfter),
		kept(23, phProof),
		kept(27, phDoor),
		kept(28, phBlast),
	}
	checkProblems(t, "unfilled skeleton", skeleton, want)

	fenced := strings.Replace(fillSkeleton(skeleton), "$ run\nok", phBefore+" "+phProof+"\n"+phDoor, 1)
	checkProblems(t, "placeholders in a fence", fenced, nil)

	hiddenFence := strings.Replace(fillSkeleton(skeleton), "Because it was missing.", "<!--\n```\n-->\n"+phWhy, 1)
	checkProblems(t, "fence marker in a comment", hiddenFence, []string{kept(8, phWhy)})

	checkProblems(t, "kept bottom line only", fillSkeleton(skeleton, phBottom), []string{kept(1, phBottom)})
	checkProblems(t, "kept door reason", fillSkeleton(skeleton, phDoorReason), []string{kept(33, phDoorReason)})
	checkProblems(t, "kept radius reason", fillSkeleton(skeleton, phBlastReason), []string{kept(34, phBlastReason)})
	checkProblems(t, "kept before", fillSkeleton(skeleton, phBefore), []string{kept(17, phBefore)})

	two := strings.Replace(fillSkeleton(skeleton), "Because it was missing.", "Because "+phWhy+" and "+phBottom+".", 1)
	checkProblems(t, "two placeholders on one line", two, []string{kept(5, phWhy)})

	const dash = string(emDashRune)
	long := strings.TrimSpace(strings.Repeat("word ", 41))
	order := strings.Replace(fillSkeleton(skeleton), "Because it was missing.", "A "+dash+" "+phWhy+" "+long, 1)
	checkProblems(t, "dash, placeholder and words on one line", order, []string{
		"line 5 has an em dash. rewrite the line with commas, colons or sentences",
		kept(5, phWhy),
		`line 5 has 52 words, more than 40 words: "A ` + dash + ` <why: the problem and". split the line into lines of 40 words or fewer`,
	})
}

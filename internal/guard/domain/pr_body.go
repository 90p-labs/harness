package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

const (
	sectionWhy         = "## Why"
	sectionWhat        = "## What"
	sectionHow         = "## How"
	sectionEvidence    = "## Evidence"
	sectionMergeDanger = "## Merge danger"

	labelBefore = "Before:"
	labelAfter  = "After:"
	labelDoor   = "Door:"
	labelBlast  = "Blast radius:"

	placeholderBottomLine  = "<bottom line: what the PR changes and what the reviewer must check>"
	placeholderWhy         = "<why: the problem and the reason for the change>"
	placeholderBefore      = "<what the behaviour was, in one line>"
	placeholderAfter       = "<what the behaviour is now, in one line>"
	placeholderProof       = "<proof: a screenshot, or a fence with the run command and its output>"
	placeholderDoorReason  = "<how to reverse the merge, or why it cannot be reversed>"
	placeholderBlastReason = "<what breaks and for whom if the change is wrong>"
	noDiagramLine          = "The change adds no diagram."
	valueTrimSet           = ".,;:"
	commentOpen            = "<!--"
	commentClose           = "-->"
	tickMark               = "`"

	nextStepProof       = "put a screenshot by its http or https URL, or a fence with the run command and its output, below the line"
	nextStepUpload      = "upload the image and link its http or https URL, or put a fence with the command and its output"
	nextStepReplace     = "replace it with your text"
	nextStepDoorReason  = "say after the value how to reverse the merge, or why it cannot be reversed"
	nextStepBlastReason = "say after the value what breaks and for whom if the change is wrong"
)

var (
	bodySections      = []string{sectionWhy, sectionWhat, sectionHow, sectionEvidence, sectionMergeDanger}
	bodyLabels        = []string{labelBefore, labelAfter, labelDoor, labelBlast}
	doorValues        = []string{"one-way", "two-way"}
	blastRadiusValues = []string{"small", "medium", "large"}

	nextStepShape    = "write the body with the lines " + joinWith(bodySections, " and ") + ", in this order"
	nextStepEvidence = fmt.Sprintf("write a %s line and an %s line under %s, in this order", labelBefore, labelAfter, sectionEvidence)
	nextStepDanger   = fmt.Sprintf("write a %s line and a %s line under %s", labelDoor, labelBlast, sectionMergeDanger)

	keptPlaceholders = []string{
		placeholderBottomLine, placeholderWhy, placeholderBefore, placeholderAfter, placeholderProof,
		valuePlaceholder(doorValues), placeholderDoorReason, valuePlaceholder(blastRadiusValues), placeholderBlastReason,
	}

	boldMarks      = strings.NewReplacer("**", "", "__", "")
	sectionHeading = regexp.MustCompile(`^##(\s|$)`)
	imageProof     = regexp.MustCompile(`!\[[^\]]*\]\(\s*([^\s)][^)]*)\)`)
	// imgTagOpen matches an img tag name in any case, then one whitespace character or a slash that starts the attributes.
	// The match keeps the byte offsets of the original text, which a lower case copy would not.
	imgTagOpen = regexp.MustCompile(`(?i)<img[\s/]`)
)

// BodySections returns a copy of the section lines of the body, in order.
func BodySections() []string { return slices.Clone(bodySections) }

// BodyLabels returns the labels of the body, in order: Before:, After:, Door: and Blast radius:.
func BodyLabels() []string { return slices.Clone(bodyLabels) }

// DoorValues returns a copy of the values of the Door line.
func DoorValues() []string { return slices.Clone(doorValues) }

// BlastRadiusValues returns a copy of the values of the Blast radius line.
func BlastRadiusValues() []string { return slices.Clone(blastRadiusValues) }

// BodySkeleton returns the body with each section and its placeholders.
// Each commit is one What bullet. Each diagram is one fence that ends in a newline.
func BodySkeleton(commits, diagrams []string) string {
	var b strings.Builder
	b.WriteString(placeholderBottomLine + "\n\n")
	b.WriteString(sectionWhy + "\n\n" + placeholderWhy + "\n\n")
	b.WriteString(sectionWhat + "\n\n")
	for _, subject := range commits {
		b.WriteString("- " + subject + "\n")
	}
	b.WriteString("\n" + sectionHow + "\n\n")
	if len(diagrams) == 0 {
		b.WriteString(noDiagramLine + "\n")
	}
	b.WriteString(strings.Join(diagrams, "\n"))
	b.WriteString("\n" + sectionEvidence + "\n\n")
	b.WriteString(labelBefore + " " + placeholderBefore + "\n\n" + placeholderProof + "\n\n")
	b.WriteString(labelAfter + " " + placeholderAfter + "\n\n" + placeholderProof + "\n\n")
	b.WriteString(sectionMergeDanger + "\n\n")
	b.WriteString("- " + labelDoor + " " + valuePlaceholder(doorValues) + ". " + placeholderDoorReason + "\n")
	b.WriteString("- " + labelBlast + " " + valuePlaceholder(blastRadiusValues) + ". " + placeholderBlastReason + "\n")
	return b.String()
}

func valuePlaceholder(values []string) string { return "<" + joinWith(values, " or ") + ">" }

// joinWith joins items with commas and the word before the last one.
func joinWith(items []string, conjunction string) string {
	last := len(items) - 1
	if last == 0 {
		return items[0]
	}
	return strings.Join(items[:last], ", ") + conjunction + items[last]
}

// bodyLine is one line of the body, trimmed. The field fence is true for a fence marker, and in for a line inside a fence.
type bodyLine struct {
	text  string
	fence bool
	in    bool
}

func (l bodyLine) outside() bool { return !l.fence && !l.in }

// scanBody reads the body once for every check.
// A fence line and a line in a fence keep their trimmed text. Another line keeps its text without HTML comments.
// A line that starts under an open comment is no fence line.
func scanBody(lines []string) []bodyLine {
	scanned := make([]bodyLine, len(lines))
	inFence, inComment := false, false
	for i, line := range lines {
		text := strings.TrimSpace(line)
		marker := strings.HasPrefix(text, fenceMarker)
		switch {
		case !inComment && marker:
			scanned[i] = bodyLine{text: text, fence: true}
			inFence = !inFence
		case !inComment && inFence:
			scanned[i] = bodyLine{text: text, in: true}
		default:
			text, inComment = dropComments(text, inComment)
			scanned[i] = bodyLine{text: strings.TrimSpace(text)}
		}
	}
	return scanned
}

// sectionLines returns the lines under the heading, up to the next level-2 heading outside a fence.
func sectionLines(lines []bodyLine, heading string) ([]bodyLine, bool) {
	start := slices.IndexFunc(lines, func(l bodyLine) bool { return l.outside() && l.text == heading })
	if start < 0 {
		return nil, false
	}
	rest := lines[start+1:]
	end := slices.IndexFunc(rest, func(l bodyLine) bool { return l.outside() && sectionHeading.MatchString(l.text) })
	if end < 0 {
		end = len(rest)
	}
	return rest[:end], true
}

// labelText returns the line without its list marker and each ** and __ mark, when it starts with the label in any case.
func labelText(l bodyLine, label string) (string, bool) {
	if !l.outside() {
		return "", false
	}
	text := l.text
	if rest, found := strings.CutPrefix(text, "- "); found {
		text = rest
	} else if rest, found := strings.CutPrefix(text, "* "); found {
		text = rest
	}
	text = strings.TrimSpace(boldMarks.Replace(text))
	if len(text) < len(label) || !strings.EqualFold(text[:len(label)], label) {
		return "", false
	}
	return text[len(label):], true
}

func labelIndex(lines []bodyLine, label string) int {
	return slices.IndexFunc(lines, func(l bodyLine) bool {
		_, found := labelText(l, label)
		return found
	})
}

func firstPlaceholder(line string) (string, bool) {
	best, at := "", -1
	for _, p := range keptPlaceholders {
		if i := strings.Index(line, p); i >= 0 && (at < 0 || i < at) {
			best, at = p, i
		}
	}
	return best, at >= 0
}

func placeholderProblem(i int, line string) []string {
	p, kept := firstPlaceholder(line)
	if !kept {
		return nil
	}
	return []string{problem(fmt.Sprintf("line %d keeps the placeholder %s", i+1, p), nextStepReplace)}
}

func evidenceProblems(lines []bodyLine) []string {
	section, found := sectionLines(lines, sectionEvidence)
	if !found {
		return nil
	}
	before, after := labelIndex(section, labelBefore), labelIndex(section, labelAfter)
	var problems []string
	if before < 0 {
		problems = append(problems, problem(fmt.Sprintf("%s has no %s line", sectionEvidence, labelBefore), nextStepEvidence))
	}
	if after < 0 {
		problems = append(problems, problem(fmt.Sprintf("%s has no %s line", sectionEvidence, labelAfter), nextStepEvidence))
	}
	if before >= 0 && after >= 0 && after < before {
		problems = append(problems, problem(fmt.Sprintf("the %s line is above the %s line", labelAfter, labelBefore), nextStepEvidence))
	}
	problems = append(problems, proofProblems(section, before, after, labelBefore)...)
	return append(problems, proofProblems(section, after, before, labelAfter)...)
}

// proofProblems checks the proof of the label at own. It runs to the other label when that one is below, else to the end.
func proofProblems(section []bodyLine, own, other int, label string) []string {
	if own < 0 {
		return nil
	}
	end := len(section)
	if other > own {
		end = other
	}
	switch hasProof(section[own:end]) {
	case proofShown:
		return nil
	case proofLocalImage:
		return []string{problem(fmt.Sprintf("the %s line shows an image with no http or https URL", label), nextStepUpload)}
	}
	return []string{problem(fmt.Sprintf("the %s line shows no proof", label), nextStepProof)}
}

// dropComments drops each HTML comment from a line and keeps each inline code span as it is.
// inComment is true when a comment from an earlier line is still open. The result says if one is open at the line end.
// A comment opener inside an inline code span opens no comment.
func dropComments(text string, inComment bool) (string, bool) {
	var out strings.Builder
	for text != "" {
		if inComment {
			_, rest, closed := strings.Cut(text, commentClose)
			if !closed {
				return out.String(), true
			}
			text, inComment = rest, false
			continue
		}
		i := firstOpener(text)
		if i < 0 {
			out.WriteString(text)
			break
		}
		if strings.HasPrefix(text[i:], commentOpen) {
			out.WriteString(text[:i])
			text, inComment = text[i+len(commentOpen):], true
			continue
		}
		end := spanEnd(text, i)
		out.WriteString(text[:end])
		text = text[end:]
	}
	return out.String(), inComment
}

// spanEnd returns the end index of the code span that opens at i, or the end of the run when no run closes it.
func spanEnd(text string, i int) int {
	n := tickRun(text[i:])
	if closed := closingRun(text[i+n:], n); closed >= 0 {
		return i + n + closed
	}
	return i + n
}

// dropSpans drops each inline code span from a line.
// A backtick run with no closing run of the same length stays as plain text.
func dropSpans(text string) string {
	var out strings.Builder
	for text != "" {
		i := strings.Index(text, tickMark)
		if i < 0 {
			out.WriteString(text)
			break
		}
		n := tickRun(text[i:])
		closed := closingRun(text[i+n:], n)
		if closed < 0 {
			out.WriteString(text[:i+n])
			text = text[i+n:]
			continue
		}
		out.WriteString(text[:i])
		text = text[i+n+closed:]
	}
	return out.String()
}

// firstOpener returns the index of the first comment opener or backtick, or -1.
func firstOpener(text string) int {
	comment, tick := strings.Index(text, commentOpen), strings.Index(text, tickMark)
	if comment < 0 || (tick >= 0 && tick < comment) {
		return tick
	}
	return comment
}

// tickRun returns the length of the run of backticks at the start of the text.
func tickRun(text string) int { return len(text) - len(strings.TrimLeft(text, tickMark)) }

// closingRun returns the end index of the first run of exactly n backticks in the text, or -1.
func closingRun(text string, n int) int {
	for i := 0; i < len(text); {
		if text[i] != tickMark[0] {
			i++
			continue
		}
		run := tickRun(text[i:])
		if run == n {
			return i + run
		}
		i += run
	}
	return -1
}

// proofState is what the window of a label shows: nothing, only an image with no web URL, or proof.
type proofState int

const (
	proofNone proofState = iota
	proofLocalImage
	proofShown
)

// imageTargets returns the target of each image of the line, without blank ones.
// It reads the line after dropSpans, so an image in a code span is no image.
func imageTargets(text string) []string {
	text = dropSpans(text)
	var targets []string
	for _, m := range imageProof.FindAllStringSubmatch(text, -1) {
		targets = append(targets, m[1])
	}
	for rest := text; ; {
		src, after, found := nextTagSrc(rest)
		if !found {
			break
		}
		targets = append(targets, src)
		rest = after
	}
	var clean []string
	for _, target := range targets {
		if target = cleanTarget(target); target != "" {
			clean = append(clean, target)
		}
	}
	return clean
}

// cleanTarget trims the target, cuts its title at the first space outside angle brackets, then drops the angle brackets.
func cleanTarget(target string) string {
	target = strings.TrimSpace(target)
	inAngle := false
cut:
	for i, r := range target {
		switch {
		case r == '<':
			inAngle = true
		case r == '>':
			inAngle = false
		case !inAngle && unicode.IsSpace(r):
			target = target[:i]
			break cut
		}
	}
	if inner, ok := strings.CutPrefix(target, "<"); ok {
		// An open angle bracket with no closing one is no link in Markdown, so the target keeps it and is no web target.
		if inner, ok = strings.CutSuffix(inner, ">"); !ok {
			return target
		}
		target = inner
	}
	return strings.TrimSpace(target)
}

// nextTagSrc finds the next img tag in the text and returns the value of its first src attribute.
// A src text inside another attribute value is no attribute. The rest starts after the tag.
func nextTagSrc(text string) (src, rest string, found bool) {
	loc := imgTagOpen.FindStringIndex(text)
	if loc == nil {
		return "", "", false
	}
	src, rest = tagSrc(text[loc[1]-1:])
	return src, rest, true
}

// tagSrc reads the attributes of a tag up to its closing angle bracket outside quotes.
func tagSrc(text string) (src, rest string) {
	seen := false
	for {
		text = strings.TrimLeft(text, " \t\r\n/")
		if text == "" || text[0] == '>' {
			return src, strings.TrimPrefix(text, ">")
		}
		end := strings.IndexAny(text, " \t\r\n=/>")
		if end < 0 {
			return src, ""
		}
		name := text[:end]
		text = strings.TrimLeft(text[end:], " \t\r\n")
		if text == "" || text[0] != '=' {
			continue
		}
		var value string
		value, text = attrValue(strings.TrimLeft(text[1:], " \t\r\n"))
		if !seen && strings.EqualFold(name, "src") {
			src, seen = value, true
		}
	}
}

// attrValue reads a quoted or unquoted attribute value and returns it with the text after it.
func attrValue(text string) (value, rest string) {
	if text != "" && (text[0] == '"' || text[0] == '\'') {
		var closed bool
		value, rest, closed = strings.Cut(text[1:], text[:1])
		if !closed {
			return value, ""
		}
		return value, rest
	}
	end := strings.IndexAny(text, " \t\r\n>")
	if end < 0 {
		return text, ""
	}
	return text[:end], text[end:]
}

// isWebTarget is true for an http:// or https:// target in any case, with text after the scheme.
func isWebTarget(target string) bool {
	lower := strings.ToLower(target)
	for _, scheme := range []string{"http://", "https://"} {
		if len(lower) > len(scheme) && strings.HasPrefix(lower, scheme) {
			return true
		}
	}
	return false
}

// hasProof reads the window of a label. A fence with content or an image with a web URL is proof.
// The scan dropped each comment already. It reads every image before it decides.
func hasProof(lines []bodyLine) proofState {
	inFence, content, local := false, false, false
	for _, l := range lines {
		switch {
		case l.fence && inFence:
			if content {
				return proofShown
			}
			inFence = false
		case l.fence:
			inFence, content = true, false
		case l.in:
			content = content || l.text != ""
		default:
			for _, target := range imageTargets(l.text) {
				if isWebTarget(target) {
					return proofShown
				}
				local = true
			}
		}
	}
	switch {
	case inFence && content:
		return proofShown
	case local:
		return proofLocalImage
	}
	return proofNone
}

// valueLine describes a merge danger line: its label, the values it accepts and the next step for a missing reason.
type valueLine struct {
	label      string
	values     []string
	reasonStep string
}

func mergeDangerProblems(lines []bodyLine) []string {
	section, found := sectionLines(lines, sectionMergeDanger)
	if !found {
		return nil
	}
	var problems []string
	for _, v := range []valueLine{
		{labelDoor, doorValues, nextStepDoorReason},
		{labelBlast, blastRadiusValues, nextStepBlastReason},
	} {
		problems = append(problems, valueProblems(section, v)...)
	}
	return problems
}

func valueProblems(section []bodyLine, v valueLine) []string {
	at := labelIndex(section, v.label)
	if at < 0 {
		return []string{problem(fmt.Sprintf("%s has no %s line", sectionMergeDanger, v.label), nextStepDanger)}
	}
	if _, kept := firstPlaceholder(section[at].text); kept {
		return nil
	}
	rest, _ := labelText(section[at], v.label)
	fields := strings.Fields(rest)
	value, reason := "", ""
	if len(fields) > 0 {
		value, reason = cleanValue(fields[0]), strings.Join(fields[1:], " ")
	}
	if !slices.ContainsFunc(v.values, func(s string) bool { return strings.EqualFold(s, value) }) {
		why := fmt.Sprintf("the %s line has the value %q", v.label, value)
		return []string{problem(why, "start it with "+joinWith(v.values, " or "))}
	}
	if !strings.ContainsFunc(reason, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return []string{problem(fmt.Sprintf("the %s line gives no reason", v.label), v.reasonStep)}
	}
	return nil
}

// cleanValue drops a backtick at each end of the word, and a trailing mark before or after the closing backtick.
func cleanValue(word string) string {
	word = strings.TrimRight(word, valueTrimSet)
	word = strings.TrimSuffix(strings.TrimPrefix(word, tickMark), tickMark)
	return strings.TrimRight(word, valueTrimSet)
}

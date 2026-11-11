package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"regexp"
	"slices"
	"strings"

	shared "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	amendBacktickMark    = "`"
	amendHeadingMark     = "#"
	amendHeadingPrefix   = "### "
	amendSectionPrefix   = "## "
	amendOwnedPrefix     = "- " + FieldOwned + ":"
	amendTestFirstPrefix = "- " + FieldTestFirst + ":"
	amendDoDPrefix       = "- " + FieldDoD + ":"
	amendLineBreak       = "\n"
	amendListJoin        = ", "
	amendGoSuffix        = "_test.go"
	amendPyPrefix        = "test_"
	amendPyExt           = ".py"
	amendPySuffix        = "_test.py"
	amendTestDataDir     = "testdata"
	amendParentDir       = ".."
	amendTestMarker      = ".test"
	amendSpecMarker      = ".spec"
)

var (
	amendCodeExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
	amendDataExts = []string{".md", ".json", ".jsonl", ".txt", ".yaml", ".yml", ".golden"}
	amendBacktick = regexp.MustCompile("`([^`]*)`")
	amendDigits   = regexp.MustCompile(`\d+`)
)

func isTestPath(p string) bool {
	if path.Clean(p) != p || slices.Contains(strings.Split(p, "/"), amendParentDir) {
		return false
	}
	dir, name := path.Split(p)
	ext := path.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	switch {
	case strings.HasSuffix(name, amendGoSuffix):
		return true
	case ext == amendPyExt && (strings.HasPrefix(name, amendPyPrefix) || strings.HasSuffix(name, amendPySuffix)):
		return true
	case slices.Contains(amendCodeExts, ext):
		return strings.HasSuffix(stem, amendTestMarker) || strings.HasSuffix(stem, amendSpecMarker)
	case slices.Contains(amendDataExts, ext):
		return slices.Contains(strings.Split(dir, "/"), amendTestDataDir)
	}
	return false
}

func StructureKeptAmend(approvedText, approvedHash, current string) bool {
	sum := sha256.Sum256([]byte(approvedText))
	return hex.EncodeToString(sum[:]) == approvedHash && structureKeptChange(approvedText, current)
}

func structureKeptChange(approved, current string) bool {
	if approved == "" || current == "" {
		return false
	}
	want, ok := amendSkeleton(approved)
	if !ok {
		return false
	}
	got, ok := amendSkeleton(current)
	if !ok || want != got {
		return false
	}
	wantDoc, _ := Parse(approved)
	gotDoc, _ := Parse(current)
	return amendParsedKept(wantDoc, gotDoc)
}

func amendParsedKept(want, got Document) bool {
	if len(want.Tasks) != len(got.Tasks) || len(want.Sketches) != len(got.Sketches) {
		return false
	}
	if !slices.EqualFunc(want.Diagrams, got.Diagrams, func(w, g Diagram) bool { return w.Kind == g.Kind && w.Body == g.Body }) {
		return false
	}
	for i, w := range want.Tasks {
		g := got.Tasks[i]
		wantFile, wantName := testOf(want, w)
		gotFile, gotName := testOf(got, g)
		if w.Number != g.Number || !slices.Equal(w.DoD, g.DoD) || wantFile != gotFile || wantName != gotName {
			return false
		}
	}
	for i, w := range want.Sketches {
		if g := got.Sketches[i]; w.Title != g.Title || w.Path != g.Path {
			return false
		}
	}
	return true
}

func amendSkeleton(text string) (string, bool) {
	var kept []string
	var fenced, treeHeading, treeBody bool
	var section string
	for _, raw := range strings.Split(text, amendLineBreak) {
		line := strings.TrimRight(raw, trailingSpace)
		trimmed := strings.TrimSpace(line)
		// Fence lines use the right-trimmed line, as Parse does. The case gates for a fence prefix and a heading use trimmed. Heading text and prefix use the right-trimmed line.
		switch {
		case fenced && shared.IsFenceClose(line):
			fenced, treeBody = false, false
			kept = append(kept, line)
		case fenced:
			if !treeBody {
				kept = append(kept, line)
			}
		case shared.IsFenceOpen(line):
			// Only a fence the parser reads as a tree may lose its body.
			isTree := strings.TrimSpace(strings.TrimPrefix(line, fenceMark)) == FenceTree
			fenced, treeBody, treeHeading = true, treeHeading && isTree, false
			kept = append(kept, line)
		case strings.HasPrefix(trimmed, fenceMark):
			kept = append(kept, line)
		case strings.HasPrefix(line, amendOwnedPrefix):
			treeHeading = false
			kept = append(kept, amendOwnedWithoutTests(line))
		case strings.HasPrefix(trimmed, amendHeadingMark):
			// Heading text is compared untrimmed on the left, as Parse does. The line is already right-trimmed.
			heading := strings.TrimPrefix(line, amendHeadingPrefix)
			treeHeading = strings.HasPrefix(line, amendHeadingPrefix) && (heading == HeadingCodeTree || heading == HeadingTestTree)
			if name, ok := strings.CutPrefix(line, amendSectionPrefix); ok {
				section = name
			}
			kept = append(kept, line)
		default:
			if free, keep := amendFreeText(section, line); keep {
				kept = append(kept, free)
			}
		}
	}
	if fenced {
		return "", false
	}
	return strings.Join(kept, amendLineBreak), true
}

func amendFreeText(section, line string) (string, bool) {
	switch section {
	case HeadingGoal:
		return line, strings.HasPrefix(line, TrackPrefix)
	case HeadingDecisions:
		return "", false
	case HeadingTasks:
		switch {
		case strings.HasPrefix(line, amendTestFirstPrefix):
			return amendTestFirstNames(line), true
		case strings.HasPrefix(line, amendDoDPrefix):
			return amendDoDNumbers(line), true
		}
	}
	return line, true
}

func amendTestFirstNames(line string) string {
	var names []string
	for _, match := range amendBacktick.FindAllStringSubmatch(line, -1) {
		names = append(names, amendBacktickMark+match[1]+amendBacktickMark)
	}
	return amendTestFirstPrefix + " " + strings.Join(names, amendListJoin)
}

func amendDoDNumbers(line string) string {
	return amendDoDPrefix + " " + strings.Join(amendDigits.FindAllString(line, -1), amendListJoin)
}

func amendOwnedWithoutTests(line string) string {
	rest := strings.TrimPrefix(line, amendOwnedPrefix)
	var sources []string
	for _, match := range amendBacktick.FindAllStringSubmatch(rest, -1) {
		if p := strings.TrimSpace(match[1]); !isTestPath(p) {
			sources = append(sources, amendBacktickMark+p+amendBacktickMark)
		}
	}
	residue := strings.Trim(amendBacktick.ReplaceAllString(rest, ""), " ,\t\r")
	return amendOwnedPrefix + " " + strings.Join(sources, amendListJoin) + residue
}

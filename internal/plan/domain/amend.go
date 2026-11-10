package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"regexp"
	"slices"
	"strings"
)

const (
	amendHeadingMark   = "#"
	amendHeadingPrefix = "### "
	amendOwnedPrefix   = "- " + FieldOwned + ":"
	amendLineBreak     = "\n"
	amendPathJoin      = ", "
	amendGoSuffix      = "_test.go"
	amendPyPrefix      = "test_"
	amendPyExt         = ".py"
	amendPySuffix      = "_test.py"
	amendTestDataDir   = "testdata"
	amendParentDir     = ".."
	amendTestMarker    = ".test"
	amendSpecMarker    = ".spec"
)

var (
	amendCodeExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
	amendDataExts = []string{".md", ".json", ".jsonl", ".txt", ".yaml", ".yml", ".golden"}
	amendBacktick = regexp.MustCompile("`([^`]*)`")
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

func TreeOnlyAmend(approvedText, approvedHash, current string) bool {
	sum := sha256.Sum256([]byte(approvedText))
	return hex.EncodeToString(sum[:]) == approvedHash && treeOnlyChange(approvedText, current)
}

func treeOnlyChange(approved, current string) bool {
	if approved == "" || current == "" {
		return false
	}
	want, ok := amendSkeleton(approved)
	if !ok {
		return false
	}
	got, ok := amendSkeleton(current)
	return ok && want == got
}

func amendSkeleton(text string) (string, bool) {
	var kept []string
	var otherFence, treeHeading, treeBody bool
	for _, line := range strings.Split(text, amendLineBreak) {
		trimmed := strings.TrimSpace(line)
		isFence := strings.HasPrefix(trimmed, fenceMark)
		switch {
		case treeBody:
			if isFence {
				treeBody = false
				kept = append(kept, line)
			}
		case otherFence:
			otherFence = otherFence && !isFence
			kept = append(kept, line)
		case isFence && treeHeading:
			treeHeading, treeBody = false, true
			kept = append(kept, line)
		case isFence:
			otherFence = true
			kept = append(kept, line)
		case strings.HasPrefix(line, amendOwnedPrefix):
			treeHeading = false
			kept = append(kept, amendOwnedWithoutTests(line))
		case strings.HasPrefix(line, amendHeadingMark):
			heading := strings.TrimSpace(strings.TrimPrefix(line, amendHeadingPrefix))
			treeHeading = strings.HasPrefix(line, amendHeadingPrefix) && (heading == HeadingCodeTree || heading == HeadingTestTree)
			kept = append(kept, line)
		default:
			kept = append(kept, line)
		}
	}
	if treeBody || otherFence {
		return "", false
	}
	return strings.Join(kept, amendLineBreak), true
}

func amendOwnedWithoutTests(line string) string {
	rest := strings.TrimPrefix(line, amendOwnedPrefix)
	var sources []string
	for _, match := range amendBacktick.FindAllStringSubmatch(rest, -1) {
		if p := strings.TrimSpace(match[1]); !isTestPath(p) {
			sources = append(sources, "`"+p+"`")
		}
	}
	residue := strings.Trim(amendBacktick.ReplaceAllString(rest, ""), " ,\t\r")
	return amendOwnedPrefix + " " + strings.Join(sources, amendPathJoin) + residue
}

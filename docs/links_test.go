package docs_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	markdownExt  = ".md"
	anchorMark   = "#"
	schemeMark   = ":"
	brokenFile   = "missing.md"
	brokenAnchor = "#no-such-heading"
	goodAnchor   = "#real-heading"
	goodSuffixed = "#real-heading-1"
	brokenSuffix = "#real-heading-2"
)

var (
	linkPattern    = regexp.MustCompile(`!?\[[^\]]*\]\(([^)\s]*)(?:\s+"[^"]*")?\)`)
	headingPattern = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*$`)
	slugDrop       = regexp.MustCompile(`[^\p{L}\p{N}\s_-]`)
	linkPages      = []string{readmeFile, "docs/*" + markdownExt, "examples/README.md", "examples/*/README.md"}
)

func outsideFences(text string) []string {
	var lines []string
	fenced := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), fencePrefix) {
			fenced = !fenced
			continue
		}
		if !fenced {
			lines = append(lines, line)
		}
	}
	return lines
}

func headingSlugs(text string) map[string]bool {
	slugs := map[string]bool{}
	seen := map[string]int{}
	for _, line := range outsideFences(text) {
		m := headingPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		slug := strings.ReplaceAll(slugDrop.ReplaceAllString(strings.ToLower(m[1]), ""), " ", "-")
		if n := seen[slug]; n > 0 {
			slugs[slug+"-"+strconv.Itoa(n)] = true
		} else {
			slugs[slug] = true
		}
		seen[slug]++
	}
	return slugs
}

func hasScheme(target string) bool {
	head, _, _ := strings.Cut(target, "/")
	head, _, _ = strings.Cut(head, anchorMark)
	return strings.Contains(head, schemeMark)
}

func anchorResolves(dest, anchor string) bool {
	content, err := os.ReadFile(dest)
	return err == nil && headingSlugs(string(content))[anchor]
}

func brokenLinks(t *testing.T, page string) []string {
	t.Helper()
	pagePath := repository.Path(t, page)
	raw, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	var broken []string
	for _, line := range outsideFences(string(raw)) {
		for _, m := range linkPattern.FindAllStringSubmatch(line, -1) {
			target := m[1]
			if hasScheme(target) {
				continue
			}
			file, anchor, _ := strings.Cut(target, anchorMark)
			dest := pagePath
			if file != "" {
				dest = filepath.Join(filepath.Dir(pagePath), file)
			}
			_, statErr := os.Stat(dest)
			needsAnchor := anchor != "" && filepath.Ext(dest) == markdownExt
			if statErr != nil || (needsAnchor && !anchorResolves(dest, anchor)) {
				broken = append(broken, page+": "+target)
			}
		}
	}
	return broken
}

func TestRelativeLinksResolve(t *testing.T) {
	for _, pattern := range linkPages {
		pages, err := repository.Glob(t, pattern)
		if err != nil || len(pages) == 0 {
			t.Fatalf("no page matches %s", pattern)
		}
		for _, page := range pages {
			for _, link := range brokenLinks(t, page) {
				t.Errorf("broken link %s", link)
			}
		}
	}
}

func TestBrokenLinks(t *testing.T) {
	page := filepath.Join(t.TempDir(), "page"+markdownExt)
	body := "# Page\n\n## Real heading\n\n## Real heading\n\n[a](" + brokenFile + ")\n[b](" + brokenAnchor + ")\n[c](" + goodAnchor + ")\n[f](" + goodSuffixed + ")\n[g](" + brokenSuffix + ")\n[d](https://example.com/x)\n\n```md\n[e](skipped.md)\n```\n"
	if err := os.WriteFile(page, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got := brokenLinks(t, page)
	want := []string{page + ": " + brokenFile, page + ": " + brokenAnchor, page + ": " + brokenSuffix}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("broken links = %q, want %q", got, want)
	}
}

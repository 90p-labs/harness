package docs_test

import (
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	panelGuideFile = "docs/panel.md"
	panelGuideLink = "](docs/panel.md)"
	documentsHead  = "## Documents"
	licenseHead    = "## License"
)

// panelGuideTerms are the commands, output lines and state files that the
// guide must name, so the guide follows the code.
var panelGuideTerms = []string{
	"harness panel start",
	"harness panel stop",
	"harness panel status",
	"panel: stopped",
	"panel: <url>",
	"pid <n>",
	"--port",
	"panel.json",
	"panel.lock",
	"panel.log",
	"0600",
	"127.0.0.1",
	"panel.enabled",
	"panel.autostart",
	"panel.open",
	"panel: <url>repos/<key>/runs/<id>",
	"plan view --open",
}

// panelUserStateTerms are the user-wide terms that the guide must name: the
// user state folder, the registry and the repo routes.
var panelUserStateTerms = []string{
	"~/.harness/",
	"HARNESS_HOME",
	"repos.json",
	"repos.lock",
	"0700",
	"/repos/<key>/runs/<id>",
	"panel: <url>repos/<key>/runs/<id>",
	"?repo=",
	"Done keeps the 20 most recent cards of each repo",
}

// panelCommonDirState is the old state folder, which the guide must not name.
const panelCommonDirState = "<git common dir>/.harness/"

// panelOneLanePhrase is the old board layout, which the guide and ADR 0027 must not name.
const panelOneLanePhrase = "one lane for each repo"

// panelOneLaneDocs are the docs that must describe the one board, not one lane per repo.
var panelOneLaneDocs = []string{
	panelGuideFile,
	"docs/adr/0027-one-panel-per-user.md",
}

func TestReadmeLinksPanelGuide(t *testing.T) {
	readme := repository.Read(t, readmeFile)
	_, afterDocs, found := strings.Cut(readme, "\n"+documentsHead+"\n")
	if !found {
		t.Fatalf("%s has no %s section", readmeFile, documentsHead)
	}
	section, _, _ := strings.Cut(afterDocs, "\n"+licenseHead)
	var links []string
	for _, line := range strings.Split(section, "\n") {
		if strings.Contains(line, panelGuideLink) {
			links = append(links, line)
		}
	}
	if len(links) != 1 {
		t.Fatalf("%s: want one link to %s in %s, got %d", readmeFile, panelGuideFile, documentsHead, len(links))
	}
}

func TestPanelGuideNamesTheLifecycle(t *testing.T) {
	guide := repository.Read(t, panelGuideFile)
	for _, term := range panelGuideTerms {
		if !strings.Contains(guide, term) {
			t.Errorf("%s does not name %q", panelGuideFile, term)
		}
	}
}

func TestPanelGuideNamesTheUserState(t *testing.T) {
	guide := repository.Read(t, panelGuideFile)
	for _, term := range panelUserStateTerms {
		if !strings.Contains(guide, term) {
			t.Errorf("%s does not name %q", panelGuideFile, term)
		}
	}
	if strings.Contains(guide, panelCommonDirState) {
		t.Errorf("%s still names the common dir %q", panelGuideFile, panelCommonDirState)
	}
	for _, file := range panelOneLaneDocs {
		if strings.Contains(repository.Read(t, file), panelOneLanePhrase) {
			t.Errorf("%s still says %q", file, panelOneLanePhrase)
		}
	}
}

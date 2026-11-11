package docs_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	skillDocName     = "SKILL.md"
	skillDocs        = "skills/*/" + skillDocName
	agentDocs        = "agents/*.md"
	readmeSkillsLine = "- The skills "
	readmeAgentsLine = "- The agents "
	installHeading   = "\n## Install\n"
	subHeading       = "### "
)

var (
	installPaths   = []string{"Claude Code", "Codex", "The harness command"}
	planViewImages = []string{"plan-view-summary.png", "plan-view-specs.png", "plan-view-tasks.png"}
)

func TestReadmeInstallOrder(t *testing.T) {
	raw, err := os.ReadFile(repository.Path(t, readmeFile))
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(raw), installHeading)
	if !found {
		t.Fatalf("%s has no Install section", readmeFile)
	}
	section, _, _ := strings.Cut(rest, nextHeading)
	var got []string
	for _, line := range strings.Split(section, "\n") {
		if title, ok := strings.CutPrefix(line, subHeading); ok {
			got = append(got, title)
		}
	}
	if !slices.Equal(got, installPaths) {
		t.Errorf("Install subsections = %q, want %q", got, installPaths)
	}
}

const (
	readmeMaxLines  = 170
	headingLine     = "## "
	firstRunHeading = "\n### Your first run\n"
	readmeImagesDir = "docs/images/"
	runsImagesDir   = "images/"

	setupPage          = "docs/setup.md"
	setupPinned        = "make install-claude"
	pluginsPage        = "docs/plugins.md"
	pluginsPinned      = `{"verdict":"allow|deny|context"`
	runsPage           = "docs/runs.md"
	runsPinned         = "stateDiagram-v2"
	contributingPage   = "docs/contributing.md"
	contributingPinned = "harness report --dry-run"
)

type movedPage struct {
	path     string
	headings []string
	pinned   string
}

var movedPages = []movedPage{
	{setupPage, []string{"The plugin binary", "Build from source", "Lint tools", "With harness init (no plugin)", "Step by step", "Example config"}, setupPinned},
	{pluginsPage, []string{"Deploy tool plugin", "Custom lint pack and lint engine plugin", "Architecture rules in any language", "Signals plugin", "Hook plugin"}, pluginsPinned},
	{runsPage, []string{"A full run", "Plan view", "Run phases"}, runsPinned},
	{contributingPage, []string{"Repository layout and tests", "Report a harness bug"}, contributingPinned},
}

func TestReadmeIsShort(t *testing.T) {
	lines := strings.Count(repository.Read(t, readmeFile), "\n")
	if lines > readmeMaxLines {
		t.Errorf("%s has %d lines, want at most %d", readmeFile, lines, readmeMaxLines)
	}
}

func TestReadmeShowsPlanView(t *testing.T) {
	firstRun := repository.Section(t, repository.Read(t, readmeFile), firstRunHeading)
	runs := repository.Read(t, runsPage)
	for _, name := range planViewImages {
		if !strings.Contains(firstRun, "("+readmeImagesDir+name+")") {
			t.Errorf("the Your first run section of %s does not link %s%s", readmeFile, readmeImagesDir, name)
		}
		if !strings.Contains(runs, "("+runsImagesDir+name+")") {
			t.Errorf("%s does not link %s%s", runsPage, runsImagesDir, name)
		}
	}
}

func TestMovedSections(t *testing.T) {
	for _, page := range movedPages {
		raw, err := os.ReadFile(repository.Path(t, page.path))
		if err != nil {
			t.Errorf("%s is missing: %v", page.path, err)
			continue
		}
		text := string(raw)
		lines := strings.Split(text, "\n")
		for _, heading := range page.headings {
			if !slices.Contains(lines, headingLine+heading) {
				t.Errorf("%s has no line %q", page.path, headingLine+heading)
			}
		}
		if !strings.Contains(text, page.pinned) {
			t.Errorf("%s does not hold %q", page.path, page.pinned)
		}
		if strings.Contains(text, repository.EmDash) || strings.ContainsFunc(text, repository.IsEmoji) {
			t.Errorf("%s has an em dash or an emoji", page.path)
		}
	}
}

func pluginNames(t *testing.T, pattern string) []string {
	t.Helper()
	paths, err := filepath.Glob(repository.Path(t, pattern))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no file matches %s", pattern)
	}
	names := make([]string, len(paths))
	for i, p := range paths {
		if filepath.Base(p) == skillDocName {
			p = filepath.Dir(p)
		}
		names[i] = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	}
	return names
}

func TestReadmeListsSkillsAndAgents(t *testing.T) {
	install := repository.Section(t, repository.Read(t, readmeFile), installHeading)
	for prefix, pattern := range map[string]string{readmeSkillsLine: skillDocs, readmeAgentsLine: agentDocs} {
		_, rest, found := strings.Cut(install, "\n"+prefix)
		line, _, _ := strings.Cut(rest, "\n")
		if !found {
			t.Errorf("the Install section of %s has no line %q", readmeFile, prefix)
		}
		for _, name := range pluginNames(t, pattern) {
			if !strings.Contains(line, "`"+name+"`") {
				t.Errorf("the line %q of %s does not name `%s`", prefix, readmeFile, name)
			}
		}
	}
}

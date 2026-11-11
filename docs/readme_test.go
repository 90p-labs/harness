package docs_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/shared/domain"
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
	readmeMaxLines  = 190
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

const (
	extendHeading   = "\n## Extend it per project\n"
	tableRowPrefix  = "| "
	tableHeaderRows = 1
	exampleConfig   = ".harness.yaml"
	examplesDir     = "examples/"
	exampleLinkEnd  = "/)"
	pluginsPageLink = "(" + pluginsPage
	ruffEntryLines  = 5
)

var (
	pluginExamples = []string{"go", "python", "symfony", "nextjs"}
	backticked     = regexp.MustCompile("`([^`]+)`")
)

type exampleEvent struct {
	example string
	event   string
}

var exampleEvents = []exampleEvent{
	{"symfony", "gates.detect"},
	{"symfony", "lint.check"},
	{"go", "lint.check"},
	{"python", "lint.check"},
}

func exampleLink(name string) string {
	return "(" + examplesDir + name + exampleLinkEnd
}

func tableRows(section string) []string {
	var rows []string
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, tableRowPrefix) {
			rows = append(rows, line)
		}
	}
	if len(rows) < tableHeaderRows {
		return nil
	}
	return rows[tableHeaderRows:]
}

func TestReadmeExplainsPlugins(t *testing.T) {
	section := repository.Section(t, repository.Read(t, readmeFile), extendHeading)
	links := []string{pluginsPageLink}
	for _, name := range pluginExamples {
		links = append(links, exampleLink(name))
	}
	for _, link := range links {
		if !strings.Contains(section, link) {
			t.Errorf("the Extend it per project section of %s does not hold %q", readmeFile, link)
		}
	}
	rows := tableRows(section)
	if len(rows) == 0 {
		t.Errorf("the Extend it per project section of %s has no table rows", readmeFile)
	}
	rowOf := map[string]string{}
	for _, row := range rows {
		cells := strings.Split(row, "|")
		for _, match := range backticked.FindAllStringSubmatch(cells[1], -1) {
			if _, ok := domain.KindOf(domain.EventName(match[1])); !ok {
				t.Errorf("the table row %q names %q, which is not a known event", row, match[1])
			}
			rowOf[match[1]] = row
		}
	}
	pythonConfig := repository.Read(t, examplesDir+"python/"+exampleConfig)
	for _, line := range strings.Split(pythonConfig, "\n")[:ruffEntryLines] {
		if !strings.Contains(section, line+"\n") {
			t.Errorf("the Extend it per project section of %s does not hold the python line %q", readmeFile, line)
		}
	}
	for _, pair := range exampleEvents {
		link := exampleLink(pair.example)
		if !strings.Contains(section, link) {
			t.Errorf("the Extend it per project section of %s does not hold %q", readmeFile, link)
		}
		config := repository.Read(t, examplesDir+pair.example+"/"+exampleConfig)
		if want := "events: [" + pair.event + "]"; !strings.Contains(config, want) {
			t.Errorf("%s%s/%s does not hold %q", examplesDir, pair.example, exampleConfig, want)
		}
		if !strings.Contains(rowOf[pair.event], link) {
			t.Errorf("the table row of %s in %s does not hold %q", pair.event, readmeFile, link)
		}
	}
}

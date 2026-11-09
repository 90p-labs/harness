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
	installPaths = []string{"Claude Code", "Codex", "Build from source"}
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

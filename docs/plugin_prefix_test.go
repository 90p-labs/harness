package docs_test

import (
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	oldPlugin       = "harness"
	marketplaceName = "harness"
	newPlugin       = "90p"
	uninstallPrefix = "plugin uninstall "
)

var (
	prefixGlobs = []string{
		"*.md", "docs/*.md", "docs/*/*.md", "examples/*.md", "examples/*/*.md",
		"skills/*/*.md", "skills/*/*/*.md", "agents/*.md",
		".claude-plugin/*.json", ".codex-plugin/*.json", ".agents/plugins/*.json", "Makefile",
	}
	allowedOldForm = uninstallPrefix + oldPlugin + "@" + marketplaceName
	readmeRequired = []string{
		"/plugin install " + newPlugin + "@" + marketplaceName,
		"codex plugin add " + newPlugin + "@" + marketplaceName,
		"/" + newPlugin + ":" + oldPlugin,
	}
	setupRequired = []string{
		"claude " + allowedOldForm,
		"claude plugin install " + newPlugin + "@" + marketplaceName,
		"/reload-plugins",
	}
)

func oldPluginForms(t *testing.T) []string {
	t.Helper()
	forms := []string{oldPlugin + "@" + marketplaceName, "/" + oldPlugin + ":", "$" + oldPlugin + ":"}
	for _, pattern := range []string{skillDocs, agentDocs} {
		for _, name := range pluginNames(t, pattern) {
			forms = append(forms, oldPlugin+":"+name)
		}
	}
	for _, name := range pluginNames(t, skillDocs) {
		forms = append(forms, "`/"+name+"`", "`/"+name+" ")
	}
	return forms
}

func TestPluginPrefix(t *testing.T) {
	forms := oldPluginForms(t)
	for _, pattern := range prefixGlobs {
		files, err := repository.Glob(t, pattern)
		if err != nil || len(files) == 0 {
			t.Fatalf("no file matches %s", pattern)
		}
		for _, file := range files {
			for i, line := range strings.Split(repository.Read(t, file), "\n") {
				line = strings.ReplaceAll(line, allowedOldForm, "")
				for _, form := range forms {
					if strings.Contains(line, form) {
						t.Errorf("%s:%d holds the old form %q", file, i+1, form)
					}
				}
			}
		}
	}
	for file, required := range map[string][]string{readmeFile: readmeRequired, setupPage: setupRequired} {
		text := repository.Read(t, file)
		for _, want := range required {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not hold %q", file, want)
			}
		}
	}
}

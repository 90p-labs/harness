package harness

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	skillsDir           = "skills"
	decisionTreeHeading = "## Decision tree"
	levelTwoHeading     = "## "
	fenceOpen           = "```"
	skillKind           = "skill"
	agentKind           = "agent"
)

var (
	crossReference = regexp.MustCompile("`([a-z][a-z-]*)` (" + skillKind + "|" + agentKind + ")\\b")
	reportLimit    = regexp.MustCompile(`\b(?:under|at most) \d+ words\b`)

	requiredReferences = map[string][]string{
		linterAgent:    {"ddd"},
		lintSetupSkill: {"ddd"},
		explorerFile:   {"ddd"},
		reviewerAgent:  {"ddd"},
		skillFile:      {"pr-open", "lint-setup", "ddd"},
	}
)

func pluginDocs(t *testing.T) map[string]string {
	t.Helper()
	docs := map[string]string{}
	err := filepath.WalkDir(skillsDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(path) == ".md" {
			docs[path] = readDoc(t, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	agents, err := filepath.Glob(agentDocs)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range agents {
		docs[path] = readDoc(t, path)
	}
	return docs
}

func TestSkillDecisionTree(t *testing.T) {
	skills, err := filepath.Glob(skillDocs)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range skills {
		_, body, _ := strings.Cut(strings.TrimPrefix(readDoc(t, path), "---\n"), frontmatterEnd)
		var headings []int
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, levelTwoHeading) {
				headings = append(headings, i)
			}
		}
		if len(headings) == 0 || lines[headings[0]] != decisionTreeHeading {
			t.Errorf("%s: the first level-2 heading is not %q", path, decisionTreeHeading)
			continue
		}
		next := ""
		for _, line := range lines[headings[0]+1:] {
			if next = strings.TrimSpace(line); next != "" {
				break
			}
		}
		if !strings.HasPrefix(next, fenceOpen) {
			t.Errorf("%s: %q has no fence below it", path, decisionTreeHeading)
		}
	}
}

func TestSkillCrossReferencesExist(t *testing.T) {
	for path, doc := range pluginDocs(t) {
		for _, m := range crossReference.FindAllStringSubmatch(doc, -1) {
			target := filepath.Join(skillsDir, m[1], skillDocName)
			if m[2] == agentKind {
				target = filepath.Join(filepath.Dir(agentDocs), m[1]+".md")
			}
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s names the %s %s, which has no file %s", path, m[2], m[1], target)
			}
		}
	}
}

func TestSkillCrossReferencesRequired(t *testing.T) {
	for path, names := range requiredReferences {
		doc := readDoc(t, path)
		for _, name := range names {
			if !strings.Contains(doc, "`"+name+"` "+skillKind) {
				t.Errorf("%s does not name the `%s` skill", path, name)
			}
		}
	}
}

func TestSkillsNoEmDash(t *testing.T) {
	docs := pluginDocs(t)
	for _, path := range []string{readmeFile, designFile} {
		docs[path] = readDoc(t, path)
	}
	for path, doc := range docs {
		if strings.Contains(doc, emDash) {
			t.Errorf("%s has an em dash", path)
		}
	}
}

func TestAgentReportWordLimit(t *testing.T) {
	agents, err := filepath.Glob(agentDocs)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range agents {
		if !reportLimit.MatchString(readDoc(t, path)) {
			t.Errorf("%s gives no word limit for its report", path)
		}
	}
}

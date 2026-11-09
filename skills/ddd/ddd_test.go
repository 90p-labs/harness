package ddd_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	lintdomain "github.com/90p-labs/harness/internal/lint/domain"
	"github.com/90p-labs/harness/internal/lint/infrastructure/packs"
	"github.com/90p-labs/harness/internal/lint/infrastructure/rulecatalogue"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	dddSkillDir         = "skills/ddd"
	dddSkill            = dddSkillDir + "/SKILL.md"
	dddReferences       = "references/"
	dddBuiltInLanguage  = "typescript"
	examplesDir         = "examples"
	decisionTreeHeading = "\n## Decision tree\n"

	maxTopicWords = 700
	maxSkillWords = 1200
)

var (
	dddRuleRef   = regexp.MustCompile("`((?:ddd|ddd-es)[/-][a-z-]+)`")
	dddLanguages = []string{"typescript", "php", "python", "go"}
	dddTopics    = []string{
		"use-cases", "aggregates", "repositories-and-mappers", "events", "composition-root",
		"delivery-edge", "api-contracts", "contexts", "tests", "porting-legacy", "style",
	}
	dddTopicsInLanguageFiles = []string{"use-cases", "composition-root", "events", "tests"}
	frameworkNames           = regexp.MustCompile(`(?i)\b(?:inversify|zod|express|jest|vitest|typeorm|prisma|nestjs|doctrine|symfony|laravel|sqlalchemy|django|fastapi|pytest|phpunit|hollywood|postgres|postgresql|mysql|mongodb|kafka|rabbitmq)\b`)
	languageSpecifics        = append([]string{"biome"}, knownLintTools...)
	languageExtension        = regexp.MustCompile(`\.(?:tsx?|jsx?|php|py|go)\b`)
	examplePath              = regexp.MustCompile("`(" + examplesDir + "/[^`]+)`")
	knownLintTools           = []string{"deptrac", "import-linter", "go-arch-lint", "depguard", "`ddd` pack"}
)

func TestDDDSkillRuleIDs(t *testing.T) {
	known := map[string]bool{}
	for _, name := range []lintdomain.PackName{lintdomain.PackDDD, lintdomain.PackDDDES} {
		pack, err := packs.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, rule := range pack.Rules {
			known[rule.ID] = true
		}
	}

	named := 0
	err := filepath.WalkDir(repository.Path(t, dddSkillDir), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range dddRuleRef.FindAllStringSubmatch(string(raw), -1) {
			named++
			if !known[m[1]] {
				t.Errorf("%s names the rule %s, which is in neither the pack ddd nor ddd-es", path, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if named == 0 {
		t.Errorf("%s names no rule id", dddSkillDir)
	}
}

func requireAgnostic(t *testing.T, path, doc string) {
	t.Helper()
	lower := strings.ToLower(doc)
	for _, specific := range languageSpecifics {
		if strings.Contains(lower, strings.ToLower(specific)) {
			t.Errorf("%s names %q, which belongs in a file of %s", path, specific, dddReferences)
		}
	}
	if ext := languageExtension.FindString(doc); ext != "" {
		t.Errorf("%s names the file extension %s, which belongs in a file of %s", path, ext, dddReferences)
	}
}

func topicPath(topic string) string {
	return filepath.Join(dddSkillDir, dddReferences+topic+".md")
}

func TestDDDSkillAgnostic(t *testing.T) {
	skill := repository.Read(t, dddSkill)
	catalogue, err := rulecatalogue.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range catalogue.Packs {
		for _, rule := range pack.Rules {
			if !strings.Contains(skill, "`"+rule.ID+"`") {
				t.Errorf("%s does not name the catalogue rule %s", dddSkill, rule.ID)
			}
		}
	}
	requireAgnostic(t, dddSkill, skill)
}

func TestDDDSkillTopics(t *testing.T) {
	tree := repository.Section(t, repository.Read(t, dddSkill), decisionTreeHeading)
	for _, topic := range dddTopics {
		if _, err := os.Stat(repository.Path(t, topicPath(topic))); err != nil {
			t.Errorf("the topic file %s does not exist", topicPath(topic))
		}
		if reference := dddReferences + topic + ".md"; !strings.Contains(tree, reference) {
			t.Errorf("the decision tree of %s does not route to %s", dddSkill, reference)
		}
	}
	entries, err := os.ReadDir(repository.Path(t, dddSkillDir, dddReferences))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name, isMarkdown := strings.CutSuffix(entry.Name(), ".md")
		if !isMarkdown || !slices.Contains(dddLanguages, name) && !slices.Contains(dddTopics, name) {
			t.Errorf("%s is neither a language file nor a topic file", filepath.Join(dddSkillDir, dddReferences, entry.Name()))
		}
	}
}

func TestDDDSkillTopicsAgnostic(t *testing.T) {
	for _, topic := range dddTopics {
		path := topicPath(topic)
		doc := repository.Read(t, path)
		requireAgnostic(t, path, doc)
		if name := frameworkNames.FindString(doc); name != "" {
			t.Errorf("%s names the framework %q, which belongs in a file of a language", path, name)
		}
	}
}

func TestDDDSkillSize(t *testing.T) {
	for _, topic := range dddTopics {
		if words := len(strings.Fields(repository.Read(t, topicPath(topic)))); words > maxTopicWords {
			t.Errorf("%s has %d words, want at most %d", topicPath(topic), words, maxTopicWords)
		}
	}
	if words := len(strings.Fields(repository.Read(t, dddSkill))); words > maxSkillWords {
		t.Errorf("%s has %d words, want at most %d", dddSkill, words, maxSkillWords)
	}
}

func TestDDDSkillReferences(t *testing.T) {
	tree := repository.Section(t, repository.Read(t, dddSkill), decisionTreeHeading)
	for _, language := range dddLanguages {
		reference := dddReferences + language + ".md"
		path := filepath.Join(dddSkillDir, reference)
		doc := repository.Read(t, path)
		if !strings.Contains(tree, reference) {
			t.Errorf("the decision tree of %s does not route to %s", dddSkill, reference)
		}
		for _, topic := range dddTopicsInLanguageFiles {
			if !strings.Contains(doc, topic+".md") {
				t.Errorf("%s does not name %s.md", path, topic)
			}
		}
		configs := examplePath.FindAllStringSubmatch(doc, -1)
		if language != dddBuiltInLanguage && len(configs) == 0 {
			t.Errorf("%s names no tested config under %s", path, examplesDir)
		}
		for _, m := range configs {
			if _, err := os.Stat(repository.Path(t, m[1])); err != nil {
				t.Errorf("%s names %s, which does not exist", path, m[1])
			}
		}
	}
	if keys := repository.FrontmatterKeys(t, dddSkill); !slices.Equal(keys, []string{"argument-hint", "description", "name"}) {
		t.Errorf("%s frontmatter keys = %v, want argument-hint, description and name", dddSkill, keys)
	}
}

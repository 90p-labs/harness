package harness

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	lintdomain "github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/packs"
	"github.com/jorge07/harness/internal/lint/infrastructure/rulecatalogue"
)

const (
	dddSkillDir   = "skills/ddd"
	dddSkill      = dddSkillDir + "/SKILL.md"
	dddReferences = "references/"

	dddBuiltInLanguage = "typescript"
)

var (
	dddRuleRef        = regexp.MustCompile("`((?:ddd|ddd-es)/[a-z-]+)`")
	dddLanguages      = []string{"typescript", "php", "python", "go"}
	languageSpecifics = append([]string{"biome"}, knownLintTools...)
	languageExtension = regexp.MustCompile(`\.(?:tsx?|jsx?|php|py|go)\b`)
	examplePath       = regexp.MustCompile("`(" + examplesDir + "/[^`]+)`")
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
	err := filepath.WalkDir(dddSkillDir, func(path string, d fs.DirEntry, err error) error {
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

func TestDDDSkillAgnostic(t *testing.T) {
	skill := readDoc(t, dddSkill)
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
	lower := strings.ToLower(skill)
	for _, specific := range languageSpecifics {
		if strings.Contains(lower, strings.ToLower(specific)) {
			t.Errorf("%s names %q, which belongs in a file of %s", dddSkill, specific, dddReferences)
		}
	}
	if ext := languageExtension.FindString(skill); ext != "" {
		t.Errorf("%s names the file extension %s, which belongs in a file of %s", dddSkill, ext, dddReferences)
	}
}

func TestDDDSkillReferences(t *testing.T) {
	skill := readDoc(t, dddSkill)
	for _, language := range dddLanguages {
		reference := dddReferences + language + ".md"
		path := filepath.Join(dddSkillDir, reference)
		doc := readDoc(t, path)
		if !strings.Contains(skill, reference) {
			t.Errorf("%s does not route to %s", dddSkill, reference)
		}
		configs := examplePath.FindAllStringSubmatch(doc, -1)
		if language != dddBuiltInLanguage && len(configs) == 0 {
			t.Errorf("%s names no tested config under %s", path, examplesDir)
		}
		for _, m := range configs {
			if _, err := os.Stat(m[1]); err != nil {
				t.Errorf("%s names %s, which does not exist", path, m[1])
			}
		}
	}
	if keys := frontmatterKeys(t, dddSkill); !slices.Equal(keys, []string{"argument-hint", "description", "name"}) {
		t.Errorf("%s frontmatter keys = %v, want argument-hint, description and name", dddSkill, keys)
	}
}

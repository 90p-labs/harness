package packs

import (
	"embed"
	"fmt"
	"path"

	"gopkg.in/yaml.v3"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

//go:embed ddd ddd-es nextjs nextjs-ddd-shadcn observability shadcn
var Files embed.FS

const packFile = "pack.yaml"

type packDocument struct {
	Name     domain.PackName `yaml:"name"`
	Requires []string        `yaml:"requires"`
	Rules    []ruleDocument  `yaml:"rules"`
}

type ruleDocument struct {
	ID       string `yaml:"id"`
	Engine   string `yaml:"engine"`
	Severity string `yaml:"severity"`
	Scope    string `yaml:"scope"`
	Message  string `yaml:"message"`
	Plugin   string `yaml:"plugin"`
	Biome    string `yaml:"biome"`
	Oxlint   string `yaml:"oxlint"`
	Reason   string `yaml:"reason"`
}

func All() ([]domain.Pack, error) {
	all := make([]domain.Pack, 0, len(domain.AllPacks))
	for _, name := range domain.AllPacks {
		pack, err := Get(name)
		if err != nil {
			return nil, err
		}
		all = append(all, pack)
	}
	return all, nil
}

func Get(name domain.PackName) (domain.Pack, error) {
	if _, err := domain.ParsePackName(string(name)); err != nil {
		return domain.Pack{}, err
	}
	raw, err := Files.ReadFile(path.Join(string(name), packFile))
	if err != nil {
		return domain.Pack{}, fmt.Errorf("read pack %s: %w", name, err)
	}
	var doc packDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return domain.Pack{}, fmt.Errorf("decode pack %s: %w", name, err)
	}
	pack := domain.Pack{Name: doc.Name}
	for _, required := range doc.Requires {
		pack.Requires = append(pack.Requires, domain.PackName(required))
	}
	for _, r := range doc.Rules {
		pack.Rules = append(pack.Rules, domain.Rule{
			ID:         r.ID,
			Engine:     domain.EngineName(r.Engine),
			Severity:   shareddomain.Severity(r.Severity),
			Scope:      r.Scope,
			Message:    r.Message,
			Plugin:     embeddedPath(doc.Name, r.Plugin),
			BiomeRule:  r.Biome,
			OxlintRule: r.Oxlint,
			Reason:     r.Reason,
		})
	}
	return pack, nil
}

func embeddedPath(pack domain.PackName, plugin string) string {
	if plugin == "" {
		return ""
	}
	return path.Join(string(pack), plugin)
}

package packs

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing/fstest"

	"gopkg.in/yaml.v3"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

//go:embed ddd ddd-es nextjs nextjs-ddd-shadcn observability shadcn
var Files embed.FS

const (
	packFile         = "pack.yaml"
	folderMarker     = "/"
	versionSeparator = "+"
	digestLength     = 12
)

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

type Catalogue struct {
	custom map[domain.PackName]domain.Pack
	files  fstest.MapFS
	taken  map[string]bool
}

type packFiles fstest.MapFS

func (p packFiles) Open(name string) (fs.File, error) {
	if _, ok := p[name]; ok {
		return fstest.MapFS(p).Open(name)
	}
	return Files.Open(name)
}

func Load(repo string, entries []string) (Catalogue, []domain.PackName, error) {
	catalogue := Catalogue{custom: map[domain.PackName]domain.Pack{}, files: fstest.MapFS{}, taken: map[string]bool{}}
	for _, entry := range entries {
		if strings.Contains(entry, folderMarker) {
			continue
		}
		pack, err := Get(domain.PackName(entry))
		if err != nil {
			return Catalogue{}, nil, err
		}
		for _, rule := range pack.Rules {
			for _, claim := range ruleClaims(rule) {
				catalogue.taken[claim] = true
			}
		}
	}
	root, err := os.OpenRoot(repo)
	if err != nil {
		return Catalogue{}, nil, err
	}
	defer root.Close()
	var active []domain.PackName
	seen := map[string]bool{}
	for _, entry := range entries {
		isFolder := strings.Contains(entry, folderMarker)
		key := entry
		if isFolder {
			key = path.Clean(entry) + folderMarker
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		var name domain.PackName
		if isFolder {
			name, err = catalogue.loadFolder(repo, root, entry)
		} else {
			name, err = domain.ParsePackName(entry)
		}
		if err != nil {
			return Catalogue{}, nil, err
		}
		active = append(active, name)
	}
	return catalogue, active, nil
}

func (c Catalogue) Get(name domain.PackName) (domain.Pack, error) {
	if pack, ok := c.custom[name]; ok {
		return pack, nil
	}
	return Get(name)
}

func (c Catalogue) Files() fs.FS {
	return packFiles(c.files)
}

func (c Catalogue) Version(binary string) string {
	if len(c.files) == 0 {
		return binary
	}
	sum := sha256.New()
	for _, name := range slices.Sorted(maps.Keys(c.files)) {
		fmt.Fprintf(sum, "%s\x00%d\x00", name, len(c.files[name].Data))
		sum.Write(c.files[name].Data)
	}
	return binary + versionSeparator + hex.EncodeToString(sum.Sum(nil))[:digestLength]
}

func ruleClaims(rule domain.Rule) []string {
	var claims []string
	for _, claim := range []string{rule.ID, rule.BiomeRule, rule.OxlintRule} {
		if claim != "" {
			claims = append(claims, claim)
		}
	}
	return claims
}

func (c Catalogue) loadFolder(repo string, root *os.Root, entry string) (domain.PackName, error) {
	dir, err := folderInRepo(repo, entry)
	if err != nil {
		return "", err
	}
	raw, err := fs.ReadFile(root.FS(), path.Join(dir, packFile))
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return "", fmt.Errorf("%w: no %s in %s", domain.ErrUnknownPack, packFile, entry)
	}
	if err != nil {
		return "", err
	}
	doc, err := decodeDocument(raw, dir)
	if err != nil {
		return "", err
	}
	pack, err := domain.NewCustomPack(string(doc.Name), doc.requires(), doc.rules())
	if err != nil {
		return "", fmt.Errorf("pack %s: %w", entry, err)
	}
	if _, taken := c.custom[pack.Name]; taken {
		return "", fmt.Errorf("%w: %s and another folder both name %q", domain.ErrPackNameTaken, entry, pack.Name)
	}
	for _, rule := range pack.Rules {
		for _, claim := range ruleClaims(rule) {
			if c.taken[claim] {
				return "", fmt.Errorf("%w: pack %s, rule %s uses %s", domain.ErrRuleTaken, entry, rule.ID, claim)
			}
			c.taken[claim] = true
		}
	}
	c.files[path.Join(string(pack.Name), packFile)] = &fstest.MapFile{Data: raw}
	for _, r := range doc.Rules {
		if r.Plugin == "" {
			continue
		}
		source, err := fs.ReadFile(root.FS(), path.Join(dir, r.Plugin))
		if err != nil {
			return "", fmt.Errorf("read plugin of pack %s: %w", entry, err)
		}
		c.files[packPath(pack.Name, r.Plugin)] = &fstest.MapFile{Data: source}
	}
	c.custom[pack.Name] = pack
	return pack.Name, nil
}

func folderInRepo(repo, entry string) (string, error) {
	dir := path.Clean(entry)
	if !fs.ValidPath(dir) {
		return "", fmt.Errorf("%w: %s", domain.ErrPackOutsideRepo, entry)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(repo, filepath.FromSlash(dir)))
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return "", fmt.Errorf("%w: no folder %s", domain.ErrUnknownPack, entry)
	}
	if err != nil {
		return "", err
	}
	realRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(realRepo, resolved); err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%w: %s", domain.ErrPackOutsideRepo, entry)
	}
	return dir, nil
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
	doc, err := readDocument(Files, string(name))
	if err != nil {
		return domain.Pack{}, err
	}
	return domain.Pack{Name: doc.Name, Requires: doc.requires(), Rules: doc.rules()}, nil
}

func readDocument(files fs.FS, dir string) (packDocument, error) {
	raw, err := fs.ReadFile(files, path.Join(dir, packFile))
	if err != nil {
		return packDocument{}, fmt.Errorf("read pack %s: %w", dir, err)
	}
	return decodeDocument(raw, dir)
}

func decodeDocument(raw []byte, dir string) (packDocument, error) {
	var doc packDocument
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return packDocument{}, fmt.Errorf("decode pack %s: %w", dir, err)
	}
	return doc, nil
}

func (doc packDocument) requires() []domain.PackName {
	var requires []domain.PackName
	for _, required := range doc.Requires {
		requires = append(requires, domain.PackName(required))
	}
	return requires
}

func (doc packDocument) rules() []domain.Rule {
	var rules []domain.Rule
	for _, r := range doc.Rules {
		rules = append(rules, domain.Rule{
			ID:         r.ID,
			Engine:     domain.EngineName(r.Engine),
			Severity:   shareddomain.Severity(r.Severity),
			Scope:      r.Scope,
			Message:    r.Message,
			Plugin:     packPath(doc.Name, r.Plugin),
			BiomeRule:  r.Biome,
			OxlintRule: r.Oxlint,
			Reason:     r.Reason,
		})
	}
	return rules
}

func packPath(pack domain.PackName, plugin string) string {
	if plugin == "" {
		return ""
	}
	// path.Join would clean a ".." away before NewCustomPack can refuse it.
	return string(pack) + folderMarker + path.Clean(plugin)
}

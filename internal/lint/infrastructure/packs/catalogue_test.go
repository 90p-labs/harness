package packs_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/lint/domain"
	"github.com/90p-labs/harness/internal/lint/infrastructure/packs"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestPackCatalogue(t *testing.T) {
	all, err := packs.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	wantRules := map[domain.PackName]int{
		domain.PackDDD:             23,
		domain.PackDDDES:           3,
		domain.PackNextjs:          8,
		domain.PackNextjsDDDShadcn: 3,
		domain.PackObservability:   10,
		domain.PackShadcn:          4,
	}
	if len(all) != len(wantRules) {
		t.Fatalf("packs = %d, want %d", len(all), len(wantRules))
	}
	ids := map[string]domain.PackName{}
	grit := 0
	for _, pack := range all {
		if got, want := len(pack.Rules), wantRules[pack.Name]; got != want {
			t.Errorf("pack %s has %d rules, want %d", pack.Name, got, want)
		}
		if _, err := fs.Stat(packs.Files, path.Join(string(pack.Name), "README.md")); err != nil {
			t.Errorf("pack %s: %v", pack.Name, err)
		}
		for _, rule := range pack.Rules {
			if owner, dup := ids[rule.ID]; dup {
				t.Errorf("rule %s is in packs %s and %s", rule.ID, owner, pack.Name)
			}
			ids[rule.ID] = pack.Name
			if !slices.Contains([]shareddomain.Severity{shareddomain.SeverityError, shareddomain.SeverityWarn, shareddomain.SeverityInfo}, rule.Severity) {
				t.Errorf("rule %s has severity %q", rule.ID, rule.Severity)
			}
			if rule.Scope == "" || rule.Message == "" {
				t.Errorf("rule %s has no scope or no message: %+v", rule.ID, rule)
			}
			switch rule.Engine {
			case domain.EngineBiomeGrit:
				grit++
				source, err := fs.ReadFile(packs.Files, rule.Plugin)
				if err != nil {
					t.Errorf("rule %s: plugin file: %v", rule.ID, err)
				}
				if !strings.Contains(string(source), "["+rule.ID+"]") {
					t.Errorf("rule %s: the plugin message does not start with the rule id", rule.ID)
				}
			case domain.EngineBiomeRule:
				if !strings.HasPrefix(rule.BiomeRule, "lint/") {
					t.Errorf("rule %s has biome category %q", rule.ID, rule.BiomeRule)
				}
			case domain.EngineOxlint:
				if rule.OxlintRule == "" || rule.Reason == "" {
					t.Errorf("rule %s has no oxlint rule or no reason", rule.ID)
				}
			case domain.EngineHarness:
			default:
				t.Errorf("rule %s has engine %q", rule.ID, rule.Engine)
			}
		}
	}
	if grit != 10 {
		t.Errorf("biome-grit rules = %d, want 10", grit)
	}

	wantRequires := map[domain.PackName][]domain.PackName{
		domain.PackDDDES:           {domain.PackDDD},
		domain.PackNextjsDDDShadcn: {domain.PackDDD, domain.PackNextjs, domain.PackShadcn},
	}
	for _, pack := range all {
		if !slices.Equal(pack.Requires, wantRequires[pack.Name]) {
			t.Errorf("pack %s requires %v, want %v", pack.Name, pack.Requires, wantRequires[pack.Name])
		}
	}
}

func TestPackByName(t *testing.T) {
	got, err := packs.Get(domain.PackShadcn)
	if err != nil || got.Name != domain.PackShadcn || len(got.Rules) != 4 {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := packs.Get("nope"); !errors.Is(err, domain.ErrUnknownPack) {
		t.Fatalf("Get unknown = %v, want ErrUnknownPack", err)
	}
}

const (
	teamFolder = "./lint/team"
	teamPack   = domain.PackName("team")
	teamPlugin = "rules/no-console.grit"
	teamGrit   = "language js\n`console.log($m)` as $c where { register_diagnostic(span=$c, message=\"[team/no-console] no console\") }\n"
)

func packYAML(name, requires string) string {
	return "name: " + name + "\nrequires: [" + requires + "]\nrules:\n" +
		"  - id: " + name + "/no-console\n    engine: biome-grit\n    severity: error\n    scope: \"src/**/*.ts\"\n" +
		"    message: A source file calls console.log.\n    plugin: " + teamPlugin + "\n"
}

func writePack(t *testing.T, root, folder, yaml string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(folder))
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(teamPlugin)), []byte(teamGrit), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoad(t *testing.T) {
	t.Run("a folder entry", func(t *testing.T) {
		repo := t.TempDir()
		writePack(t, repo, teamFolder, packYAML(string(teamPack), string(domain.PackDDD)))

		catalogue, active, err := packs.Load(repo, []string{string(domain.PackDDD), teamFolder})

		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if want := []domain.PackName{domain.PackDDD, teamPack}; !reflect.DeepEqual(active, want) {
			t.Fatalf("active = %v, want %v", active, want)
		}
		pack, err := catalogue.Get(teamPack)
		if err != nil || len(pack.Rules) != 1 || !slices.Equal(pack.Requires, []domain.PackName{domain.PackDDD}) {
			t.Fatalf("Get = %+v, %v", pack, err)
		}
		source, err := fs.ReadFile(catalogue.Files(), pack.Rules[0].Plugin)
		if err != nil || string(source) != teamGrit {
			t.Fatalf("plugin %s = %q, %v", pack.Rules[0].Plugin, source, err)
		}
		builtin, err := catalogue.Get(domain.PackDDD)
		if err != nil || builtin.Name != domain.PackDDD {
			t.Fatalf("Get ddd = %+v, %v", builtin, err)
		}
		if _, err := fs.ReadFile(catalogue.Files(), "ddd/rules/event-readonly.grit"); err != nil {
			t.Fatalf("built-in plugin: %v", err)
		}
	})

	t.Run("outside the repo or missing", func(t *testing.T) {
		outside := t.TempDir()
		writePack(t, outside, "team", packYAML(string(teamPack), ""))
		repo := t.TempDir()
		if err := os.MkdirAll(filepath.Join(repo, "lint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(outside, "team"), filepath.Join(repo, "lint", "link")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, "lint", "file.yaml"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		cases := map[string]struct {
			entry string
			want  error
		}{
			"a parent path":         {entry: "../" + filepath.Base(outside) + "/team", want: domain.ErrPackOutsideRepo},
			"a cleaned parent path": {entry: "./lint/../../team", want: domain.ErrPackOutsideRepo},
			"an absolute path":      {entry: filepath.Join(outside, "team"), want: domain.ErrPackOutsideRepo},
			"a symlink outside":     {entry: "./lint/link", want: domain.ErrPackOutsideRepo},
			"a missing folder":      {entry: "./lint/tema", want: domain.ErrUnknownPack},
			"the repo root":         {entry: "lint/..", want: domain.ErrUnknownPack},
			"a file":                {entry: "./lint/file.yaml", want: domain.ErrUnknownPack},
			"a path through a file": {entry: "./lint/file.yaml/team", want: domain.ErrUnknownPack},
		}
		for name, tc := range cases {
			t.Run(name, func(t *testing.T) {
				_, _, err := packs.Load(repo, []string{tc.entry})

				if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), tc.entry) {
					t.Fatalf("Load = %v, want %v naming %s", err, tc.want, tc.entry)
				}
			})
		}
	})

	t.Run("taken names", func(t *testing.T) {
		repo := t.TempDir()
		writePack(t, repo, "lint/ddd", packYAML(string(domain.PackDDD), ""))
		writePack(t, repo, "lint/a", packYAML(string(teamPack), ""))
		writePack(t, repo, "lint/b", packYAML(string(teamPack), ""))
		cases := map[string][]string{
			"a built-in name":       {"./lint/ddd"},
			"two folders, one name": {"./lint/a", "./lint/b"},
		}
		for name, entries := range cases {
			t.Run(name, func(t *testing.T) {
				if _, _, err := packs.Load(repo, entries); !errors.Is(err, domain.ErrPackNameTaken) {
					t.Fatalf("Load = %v, want ErrPackNameTaken", err)
				}
			})
		}
	})

	t.Run("taken rule ids", func(t *testing.T) {
		const otherPack = "other"
		repo := t.TempDir()
		writePack(t, repo, "lint/a", packYAML(string(teamPack), ""))
		writePack(t, repo, "lint/b", strings.Replace(packYAML(otherPack, ""), otherPack+"/no-console", string(teamPack)+"/no-console", 1))
		writePack(t, repo, "lint/c", strings.Replace(packYAML(string(teamPack), ""), string(teamPack)+"/no-console", "ddd/event-readonly", 1))
		team := packYAML(string(teamPack), "")
		writePack(t, repo, "lint/d", team+team[strings.Index(team, "  - id:"):])
		engineRule := "  - id: team/%s\n    engine: %s\n    severity: info\n    scope: \"docs/**\"\n    message: A copy.\n    %s: %s\n"
		writePack(t, repo, "lint/e", team+fmt.Sprintf(engineRule, "img", domain.EngineBiomeRule, "biome", "lint/performance/noImgElement"))
		writePack(t, repo, "lint/f", team+fmt.Sprintf(engineRule, "link", domain.EngineOxlint, "oxlint", "nextjs/no-html-link-for-pages"))
		ddd, nextjs := string(domain.PackDDD), string(domain.PackNextjs)
		cases := map[string][]string{
			"an active built-in rule id":      {ddd, "./lint/c"},
			"a built-in rule id listed later": {"./lint/c", ddd},
			"two packs, one rule id":          {"./lint/a", "./lint/b"},
			"one rule id twice":               {"./lint/d"},
			"an active built-in biome rule":   {nextjs, "./lint/e"},
			"an active built-in oxlint rule":  {"./lint/f", nextjs},
		}
		for name, entries := range cases {
			t.Run(name, func(t *testing.T) {
				if _, _, err := packs.Load(repo, entries); !errors.Is(err, domain.ErrRuleTaken) {
					t.Fatalf("Load = %v, want ErrRuleTaken", err)
				}
			})
		}

		for _, entry := range []string{"./lint/c", "./lint/e", "./lint/f"} {
			t.Run("the claim of an inactive built-in pack is free: "+entry, func(t *testing.T) {
				if _, _, err := packs.Load(repo, []string{entry}); err != nil {
					t.Fatalf("Load = %v, want the custom pack to load while its built-in pack is not active", err)
				}
			})
		}
	})

	t.Run("requires a custom pack", func(t *testing.T) {
		repo := t.TempDir()
		writePack(t, repo, teamFolder, packYAML(string(teamPack), "other"))

		if _, _, err := packs.Load(repo, []string{teamFolder}); !errors.Is(err, domain.ErrUnknownPack) {
			t.Fatalf("Load = %v, want ErrUnknownPack", err)
		}
	})

	t.Run("a plugin that leaves its folder", func(t *testing.T) {
		repo := t.TempDir()
		writePack(t, repo, teamFolder, packYAML(string(teamPack), ""))
		escaping := strings.Replace(packYAML(string(teamPack), ""), teamPlugin, "../team/"+teamPlugin, 1)
		writePack(t, repo, "lint/a", escaping)

		if _, _, err := packs.Load(repo, []string{"./lint/a"}); !errors.Is(err, domain.ErrPluginOutsidePack) {
			t.Fatalf("Load = %v, want ErrPluginOutsidePack", err)
		}
	})

	t.Run("a plugin path with a dot element", func(t *testing.T) {
		repo := t.TempDir()
		writePack(t, repo, teamFolder, strings.Replace(packYAML(string(teamPack), ""), teamPlugin, "./"+teamPlugin, 1))

		catalogue, _, err := packs.Load(repo, []string{teamFolder})

		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if _, err := fs.ReadFile(catalogue.Files(), path.Join(string(teamPack), teamPlugin)); err != nil {
			t.Fatalf("plugin: %v", err)
		}
	})

	t.Run("the version follows the custom pack files", func(t *testing.T) {
		const binary = "v1.0.0"
		repo := t.TempDir()
		writePack(t, repo, teamFolder, packYAML(string(teamPack), ""))
		version := func() string {
			catalogue, _, err := packs.Load(repo, []string{string(domain.PackDDD), teamFolder})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			return catalogue.Version(binary)
		}
		builtinOnly, _, err := packs.Load(repo, []string{string(domain.PackDDD)})
		if err != nil {
			t.Fatal(err)
		}
		if got := builtinOnly.Version(binary); got != binary {
			t.Fatalf("Version with built-in packs = %q, want %q", got, binary)
		}

		first := version()
		packFile := filepath.Join(repo, filepath.FromSlash(teamFolder), "pack.yaml")
		edited := strings.Replace(packYAML(string(teamPack), ""), "severity: error", "severity: warn", 1)
		if err := os.WriteFile(packFile, []byte(edited), 0o600); err != nil {
			t.Fatal(err)
		}
		second := version()

		if !strings.HasPrefix(first, binary) || first == binary || first == second || version() != second {
			t.Fatalf("versions = %q then %q, want a stable %s version that changes with pack.yaml", first, second, binary)
		}
	})

	t.Run("a folder named like a built-in pack", func(t *testing.T) {
		repo := t.TempDir()
		writePack(t, repo, "ddd", packYAML("other", ""))

		_, active, err := packs.Load(repo, []string{"./ddd", "ddd"})

		if want := []domain.PackName{"other", domain.PackDDD}; err != nil || !reflect.DeepEqual(active, want) {
			t.Fatalf("Load = %v, %v, want %v", active, err, want)
		}
	})

	t.Run("builtin names", func(t *testing.T) {
		repo := t.TempDir()
		writePack(t, repo, teamFolder, packYAML(string(teamPack), ""))

		_, active, err := packs.Load(repo, []string{"ddd", "shadcn", "ddd", teamFolder, "lint/team"})

		if want := []domain.PackName{domain.PackDDD, domain.PackShadcn, teamPack}; err != nil || !reflect.DeepEqual(active, want) {
			t.Fatalf("Load = %v, %v, want %v", active, err, want)
		}
		if _, active, err := packs.Load(repo, nil); err != nil || len(active) != 0 {
			t.Fatalf("Load no entry = %v, %v, want no pack and no error", active, err)
		}
		_, _, err = packs.Load(repo, []string{"ddd", "django"})
		if !errors.Is(err, domain.ErrUnknownPack) || !strings.Contains(err.Error(), "django") {
			t.Fatalf("Load = %v, want ErrUnknownPack naming django", err)
		}
	})
}

const (
	phpFolder = "./lint/architecture"
	phpPack   = domain.PackName("architecture")
	phpFile   = "src/Order/Domain/Order.php"
	tsFile    = "src/contexts/order/domain/order.entity.ts"
)

const (
	phpScope       = "src/**/*.php"
	phpRootFile    = "src/Kernel.php"
	domainPurity   = "ddd/domain-purity"
	srcRoots       = "ddd/src-roots"
	notInCatalogue = "ddd/context-layers"
)

var sharedCatalogue = domain.RuleCatalogue{Packs: []domain.CataloguePack{{Name: domain.PackDDD, Rules: []domain.CatalogueRule{{ID: domainPurity}, {ID: srcRoots}}}}}

func phpPackYAML(id, scope string) string {
	return "name: architecture\nrules:\n  - id: " + id + "\n    engine: deptrac\n    severity: error\n" +
		"    scope: \"" + scope + "\"\n    message: The domain uses only itself.\n"
}

func gitRepo(t *testing.T, files ...string) string {
	t.Helper()
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for _, file := range files {
		target := filepath.Join(repo, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestSharedCatalogueRule(t *testing.T) {
	for _, id := range []string{domainPurity, srcRoots} {
		t.Run("loads "+id+" on another engine with no script file in both scopes", func(t *testing.T) {
			repo := gitRepo(t, phpFile, phpRootFile, tsFile)
			writePack(t, repo, phpFolder, phpPackYAML(id, phpScope))

			_, active, err := packs.LoadSharing(repo, []string{string(domain.PackDDD), phpFolder}, sharedCatalogue)

			if want := []domain.PackName{domain.PackDDD, phpPack}; err != nil || !reflect.DeepEqual(active, want) {
				t.Fatalf("LoadSharing = %v, %v, want %v", active, err, want)
			}
		})
	}

	t.Run("refuses a second pack on the same engine", func(t *testing.T) {
		const otherFolder = "./lint/other"
		repo := gitRepo(t, phpFile, tsFile)
		writePack(t, repo, phpFolder, phpPackYAML(domainPurity, phpScope))
		writePack(t, repo, otherFolder, strings.Replace(phpPackYAML(domainPurity, "src/Other/**/*.php"), "name: "+string(phpPack), "name: other", 1))

		_, _, err := packs.LoadSharing(repo, []string{phpFolder, otherFolder}, sharedCatalogue)

		want := fmt.Sprintf("pack %s, rule %s uses %s, which pack %s already holds", otherFolder, domainPurity, domainPurity, phpPack)
		if !errors.Is(err, domain.ErrRuleTaken) || !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadSharing = %v, want ErrRuleTaken containing %q", err, want)
		}
	})

	t.Run("refuses an id twice in one pack", func(t *testing.T) {
		repo := gitRepo(t, phpFile, tsFile)
		secondRule := strings.TrimPrefix(strings.Replace(phpPackYAML(domainPurity, "src/Other/**/*.php"), "deptrac", "phpstan", 1), "name: "+string(phpPack)+"\nrules:\n")
		writePack(t, repo, phpFolder, phpPackYAML(domainPurity, phpScope)+secondRule)

		_, _, err := packs.LoadSharing(repo, []string{phpFolder}, sharedCatalogue)

		want := fmt.Sprintf("pack %s, rule %s uses %s, which pack %s already holds", phpFolder, domainPurity, domainPurity, phpPack)
		if !errors.Is(err, domain.ErrRuleTaken) || !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadSharing = %v, want ErrRuleTaken containing %q", err, want)
		}
	})

	t.Run("refuses a file in both scopes", func(t *testing.T) {
		repo := gitRepo(t, phpFile, tsFile)
		writePack(t, repo, phpFolder, phpPackYAML(domainPurity, "src/**"))

		_, _, err := packs.LoadSharing(repo, []string{string(domain.PackDDD), phpFolder}, sharedCatalogue)

		want := "pack " + phpFolder + " on the engine deptrac and pack ddd on the engine harness both check ddd/domain-purity in " + tsFile
		if !errors.Is(err, domain.ErrScopeOverlap) || !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadSharing = %v, want ErrScopeOverlap containing %q", err, want)
		}
	})

	refused := map[string]struct {
		id   string
		load func(repo string, entries []string) (packs.Catalogue, []domain.PackName, error)
	}{
		"refuses an id outside the catalogue": {id: notInCatalogue, load: func(repo string, entries []string) (packs.Catalogue, []domain.PackName, error) {
			return packs.LoadSharing(repo, entries, sharedCatalogue)
		}},
		"Load shares no id": {id: domainPurity, load: packs.Load},
	}
	for name, tc := range refused {
		t.Run(name, func(t *testing.T) {
			repo := gitRepo(t, phpFile, tsFile)
			writePack(t, repo, phpFolder, phpPackYAML(tc.id, phpScope))

			_, _, err := tc.load(repo, []string{string(domain.PackDDD), phpFolder})

			if !errors.Is(err, domain.ErrRuleTaken) {
				t.Fatalf("load = %v, want ErrRuleTaken", err)
			}
		})
	}
}

func TestRuleTakenNamesBothPacks(t *testing.T) {
	const (
		otherPack = "other"
		claimedID = "ddd/domain-purity"
	)
	repo := t.TempDir()
	writePack(t, repo, "lint/architecture", strings.Replace(packYAML(string(teamPack), ""), string(teamPack)+"/no-console", claimedID, 1))
	writePack(t, repo, "lint/other", strings.Replace(packYAML(otherPack, ""), otherPack+"/no-console", claimedID, 1))
	cases := map[string]struct {
		entries []string
		holder  string
		clash   string
	}{
		"a built-in pack holds it": {entries: []string{string(domain.PackDDD), "./lint/architecture"}, holder: string(domain.PackDDD), clash: "./lint/architecture"},
		"a custom pack holds it":   {entries: []string{"./lint/architecture", "./lint/other"}, holder: string(teamPack), clash: "./lint/other"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := packs.Load(repo, tc.entries)

			if !errors.Is(err, domain.ErrRuleTaken) {
				t.Fatalf("Load = %v, want ErrRuleTaken", err)
			}
			want := fmt.Sprintf("pack %s, rule %s uses %s, which pack %s already holds", tc.clash, claimedID, claimedID, tc.holder)
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("Load = %q, want it to contain %q", err, want)
			}
		})
	}
}

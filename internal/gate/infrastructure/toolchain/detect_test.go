package toolchain_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/gate/infrastructure/toolchain"
)

const (
	packageJSON = "package.json"
	pnpmLock    = "pnpm-lock.yaml"
	npmLock     = "package-lock.json"
	goMod       = "go.mod"
	biomeJSON   = "biome.json"
	biomeJSONC  = "biome.jsonc"
	allScripts  = `{"name":"app","scripts":{"build":"tsc","test":"vitest run","lint":"eslint .","typecheck":"tsc --noEmit"}}`
	someScripts = `{"scripts":{"test":"vitest run","dev":"vite"}}`
)

var goSources = []string{"**/*.go"}

func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func gate(t *testing.T, id, run string, paths []string) domain.Gate {
	t.Helper()
	g, err := domain.NewGate(id, run, paths, 0, "")
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}
	return g
}

func TestDetectToolchain(t *testing.T) {
	tests := map[string]struct {
		files map[string]string
		want  []domain.Gate
	}{
		"go": {
			files: map[string]string{goMod: "module x\n"},
			want: []domain.Gate{
				gate(t, "go-fmt", `test -z "$(gofmt -l .)"`, goSources),
				gate(t, "go-vet", "go vet ./...", nil),
				gate(t, "go-test", "go test ./...", nil),
			},
		},
		"pnpm with biome": {
			files: map[string]string{packageJSON: allScripts, pnpmLock: "", biomeJSON: "{}"},
			want: []domain.Gate{
				gate(t, "typecheck", "pnpm run typecheck", nil),
				gate(t, "lint", "pnpm run lint", nil),
				gate(t, "test", "pnpm run test", nil),
				gate(t, "biome", "pnpm exec biome ci .", nil),
			},
		},
		"npm with some scripts": {
			files: map[string]string{packageJSON: someScripts, npmLock: ""},
			want:  []domain.Gate{gate(t, "test", "npm run test", nil)},
		},
		"npm with biome jsonc": {
			files: map[string]string{packageJSON: someScripts, npmLock: "", biomeJSONC: "{}"},
			want: []domain.Gate{
				gate(t, "test", "npm run test", nil),
				gate(t, "biome", "npx biome ci .", nil),
			},
		},
		"package without lock file": {
			files: map[string]string{packageJSON: allScripts},
		},
		"empty repo": {files: map[string]string{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := toolchain.Detect(repo(t, tt.files), toolchain.Lint{})
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("gates = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestDetectBrokenPackageJSON(t *testing.T) {
	_, err := toolchain.Detect(repo(t, map[string]string{packageJSON: "{", pnpmLock: ""}), toolchain.Lint{})
	if err == nil {
		t.Fatal("want an error for a broken package.json")
	}
}

func TestDetectLintGates(t *testing.T) {
	const baseline = ".harness-baseline/lint.json"
	lintGate := func() domain.Gate {
		g, err := domain.NewGate("lint", "harness lint --json", nil, 0, baseline)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	tests := map[string]struct {
		lint toolchain.Lint
		want []domain.Gate
	}{
		"no pack": {lint: toolchain.Lint{Chart: "deploy/chart", Baseline: baseline}},
		"packs": {
			lint: toolchain.Lint{Packs: []string{"ddd", "nextjs"}, Chart: "deploy/chart", Rules: "deploy/**/*.rules.yaml", Baseline: baseline},
			want: []domain.Gate{lintGate()},
		},
		"observability": {
			lint: toolchain.Lint{Packs: []string{"ddd", "observability"}, Chart: "deploy/chart", Rules: "deploy/**/*.rules.yaml", Baseline: baseline},
			want: []domain.Gate{
				lintGate(),
				gate(t, "helm-render", "helm template deploy/chart > /dev/null", nil),
				gate(t, "promtool-test", "find deploy -name '*.rules.test.yaml' -exec promtool test rules {} +", nil),
			},
		},
		"observability rules at the root": {
			lint: toolchain.Lint{Packs: []string{"observability"}, Chart: "chart", Rules: "**/*.rules.yaml", Baseline: baseline},
			want: []domain.Gate{
				lintGate(),
				gate(t, "helm-render", "helm template chart > /dev/null", nil),
				gate(t, "promtool-test", "find . -name '*.rules.test.yaml' -exec promtool test rules {} +", nil),
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := toolchain.Detect(repo(t, nil), tt.lint)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("gates = %+v, want %+v", got, tt.want)
			}
		})
	}
}

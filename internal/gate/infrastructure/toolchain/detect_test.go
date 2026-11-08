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
				gate(t, "node-typecheck", "pnpm run typecheck", nil),
				gate(t, "node-lint", "pnpm run lint", nil),
				gate(t, "node-test", "pnpm run test", nil),
				gate(t, "biome", "pnpm exec biome ci .", nil),
			},
		},
		"npm with some scripts": {
			files: map[string]string{packageJSON: someScripts, npmLock: ""},
			want:  []domain.Gate{gate(t, "node-test", "npm run test", nil)},
		},
		"npm with biome jsonc": {
			files: map[string]string{packageJSON: someScripts, npmLock: "", biomeJSONC: "{}"},
			want: []domain.Gate{
				gate(t, "node-test", "npm run test", nil),
				gate(t, "biome", "npx biome ci .", nil),
			},
		},
		"package without lock file runs npm": {
			files: map[string]string{packageJSON: allScripts},
			want: []domain.Gate{
				gate(t, "node-typecheck", "npm run typecheck", nil),
				gate(t, "node-lint", "npm run lint", nil),
				gate(t, "node-test", "npm run test", nil),
			},
		},
		"go and node without lock file": {
			files: map[string]string{goMod: "module x\n", packageJSON: allScripts},
			want: []domain.Gate{
				gate(t, "go-fmt", `test -z "$(gofmt -l .)"`, goSources),
				gate(t, "go-vet", "go vet ./...", nil),
				gate(t, "go-test", "go test ./...", nil),
				gate(t, "node-typecheck", "npm run typecheck", nil),
				gate(t, "node-lint", "npm run lint", nil),
				gate(t, "node-test", "npm run test", nil),
			},
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

func TestDetectEveryStack(t *testing.T) {
	dir := repo(t, map[string]string{goMod: "module x\n", packageJSON: allScripts, pnpmLock: ""})

	got, err := toolchain.Detect(dir, toolchain.Lint{Packs: []string{"ddd"}})
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if err := domain.ValidateGates(got); err != nil {
		t.Fatalf("gates %+v: %v", got, err)
	}
	if len(got) != 7 {
		t.Fatalf("got %d gates, want 3 Go, 3 Node and the lint gate: %+v", len(got), got)
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
		"no pack": {lint: toolchain.Lint{Baseline: baseline}},
		"packs": {
			lint: toolchain.Lint{Packs: []string{"ddd", "nextjs"}, Baseline: baseline},
			want: []domain.Gate{lintGate()},
		},
		"observability leaves the deploy gates to the deploy plugins": {
			lint: toolchain.Lint{Packs: []string{"ddd", "observability"}, Baseline: baseline},
			want: []domain.Gate{lintGate()},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := toolchain.Detect(repo(t, map[string]string{"Chart.yaml": "", "api.rules.test.yaml": ""}), tt.lint)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("gates = %+v, want %+v", got, tt.want)
			}
		})
	}
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	gateconfig "github.com/jorge07/harness/internal/gate/infrastructure/config"
	lintmodule "github.com/jorge07/harness/internal/lint"
	lintdomain "github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

var (
	packNextjs = string(lintdomain.PackNextjs)
	packDDD    = string(lintdomain.PackDDD)
	packShadcn = string(lintdomain.PackShadcn)
)

const (
	hostClaude          = string(shareddomain.HostClaude)
	initHostFlag        = "--" + flagHost
	observabilityConfig = "lint:\n  packs: [observability]\n  observability:\n    chart: deploy/chart\n    rules: deploy/rules/**\n    dashboards: deploy/dashboards/**\n"
)

var setupFiles = []string{".harness-lint/VERSION", ".claude/settings.json"}

func writeRepoFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readConfig(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, configFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestInitSetsUpARepository(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	writeRepoFile(t, dir, "go.mod", goModuleFile)

	code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude)

	if code != exitOK || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	want := []string{
		"lint packs: none detected",
		fmt.Sprintf(strings.TrimSuffix(lintSetupFormat, "\n"), languageGo),
		"gates: wrote go-fmt, go-vet, go-test",
		"lint init: wrote .harness-lint",
		"lint baseline: skipped, no lint pack",
		"host: wrote " + filepath.Join(dir, ".claude", "settings.json"),
	}
	if got := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n"); !reflect.DeepEqual(got, want) {
		t.Errorf("stdout lines = %q, want %q", got, want)
	}
	if config := readConfig(t, dir); !strings.Contains(config, "gates:") || strings.Contains(config, "lint:") {
		t.Errorf(".harness.yaml = %q, want the gates key and no lint key", config)
	}
	for _, name := range setupFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, lintmodule.BaselinePath)); !os.IsNotExist(err) {
		t.Errorf("%s exists with no lint pack", lintmodule.BaselinePath)
	}
}

func TestInitRefusesPacksAfterGates(t *testing.T) {
	for _, key := range []string{gateconfig.KeyGates, gateconfig.KeySetup} {
		t.Run(key, func(t *testing.T) {
			clearHarnessEnv(t)
			dir := initRepo(t)
			original := key + ":\n  - id: test\n    run: go test ./...\n"
			writeRepoFile(t, dir, configFile, original)
			writeRepoFile(t, dir, packageJSON, `{"dependencies":{"next":"15.0.0"}}`)

			code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude)

			wantKey := fmt.Errorf(gatesBeforePacksFormat, errGatesBeforePacks, key, key, lintdomain.PackNextjs).Error()
			if code != exitRuntime || !strings.Contains(stderr, wantKey) || stdout != "" {
				t.Fatalf("code = %d, stdout = %q, stderr = %q, want %d and %q", code, stdout, stderr, exitRuntime, wantKey)
			}
			if config := readConfig(t, dir); config != original {
				t.Errorf(".harness.yaml = %q, want %q", config, original)
			}
			if _, err := os.Stat(filepath.Join(dir, lintdomain.LintDir)); !os.IsNotExist(err) {
				t.Errorf("%s exists after a refused init", lintdomain.LintDir)
			}
		})
	}
}

func TestInitTwiceKeepsTheSetup(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	writeRepoFile(t, dir, configFile, observabilityConfig)
	writeRepoFile(t, dir, "charts/api/Chart.yaml", "")
	writeRepoFile(t, dir, "alerts/api.rules.test.yaml", "")

	code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude)
	if code != exitOK || !strings.Contains(stdout, "lint packs: kept observability\n") || !strings.Contains(stdout, "gates: wrote lint, helm-render, promtool-test\n") {
		t.Fatalf("first run: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	first := readConfig(t, dir)

	code, stdout, stderr = execute(t, commandInit, initHostFlag, hostClaude)

	if code != exitOK || stderr != "" {
		t.Fatalf("second run: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	for _, line := range []string{"lint packs: kept observability\n", "gates: kept\n", "lint baseline: wrote .harness-baseline/lint.json\n"} {
		if !strings.Contains(stdout, line) {
			t.Errorf("second run stdout = %q, want %q", stdout, line)
		}
	}
	if second := readConfig(t, dir); second != first {
		t.Errorf(".harness.yaml changed:\n%s\nwant:\n%s", second, first)
	}
}

func TestInitWithAManifest(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	writeRepoFile(t, dir, "go.mod", goModuleFile)
	writeRepoFile(t, dir, lintdomain.ManifestFile, "pack: ddd\n")

	code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude)

	if code != exitOK || strings.Contains(stdout, lintSetupPrefix) {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want no lint-setup line with %s", code, stdout, stderr, lintdomain.ManifestFile)
	}
	if !strings.Contains(stdout, "gates: wrote go-fmt, go-vet, go-test, lint-canaries\n") {
		t.Errorf("stdout = %q, want the lint-canaries gate", stdout)
	}
	if config := readConfig(t, dir); !strings.Contains(config, "run: harness lint canaries") {
		t.Errorf(".harness.yaml = %q, want the canary gate", config)
	}
}

func TestInitTwiceWithNoPack(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	writeRepoFile(t, dir, "go.mod", goModuleFile)
	if code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude); code != exitOK {
		t.Fatalf("first run: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	first := readConfig(t, dir)

	code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude)

	if code != exitOK || stderr != "" {
		t.Fatalf("second run: code = %d, stderr = %q", code, stderr)
	}
	for _, line := range []string{"lint packs: none detected\n", "gates: kept\n", "lint baseline: skipped, no lint pack\n"} {
		if !strings.Contains(stdout, line) {
			t.Errorf("second run stdout = %q, want %q", stdout, line)
		}
	}
	if second := readConfig(t, dir); second != first {
		t.Errorf(".harness.yaml changed:\n%s\nwant:\n%s", second, first)
	}
}

func TestInitRefusesAnUnknownHost(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	writeRepoFile(t, dir, "go.mod", goModuleFile)

	code, _, stderr := execute(t, commandInit, initHostFlag, "nope")

	if code != exitUsage || !strings.Contains(stderr, "unknown host") {
		t.Fatalf("code = %d, stderr = %q, want %d and the unknown host error", code, stderr, exitUsage)
	}
	for _, name := range append([]string{configFile, lintmodule.BaselinePath}, setupFiles...) {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s exists after a refused init", name)
		}
	}
}

func TestInitNamesEachLanguageWithNoBuiltInPack(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	writeRepoFile(t, dir, packageJSON, `{"dependencies":{"next":"16.0.0"}}`)
	writeRepoFile(t, dir, "composer.json", "{}\n")
	withOnlyGitOnPath(t)

	code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude)

	if code != exitRuntime || !strings.Contains(stderr, "biome") {
		t.Fatalf("code = %d, stderr = %q, want %d and the biome error after the packs step", code, stderr, exitRuntime)
	}
	want := "lint packs: wrote " + packNextjs + "\n" + fmt.Sprintf(lintSetupFormat, languagePHP)
	if !strings.HasPrefix(stdout, want) || strings.Count(stdout, lintSetupPrefix) != 1 {
		t.Fatalf("stdout = %q, want it to start with %q and hold one lint-setup line", stdout, want)
	}
}

func withOnlyGitOnPath(t *testing.T) {
	t.Helper()
	git, err := exec.LookPath(gitBinary)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.Symlink(git, filepath.Join(bin, gitBinary)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func TestInitStopsAtAMissingEngine(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	writeRepoFile(t, dir, componentsJSON, "{}\n")
	withOnlyGitOnPath(t)

	code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude)

	if code != exitRuntime || !strings.Contains(stderr, "biome") {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want %d and the biome error", code, stdout, stderr, exitRuntime)
	}
	if strings.Contains(stdout, lintSetupPrefix) {
		t.Errorf("stdout = %q, want no lint-setup line for a TypeScript repo", stdout)
	}
	config := readConfig(t, dir)
	if !strings.Contains(config, "- "+packShadcn) || !strings.Contains(config, "id: lint") {
		t.Errorf(".harness.yaml = %q, want the shadcn pack and the lint gate", config)
	}
	if _, err := os.Stat(filepath.Join(dir, setupFiles[0])); err != nil {
		t.Errorf("lint init files: %v", err)
	}
}

func TestChoosePacks(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"next dependency", map[string]string{packageJSON: `{"dependencies":{"next":"15.0.0"}}`}, []string{packNextjs}},
		{"nest dev dependency", map[string]string{packageJSON: `{"devDependencies":{"@nestjs/core":"11.0.0"}}`}, []string{packDDD}},
		{"shadcn components file", map[string]string{componentsJSON: "{}"}, []string{packShadcn}},
		{"next app with shadcn", map[string]string{packageJSON: `{"dependencies":{"next":"15.0.0","react":"19.0.0"}}`, componentsJSON: "{}"}, []string{packNextjs, packShadcn}},
		{"go repository", map[string]string{"go.mod": goModuleFile}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				writeRepoFile(t, dir, name, content)
			}
			got, err := choosePacks(dir)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Errorf("choosePacks = %v, %v, want %v", got, err, tc.want)
			}
		})
	}

	t.Run("broken package.json", func(t *testing.T) {
		dir := t.TempDir()
		writeRepoFile(t, dir, packageJSON, "{")
		if _, err := choosePacks(dir); err == nil || !strings.Contains(err.Error(), packageJSON) {
			t.Errorf("err = %v, want a parse error that names %s", err, packageJSON)
		}
	})

	t.Run("components.json folder", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, componentsJSON), 0o750); err != nil {
			t.Fatal(err)
		}
		if got, err := choosePacks(dir); err != nil || got != nil {
			t.Errorf("choosePacks = %v, %v, want no pack", got, err)
		}
	})
}

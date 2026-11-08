package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"

	"github.com/jorge07/harness/internal/gate/domain"
	gateconfig "github.com/jorge07/harness/internal/gate/infrastructure/config"
	sharedconfig "github.com/jorge07/harness/internal/shared/infrastructure/config"
)

const configFile = ".harness.yaml"

var opaquePacks = []string{"pack-one", "pack-two"}

func TestDecodeGates(t *testing.T) {
	t.Run("good list", func(t *testing.T) {
		got, err := gateconfig.Decode([]sharedconfig.Gate{
			{ID: "go-test", Run: "go test ./...", Paths: []string{"**/*.go"}, Timeout: time.Minute, Baseline: "b.json"},
			{ID: "lint", Run: "make lint"},
		})
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		want := []domain.Gate{
			{ID: "go-test", Run: "go test ./...", Paths: []string{"**/*.go"}, Timeout: time.Minute, Baseline: "b.json"},
			{ID: "lint", Run: "make lint", Timeout: domain.DefaultTimeout},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("gates = %+v, want %+v", got, want)
		}
	})
	t.Run("bad gate", func(t *testing.T) {
		_, err := gateconfig.Decode([]sharedconfig.Gate{{ID: "ok", Run: "x"}, {ID: "Bad_ID", Run: "x"}})
		if !errors.Is(err, domain.ErrInvalidGate) || !strings.Contains(err.Error(), "Bad_ID") {
			t.Errorf("err = %v, want invalid gate naming Bad_ID", err)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		_, err := gateconfig.Decode([]sharedconfig.Gate{{ID: "a", Run: "x"}, {ID: "a", Run: "y"}})
		if !errors.Is(err, domain.ErrInvalidGate) || !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("err = %v, want duplicate id", err)
		}
	})
}

func TestWriterKeepsExistingKeys(t *testing.T) {
	dir := t.TempDir()
	original := "# project config\nrun:\n  approval: false # keep\nstate:\n  dsn: sqlite:x.db\n"
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	var w gateconfig.Writer
	key, err := w.BlockingKey(dir)
	if err != nil || key != "" {
		t.Fatalf("BlockingKey = %q, %v, want none", key, err)
	}
	gates := []domain.Gate{
		{ID: "go-test", Run: "go test ./...", Paths: []string{"**/*.go"}, Timeout: domain.DefaultTimeout},
		{ID: "slow", Run: "make e2e", Timeout: 90 * time.Second, Baseline: "b.json"},
	}
	if err := w.WriteGates(dir, nil, gates); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, configFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.HasPrefix(text, "# project config\n") || !strings.Contains(text, "approval: false # keep") || !strings.Contains(text, "dsn: sqlite:x.db") {
		t.Errorf("existing content lost:\n%s", text)
	}
	if strings.Count(text, "timeout:") != 1 || !strings.Contains(text, "timeout: 1m30s") {
		t.Errorf("timeout must be written only when not the default:\n%s", text)
	}
	loaded, err := sharedconfig.Load(pflag.NewFlagSet("test", pflag.ContinueOnError), dir)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	decoded, err := gateconfig.Decode(loaded.Gates)
	if err != nil || !reflect.DeepEqual(decoded, gates) {
		t.Errorf("round trip = %+v, %v, want %+v", decoded, err, gates)
	}
	if key, err := w.BlockingKey(dir); err != nil || key != gateconfig.KeyGates {
		t.Errorf("BlockingKey after write = %q, %v, want %q", key, err, gateconfig.KeyGates)
	}
}

func TestDecodeGateConfig(t *testing.T) {
	got, err := gateconfig.Decode([]sharedconfig.Gate{
		{ID: "format", Run: "pnpm format"},
		{ID: "lint", Run: "pnpm lint"},
		{ID: "test", Run: "pnpm test", Needs: []string{"format", "lint"}},
	})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := []domain.Gate{
		{ID: "format", Run: "pnpm format", Timeout: domain.DefaultTimeout},
		{ID: "lint", Run: "pnpm lint", Timeout: domain.DefaultTimeout},
		{ID: "test", Run: "pnpm test", Timeout: domain.DefaultTimeout, Needs: []string{"format", "lint"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("gates = %+v, want %+v", got, want)
	}

	_, err = gateconfig.Decode([]sharedconfig.Gate{{ID: "test", Run: "x", Needs: []string{"lint"}}})
	if !errors.Is(err, domain.ErrInvalidGate) || !strings.Contains(err.Error(), `gate "test"`) {
		t.Errorf("unknown need error = %v, want invalid gate naming test", err)
	}
}

func TestWriteGateConfig(t *testing.T) {
	dir := t.TempDir()
	gates := []domain.Gate{
		{ID: "lint", Run: "pnpm lint", Timeout: domain.DefaultTimeout},
		{ID: "test", Run: "pnpm test", Timeout: domain.DefaultTimeout, Needs: []string{"lint"}},
	}
	var w gateconfig.Writer
	if err := w.WriteGates(dir, nil, gates); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, configFile))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(raw); strings.Count(text, "needs:") != 1 {
		t.Errorf("want one needs key:\n%s", text)
	}
	loaded, err := sharedconfig.Load(pflag.NewFlagSet("test", pflag.ContinueOnError), dir)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	decoded, err := gateconfig.Decode(loaded.Gates)
	if err != nil || !reflect.DeepEqual(decoded, gates) {
		t.Errorf("round trip = %+v, %v, want %+v", decoded, err, gates)
	}
}

func TestDecodeSetupConfig(t *testing.T) {
	got, err := gateconfig.DecodeSetup([]sharedconfig.Setup{
		{ID: "deps", Run: "composer install", Paths: []string{"composer.lock"}, Creates: "vendor/autoload.php"},
	})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want, _ := domain.NewSetup("deps", "composer install", []string{"composer.lock"}, "vendor/autoload.php")
	if !reflect.DeepEqual(got, []domain.Setup{want}) {
		t.Errorf("setup = %+v, want %+v", got, want)
	}

	_, err = gateconfig.DecodeSetup([]sharedconfig.Setup{{ID: "deps", Run: "x"}, {ID: "deps", Run: "y"}})
	if !errors.Is(err, domain.ErrInvalidSetup) {
		t.Errorf("duplicate err = %v, want an invalid setup error", err)
	}
}

func TestWriteSetupConfig(t *testing.T) {
	dir := t.TempDir()
	deps, _ := domain.NewSetup("deps", "make install", []string{"composer.lock"}, "vendor/autoload.php")
	gates := []domain.Gate{{ID: "test", Run: "make test", Timeout: domain.DefaultTimeout}}
	var w gateconfig.Writer
	if err := w.WriteGates(dir, []domain.Setup{deps}, gates); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, configFile))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(raw); strings.Index(text, "setup:") > strings.Index(text, "gates:") || strings.Contains(text, "timeout:") {
		t.Errorf("want the setup key before the gates key and no timeout:\n%s", text)
	}
	loaded, err := sharedconfig.Load(pflag.NewFlagSet("test", pflag.ContinueOnError), dir)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	decoded, err := gateconfig.DecodeSetup(loaded.Setup)
	if err != nil || !reflect.DeepEqual(decoded, []domain.Setup{deps}) {
		t.Errorf("round trip = %+v, %v, want %+v", decoded, err, deps)
	}

	setupOnly := t.TempDir()
	if err := os.WriteFile(filepath.Join(setupOnly, configFile), []byte("setup:\n  - {id: deps, run: x}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if key, err := w.BlockingKey(setupOnly); err != nil || key != gateconfig.KeySetup {
		t.Errorf("BlockingKey with a setup key = %q, %v, want %q", key, err, gateconfig.KeySetup)
	}
}

func TestWriterMissingFile(t *testing.T) {
	dir := t.TempDir()
	var w gateconfig.Writer
	if key, err := w.BlockingKey(dir); err != nil || key != "" {
		t.Fatalf("BlockingKey = %q, %v, want none", key, err)
	}
	if err := w.WriteGates(dir, nil, []domain.Gate{{ID: "a", Run: "x", Timeout: domain.DefaultTimeout}}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if key, err := w.BlockingKey(dir); err != nil || key != gateconfig.KeyGates {
		t.Errorf("BlockingKey = %q, %v, want %q", key, err, gateconfig.KeyGates)
	}
}

func TestWriteLintPacks(t *testing.T) {
	packs := opaquePacks
	cases := []struct {
		name     string
		original string
		keep     []string
	}{
		{name: "no file"},
		{
			name:     "other keys and lint settings",
			original: "# project config\nrun:\n  approval: false # keep\nlint:\n  ddd:\n    root: src/core\n",
			keep:     []string{"# project config\n", "approval: false # keep", "root: src/core"},
		},
		{name: "empty packs list", original: "lint:\n  packs: []\n  shadcn:\n    ui_allow: [src/x]\n", keep: []string{"ui_allow:"}},
		{name: "null lint value", original: "lint:\nstate:\n  dsn: sqlite:x.db\n", keep: []string{"dsn: sqlite:x.db"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.original != "" {
				if err := os.WriteFile(filepath.Join(dir, configFile), []byte(tc.original), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := (gateconfig.Writer{}).WriteLintPacks(dir, packs); err != nil {
				t.Fatalf("write: %v", err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, configFile))
			if err != nil {
				t.Fatal(err)
			}
			for _, kept := range tc.keep {
				if !strings.Contains(string(raw), kept) {
					t.Errorf("lost %q:\n%s", kept, raw)
				}
			}
			loaded, err := sharedconfig.Load(pflag.NewFlagSet("test", pflag.ContinueOnError), dir)
			if err != nil {
				t.Fatalf("load written config: %v\n%s", err, raw)
			}
			if !reflect.DeepEqual(loaded.Lint.Packs, packs) {
				t.Errorf("lint.packs = %v, want %v\n%s", loaded.Lint.Packs, packs, raw)
			}
		})
	}
}

func TestWriteLintPacksRefusesAScalarLint(t *testing.T) {
	dir := t.TempDir()
	original := "lint: oops\n"
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	err := (gateconfig.Writer{}).WriteLintPacks(dir, opaquePacks)
	if !errors.Is(err, gateconfig.ErrLintNotMapping) {
		t.Fatalf("err = %v, want ErrLintNotMapping", err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, configFile))
	if string(raw) != original {
		t.Errorf("file changed to %q", raw)
	}
}

func TestWriterBrokenYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte("a: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (gateconfig.Writer{}).BlockingKey(dir); err == nil {
		t.Error("want an error for broken yaml")
	}
}

func TestBlockingKey(t *testing.T) {
	const filled = "  - {id: a, run: x}\n"
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{"an empty gates list is absent", "gates: []\n", ""},
		{"a null gates key is absent", "gates:\n", ""},
		{"an empty setup list is absent", "setup: []\n", ""},
		{"a null setup key is absent", "setup:\n", ""},
		{"a gates list blocks", "gates:\n" + filled, gateconfig.KeyGates},
		{"a setup list blocks", "setup:\n" + filled, gateconfig.KeySetup},
		{"gates win over setup", "setup:\n" + filled + "gates:\n" + filled, gateconfig.KeyGates},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, configFile), []byte(tc.yaml), 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := (gateconfig.Writer{}).BlockingKey(dir); err != nil || got != tc.want {
				t.Errorf("BlockingKey = %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestBlockingKeyEmptyKeysAreReplaced(t *testing.T) {
	step := domain.Setup{ID: "deps", Run: "make deps"}
	gate := domain.Gate{ID: "a", Run: "x", Timeout: domain.DefaultTimeout}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte("setup: []\ngates: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (gateconfig.Writer{}).WriteGates(dir, []domain.Setup{step}, []domain.Gate{gate}); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, configFile))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Count(text, "gates:") != 1 || strings.Count(text, "setup:") != 1 || strings.Contains(text, "[]") {
		t.Errorf("want one setup key and one gates key, got:\n%s", text)
	}
}

func TestBlockingKeyNothingToWriteAddsNoKey(t *testing.T) {
	for name, setup := range map[string][]domain.Setup{
		"nothing":           nil,
		"only a setup step": {{ID: "deps", Run: "npm ci"}},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := (gateconfig.Writer{}).WriteGates(dir, setup, nil); err != nil {
				t.Fatalf("write: %v", err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, configFile))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), gateconfig.KeyGates) {
				t.Errorf("want no gates key, got:\n%s", raw)
			}
		})
	}
}

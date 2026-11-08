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
	has, err := w.HasGates(dir)
	if err != nil || has {
		t.Fatalf("HasGates = %v, %v, want false", has, err)
	}
	gates := []domain.Gate{
		{ID: "go-test", Run: "go test ./...", Paths: []string{"**/*.go"}, Timeout: domain.DefaultTimeout},
		{ID: "slow", Run: "make e2e", Timeout: 90 * time.Second, Baseline: "b.json"},
	}
	if err := w.WriteGates(dir, gates); err != nil {
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
	if has, err := w.HasGates(dir); err != nil || !has {
		t.Errorf("HasGates after write = %v, %v, want true", has, err)
	}
}

func TestWriterMissingFile(t *testing.T) {
	dir := t.TempDir()
	var w gateconfig.Writer
	if has, err := w.HasGates(dir); err != nil || has {
		t.Fatalf("HasGates = %v, %v, want false", has, err)
	}
	if err := w.WriteGates(dir, []domain.Gate{{ID: "a", Run: "x", Timeout: domain.DefaultTimeout}}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if has, err := w.HasGates(dir); err != nil || !has {
		t.Errorf("HasGates = %v, %v, want true", has, err)
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
	if _, err := (gateconfig.Writer{}).HasGates(dir); err == nil {
		t.Error("want an error for broken yaml")
	}
}

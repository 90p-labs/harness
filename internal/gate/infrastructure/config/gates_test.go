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

func TestWriterBrokenYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte("a: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (gateconfig.Writer{}).HasGates(dir); err == nil {
		t.Error("want an error for broken yaml")
	}
}

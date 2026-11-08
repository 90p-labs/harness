package initlint_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/application/command/initlint"
	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const binaryVersion = "v1.2.0"

var errPrepare = errors.New("prepare failed")

type fakeEngine struct {
	name      domain.EngineName
	err       error
	prepared  []domain.Rule
	called    bool
	lintDirOK bool
}

type fakeEngines []*fakeEngine

func (fakeEngines) Check(context.Context, string, domain.EngineName, []domain.Rule) ([]shareddomain.Finding, error) {
	return nil, nil
}

func (f fakeEngines) Prepare(_ context.Context, dir string, engine domain.EngineName, rules []domain.Rule) error {
	for _, e := range f {
		if e.name == engine {
			e.called = true
			e.prepared = rules
			info, err := os.Stat(filepath.Join(dir, domain.LintDir))
			e.lintDirOK = err == nil && info.IsDir()
			return e.err
		}
	}
	return nil
}

func lookup(name domain.PackName) (domain.Pack, error) {
	return domain.Pack{Name: name, Rules: []domain.Rule{
		{ID: "ddd/grit", Engine: domain.EngineBiomeGrit},
		{ID: "next/biome", Engine: domain.EngineBiomeRule},
		{ID: "ddd/layout", Engine: domain.EngineHarness},
	}}, nil
}

func readVersion(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".harness-lint", "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestLintInitVersion(t *testing.T) {
	t.Run("prepares each engine with its rules then writes the version", func(t *testing.T) {
		dir := t.TempDir()
		biome := &fakeEngine{name: domain.EngineBiomeGrit}
		oxlint := &fakeEngine{name: domain.EngineOxlint}
		handler := initlint.NewHandler([]domain.PackName{domain.PackDDD}, lookup, fakeEngines{biome, oxlint}, binaryVersion)

		if err := handler.Handle(context.Background(), initlint.Command{Dir: dir}); err != nil {
			t.Fatal(err)
		}

		if len(biome.prepared) != 2 || biome.prepared[1].ID != "next/biome" {
			t.Fatalf("biome rules = %v", biome.prepared)
		}
		if oxlint.called {
			t.Fatal("an engine with no rule is not prepared")
		}
		if got := readVersion(t, dir); got != binaryVersion {
			t.Fatalf("VERSION = %q, want %q", got, binaryVersion)
		}
	})

	t.Run("an engine prepares into an existing lint folder", func(t *testing.T) {
		dir := t.TempDir()
		biome := &fakeEngine{name: domain.EngineBiomeGrit}

		if err := initlint.NewHandler([]domain.PackName{domain.PackDDD}, lookup, fakeEngines{biome}, binaryVersion).Handle(context.Background(), initlint.Command{Dir: dir}); err != nil {
			t.Fatal(err)
		}

		if !biome.lintDirOK {
			t.Fatal("the lint folder did not exist when the engine prepared")
		}
	})

	t.Run("no pack writes only the version", func(t *testing.T) {
		dir := t.TempDir()
		biome := &fakeEngine{name: domain.EngineBiomeGrit}

		if err := initlint.NewHandler(nil, lookup, fakeEngines{biome}, binaryVersion).Handle(context.Background(), initlint.Command{Dir: dir}); err != nil {
			t.Fatal(err)
		}

		entries, err := os.ReadDir(filepath.Join(dir, ".harness-lint"))
		if err != nil || len(entries) != 1 || biome.called {
			t.Fatalf("entries = %v, err = %v, prepared = %v", entries, err, biome.called)
		}
	})

	t.Run("a second init drops the files of the first", func(t *testing.T) {
		dir := t.TempDir()
		stale := filepath.Join(dir, ".harness-lint", "biome", "sold", "biome.json")
		if err := os.MkdirAll(filepath.Dir(stale), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stale, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}

		if err := initlint.NewHandler(nil, lookup, nil, binaryVersion).Handle(context.Background(), initlint.Command{Dir: dir}); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stale file stat err = %v, want not exist", err)
		}
	})

	t.Run("a second init keeps the baseline", func(t *testing.T) {
		dir := t.TempDir()
		baseline := filepath.Join(dir, domain.BaselinePath)
		stale := filepath.Join(dir, domain.LintDir, "oxlintrc.json")
		want := []byte("{\"version\":1,\"findings\":[]}\n")
		if err := os.MkdirAll(filepath.Dir(baseline), 0o750); err != nil {
			t.Fatal(err)
		}
		for file, content := range map[string][]byte{baseline: want, stale: []byte("{}")} {
			if err := os.WriteFile(file, content, 0o600); err != nil {
				t.Fatal(err)
			}
		}

		if err := initlint.NewHandler(nil, lookup, nil, binaryVersion).Handle(context.Background(), initlint.Command{Dir: dir}); err != nil {
			t.Fatal(err)
		}

		if got, err := os.ReadFile(baseline); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("baseline = %q, err = %v, want %q", got, err, want)
		}
		if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stale file stat err = %v, want not exist", err)
		}
	})

	t.Run("a missing required pack writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		needsDDD := func(name domain.PackName) (domain.Pack, error) {
			return domain.Pack{Name: name, Requires: []domain.PackName{domain.PackDDD}}, nil
		}

		err := initlint.NewHandler([]domain.PackName{domain.PackDDDES}, needsDDD, nil, binaryVersion).Handle(context.Background(), initlint.Command{Dir: dir})

		if !errors.Is(err, domain.ErrMissingPack) {
			t.Fatalf("err = %v, want ErrMissingPack", err)
		}
		if _, statErr := os.Stat(filepath.Join(dir, ".harness-lint", "VERSION")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("VERSION must not exist, stat err = %v", statErr)
		}
	})

	t.Run("a prepare error writes no version", func(t *testing.T) {
		dir := t.TempDir()
		engine := &fakeEngine{name: domain.EngineHarness, err: errPrepare}
		handler := initlint.NewHandler([]domain.PackName{domain.PackDDD}, lookup, fakeEngines{engine}, binaryVersion)

		err := handler.Handle(context.Background(), initlint.Command{Dir: dir})

		if !errors.Is(err, errPrepare) {
			t.Fatalf("err = %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(dir, ".harness-lint", "VERSION")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("VERSION must not exist, stat err = %v", statErr)
		}
	})
}

func TestLintInitLegacyBaseline(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, domain.OldBaselinePath)
	version := filepath.Join(dir, domain.LintDir, domain.VersionFile)
	for _, file := range []string{old, version} {
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("v1.0.0"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	err := initlint.NewHandler(nil, lookup, nil, binaryVersion).Handle(context.Background(), initlint.Command{Dir: dir})

	if !errors.Is(err, domain.ErrOldBaseline) {
		t.Fatalf("err = %v, want ErrOldBaseline", err)
	}
	for _, part := range []string{domain.OldBaselinePath, domain.BaselinePath, ".harness.yaml"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("err = %q, want it to name %s", err, part)
		}
	}
	if _, statErr := os.Stat(old); statErr != nil {
		t.Errorf("old baseline stat err = %v, want it kept", statErr)
	}
	if got := readVersion(t, dir); got != "v1.0.0" {
		t.Errorf("VERSION = %q, want it unchanged", got)
	}
}

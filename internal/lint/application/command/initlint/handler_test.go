package initlint_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorge07/harness/internal/lint/application/command/initlint"
	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const binaryVersion = "v1.2.0"

var errPrepare = errors.New("prepare failed")

type fakeEngine struct {
	name     domain.EngineName
	err      error
	prepared []domain.Rule
	called   bool
}

func (f *fakeEngine) Name() domain.EngineName { return f.name }
func (f *fakeEngine) Prepare(_ string, rules []domain.Rule) error {
	f.called = true
	f.prepared = rules
	return f.err
}
func (f *fakeEngine) Run(context.Context, string, []domain.Rule) ([]shareddomain.Finding, error) {
	return nil, nil
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
		handler := initlint.NewHandler([]domain.PackName{domain.PackDDD}, lookup, []domain.Engine{biome, oxlint}, binaryVersion)

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

	t.Run("no pack writes only the version", func(t *testing.T) {
		dir := t.TempDir()
		biome := &fakeEngine{name: domain.EngineBiomeGrit}

		if err := initlint.NewHandler(nil, lookup, []domain.Engine{biome}, binaryVersion).Handle(context.Background(), initlint.Command{Dir: dir}); err != nil {
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
		handler := initlint.NewHandler([]domain.PackName{domain.PackDDD}, lookup, []domain.Engine{engine}, binaryVersion)

		err := handler.Handle(context.Background(), initlint.Command{Dir: dir})

		if !errors.Is(err, errPrepare) {
			t.Fatalf("err = %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(dir, ".harness-lint", "VERSION")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("VERSION must not exist, stat err = %v", statErr)
		}
	})
}

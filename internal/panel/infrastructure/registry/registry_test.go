package registry_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/panel/infrastructure/registry"
)

// layout makes a fake repo: <root>/<name>/.git as the common dir.
func layout(t *testing.T, root, name string) (commonDir, worktree string) {
	t.Helper()
	worktree = filepath.Join(root, name)
	commonDir = filepath.Join(worktree, ".git")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return commonDir, worktree
}

func repoAt(commonDir, worktree, stateDB string, usedAt time.Time) domain.Repo {
	return domain.Repo{
		Key:       domain.RepoKey(commonDir),
		Name:      filepath.Base(worktree),
		CommonDir: commonDir,
		Worktree:  worktree,
		StateDB:   stateDB,
		UsedAt:    usedAt,
	}
}

func TestCorruptRegistryNamesPathAndHint(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.json")
	if err := os.WriteFile(path, []byte("{garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := registry.New(dir, &bytes.Buffer{})
	common, tree := layout(t, t.TempDir(), "a")
	_, listErr := reg.List(ctx)
	regErr := reg.Register(ctx, repoAt(common, tree, "", time.Now()))
	for name, err := range map[string]error{"List": listErr, "Register": regErr} {
		if err == nil {
			t.Fatalf("%s: no error on a corrupt file", name)
		}
		if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "fix or delete") {
			t.Errorf("%s: error %q lacks the path or the hint", name, err)
		}
	}
	if got, _ := os.ReadFile(path); string(got) != "{garbage" {
		t.Errorf("corrupt file was rewritten: %q", got)
	}
}

func TestRegistry(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	root := t.TempDir()
	commonA, treeA := layout(t, root, "a")
	commonB, treeB := layout(t, root, "b")
	base := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

	t.Run("add and refresh", func(t *testing.T) {
		reg := registry.New(dir, &bytes.Buffer{})
		if err := reg.Register(ctx, repoAt(commonA, treeA, "", base)); err != nil {
			t.Fatal(err)
		}
		moved := filepath.Join(root, "a-moved")
		if err := os.MkdirAll(moved, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := reg.Register(ctx, repoAt(commonA, moved, "", base.Add(time.Hour))); err != nil {
			t.Fatal(err)
		}
		repos, err := reg.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(repos) != 1 {
			t.Fatalf("got %d repos, want 1: %+v", len(repos), repos)
		}
		if repos[0].Worktree != moved || !repos[0].UsedAt.Equal(base.Add(time.Hour)) {
			t.Errorf("repo not refreshed: %+v", repos[0])
		}
	})

	t.Run("refresh keeps a known state db and updates the name", func(t *testing.T) {
		reg := registry.New(t.TempDir(), &bytes.Buffer{})
		common, tree := layout(t, t.TempDir(), "late")
		db := filepath.Join(common, ".harness", "state.db")
		if err := reg.Register(ctx, repoAt(common, tree, "", base)); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(db), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(db, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		renamed := repoAt(common, tree, db, base.Add(time.Minute))
		renamed.Name = "renamed"
		if err := reg.Register(ctx, renamed); err != nil {
			t.Fatal(err)
		}
		// a later register with no state file keeps the known one
		if err := reg.Register(ctx, repoAt(common, tree, "", base.Add(2*time.Minute))); err != nil {
			t.Fatal(err)
		}
		repos, _ := reg.List(ctx)
		if len(repos) != 1 || repos[0].StateDB != db {
			t.Fatalf("repos = %+v, want state db %s", repos, db)
		}
		if err := os.Remove(db); err != nil {
			t.Fatal(err)
		}
		if repos, _ = reg.List(ctx); len(repos) != 0 {
			t.Errorf("kept a repo whose state file is gone: %+v", repos)
		}
	})

	t.Run("refresh updates the name", func(t *testing.T) {
		reg := registry.New(t.TempDir(), &bytes.Buffer{})
		common, tree := layout(t, t.TempDir(), "named")
		_ = reg.Register(ctx, repoAt(common, tree, "", base))
		r := repoAt(common, tree, "", base)
		r.Name = "other"
		_ = reg.Register(ctx, r)
		if repos, _ := reg.List(ctx); len(repos) != 1 || repos[0].Name != "other" {
			t.Errorf("repos = %+v, want name other", repos)
		}
	})

	t.Run("mode 0600", func(t *testing.T) {
		info, err := os.Stat(filepath.Join(dir, "repos.json"))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("repos.json mode = %o, want 600", got)
		}
	})

	t.Run("two concurrent registers keep both", func(t *testing.T) {
		shared := t.TempDir()
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for _, r := range []domain.Repo{repoAt(commonA, treeA, "", base), repoAt(commonB, treeB, "", base)} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// each goroutine has its own adapter, so its own file handles
				errs <- registry.New(shared, &bytes.Buffer{}).Register(ctx, r)
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		repos, err := registry.New(shared, &bytes.Buffer{}).List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(repos) != 2 {
			t.Fatalf("got %d repos, want both: %+v", len(repos), repos)
		}
	})

	t.Run("no rewrite when nothing changed", func(t *testing.T) {
		path := filepath.Join(dir, "repos.json")
		before, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
		reg := registry.New(dir, &bytes.Buffer{})
		for range 2 {
			if _, err := reg.List(ctx); err != nil {
				t.Fatal(err)
			}
		}
		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
			t.Errorf("List rewrote an unchanged file")
		}
	})

	t.Run("sorted newest first", func(t *testing.T) {
		shared := t.TempDir()
		reg := registry.New(shared, &bytes.Buffer{})
		_ = reg.Register(ctx, repoAt(commonA, treeA, "", base))
		_ = reg.Register(ctx, repoAt(commonB, treeB, "", base.Add(time.Minute)))
		repos, err := reg.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(repos) != 2 || repos[0].Worktree != treeB {
			t.Errorf("order = %+v, want b first", repos)
		}
	})
}

func TestRegistryPrune(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

	t.Run("common dir gone", func(t *testing.T) {
		var log bytes.Buffer
		reg := registry.New(t.TempDir(), &log)
		root := t.TempDir()
		common, tree := layout(t, root, "gone")
		if err := reg.Register(ctx, repoAt(common, tree, "", base)); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(tree); err != nil {
			t.Fatal(err)
		}
		repos, err := reg.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(repos) != 0 {
			t.Errorf("kept %+v", repos)
		}
		if want := "panel: dropped repo " + tree; !strings.Contains(log.String(), want) {
			t.Errorf("log = %q, want %q", log.String(), want)
		}
		again, _ := reg.List(ctx)
		if len(again) != 0 {
			t.Errorf("repo came back: %+v", again)
		}
	})

	t.Run("sqlite file gone", func(t *testing.T) {
		var log bytes.Buffer
		reg := registry.New(t.TempDir(), &log)
		root := t.TempDir()
		common, tree := layout(t, root, "lite")
		db := filepath.Join(common, ".harness", "state.db")
		if err := reg.Register(ctx, repoAt(common, tree, db, base)); err != nil {
			t.Fatal(err)
		}
		repos, _ := reg.List(ctx)
		if len(repos) != 0 {
			t.Fatalf("kept a repo with no state file: %+v", repos)
		}
		if !strings.Contains(log.String(), "panel: dropped repo") {
			t.Errorf("log = %q", log.String())
		}

		// with the file present the repo stays
		if err := os.MkdirAll(filepath.Dir(db), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(db, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		_ = reg.Register(ctx, repoAt(common, tree, db, base))
		if repos, _ = reg.List(ctx); len(repos) != 1 {
			t.Errorf("dropped a repo with a state file: %+v", repos)
		}
	})

	t.Run("empty state db kept while the common dir exists", func(t *testing.T) {
		reg := registry.New(t.TempDir(), &bytes.Buffer{})
		common, tree := layout(t, t.TempDir(), "pg")
		_ = reg.Register(ctx, repoAt(common, tree, "", base))
		if repos, _ := reg.List(ctx); len(repos) != 1 {
			t.Errorf("dropped a non-sqlite repo: %+v", repos)
		}
	})

	t.Run("worktree gone falls back to the main worktree", func(t *testing.T) {
		dir := t.TempDir()
		reg := registry.New(dir, &bytes.Buffer{})
		root := t.TempDir()
		common, main := layout(t, root, "main")
		linked := filepath.Join(root, "linked")
		if err := reg.Register(ctx, repoAt(common, linked, "", base)); err != nil {
			t.Fatal(err)
		}
		repos, err := reg.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(repos) != 1 || repos[0].Worktree != main {
			t.Fatalf("repos = %+v, want worktree %s", repos, main)
		}
		// the fallback is written back
		again, _ := registry.New(dir, &bytes.Buffer{}).List(ctx)
		if len(again) != 1 || again[0].Worktree != main {
			t.Errorf("fallback not persisted: %+v", again)
		}
	})
}

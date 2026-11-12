package apply_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/90p-labs/harness/internal/recipe/application/command/apply"
	"github.com/90p-labs/harness/internal/recipe/domain"
)

type catalogue struct{ recipes map[domain.Name]domain.Recipe }

func (c catalogue) List() []domain.Recipe { return nil }

func (c catalogue) Get(name domain.Name) (domain.Recipe, error) {
	recipe, ok := c.recipes[name]
	if !ok {
		return domain.Recipe{}, domain.ErrUnknownRecipe
	}
	return recipe, nil
}

func (c catalogue) Read(name domain.Name, path string) ([]byte, error) {
	return []byte("recipe:" + path), nil
}

type workspace struct {
	entries map[string]domain.Entry
	writes  []string
	failOn  string
}

func (w *workspace) Lookup(path string) (domain.Entry, error) { return w.entries[path], nil }

func (w *workspace) Write(path string, data []byte) error {
	if path == w.failOn {
		return errors.New("disk full")
	}
	w.writes = append(w.writes, path)
	w.entries[path] = domain.Entry{Kind: domain.File, Data: data}
	return nil
}

func setup(entries map[string]domain.Entry) (apply.Handler, *workspace, string) {
	target := "/repo"
	space := &workspace{entries: entries}
	cat := catalogue{recipes: map[domain.Name]domain.Recipe{
		"go": {Name: "go", Summary: "s", Files: []string{".harness.yaml", "lint/pack.yaml", "compose.yaml"}},
	}}
	open := func(root string) domain.Workspace {
		if root != target {
			panic(fmt.Sprintf("opened %s", root))
		}
		return space
	}
	return apply.NewHandler(cat, open), space, target
}

func actions(changes []domain.Change) []string {
	var out []string
	for _, change := range changes {
		out = append(out, change.Path+" "+string(change.Action))
	}
	return out
}

func TestApplyWrites(t *testing.T) {
	handler, space, target := setup(map[string]domain.Entry{})

	result, err := handler.Handle(apply.Command{Name: "go", Target: target})
	if err != nil {
		t.Fatalf("Handle() = %v", err)
	}
	want := []string{".harness.yaml wrote", "lint/pack.yaml wrote", "compose.yaml wrote"}
	if got := actions(result.Changes); !slices.Equal(got, want) {
		t.Errorf("changes = %v, want %v", got, want)
	}
	if !bytes.Equal(space.entries["lint/pack.yaml"].Data, []byte("recipe:lint/pack.yaml")) {
		t.Errorf("lint/pack.yaml = %q", space.entries["lint/pack.yaml"].Data)
	}
}

func TestApplyKeeps(t *testing.T) {
	entries := map[string]domain.Entry{
		".harness.yaml":  {Kind: domain.File, Data: []byte("recipe:.harness.yaml")},
		"compose.yaml":   {Kind: domain.File, Data: []byte("mine")},
		"lint/pack.yaml": {Kind: domain.Symlink},
	}
	handler, space, target := setup(entries)

	result, err := handler.Handle(apply.Command{Name: "go", Target: target})
	if err != nil {
		t.Fatalf("Handle() = %v", err)
	}
	want := []string{".harness.yaml kept", "lint/pack.yaml kept", "compose.yaml differs"}
	if got := actions(result.Changes); !slices.Equal(got, want) {
		t.Errorf("changes = %v, want %v", got, want)
	}
	if len(space.writes) != 0 {
		t.Errorf("writes = %v, want none", space.writes)
	}
	if string(space.entries["compose.yaml"].Data) != "mine" {
		t.Errorf("compose.yaml changed")
	}
}

func TestApplyDryRun(t *testing.T) {
	entries := map[string]domain.Entry{"compose.yaml": {Kind: domain.File, Data: []byte("recipe:compose.yaml")}}
	handler, space, target := setup(entries)

	result, err := handler.Handle(apply.Command{Name: "go", Target: target, DryRun: true})
	if err != nil {
		t.Fatalf("Handle() = %v", err)
	}
	want := []string{".harness.yaml wrote", "lint/pack.yaml wrote", "compose.yaml kept"}
	if got := actions(result.Changes); !slices.Equal(got, want) {
		t.Errorf("changes = %v, want %v", got, want)
	}
	if len(space.writes) != 0 {
		t.Errorf("writes = %v, want none", space.writes)
	}
}

func TestApplyConflictWritesNothing(t *testing.T) {
	cases := map[string]map[string]domain.Entry{
		"a folder at a file path":      {"compose.yaml": {Kind: domain.Folder}},
		"a file at a parent folder":    {"lint": {Kind: domain.File}},
		"a symlink at a parent folder": {"lint": {Kind: domain.Symlink}},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			handler, space, target := setup(entries)

			_, err := handler.Handle(apply.Command{Name: "go", Target: target})
			if !errors.Is(err, apply.ErrConflict) {
				t.Fatalf("Handle() = %v, want ErrConflict", err)
			}
			if len(space.writes) != 0 {
				t.Errorf("writes = %v, want none", space.writes)
			}
		})
	}
}

func TestApplyWriteErrorReturnsPartialResult(t *testing.T) {
	handler, space, target := setup(map[string]domain.Entry{})
	space.failOn = "lint/pack.yaml"

	result, err := handler.Handle(apply.Command{Name: "go", Target: target})

	if err == nil {
		t.Fatal("Handle() = nil, want the write error")
	}
	if got, want := actions(result.Changes), []string{".harness.yaml wrote"}; !slices.Equal(got, want) {
		t.Errorf("changes = %v, want the files already written %v", got, want)
	}
	if result.Recipe.Name != "go" {
		t.Errorf("recipe = %q, want go", result.Recipe.Name)
	}
}

func TestApplyUnknownRecipe(t *testing.T) {
	handler, _, target := setup(map[string]domain.Entry{})

	_, err := handler.Handle(apply.Command{Name: "nope", Target: target})
	if !errors.Is(err, domain.ErrUnknownRecipe) {
		t.Fatalf("Handle() = %v, want ErrUnknownRecipe", err)
	}
}

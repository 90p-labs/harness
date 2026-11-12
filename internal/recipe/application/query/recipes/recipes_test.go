package recipes_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/recipe/application/query/recipes"
	"github.com/90p-labs/harness/internal/recipe/domain"
)

type catalogue struct{ recipes []domain.Recipe }

func (c catalogue) List() []domain.Recipe { return c.recipes }

func (c catalogue) Get(domain.Name) (domain.Recipe, error) { return domain.Recipe{}, nil }

func (c catalogue) Read(domain.Name, string) ([]byte, error) { return nil, nil }

// The catalogue lists its recipes sorted by name, and the query keeps that order.
func TestListIsSortedByName(t *testing.T) {
	handler := recipes.NewHandler(catalogue{recipes: []domain.Recipe{
		{Name: "go", Summary: "g"},
		{Name: "nextjs", Summary: "n"},
		{Name: "python", Summary: "p"},
	}})

	got := handler.Handle()

	want := []domain.Name{"go", "nextjs", "python"}
	if len(got) != len(want) {
		t.Fatalf("List() = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Errorf("List()[%d] = %s, want %s", i, got[i].Name, name)
		}
	}
	sorted := slices.IsSortedFunc(got, func(a, b domain.Recipe) int {
		return strings.Compare(a.Name.String(), b.Name.String())
	})
	if !sorted {
		t.Errorf("List() = %v, want sorted by name", got)
	}
	if got[0].Summary != "g" {
		t.Errorf("summary = %q, want g", got[0].Summary)
	}
}

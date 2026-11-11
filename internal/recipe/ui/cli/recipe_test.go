package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/recipe/domain"
	"github.com/90p-labs/harness/internal/recipe/ui/cli"
)

type fakeCatalogue struct{ recipes []domain.Recipe }

func (f fakeCatalogue) List() []domain.Recipe { return f.recipes }

func (f fakeCatalogue) Get(name domain.Name) (domain.Recipe, error) {
	for _, r := range f.recipes {
		if r.Name == name {
			return r, nil
		}
	}
	return domain.Recipe{}, fmt.Errorf("%w: %s", domain.ErrUnknownRecipe, name)
}

func (f fakeCatalogue) Read(domain.Name, string) ([]byte, error) { return []byte("x"), nil }

// failingWorkspace has no file and fails the write of failOn.
type failingWorkspace struct{ failOn string }

func (w failingWorkspace) Lookup(string) (domain.Entry, error) { return domain.Entry{}, nil }

func (w failingWorkspace) Write(path string, _ []byte) error {
	if path == w.failOn {
		return errors.New("disk full")
	}
	return nil
}

func run(t *testing.T, cat domain.Catalogue, args ...string) (string, error) {
	t.Helper()
	return runIn(t, cat, func(string) domain.Workspace { return nil }, args...)
}

func runIn(t *testing.T, cat domain.Catalogue, open domain.Opener, args ...string) (string, error) {
	t.Helper()
	deps := cli.Deps{
		Catalogue: func() (domain.Catalogue, error) { return cat, nil },
		Open:      open,
		Target:    func(context.Context) (string, error) { return t.TempDir(), nil },
	}
	cmd := cli.NewCommand(deps)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func sample(guide string) fakeCatalogue {
	return fakeCatalogue{recipes: []domain.Recipe{
		{
			Name: "demo", Summary: "a demo", Files: []string{"a.yaml", "b/c"},
			Scaffold: []string{"npx create-demo@1"}, Next: []string{"harness verify"}, Guide: guide,
		},
		{Name: "zed", Summary: "last", Files: []string{"z"}},
	}}
}

func TestShowPrintsGuide(t *testing.T) {
	out, err := run(t, sample("# Guide\nstep one\n"), "show", "demo")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"a demo", "a.yaml", "b/c", "npx create-demo@1", "next: harness verify", "# Guide\nstep one"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "next: harness verify") > strings.Index(out, "# Guide") {
		t.Errorf("the guide must come after the next steps:\n%s", out)
	}

	if !strings.Contains(out, "scaffold (printed, never run; run them in order in one shell):") {
		t.Errorf("the scaffold header must say the steps share one shell:\n%s", out)
	}

	out, err = run(t, sample(""), "show", "demo")
	if err != nil || strings.Contains(out, "Guide") {
		t.Errorf("no guide expected: err=%v out=%s", err, out)
	}
}

func TestApplyWriteErrorPrintsWrittenFiles(t *testing.T) {
	open := func(string) domain.Workspace { return failingWorkspace{failOn: "b/c"} }

	out, err := runIn(t, sample(""), open, "apply", "demo")

	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err = %v, want the write error", err)
	}
	if !strings.Contains(out, "wrote a.yaml\n") {
		t.Errorf("output lacks the file written before the error:\n%s", out)
	}
	if strings.Contains(out, "wrote b/c") || strings.Contains(out, nextLine) {
		t.Errorf("output names a file that was not written, or the next steps:\n%s", out)
	}
}

const nextLine = "next: "

func TestListKeepsTheCatalogueOrder(t *testing.T) {
	out, err := run(t, sample(""), "list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(out, "demo") > strings.Index(out, "zed") || !strings.Contains(out, "a demo") {
		t.Errorf("list output:\n%s", out)
	}
}

func TestUnknownRecipeListsNames(t *testing.T) {
	for _, sub := range []string{"show", "apply"} {
		_, err := run(t, sample(""), sub, "nope")
		if !errors.Is(err, domain.ErrUnknownRecipe) {
			t.Fatalf("%s: err = %v", sub, err)
		}
		if !strings.Contains(err.Error(), "demo") || !strings.Contains(err.Error(), "zed") {
			t.Errorf("%s: error lacks the known names: %v", sub, err)
		}
	}
}

package view_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/infrastructure/view"
)

func TestRegionsAreInPage(t *testing.T) {
	in := baseInput(t)
	r := view.NewRenderer()
	full, err := r.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	page := string(full)

	names := map[string]bool{}
	ids := map[string]bool{}
	regions := view.Regions()
	if len(regions) == 0 {
		t.Fatal("no regions")
	}
	for _, reg := range regions {
		if names[reg.Name] {
			t.Errorf("duplicate region name %q", reg.Name)
		}
		if ids[reg.ID] {
			t.Errorf("duplicate region id %q", reg.ID)
		}
		names[reg.Name], ids[reg.ID] = true, true

		out, err := r.RenderRegion(in, reg.Name)
		if err != nil {
			t.Errorf("RenderRegion(%q): %v", reg.Name, err)
			continue
		}
		got := string(out)
		if strings.TrimSpace(got) == "" {
			t.Errorf("region %q is empty", reg.Name)
		}
		attr := `id="` + reg.ID + `"`
		if !strings.Contains(got, attr) {
			t.Errorf("region %q lacks %s", reg.Name, attr)
		}
		if !strings.Contains(page, got) {
			t.Errorf("region %q is not a substring of the page", reg.Name)
		}
		if n := strings.Count(page, attr); n != 1 {
			t.Errorf("%s appears %d times in the page, want 1", attr, n)
		}
	}
}

func TestRegionsReturnsCopy(t *testing.T) {
	first := view.Regions()
	first[0].Name = "changed"
	if view.Regions()[0].Name == "changed" {
		t.Fatal("Regions shares its slice")
	}
}

func TestRegionUnknown(t *testing.T) {
	in := baseInput(t)
	r := view.NewRenderer()
	for _, name := range []string{"nope", "chips", "ico", "page", ""} {
		out, err := r.RenderRegion(in, name)
		if err == nil {
			t.Errorf("RenderRegion(%q) gave no error", name)
			continue
		}
		if len(out) != 0 {
			t.Errorf("RenderRegion(%q) gave output with an error", name)
		}
		if !strings.Contains(err.Error(), strconv.Quote(name)) {
			t.Errorf("error %q does not name %q", err, name)
		}
	}
}

func TestRegionProgramInput(t *testing.T) {
	_, in := programInput(t)
	out, err := view.NewRenderer().RenderRegion(in, view.Regions()[0].Name)
	if err == nil {
		t.Fatal("RenderRegion gave no error for a program input")
	}
	if len(out) != 0 {
		t.Error("RenderRegion gave output with an error")
	}
}

package excalidraw_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/plan/infrastructure/excalidraw"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	kindsScene  = "testdata/kinds.excalidraw"
	kindsGolden = "testdata/kinds.svg.golden"
	plainStroke = `stroke="#1e1e1e"`
)

var kinds = []string{"rectangle", "ellipse", "diamond", "line", "arrow", "text", "freedraw"}

func TestRenderSVG(t *testing.T) {
	scene, err := os.ReadFile(kindsScene)
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile(kindsGolden)
	if err != nil {
		t.Fatal(err)
	}

	first, err := excalidraw.RenderSVG(scene)
	if err != nil {
		t.Fatalf("RenderSVG: %v", err)
	}
	second, err := excalidraw.RenderSVG(scene)
	if err != nil {
		t.Fatalf("RenderSVG again: %v", err)
	}

	if string(first) != string(second) {
		t.Error("two renders of the same scene differ")
	}
	if string(first) != string(golden) {
		t.Errorf("render differs from %s\n%s", kindsGolden, first)
	}
	if !strings.Contains(string(first), `font-family="Excalifont"`) {
		t.Error("text does not use the font family Excalifont")
	}
}

func TestRenderSVGChange(t *testing.T) {
	marks := []struct {
		mark  shareddomain.Mark
		color string
	}{
		{shareddomain.MarkNew, "#00b34e"},
		{shareddomain.MarkChanged, "#e0a84a"},
		{shareddomain.MarkRemoved, "#e2001f"},
	}
	for _, m := range marks {
		t.Run(string(m.mark), func(t *testing.T) {
			scene := `{"elements":[{"id":"box","type":"rectangle","x":0,"y":0,"width":40,"height":20,"seed":3,"roughness":1,"strokeWidth":1,"customData":{"change":"` + string(m.mark) + `"}}]}`
			svg, err := excalidraw.RenderSVG([]byte(scene))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(svg), `stroke="`+m.color+`"`) {
				t.Errorf("no stroke %s in %s", m.color, svg)
			}
			if strings.Contains(string(svg), plainStroke) {
				t.Errorf("marked element keeps the plain stroke: %s", svg)
			}
		})
	}

	t.Run("unmarked element keeps the plain stroke", func(t *testing.T) {
		svg, err := excalidraw.RenderSVG([]byte(`{"elements":[{"id":"box","type":"rectangle","width":40,"height":20,"seed":3}]}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(svg), plainStroke) {
			t.Errorf("no plain stroke in %s", svg)
		}
	})

	t.Run("every kind renders", func(t *testing.T) {
		for _, kind := range kinds {
			scene := `{"elements":[{"id":"e","type":"` + kind + `","x":1,"y":1,"width":30,"height":30,"seed":5,"fontSize":16,"text":"t","points":[[0,0],[10,5],[20,0]]}]}`
			if _, err := excalidraw.RenderSVG([]byte(scene)); err != nil {
				t.Errorf("%s: %v", kind, err)
			}
		}
	})

	t.Run("unknown kind names the element id", func(t *testing.T) {
		_, err := excalidraw.RenderSVG([]byte(`{"elements":[{"id":"pic-7","type":"image"}]}`))
		if !errors.Is(err, excalidraw.ErrUnknownElement) {
			t.Fatalf("error = %v, want ErrUnknownElement", err)
		}
		if !strings.Contains(err.Error(), "pic-7") {
			t.Errorf("error %q does not name the element id", err)
		}
	})

	t.Run("a hostile color stays inside its attribute", func(t *testing.T) {
		scene := `{"elements":[{"id":"box","type":"rectangle","width":40,"height":20,"seed":3,"fillStyle":"solid","backgroundColor":"#fff\" onmouseover=\"alert(1)"}]}`
		svg, err := excalidraw.RenderSVG([]byte(scene))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(svg), `onmouseover="`) {
			t.Errorf("the color adds an attribute: %s", svg)
		}
	})

	t.Run("negative coordinates stay in the view box", func(t *testing.T) {
		svg, err := excalidraw.RenderSVG([]byte(`{"elements":[{"id":"box","type":"rectangle","x":-200,"y":-100,"width":40,"height":20,"seed":3}]}`))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(svg), `viewBox="-220 -120 80 60"`) {
			t.Errorf("view box does not hold the element: %s", svg)
		}
	})

	t.Run("a deleted element is not drawn", func(t *testing.T) {
		svg, err := excalidraw.RenderSVG([]byte(`{"elements":[{"id":"gone","type":"rectangle","width":40,"height":20,"seed":3,"isDeleted":true}]}`))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(svg), `data-id="gone"`) {
			t.Errorf("deleted element drawn: %s", svg)
		}
	})

	t.Run("invalid JSON fails", func(t *testing.T) {
		if _, err := excalidraw.RenderSVG([]byte(`{`)); err == nil {
			t.Error("want an error for invalid JSON")
		}
	})
}

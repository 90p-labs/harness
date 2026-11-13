package webkit

import (
	"strings"
	"testing"
)

// A run-only swap re-renders the specs region with empty diagram divs, and
// view.js renders mermaid on load only. The morph must keep rendered diagrams
// and the open state of details.
func TestPanelMorphKeepsDiagrams(t *testing.T) {
	src := string(panelJS)
	for _, want := range []string{
		"Idiomorph.defaults.callbacks.beforeNodeMorphed",
		".mm[data-diagram]",
		"data-diagram",
		"Idiomorph.defaults.callbacks.beforeAttributeUpdated",
		"DETAILS",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("panel.js does not contain %q", want)
		}
	}
}

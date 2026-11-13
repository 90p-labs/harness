package webkit

import (
	"strings"
	"testing"
)

// The poll morph must not reset a node that the page marks with data-morph-keep,
// such as the theme toggle whose label the page script set.
func TestPanelMorphKeepsMarkedNodes(t *testing.T) {
	src := string(panelJS)
	for _, want := range []string{
		"data-morph-keep",
		"hasAttribute",
		"oldNode.hasAttribute(KEEP_ATTR)",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("panel.js does not contain %q", want)
		}
	}
	// The callback must return false for a kept node, or the morph still resets it.
	if !strings.Contains(src, "oldNode.hasAttribute(KEEP_ATTR)) return false;") {
		t.Errorf("panel.js does not return false for a kept node")
	}
	if !strings.Contains(src, `KEEP_ATTR = "data-morph-keep"`) {
		t.Errorf("panel.js does not name the keep attribute")
	}
}

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

// The stream sets the live flag, so the poll stops while it is open, and the
// body carries data-live for the live dot.
func TestPanelScriptLive(t *testing.T) {
	src := string(panelJS)
	for _, want := range []string{
		`"htmx:sseOpen"`,
		`"htmx:sseError"`,
		"window.harnessLive = true;",
		"window.harnessLive = false;",
		`document.body.dataset.live = "on";`,
		`document.body.dataset.live = "off";`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("panel.js does not contain %q", want)
		}
	}
}

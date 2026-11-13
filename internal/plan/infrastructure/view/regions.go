package view

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/90p-labs/harness/internal/plan/domain"
)

// Region names a block of the plan page and the id of its element.
type Region struct{ Name, ID string }

var regions = []Region{
	{"status", "top-status"},
	{"summary", anchorSummary},
	{"criteria", anchorCriteria},
	{"notes", anchorNotes},
	{"specs", anchorSpecs},
	{"tasks", anchorTasks},
	{"trees", anchorTrees},
	{"delivery", anchorDelivery},
	{"problems", anchorProblems},
	{"trace-matrix", anchorMatrix},
	{"proofs", anchorProofs},
	{"source", anchorSource},
	{"signals", anchorSignals},
}

// Regions lists the regions in page order.
func Regions() []Region {
	return append([]Region(nil), regions...)
}

// RenderRegion renders one region of the plan page. Only a registered name is
// valid, and a program input has no regions.
func (Renderer) RenderRegion(in domain.ViewInput, name string) ([]byte, error) {
	if !slices.ContainsFunc(regions, func(r Region) bool { return r.Name == name }) {
		return nil, fmt.Errorf("unknown plan region %q", name)
	}
	if in.Program != nil {
		return nil, fmt.Errorf("plan region %q: a program page has no regions", name)
	}
	// A region reads the model only, so it skips the 4 MB of inline assets.
	p := page{model: newBuilder(in).model()}
	var buf bytes.Buffer
	if err := pageTemplate.ExecuteTemplate(&buf, "plan/"+name, p); err != nil {
		return nil, fmt.Errorf("render plan region %q: %w", name, err)
	}
	return buf.Bytes(), nil
}

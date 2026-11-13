package domain

import "errors"

// ErrUnknownRegion marks a region name outside the registry of the plan page.
var ErrUnknownRegion = errors.New("unknown region")

// ErrNoRegions marks a page that has no regions, such as a program page.
var ErrNoRegions = errors.New("page has no regions")

// ViewServed switches the page to the served mode. Version goes in each
// region as data-v, and Base is the route prefix of the region fragments.
// Home is the link of the Runs crumb; an empty Home keeps the crumb a plain label.
// Events is the route of the event stream. When set, the body connects to it and
// each region refetches on its signal; the poll stays as the fallback.
type ViewServed struct{ Version, Base, Home, Events string }

type ViewRun struct {
	ID     string
	Phase  string
	Track  string
	Branch string
}

type ViewInput struct {
	Doc      Document
	Program  *Program
	Problems []Problem
	Links    Links
	Run      ViewRun
	Branch   *Branch
	Proofs   map[int]bool
	Charts   map[int]string
	Sketches map[string][]byte
	// Served is nil in the file mode, which keeps the bytes of the page.
	Served *ViewServed
}

type ViewRenderer interface {
	Render(in ViewInput) ([]byte, error)
}

// ViewRegionRenderer renders one region of the plan page.
type ViewRegionRenderer interface {
	RenderRegion(in ViewInput, name string) ([]byte, error)
}

type ChartConverter interface {
	StateChart(body string) (string, error)
}

type SketchRenderer interface {
	RenderSVG(scene []byte) ([]byte, error)
}

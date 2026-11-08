package domain

type ViewRun struct {
	ID     string
	Phase  string
	Track  string
	Branch string
}

type ViewInput struct {
	Doc      Document
	Problems []Problem
	Links    Links
	Run      ViewRun
	Branch   *Branch
	Proofs   map[int]bool
	Charts   map[int]string
	Sketches map[string][]byte
}

type ViewRenderer interface {
	Render(in ViewInput) ([]byte, error)
}

type ChartConverter interface {
	StateChart(body string) (string, error)
}

type SketchRenderer interface {
	RenderSVG(scene []byte) ([]byte, error)
}

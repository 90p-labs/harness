package domain

const (
	DiagramState = "stateDiagram-v2"
	DiagramClass = "classDiagram"
)

type Diagram struct {
	Kind string
	Body string
	Line int
}

func (d Diagram) Checked() bool {
	return d.Kind == DiagramState || d.Kind == DiagramClass
}

type DiagramParser interface {
	Parse(kind, body string) error
}

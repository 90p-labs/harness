package domain

type SourceReader interface {
	ReadSource(path string) ([]byte, error)
}

type Tree struct {
	Files  []string
	Reader SourceReader
}

func (t Tree) Source(path string) ([]byte, error) {
	return t.Reader.ReadSource(path)
}

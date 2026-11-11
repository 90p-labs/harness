package domain

type Action string

const (
	Wrote   Action = "wrote"
	Kept    Action = "kept"
	Differs Action = "differs"
)

type Change struct {
	Path   string
	Action Action
}

// Kind tells what a path holds in the target folder.
type Kind int

const (
	Missing Kind = iota
	Folder
	Symlink
	File
)

// Entry is the answer of Lookup. Data is set only for a File.
type Entry struct {
	Kind Kind
	Data []byte
}

// Workspace is the port to the target folder. A symlink is never followed.
type Workspace interface {
	Lookup(path string) (Entry, error)
	Write(path string, data []byte) error
}

// Opener roots a Workspace in a target folder.
type Opener func(target string) Workspace

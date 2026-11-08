package domain

type Import struct {
	From      string
	To        string
	Specifier string
	TypeOnly  bool
	Package   bool
	Line      int
}

type Graph struct {
	Edges []Import
}

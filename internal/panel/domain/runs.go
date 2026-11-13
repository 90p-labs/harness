package domain

import "context"

// Served carries the version and the route base a served page polls with.
type Served struct {
	Version string
	Base    string
	// Events is the route of the event stream of the run.
	Events string
}

// Runs lists the runs of the current repo.
type Runs interface {
	List(ctx context.Context) ([]Card, error)
	Get(ctx context.Context, id string) (Card, error)
}

// Pages renders the page of a run, or one region of it.
type Pages interface {
	Page(ctx context.Context, card Card, served Served) ([]byte, error)
	Region(ctx context.Context, card Card, name string, served Served) ([]byte, error)
	// HasRegion reports whether name is a region of the page, so a query can refuse
	// an unknown name before it compares any version.
	HasRegion(name string) bool
}

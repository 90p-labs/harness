package domain

import (
	"context"
	"time"
)

type Commit struct {
	SHA     string
	Subject string
}

type Base struct {
	Ref string
	SHA string
}

type Outcome struct {
	Result   Result
	Duration time.Duration
	Tail     string
	Stdout   string
	TimedOut bool
}

type GateSource interface {
	Gates(ctx context.Context) ([]Gate, error)
}

type Hasher interface {
	InputHash(ctx context.Context, dir string, gate Gate) (string, error)
}

// BatchHasher hashes many gates from one tree snapshot, one hash per gate in gate order.
type BatchHasher interface {
	InputHashes(ctx context.Context, dir string, gates []Gate) ([]string, error)
}

type Runner interface {
	Run(ctx context.Context, dir string, gate Gate) (Outcome, error)
}

type Commits interface {
	Head(ctx context.Context, dir string) (string, error)
	ResolveBase(ctx context.Context, dir, base string) (Base, error)
	List(ctx context.Context, dir, base string) ([]Commit, error)
	AddWorktree(ctx context.Context, dir, sha string) (string, error)
	RemoveWorktree(ctx context.Context, dir, path string) error
}

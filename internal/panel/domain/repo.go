package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const repoKeyLen = 12

// Repo is one repo that started or used the panel.
type Repo struct {
	Key       string    `json:"key"`
	Name      string    `json:"name"`
	CommonDir string    `json:"common_dir"`
	Worktree  string    `json:"worktree"`
	StateDB   string    `json:"state_db"`
	UsedAt    time.Time `json:"used_at"`
}

// RepoKey returns the first 12 hex digits of the sha256 of the absolute common dir.
func RepoKey(commonDir string) string {
	sum := sha256.Sum256([]byte(commonDir))
	return hex.EncodeToString(sum[:])[:repoKeyLen]
}

// ParseRepoKey accepts only 12 lowercase hex digits.
func ParseRepoKey(s string) (string, error) {
	if len(s) != repoKeyLen {
		return "", fmt.Errorf("repo key %q: want %d lowercase hex digits", s, repoKeyLen)
	}
	for _, c := range []byte(s) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", fmt.Errorf("repo key %q: want %d lowercase hex digits", s, repoKeyLen)
		}
	}
	return s, nil
}

// Registry stores the repos that use the panel.
type Registry interface {
	// Register adds the repo, or refreshes its worktree and last use time.
	Register(ctx context.Context, repo Repo) error
	// List returns the live repos, newest use first, and drops the stale ones.
	List(ctx context.Context) ([]Repo, error)
}

// RunRef names one run of one repo.
type RunRef struct {
	Repo string
	ID   string
}

// Listing is what the board reads: the repos, the cards of all of them,
// and one problem line for each repo key that failed to load.
type Listing struct {
	Repos    []Repo
	Cards    []Card
	Problems map[string]string
}

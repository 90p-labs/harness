package gittree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/git"
)

const (
	indexFileEnv       = "GIT_INDEX_FILE="
	indexFileName      = "index"
	worktreeDirPattern = "harness-gate-*"
	recursiveSegment   = "**"
	segmentSeparator   = "/"
	nulSeparator       = "\x00"
	logFormat          = "--format=%H%x00%s"
	entryTabSeparator  = "\t"
	modeFieldIndex     = 0
	blobFieldIndex     = 1
	minEntryFields     = 2
)

type Tree struct{}

func (Tree) InputHash(ctx context.Context, dir string, gate domain.Gate) (string, error) {
	files, err := inputFiles(ctx, dir, gate)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{gate.Run, gate.Baseline, files}, nulSeparator)))
	return hex.EncodeToString(sum[:]), nil
}

func inputFiles(ctx context.Context, dir string, gate domain.Gate) (string, error) {
	scratch, err := os.MkdirTemp("", "harness-index-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	env := []string{indexFileEnv + filepath.Join(scratch, indexFileName)}

	if _, err := git.Run(ctx, dir, env, "add", "-A"); err != nil {
		return "", err
	}
	if len(gate.Paths) == 0 {
		return git.Run(ctx, dir, env, "write-tree")
	}
	listing, err := git.Run(ctx, dir, env, "ls-files", "-s", "-z")
	if err != nil {
		return "", err
	}

	patterns := gate.Paths
	if gate.Baseline != "" {
		patterns = append(slices.Clip(patterns), gate.Baseline)
	}
	var lines []string
	for _, entry := range strings.Split(listing, nulSeparator) {
		meta, name, found := strings.Cut(entry, entryTabSeparator)
		fields := strings.Fields(meta)
		if !found || len(fields) < minEntryFields || !matchesAny(patterns, name) {
			continue
		}
		lines = append(lines, name+" "+fields[modeFieldIndex]+" "+fields[blobFieldIndex]+"\n")
	}
	sort.Strings(lines)
	return strings.Join(lines, ""), nil
}

func (Tree) Head(ctx context.Context, dir string) (string, error) {
	return git.Head(ctx, dir)
}

func (Tree) List(ctx context.Context, dir, base string) ([]domain.Commit, error) {
	out, err := git.Run(ctx, dir, nil, "log", "--reverse", logFormat, base+"..HEAD")
	if err != nil || out == "" {
		return nil, err
	}
	var commits []domain.Commit
	for _, line := range strings.Split(out, "\n") {
		sha, subject, _ := strings.Cut(line, nulSeparator)
		commits = append(commits, domain.Commit{SHA: sha, Subject: subject})
	}
	return commits, nil
}

func (Tree) AddWorktree(ctx context.Context, dir, sha string) (string, error) {
	target, err := os.MkdirTemp("", worktreeDirPattern)
	if err != nil {
		return "", err
	}
	if _, err := git.Run(ctx, dir, nil, "worktree", "add", "--detach", target, sha); err != nil {
		_ = os.RemoveAll(target)
		return "", fmt.Errorf("add worktree for %s: %w", sha, err)
	}
	return target, nil
}

func (Tree) RemoveWorktree(ctx context.Context, dir, worktree string) error {
	_, err := git.Run(ctx, dir, nil, "worktree", "remove", "--force", worktree)
	return err
}

func matchesAny(patterns []string, name string) bool {
	segments := strings.Split(name, segmentSeparator)
	for _, pattern := range patterns {
		if matchSegments(strings.Split(pattern, segmentSeparator), segments) {
			return true
		}
	}
	return false
}

func matchSegments(pattern, name []string) bool {
	if len(pattern) == 0 {
		return len(name) == 0
	}
	if pattern[0] == recursiveSegment {
		for skip := 0; skip <= len(name); skip++ {
			if matchSegments(pattern[1:], name[skip:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], name[0])
	return err == nil && ok && matchSegments(pattern[1:], name[1:])
}

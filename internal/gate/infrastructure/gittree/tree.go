package gittree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

const (
	indexFileEnv       = "GIT_INDEX_FILE="
	indexFileName      = "index"
	worktreeDirPattern = "harness-gate-*"
	nulSeparator       = "\x00"
	logFormat          = "--format=%H%x00%s"
	entryTabSeparator  = "\t"
	modeFieldIndex     = 0
	blobFieldIndex     = 1
	minEntryFields     = 2
	remoteName         = "origin"
	remoteRefsPrefix   = "refs/remotes/"
	headRevision       = "HEAD"
	revisionMarks      = "~^@:"
	noExit             = 1
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
		if !found || len(fields) < minEntryFields || !git.MatchesAny(patterns, name) {
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

func (Tree) ResolveBase(ctx context.Context, dir, base string) (domain.Base, error) {
	local, err := mergeBase(ctx, dir, base)
	if err != nil {
		return domain.Base{}, err
	}
	if !isPlainBranch(base) {
		return domain.Base{Ref: base, SHA: local}, nil
	}
	remoteRef := remoteName + "/" + base
	exists, err := gitYes(ctx, dir, "show-ref", "--verify", "--quiet", remoteRefsPrefix+remoteRef)
	if err != nil {
		return domain.Base{}, fmt.Errorf("probe %s: %w", remoteRef, err)
	}
	if !exists {
		return domain.Base{Ref: base, SHA: local}, nil
	}
	remote, err := mergeBase(ctx, dir, remoteRef)
	if err != nil {
		return domain.Base{}, err
	}
	newer, err := gitYes(ctx, dir, "merge-base", "--is-ancestor", local, remote)
	if err != nil {
		return domain.Base{}, fmt.Errorf("compare the merge bases of %s and %s: %w", base, remoteRef, err)
	}
	if !newer {
		return domain.Base{Ref: base, SHA: local}, nil
	}
	return domain.Base{Ref: remoteRef, SHA: remote}, nil
}

// gitYes runs a git test command: exit 0 is yes, exit 1 is no, any other failure is an error.
func gitYes(ctx context.Context, dir string, args ...string) (bool, error) {
	_, err := git.Run(ctx, dir, nil, args...)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == noExit {
		return false, nil
	}
	return err == nil, err
}

func isPlainBranch(base string) bool {
	return base != headRevision && !strings.ContainsAny(base, revisionMarks) && !strings.HasPrefix(base, remoteName+"/")
}

func mergeBase(ctx context.Context, dir, ref string) (string, error) {
	sha, err := git.Run(ctx, dir, nil, "merge-base", ref, headRevision)
	if err != nil {
		return "", fmt.Errorf("merge base of %s and %s: %w", ref, headRevision, err)
	}
	return sha, nil
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

package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrNotRepository = errors.New("not in a git repository")
	ErrNoHead        = errors.New("repository has no commit")
)

const (
	notRepositoryMessage = "not a git repository"
	schemeSeparator      = "://"
	userSeparator        = "@"
	hostPathSeparators   = "/:"
	pathSeparator        = "/"
	scpSeparator         = ':'
	gitSuffix            = ".git"
	missingConfigExit    = 1
	quietMissingRefExit  = 1
	originRemotePrefix   = "origin/"
	fallbackBranch       = "main"
)

func DefaultBranch(ctx context.Context, dir string) (string, error) {
	out, err := Run(ctx, dir, nil, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == quietMissingRefExit {
		return fallbackBranch, nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(out, originRemotePrefix), nil
}

func TopLevel(ctx context.Context, dir string) (string, error) {
	out, err := Run(ctx, dir, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return out, nil
}

func CommonDir(ctx context.Context, dir string) (string, error) {
	out, err := Run(ctx, dir, nil, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return absoluteFrom(dir, out)
}

// RepoKey returns the normalized origin URL, or the common dir without an origin, and the worktree top level.
func RepoKey(ctx context.Context, dir string) (repo, worktree string, err error) {
	out, err := Run(ctx, dir, nil, "rev-parse", "--show-toplevel", "--git-common-dir")
	if err != nil {
		return "", "", err
	}
	worktree, commonDir, _ := strings.Cut(out, "\n")

	origin, err := Run(ctx, dir, nil, "config", "--get", "remote.origin.url")
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == missingConfigExit {
		commonDir, err = absoluteFrom(dir, strings.TrimSpace(commonDir))
		return commonDir, worktree, err
	}
	if err != nil {
		return "", "", err
	}
	return normalizeOrigin(origin), worktree, nil
}

// git prints the common dir of the main worktree relative to dir; git 2.31 adds --path-format=absolute.
func absoluteFrom(dir, path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.EvalSymlinks(filepath.Join(dir, path))
}

func normalizeOrigin(url string) string {
	rest, hasScheme := url, false
	if _, afterScheme, found := strings.Cut(rest, schemeSeparator); found {
		rest = afterScheme
		hasScheme = true
	}
	authority, _, _ := strings.Cut(rest, pathSeparator)
	if user := strings.Index(authority, userSeparator); user >= 0 {
		rest = rest[user+1:]
	}
	i := strings.IndexAny(rest, hostPathSeparators)
	if i < 0 {
		return strings.TrimSuffix(rest, gitSuffix)
	}
	host, separator, path := rest[:i], rest[i], rest[i+1:]
	if hasScheme && separator == scpSeparator {
		_, path, _ = strings.Cut(path, pathSeparator)
	}
	return host + pathSeparator + strings.TrimSuffix(strings.TrimSuffix(path, pathSeparator), gitSuffix)
}

func Head(ctx context.Context, dir string) (string, error) {
	out, err := Run(ctx, dir, nil, "rev-parse", "--verify", "--quiet", "HEAD")
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == quietMissingRefExit {
		return "", fmt.Errorf("%w in %s", ErrNoHead, dir)
	}
	return out, err
}

func Run(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// git translates its messages, and the C locale keeps the matched text stable.
	cmd.Env = append(append(os.Environ(), "LC_ALL=C"), env...)
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && bytes.Contains(exitErr.Stderr, []byte(notRepositoryMessage)) {
		return "", ErrNotRepository
	}
	if exitErr != nil {
		return "", fmt.Errorf("git %s in %s: %w: %s", args[0], dir, err, bytes.TrimSpace(exitErr.Stderr))
	}
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w", args[0], dir, err)
	}
	return string(bytes.TrimSpace(out)), nil
}

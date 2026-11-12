package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

var (
	ErrNotRepository = errors.New("not in a git repository")
	ErrNoHead        = errors.New("repository has no commit")
	// ErrNotBranch means a name is neither a local branch nor an origin branch.
	ErrNotBranch = errors.New("not a branch")

	defaultBranchGuesses = []string{fallbackBranch, "master"}
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
	nulSeparator         = "\x00"
	numstatMinColumns    = 3
)

func DefaultBranch(ctx context.Context, dir string) (string, error) {
	out, err := Run(ctx, dir, nil, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == quietMissingRefExit {
		return guessedDefaultBranch(ctx, dir)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(out, originRemotePrefix), nil
}

// guessedDefaultBranch is the first of main and master that exists, for a repository with no origin/HEAD.
func guessedDefaultBranch(ctx context.Context, dir string) (string, error) {
	for _, name := range defaultBranchGuesses {
		_, err := BranchRef(ctx, dir, name)
		if err == nil {
			return name, nil
		}
		if !errors.Is(err, ErrNotBranch) {
			return "", err
		}
	}
	return fallbackBranch, nil
}

// BranchRef returns the full ref of a branch: refs/heads/<name>, else refs/remotes/origin/<name>.
// A revision such as HEAD~1 or a tag is an error, so a diff base cannot hide earlier changes.
// The full ref also stops a tag of the same name from winning in git's ref resolution.
func BranchRef(ctx context.Context, dir, name string) (string, error) {
	for _, ref := range []string{"refs/heads/" + name, "refs/remotes/" + originRemotePrefix + name} {
		_, err := Run(ctx, dir, nil, "show-ref", "--verify", "--quiet", ref)
		if err == nil {
			return ref, nil
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != quietMissingRefExit {
			return "", err
		}
	}
	return "", fmt.Errorf("%w: %s", ErrNotBranch, name)
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

func Files(ctx context.Context, dir string, globs ...string) ([]string, error) {
	out, err := Run(ctx, dir, nil, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	deleted, err := Run(ctx, dir, nil, "ls-files", "-z", "--deleted")
	if err != nil {
		return nil, err
	}
	gone := map[string]bool{}
	for _, name := range strings.Split(deleted, nulSeparator) {
		gone[name] = true
	}
	var files []string
	for _, name := range strings.Split(out, nulSeparator) {
		if name != "" && !gone[name] && (len(globs) == 0 || MatchesAny(globs, name)) {
			files = append(files, name)
		}
	}
	return files, nil
}

func PatchID(ctx context.Context, dir, base string) (string, error) {
	mergeBase, err := Run(ctx, dir, nil, "merge-base", base, "HEAD")
	if err != nil {
		return "", err
	}
	diff, err := run(ctx, dir, nil, "", "diff", mergeBase, "HEAD")
	if err != nil || diff == "" {
		return "", err
	}
	out, err := run(ctx, dir, nil, diff, "patch-id", "--verbatim")
	id, _, _ := strings.Cut(out, " ")
	return id, err
}

func ChangedFiles(ctx context.Context, dir, base string) ([]string, error) {
	return changedFilesWith(ctx, dir, nil, nil, "diff", "--name-only", "-z", base+"...HEAD")
}

// ChangedFilesNoRenames lists the changed names without rename detection, so a moved file names its old path too.
func ChangedFilesNoRenames(ctx context.Context, dir, base string) ([]string, error) {
	// A replace ref, a graft or a forged commit graph can rewrite history, so the list reads the real parents.
	// Local diff.ignoreSubmodules and diff.relative can hide or shorten a path, so the args override them.
	// The -c flag turns the commit graph off and works on git before 2.31, which reads no GIT_CONFIG_COUNT env.
	env := []string{
		"GIT_NO_REPLACE_OBJECTS=1",
		"GIT_GRAFT_FILE=" + os.DevNull,
	}
	// An inherited GIT_DIR or object path can point the diff at another history, so git finds the repo from dir.
	// Config variables stay, because the args already override the diff options and GIT_CONFIG_GLOBAL holds the user setup.
	return changedFilesWith(ctx, dir, env, locationVars, "-c", "core.commitGraph=false", "diff", "--name-only", "--no-renames", "--ignore-submodules=none", "--no-relative", "-z", base+"...HEAD")
}

// locationVars are the variables that tell git where the repository, its objects and its index are.
var locationVars = []string{
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_COMMON_DIR",
	"GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_INDEX_FILE",
	"GIT_NAMESPACE",
	"GIT_CEILING_DIRECTORIES",
	"GIT_DISCOVERY_ACROSS_FILESYSTEM",
}

func changedFilesWith(ctx context.Context, dir string, env, unset []string, args ...string) ([]string, error) {
	out, err := runWith(ctx, dir, env, unset, "", args...)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, name := range strings.Split(out, nulSeparator) {
		if name != "" {
			files = append(files, name)
		}
	}
	return files, nil
}

func Run(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	out, err := run(ctx, dir, env, "", args...)
	return strings.TrimSpace(out), err
}

func run(ctx context.Context, dir string, env []string, stdin string, args ...string) (string, error) {
	return runWith(ctx, dir, env, nil, stdin, args...)
}

// runWith drops the unset variables from the inherited environment before it adds env.
func runWith(ctx context.Context, dir string, env, unset []string, stdin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	// git translates its messages, and the C locale keeps the matched text stable.
	inherited := slices.DeleteFunc(os.Environ(), func(kv string) bool {
		for _, key := range unset {
			if strings.HasPrefix(kv, key+"=") {
				return true
			}
		}
		return false
	})
	cmd.Env = append(append(inherited, "LC_ALL=C"), env...)
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && bytes.Contains(exitErr.Stderr, []byte(notRepositoryMessage)) {
		return "", ErrNotRepository
	}
	if exitErr != nil {
		return "", fmt.Errorf("git %s in %s: %w: %s", subcommand(args), dir, err, bytes.TrimSpace(exitErr.Stderr))
	}
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w", subcommand(args), dir, err)
	}
	return string(out), nil
}

// subcommand skips leading `-c name=value` pairs, so an error names the command and not the flag.
func subcommand(args []string) string {
	for len(args) >= 2 && args[0] == "-c" {
		args = args[2:]
	}
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// DiffLines returns the added plus deleted lines of `git diff --numstat` from a commit to HEAD.
// A binary file has no line count, so it is an error and the caller treats the size as unknown.
func DiffLines(ctx context.Context, dir, from string) (int, error) {
	out, err := Run(ctx, dir, nil, "diff", "--numstat", from, "HEAD")
	if err != nil {
		return 0, err
	}
	total := 0
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < numstatMinColumns {
			return 0, fmt.Errorf("git diff --numstat: unexpected line %q", line)
		}
		for _, field := range fields[:2] {
			count, err := strconv.Atoi(field)
			if err != nil {
				return 0, fmt.Errorf("git diff --numstat: no line count in %q", line)
			}
			total += count
		}
	}
	return total, nil
}

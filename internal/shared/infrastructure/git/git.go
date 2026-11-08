package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var ErrNotRepository = errors.New("not in a git repository")

const notRepositoryMessage = "not a git repository"

func TopLevel(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	// git translates its messages, and the C locale keeps the matched text stable.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && bytes.Contains(exitErr.Stderr, []byte(notRepositoryMessage)) {
		return "", ErrNotRepository
	}
	if exitErr != nil {
		return "", fmt.Errorf("git rev-parse in %s: %w: %s", dir, err, bytes.TrimSpace(exitErr.Stderr))
	}
	if err != nil {
		return "", fmt.Errorf("git rev-parse in %s: %w", dir, err)
	}
	return string(bytes.TrimSpace(out)), nil
}

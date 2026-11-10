package handoff

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type File struct{}

var _ domain.HandoffReader = File{}

func (File) ReadHandoff(_ context.Context, worktree, runID string) ([]byte, error) {
	name := shareddomain.RunHandoffFile(runID)
	path := filepath.Join(worktree, name)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	if info.Size() > domain.HandoffLimit {
		return nil, fmt.Errorf(domain.HandoffSizeFormat, info.Size(), domain.HandoffLimit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(io.LimitReader(file, domain.HandoffLimit+1))
}

package findings

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type File struct{}

var _ domain.FindingsReader = File{}

func (File) Read(_ context.Context, run domain.Run) ([]shareddomain.ReviewFinding, error) {
	name := shareddomain.RunFindingsFile(run.ID)
	data, err := os.ReadFile(filepath.Join(run.Key.Worktree, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s does not exist: write the findings table there, then send the rework again", name)
	}
	if err != nil {
		return nil, err
	}
	findings, err := shareddomain.ParseReviewFindings(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return findings, nil
}

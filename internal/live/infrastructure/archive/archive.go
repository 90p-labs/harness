package archive

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/jorge07/harness/internal/live/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	planFile   = "plan"
	reviewFile = "review"
	extension  = ".md"
	fileMode   = 0o644
)

type Folder struct{}

func New() Folder {
	return Folder{}
}

func (Folder) Archive(run domain.LiveRun, round int) error {
	dir := filepath.Join(run.Worktree, filepath.FromSlash(path.Dir(shareddomain.RunPlanFile(run.ID))))
	type copyJob struct{ source, target string }
	var jobs []copyJob
	for _, name := range []string{planFile, reviewFile} {
		target := filepath.Join(dir, fmt.Sprintf("%s-%d%s", name, round, extension))
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("%w: %s", domain.ErrArchiveExists, target)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		jobs = append(jobs, copyJob{source: filepath.Join(dir, name+extension), target: target})
	}
	for _, job := range jobs {
		body, err := os.ReadFile(job.source)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err := os.WriteFile(job.target, body, fileMode); err != nil {
			return err
		}
	}
	return nil
}

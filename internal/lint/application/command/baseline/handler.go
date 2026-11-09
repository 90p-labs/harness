package baseline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	dirMode     = 0o750
	fileMode    = 0o600
	indent      = "  "
	parseFormat = "baseline %s: %w"
)

type WriteBaseline struct {
	Dir  string
	Path string
}

type Lint func(ctx context.Context, dir string) (shareddomain.Report, error)

type Handler struct {
	lint Lint
}

func NewHandler(lint Lint) Handler {
	return Handler{lint: lint}
}

func (h Handler) Handle(ctx context.Context, cmd WriteBaseline) (int, error) {
	current, err := h.lint(ctx, cmd.Dir)
	if err != nil {
		return 0, err
	}
	path := filepath.Join(cmd.Dir, cmd.Path)
	kept := current.Findings
	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return 0, err
	default:
		var existing shareddomain.Report
		if err := json.Unmarshal(raw, &existing); err != nil {
			return 0, fmt.Errorf(parseFormat, cmd.Path, err)
		}
		still := map[shareddomain.FindingKey]bool{}
		for _, f := range current.Findings {
			still[f.Key()] = true
		}
		kept = []shareddomain.Finding{}
		for _, f := range existing.Findings {
			if still[f.Key()] {
				kept = append(kept, f)
			}
		}
	}
	out, err := json.MarshalIndent(shareddomain.Report{Version: shareddomain.ReportVersion, Findings: kept}, "", indent)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return 0, err
	}
	return len(kept), os.WriteFile(path, append(out, '\n'), fileMode)
}

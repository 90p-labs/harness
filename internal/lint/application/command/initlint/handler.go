package initlint

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/90p-labs/harness/internal/lint/domain"
)

const (
	dirMode  = 0o750
	fileMode = 0o600

	engineErrorFormat = "prepare lint engine %s: %w"
)

type Command struct {
	Dir string
}

type Lookup func(domain.PackName) (domain.Pack, error)

type Handler struct {
	packs   []domain.PackName
	lookup  Lookup
	engines domain.EngineSet
	version string
}

func NewHandler(packs []domain.PackName, lookup Lookup, engines domain.EngineSet, version string) Handler {
	return Handler{packs: packs, lookup: lookup, engines: engines, version: version}
}

func (h Handler) Handle(ctx context.Context, cmd Command) error {
	byEngine, err := domain.RulesByEngine(h.packs, h.lookup)
	if err != nil {
		return err
	}
	_, err = os.Stat(filepath.Join(cmd.Dir, domain.OldBaselinePath))
	switch {
	case err == nil:
		return domain.ErrOldBaseline
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	lintDir := filepath.Join(cmd.Dir, domain.LintDir)
	if err := clearKeepingBaseline(lintDir); err != nil {
		return err
	}
	if err := os.MkdirAll(lintDir, dirMode); err != nil {
		return err
	}
	for _, engine := range slices.Sorted(maps.Keys(byEngine)) {
		if err := h.engines.Prepare(ctx, cmd.Dir, engine, byEngine[engine]); err != nil {
			return fmt.Errorf(engineErrorFormat, engine, err)
		}
	}
	return os.WriteFile(filepath.Join(lintDir, domain.VersionFile), []byte(h.version), fileMode)
}

func clearKeepingBaseline(lintDir string) error {
	entries, err := os.ReadDir(lintDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == domain.BaselineFile {
			continue
		}
		if err := os.RemoveAll(filepath.Join(lintDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

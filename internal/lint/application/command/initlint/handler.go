package initlint

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/jorge07/harness/internal/lint/domain"
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
	lintDir := filepath.Join(cmd.Dir, domain.LintDir)
	if err := os.RemoveAll(lintDir); err != nil {
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

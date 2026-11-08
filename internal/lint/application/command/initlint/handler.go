package initlint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	engines []domain.Engine
	version string
}

func NewHandler(packs []domain.PackName, lookup Lookup, engines []domain.Engine, version string) Handler {
	return Handler{packs: packs, lookup: lookup, engines: engines, version: version}
}

func (h Handler) Handle(_ context.Context, cmd Command) error {
	byEngine, err := domain.RulesByEngine(h.packs, h.lookup)
	if err != nil {
		return err
	}
	lintDir := filepath.Join(cmd.Dir, domain.LintDir)
	if err := os.RemoveAll(lintDir); err != nil {
		return err
	}
	for _, engine := range h.engines {
		if len(byEngine[engine.Name()]) == 0 {
			continue
		}
		if err := engine.Prepare(cmd.Dir, byEngine[engine.Name()]); err != nil {
			return fmt.Errorf(engineErrorFormat, engine.Name(), err)
		}
	}
	if err := os.MkdirAll(lintDir, dirMode); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(lintDir, domain.VersionFile), []byte(h.version), fileMode)
}

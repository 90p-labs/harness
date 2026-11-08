//go:build linters

package lint_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorge07/harness/internal/deploy/infrastructure/toolplugin"
	lintmodule "github.com/jorge07/harness/internal/lint"
	"github.com/jorge07/harness/internal/lint/application/command/initlint"
	lintcommand "github.com/jorge07/harness/internal/lint/application/command/lint"
	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/fixture"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
)

const (
	customPackVersion = "v0.0.1"
	customRuleID      = "team/no-console"
	customRuleFile    = "src/orders/infrastructure/order.adapter.ts"
)

func TestCustomPackFromFolder(t *testing.T) {
	dir := fixture.Extract(t, filepath.Join("testdata", "custom-pack.txtar"))
	handlers, err := lintmodule.New(config.Lint{Packs: []string{"./lint/team"}}, nil, dir, customPackVersion, toolplugin.Template)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()

	if err := handlers.Init.Handle(ctx, initlint.Command{Dir: dir}); err != nil {
		t.Fatalf("lint init: %v", err)
	}
	report, err := handlers.Lint.Handle(ctx, lintcommand.Command{Dir: dir})

	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %+v, want one %s finding", report.Findings, customRuleID)
	}
	if got := report.Findings[0]; got.Rule != customRuleID || got.File != customRuleFile || got.Line != 2 {
		t.Fatalf("finding = %+v, want %s at %s:2", got, customRuleID, customRuleFile)
	}
}

func TestCustomPackEditIsStale(t *testing.T) {
	dir := fixture.Extract(t, filepath.Join("testdata", "custom-pack.txtar"))
	cfg := config.Lint{Packs: []string{"./lint/team"}}
	handlers, err := lintmodule.New(cfg, nil, dir, customPackVersion, toolplugin.Template)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	if err := handlers.Init.Handle(ctx, initlint.Command{Dir: dir}); err != nil {
		t.Fatalf("lint init: %v", err)
	}
	plugin := filepath.Join(dir, "lint", "team", "rules", "no-console.grit")
	source, err := os.ReadFile(plugin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plugin, append(source, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	edited, err := lintmodule.New(cfg, nil, dir, customPackVersion, toolplugin.Template)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = edited.Lint.Handle(ctx, lintcommand.Command{Dir: dir})

	if !errors.Is(err, domain.ErrStaleLintFiles) {
		t.Fatalf("lint after a plugin edit = %v, want ErrStaleLintFiles", err)
	}
}

package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jorge07/harness/internal/deploy"
	gatemodule "github.com/jorge07/harness/internal/gate"
	"github.com/jorge07/harness/internal/gate/application/command/initgates"
	gatedomain "github.com/jorge07/harness/internal/gate/domain"
	gateconfig "github.com/jorge07/harness/internal/gate/infrastructure/config"
	"github.com/jorge07/harness/internal/gate/infrastructure/toolchain"
	"github.com/jorge07/harness/internal/host"
	hostdomain "github.com/jorge07/harness/internal/host/domain"
	hostcli "github.com/jorge07/harness/internal/host/ui/cli"
	lintmodule "github.com/jorge07/harness/internal/lint"
	"github.com/jorge07/harness/internal/lint/application/command/baseline"
	"github.com/jorge07/harness/internal/lint/application/command/initlint"
	lintdomain "github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	commandInit = "init"
	flagHost    = "host"

	packageJSON    = "package.json"
	componentsJSON = "components.json"
	dependencyNext = "next"
	dependencyNest = "@nestjs/core"

	languagePHP     = "PHP"
	languagePython  = "Python"
	languageGo      = "Go"
	composerJSON    = "composer.json"
	pyprojectTOML   = "pyproject.toml"
	requirementsTXT = "requirements.txt"
	goMod           = "go.mod"

	listSeparator = ", "
	noItems       = "none"

	packsWroteFormat    = "lint packs: wrote %s\n"
	packsKeptFormat     = "lint packs: kept %s\n"
	packsNoneLine       = "lint packs: none detected\n"
	lintSetupPrefix     = "next: run the lint-setup skill"
	lintSetupFormat     = lintSetupPrefix + " to set up the architecture rules for %s\n"
	gatesWroteFormat    = "gates: wrote %s\n"
	setupWroteFormat    = "setup: wrote %s\n"
	gatesKeptLine       = "gates: kept\n"
	lintInitFormat      = "lint init: wrote %s\n"
	baselineFormat      = "lint baseline: wrote %s\n"
	baselineSkippedLine = "lint baseline: skipped, no lint pack\n"
	hostFormat          = "host: wrote %s\n"
)

var languagesWithNoPack = []struct {
	name    string
	markers []string
}{
	{languagePHP, []string{composerJSON}},
	{languagePython, []string{pyprojectTOML, requirementsTXT}},
	{languageGo, []string{goMod}},
}

var errGatesBeforePacks = errors.New("the gates key exists without lint.packs, so gates init cannot add the lint gate: " +
	"set lint.packs and the lint gate in .harness.yaml by hand, or remove the gates key and run harness init again")

func newInitCommand(flags *pflag.FlagSet, version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandInit,
		Short: "Set up the lint packs, the gates, the lint files, the lint baseline and the host hooks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			value, err := cmd.Flags().GetString(flagHost)
			if err != nil {
				return err
			}
			return initRepository(cmd.Context(), flags, shareddomain.Host(value), version, cmd.OutOrStdout())
		},
	}
	cmd.Flags().String(flagHost, "", "the host: claude or codex")
	_ = cmd.MarkFlagRequired(flagHost)
	return cmd
}

func initRepository(ctx context.Context, flags *pflag.FlagSet, agentHost shareddomain.Host, version string, out io.Writer) error {
	env, err := newEnvironment(ctx, flags)
	if err != nil {
		return err
	}
	cfg, worktree := env.cfg, env.key.Worktree
	hostHandlers, err := host.NewHostConnect(func(context.Context) (string, error) { return worktree, nil })(ctx)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(hostHandlers.Codecs, func(codec hostdomain.Codec) bool { return codec.Host() == agentHost }) {
		return asUsageError(fmt.Errorf("%w: %q", hostcli.ErrUnknownHost, agentHost))
	}

	writer := gateconfig.Writer{}
	kept := len(cfg.Lint.Packs) > 0
	if !kept {
		if cfg.Lint.Packs, err = choosePacks(worktree); err != nil {
			return err
		}
		hasGates, err := writer.HasGates(worktree)
		if err != nil {
			return err
		}
		if hasGates && len(cfg.Lint.Packs) > 0 {
			return fmt.Errorf("%w (chosen packs: %s)", errGatesBeforePacks, strings.Join(cfg.Lint.Packs, listSeparator))
		}
	}
	lint, err := lintmodule.New(cfg.Lint, cfg.Plugins, worktree, version, deploy.RenderChart(env.bus))
	if err != nil {
		return err
	}
	packs := strings.Join(cfg.Lint.Packs, listSeparator)
	switch {
	case kept:
		fmt.Fprintf(out, packsKeptFormat, packs)
	case packs == "":
		fmt.Fprint(out, packsNoneLine)
	default:
		if err := writer.WriteLintPacks(worktree, cfg.Lint.Packs); err != nil {
			return err
		}
		fmt.Fprintf(out, packsWroteFormat, packs)
	}
	languages, err := detectLanguagesWithNoManifest(worktree)
	if err != nil {
		return err
	}
	for _, language := range languages {
		fmt.Fprintf(out, lintSetupFormat, language)
	}

	written, err := gatemodule.NewInit(toolchain.Lint{Packs: cfg.Lint.Packs, Baseline: lintmodule.BaselinePath}, env.bus).
		Handle(ctx, initgates.Command{Dir: worktree})
	switch {
	case errors.Is(err, gatedomain.ErrGatesExist):
		fmt.Fprint(out, gatesKeptLine)
	case err != nil:
		return err
	default:
		if len(written.Setup) > 0 {
			steps := make([]string, len(written.Setup))
			for i, step := range written.Setup {
				steps[i] = step.ID
			}
			fmt.Fprintf(out, setupWroteFormat, strings.Join(steps, listSeparator))
		}
		ids := make([]string, len(written.Gates))
		for i, gate := range written.Gates {
			ids[i] = gate.ID
		}
		fmt.Fprintf(out, gatesWroteFormat, cmp.Or(strings.Join(ids, listSeparator), noItems))
	}

	if err := lint.Init.Handle(ctx, initlint.Command{Dir: worktree}); err != nil {
		return err
	}
	fmt.Fprintf(out, lintInitFormat, lintdomain.LintDir)
	if packs == "" {
		fmt.Fprint(out, baselineSkippedLine)
	} else {
		if err := lint.Baseline.Handle(ctx, baseline.WriteBaseline{Dir: worktree, Path: lintmodule.BaselinePath}); err != nil {
			return err
		}
		fmt.Fprintf(out, baselineFormat, lintmodule.BaselinePath)
	}
	path, err := hostHandlers.Install(ctx, agentHost)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, hostFormat, path)
	return err
}

func detectLanguagesWithNoManifest(dir string) ([]string, error) {
	_, err := os.Stat(filepath.Join(dir, lintdomain.ManifestFile))
	if err == nil {
		return nil, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", lintdomain.ManifestFile, err)
	}
	var found []string
	for _, language := range languagesWithNoPack {
		for _, marker := range language.markers {
			_, err := os.Stat(filepath.Join(dir, marker))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", marker, err)
			}
			found = append(found, language.name)
			break
		}
	}
	return found, nil
}

func choosePacks(dir string) ([]string, error) {
	var manifest struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, packageJSON))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, fmt.Errorf("read %s: %w", packageJSON, err)
	default:
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, fmt.Errorf("parse %s: %w", packageJSON, err)
		}
	}
	depends := func(name string) bool {
		_, runtime := manifest.Dependencies[name]
		_, dev := manifest.DevDependencies[name]
		return runtime || dev
	}
	components, err := os.Stat(filepath.Join(dir, componentsJSON))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", componentsJSON, err)
	}
	signals := map[lintdomain.PackName]bool{
		lintdomain.PackNextjs: depends(dependencyNext),
		lintdomain.PackDDD:    depends(dependencyNest),
		lintdomain.PackShadcn: err == nil && components.Mode().IsRegular(),
	}
	var packs []string
	for _, name := range lintdomain.AllPacks {
		if signals[name] {
			packs = append(packs, string(name))
		}
	}
	return packs, nil
}

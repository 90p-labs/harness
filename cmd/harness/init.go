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
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/90p-labs/harness/internal/deploy"
	gatemodule "github.com/90p-labs/harness/internal/gate"
	"github.com/90p-labs/harness/internal/gate/application/command/initgates"
	gatedomain "github.com/90p-labs/harness/internal/gate/domain"
	gateconfig "github.com/90p-labs/harness/internal/gate/infrastructure/config"
	"github.com/90p-labs/harness/internal/gate/infrastructure/toolchain"
	"github.com/90p-labs/harness/internal/host"
	hostdomain "github.com/90p-labs/harness/internal/host/domain"
	hostcli "github.com/90p-labs/harness/internal/host/ui/cli"
	lintmodule "github.com/90p-labs/harness/internal/lint"
	"github.com/90p-labs/harness/internal/lint/application/command/baseline"
	"github.com/90p-labs/harness/internal/lint/application/command/initlint"
	lintdomain "github.com/90p-labs/harness/internal/lint/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
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

	packsWroteFormat     = "lint packs: wrote %s\n"
	packsKeptFormat      = "lint packs: kept %s\n"
	packsNoneLine        = "lint packs: none detected\n"
	lintSetupPrefix      = "next: run the lint-setup skill"
	lintSetupFormat      = lintSetupPrefix + " to set up the architecture rules for %s\n"
	gatesWroteFormat     = "gates: wrote %s\n"
	setupWroteFormat     = "setup: wrote %s\n"
	gatesKeptLine        = "gates: kept\n"
	gatesNoneLine        = "gates: none detected\n"
	lintInitFormat       = "lint init: wrote %s\n"
	baselineFormat       = "lint baseline: wrote %s (%d findings, run harness lint to see them)\n"
	baselineSkippedLine  = "lint baseline: skipped, no lint pack\n"
	hostFormat           = "host: wrote %s\n"
	hostSkippedLine      = "host: skipped, the 90p plugin runs the hooks\n"
	hostRemovedFormat    = "host: skipped, the 90p plugin runs the hooks; removed %d harness hooks from %s\n"
	gitignoreAddedFormat = "gitignore: added %s\n"
	gitignoreKeptLine    = "gitignore: kept\n"

	gitignoreFile = ".gitignore"
	lineFeed      = "\n"
	carriageLine  = "\r\n"
)

var languagesWithNoPack = []struct {
	name      string
	extension string
	markers   []string
}{
	{languagePHP, ".php", []string{composerJSON}},
	{languagePython, ".py", []string{pyprojectTOML, requirementsTXT}},
	{languageGo, ".go", []string{goMod}},
}

var errGatesBeforePacks = errors.New("gates init cannot add the lint gate")

const gatesBeforePacksFormat = "%w: the %s key is set without lint.packs, so set lint.packs and the lint gate in .harness.yaml by hand, " +
	"or remove the %s key and run harness init again (chosen packs: %s)"

func newInitCommand(flags *pflag.FlagSet, version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandInit,
		Short: "Set up the lint packs, the gates, the lint files, the lint baseline, the host hooks and the runs rule in .gitignore",
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

// installHooks writes the hooks of the host. When the harness plugin already runs them, it writes none and removes the
// harness entries an earlier init left in the host config, so no hook runs twice.
func installHooks(ctx context.Context, handlers hostcli.HostHandlers, agentHost shareddomain.Host, out io.Writer) error {
	plugin, err := handlers.Plugin(ctx, agentHost)
	if err != nil {
		return err
	}
	if !plugin {
		hostConfig, err := handlers.Install(ctx, agentHost)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, hostFormat, hostConfig)
		return err
	}
	hostConfig, removed, err := handlers.Uninstall(ctx, agentHost)
	if err != nil {
		return err
	}
	if removed == 0 {
		_, err = fmt.Fprint(out, hostSkippedLine)
		return err
	}
	_, err = fmt.Fprintf(out, hostRemovedFormat, removed, hostConfig)
	return err
}

func initRepository(ctx context.Context, flags *pflag.FlagSet, agentHost shareddomain.Host, version string, out io.Writer) error {
	env, err := newEnvironment(ctx, flags)
	if err != nil {
		return err
	}
	cfg, worktree := env.cfg, env.key.Worktree
	hostHandlers, err := host.NewHostConnect(func(context.Context) (string, bool, error) { return worktree, cfg.Hooks.Matched, nil })(ctx)
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
		blockingKey, err := writer.BlockingKey(worktree)
		if err != nil {
			return err
		}
		if blockingKey != "" && len(cfg.Lint.Packs) > 0 {
			return fmt.Errorf(gatesBeforePacksFormat, errGatesBeforePacks, blockingKey, blockingKey, strings.Join(cfg.Lint.Packs, listSeparator))
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
	lines, err := lintSetupLines(worktree, lint.PluginRules)
	if err != nil {
		return err
	}
	for _, line := range lines {
		fmt.Fprint(out, line)
	}

	written, err := gatemodule.NewInit(toolchain.Lint{Packs: cfg.Lint.Packs, Baseline: lintdomain.BaselinePath}, env.bus).
		Handle(ctx, initgates.Command{Dir: worktree})
	switch {
	case errors.Is(err, gatedomain.ErrGatesExist):
		fmt.Fprint(out, gatesKeptLine)
	case err != nil:
		return err
	case len(written.Setup) == 0 && len(written.Gates) == 0:
		fmt.Fprint(out, gatesNoneLine)
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
		kept, err := lint.Baseline.Handle(ctx, baseline.WriteBaseline{Dir: worktree, Path: lintdomain.BaselinePath})
		if err != nil {
			return err
		}
		fmt.Fprintf(out, baselineFormat, lintdomain.BaselinePath, kept)
	}
	if err := installHooks(ctx, hostHandlers, agentHost, out); err != nil {
		return err
	}
	added, err := addRunsRule(worktree)
	if err != nil {
		return err
	}
	if !added {
		_, err = fmt.Fprint(out, gitignoreKeptLine)
		return err
	}
	_, err = fmt.Fprintf(out, gitignoreAddedFormat, ignoreRule)
	return err
}

const ignoreRule = shareddomain.RunsFolder + "/"

// ignoringLines holds each line of a .gitignore that already ignores the runs folder.
var ignoringLines = func() map[string]bool {
	lines := map[string]bool{}
	for _, base := range []string{path.Dir(shareddomain.RunsFolder), shareddomain.RunsFolder} {
		for _, suffix := range []string{"", "/", "/*", "/**"} {
			form := base + suffix
			lines[form] = true
			lines["/"+form] = true
			lines["**/"+form] = true
		}
	}
	return lines
}()

// addRunsRule appends the runs rule to .gitignore and reports whether it wrote.
func addRunsRule(worktree string) (added bool, err error) {
	file := filepath.Join(worktree, gitignoreFile)
	raw, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("read %s: %w", gitignoreFile, err)
	}
	content := string(raw)
	ignored := false
	for line := range strings.SplitSeq(content, lineFeed) {
		line = strings.TrimRight(strings.TrimSuffix(line, "\r"), " ")
		if ignoringLines[line] {
			ignored = true
		} else if key, negated := strings.CutPrefix(line, "!"); negated && ignoringLines[key] {
			ignored = false
		}
	}
	if ignored {
		return false, nil
	}
	newline := lineFeed
	if strings.Contains(content, carriageLine) {
		newline = carriageLine
	}
	addition := ignoreRule + newline
	if content != "" && !strings.HasSuffix(content, lineFeed) {
		addition = newline + addition
	}
	handle, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return false, fmt.Errorf("write %s: %w", gitignoreFile, err)
	}
	if _, err := handle.WriteString(addition); err != nil {
		_ = handle.Close()
		return false, fmt.Errorf("write %s: %w", gitignoreFile, err)
	}
	if err := handle.Close(); err != nil {
		return false, fmt.Errorf("write %s: %w", gitignoreFile, err)
	}
	return true, nil
}

func lintSetupLines(dir string, pluginRules []lintdomain.Rule) ([]string, error) {
	var lines []string
	for _, language := range languagesWithNoPack {
		found, err := anyFile(dir, language.markers)
		if err != nil {
			return nil, err
		}
		covered := slices.ContainsFunc(pluginRules, func(r lintdomain.Rule) bool { return strings.HasSuffix(r.Scope, language.extension) })
		if found && !covered {
			lines = append(lines, fmt.Sprintf(lintSetupFormat, language.name))
		}
	}
	return lines, nil
}

func anyFile(dir string, names []string) (bool, error) {
	for _, name := range names {
		_, err := os.Stat(filepath.Join(dir, name))
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("read %s: %w", name, err)
		}
	}
	return false, nil
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

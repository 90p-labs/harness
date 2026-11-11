package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/90p-labs/harness/internal/guard/application/guardplugin"
	guarddomain "github.com/90p-labs/harness/internal/guard/domain"
	guardcli "github.com/90p-labs/harness/internal/guard/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
	"github.com/90p-labs/harness/internal/shared/infrastructure/telemetry"
)

const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2
)

const (
	commandName    = "harness"
	commandVersion = "version"
	commandConfig  = "config"
)

var errNoCommand = errors.New("no command given")

// prBodySource reads the plan of the active run and the branch commits for `harness pr body`. It writes no state.
type prBodySource struct{ flags *pflag.FlagSet }

func (s prBodySource) Plan(ctx context.Context) (string, error) {
	// The pre-run resolved the run from an existing state file, so this opens no state and creates none.
	ref := runRefFrom(ctx)
	if ref.ID == "" {
		return "", nil
	}
	worktree, err := topLevel(ctx)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(planPath(worktree, ref.ID))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(content), err
}

func topLevel(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return git.TopLevel(ctx, cwd)
}

func (s prBodySource) Commits(ctx context.Context, base string) ([]string, error) {
	worktree, err := topLevel(ctx)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(base, "-") {
		return nil, usageError{fmt.Errorf("--base %q must be a ref, not an option", base)}
	}
	if base == "" {
		cfg, err := config.Load(s.flags, worktree)
		if err != nil {
			return nil, err
		}
		base = cfg.Git.DefaultBranch
	}
	if base == "" {
		if base, err = git.DefaultBranch(ctx, worktree); err != nil {
			return nil, err
		}
	}
	out, err := git.Run(ctx, worktree, nil, "log", "--reverse", "--format=%s", "--end-of-options", base+"..HEAD")
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

// reviewPanelSource gives `harness review panel` the same call the guard makes, so both print one panel.
func reviewPanelSource(flags *pflag.FlagSet) guardcli.PanelSource {
	return func(ctx context.Context) ([]string, string, error) {
		worktree, err := topLevel(ctx)
		if err != nil {
			return nil, "", err
		}
		cfg, err := config.Load(flags, worktree)
		if err != nil {
			return nil, "", err
		}
		runID := runRefFrom(ctx).ID
		adapter := reviewRecordAdapter{cfg: cfg}
		// Mirror the guard: an empty patch id never matches, and the run id only scopes a run that has findings.
		scope := guarddomain.ReviewScope{}
		base := cfg.Git.DefaultBranch
		if base == "" {
			base, err = git.DefaultBranch(ctx, worktree)
		}
		if err == nil {
			if patchID, perr := git.PatchID(ctx, worktree, base); perr == nil {
				scope.PatchID = patchID
			}
		}
		if _, err := os.Stat(filepath.Join(worktree, shareddomain.RunFindingsFile(runID))); err == nil && !cfg.Run.StrictMinor {
			scope.RunID = runID
		}
		reviews, err := adapter.Reviews(ctx, worktree, scope)
		if err != nil {
			return nil, "", err
		}
		counted, _ := guarddomain.ReviewedModels(reviews)
		deps := guardplugin.Deps{
			ReadFile: os.ReadFile,
			Models:   modelsByHost(cfg.Roles),
			Worktree: worktree,
			Panel:    panelSource(cfg),
		}
		_, _, panel, reason := guardplugin.FinalPanel(ctx, deps, runID, counted)
		if len(panel) == 0 {
			return nil, "", errors.New("no reviewer models are listed under roles")
		}
		return panel, reason, nil
	}
}

type usageError struct{ error }

func (e usageError) Unwrap() error { return e.error }

type refusalError struct {
	error
	hint string
}

func (e refusalError) Error() string { return e.error.Error() + "\n" + e.hint }

func (e refusalError) Unwrap() error { return e.error }

func usageArgs(check cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := check(cmd, args); err != nil {
			return usageError{err}
		}
		return nil
	}
}

// version is set at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	shutdown, err := telemetry.Setup(ctx, version)
	reportTelemetry(stderr, err)
	defer func() { reportTelemetry(stderr, shutdown(ctx)) }()

	root := newRootCommand()
	// cobra reads os.Args[1:] when the args slice is nil.
	root.SetArgs(append([]string{}, args...))
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if failed, err := root.ExecuteContextC(telemetry.ParentFromEnv(ctx)); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		switch {
		case errors.As(err, new(refusalError)):
			return exitUsage
		case errors.As(err, new(usageError)):
			fmt.Fprint(stderr, failed.UsageString())
			return exitUsage
		}
		return exitRuntime
	}
	return exitOK
}

func reportTelemetry(stderr io.Writer, err error) {
	if err != nil {
		fmt.Fprintf(stderr, "telemetry: %s\n", strings.ReplaceAll(err.Error(), "\n", "; "))
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           commandName,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			resolveRun(cmd)
			return nil
		},
		RunE: func(*cobra.Command, []string) error {
			return usageError{errNoCommand}
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })
	root.PersistentFlags().String(config.FlagConfig, "", "path of the config file")
	root.PersistentFlags().String(config.FlagStateDSN, "", "state database DSN")
	root.AddCommand(newConfigCommand(), newRunCommand(root.PersistentFlags()), newPlanCommand(root.PersistentFlags()), newDiagramCommand(), newHostCommand(root.PersistentFlags()))
	root.AddCommand(newVerifyCommand(root.PersistentFlags()), newGatesCommand(root.PersistentFlags()), newLintCommand(root.PersistentFlags(), version), newLiveCommand(root.PersistentFlags()))
	root.AddCommand(newInitCommand(root.PersistentFlags(), version))
	root.AddCommand(newReportCommand(root.PersistentFlags()))
	root.AddCommand(guardcli.NewReviewCommand(reviewPanelSource(root.PersistentFlags())))
	root.AddCommand(guardcli.NewPRCommand(prBodySource{flags: root.PersistentFlags()}))
	root.AddCommand(&cobra.Command{
		Use:   commandVersion,
		Short: "Print the build version",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", commandName, version)
		},
	})
	wrapArgs(root)
	// Claude Code reads exit status 2 of a hook as a block, so no hook error is a usage error.
	root.AddCommand(newHookCommand(root.PersistentFlags()))
	return root
}

package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/jorge07/harness/internal/plan/application/command/submitplan"
	"github.com/jorge07/harness/internal/plan/application/query/checkplan"
	"github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/plan/ui/cli"
)

const (
	validPlan  = "../../domain/testdata/valid.md"
	brokenPlan = "# Plan\n"
	flagFile   = "--file"
	subCommand = "check"

	maxHelpLines = 45
)

var errDefault = errors.New("no default plan")

func connect(defaultPlan func(context.Context) (checkplan.Query, error), closed *bool) cli.Connect {
	return func(context.Context) (cli.Handlers, func() error, error) {
		handlers := cli.Handlers{
			Check:       checkplan.NewHandler(nil, noop.NewTracerProvider().Tracer("test")),
			Submit:      submitplan.Handler{},
			DefaultPlan: defaultPlan,
		}
		return handlers, func() error { *closed = true; return nil }, nil
	}
}

func execute(t *testing.T, connect cli.Connect, args ...string) (string, error) {
	t.Helper()
	cmd := cli.NewCommand(connect)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}

func TestPlanHelp(t *testing.T) {
	var closed bool

	out, err := execute(t, connect(nil, &closed), "--help")

	if err != nil {
		t.Fatalf("help: %v", err)
	}
	var want []string
	for _, list := range [][]string{
		domain.TopHeadings, domain.FinalHeadings, domain.TaskFields, domain.TaskSides, domain.CriteriaHeader, domain.SignalsHeader, domain.SignalFolders,
		{
			domain.MarkNew, domain.MarkChanged, domain.MarkDeleted, domain.TrackPrefix, domain.CriteriaKey, domain.FenceTree, domain.ContractNames,
			domain.FormTaskHeading, domain.FormDoD, domain.FormDependsOn, domain.FormSpecCriteria, domain.FormCommit, domain.FormPR,
			domain.DiagramER, domain.DiagramSequence, domain.DiagramState, domain.DiagramClass, domain.SketchExt, "harness diagram --help",
		},
	} {
		want = append(want, list...)
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("help does not hold %q", w)
		}
	}
	if lines := strings.Count(out, "\n"); lines >= maxHelpLines {
		t.Errorf("help has %d lines, want fewer than %d", lines, maxHelpLines)
	}
}

func TestPlanCheckCommand(t *testing.T) {
	broken := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(broken, []byte(brokenPlan), 0o600); err != nil {
		t.Fatal(err)
	}
	failDefault := func(context.Context) (checkplan.Query, error) {
		t.Error("DefaultPlan called")
		return checkplan.Query{}, errDefault
	}

	t.Run("a clean file prints nothing and succeeds", func(t *testing.T) {
		var closed bool

		out, err := execute(t, connect(failDefault, &closed), subCommand, flagFile, validPlan)

		if err != nil || out != "" {
			t.Fatalf("err = %v, out = %q, want nil and empty", err, out)
		}
		if !closed {
			t.Error("the store was not closed")
		}
	})

	t.Run("a broken file prints the problems and returns ErrPlanHasProblems", func(t *testing.T) {
		var closed bool

		out, err := execute(t, connect(failDefault, &closed), subCommand, flagFile, broken)

		if !errors.Is(err, domain.ErrPlanHasProblems) {
			t.Fatalf("err = %v, want ErrPlanHasProblems", err)
		}
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(lines) < 2 || !strings.HasPrefix(lines[0], "plan.md:1: ") {
			t.Errorf("out = %q, want one problem on each line", out)
		}
	})

	t.Run("no file uses the default plan", func(t *testing.T) {
		var closed bool
		defaultPlan := func(context.Context) (checkplan.Query, error) {
			return checkplan.Query{Path: validPlan}, nil
		}

		out, err := execute(t, connect(defaultPlan, &closed), subCommand)

		if err != nil || out != "" {
			t.Fatalf("err = %v, out = %q, want nil and empty", err, out)
		}
	})

	t.Run("the default plan error goes to the caller", func(t *testing.T) {
		var closed bool
		defaultPlan := func(context.Context) (checkplan.Query, error) { return checkplan.Query{}, cli.ErrNoPlanFile }

		_, err := execute(t, connect(defaultPlan, &closed), subCommand)

		if !errors.Is(err, cli.ErrNoPlanFile) {
			t.Fatalf("err = %v, want ErrNoPlanFile", err)
		}
	})
}

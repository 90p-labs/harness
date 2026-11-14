package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/90p-labs/harness/internal/plan/application/command/submitplan"
	"github.com/90p-labs/harness/internal/plan/application/query/checkplan"
	"github.com/90p-labs/harness/internal/plan/domain"
	"github.com/90p-labs/harness/internal/plan/ui/cli"
)

const (
	validPlan  = "../../domain/testdata/valid.md"
	brokenPlan = "# Plan\n"
	flagFile   = "--file"
	subCommand = "check"

	maxHelpLines = 50

	mapPlan  = "../../domain/testdata/map.md"
	clearMap = `# Map: Tiny

## Destination

A tiny goal.

## Decisions so far

- T1: the answer.

## Not yet specified

- none

## Out of scope

- none

## Tickets

### T1 only-question

- Type: grilling
- Blocked by: none
- Status: closed
- Question: Is it tiny?
- Answer: Yes.
`
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
			"## " + domain.HeadingOpenQuestions, domain.OpenQuestionsNone,
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

func TestPlanHelpSeam(t *testing.T) {
	out, err := execute(t, connect(nil, new(bool)), "--help")

	if err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, w := range []string{domain.FormSeam, string(domain.RuleSeamMissing)} {
		if !strings.Contains(out, w) {
			t.Errorf("help does not hold %q", w)
		}
	}
}

func TestReproductionSurfaceHelp(t *testing.T) {
	var closed bool

	out, err := execute(t, connect(nil, &closed), "--help")

	if err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, w := range []string{
		domain.CausePrefix + " " + domain.CauseBug, "## " + domain.HeadingReproduction, string(domain.RuleReproduction), string(domain.RuleReproductionRed),
		domain.ReproCriterion, domain.ReproSymptom, domain.ReproHumanOnly, domain.ReproRegression, domain.FormHypothesis,
	} {
		if !strings.Contains(out, w) {
			t.Errorf("help does not hold %q", w)
		}
	}
}

func TestPlanHelpProgram(t *testing.T) {
	out, err := execute(t, connect(nil, new(bool)), "--help")

	if err != nil {
		t.Fatalf("help: %v", err)
	}
	want := append([]string{domain.HeadingRuns, domain.HeadingMeasurement, domain.FormRunHeading, domain.FormRunDepends, domain.FormScope, domain.FormWave, domain.FormBranch, string(domain.RuleRunField), string(domain.RuleRunDepends), string(domain.RuleRunOverlap)}, domain.ProgramFields...)
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("help does not hold %q", w)
		}
	}
}

func TestPlanHelpMap(t *testing.T) {
	out, err := execute(t, connect(nil, new(bool)), "--help")

	if err != nil {
		t.Fatalf("help: %v", err)
	}
	want := slices.Concat([]string{
		domain.HeadingDestination, domain.HeadingDecisionsSoFar, domain.HeadingUnspecified, domain.HeadingOutOfScope, domain.HeadingTickets,
		domain.FormTicketHeading, domain.FormTicketBlocked, string(domain.RuleMapField), string(domain.RuleMapBlocked), string(domain.RuleMapQuestion), string(domain.RuleMapIndex),
	}, domain.MapFields, domain.TicketTypes, domain.TicketStatuses)
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("help does not hold %q", w)
		}
	}
	if strings.Contains(out, "plan view") {
		t.Error("help names plan view, which another run of the program owns")
	}
}

func TestCheckPrintsMapFrontier(t *testing.T) {
	raw, err := os.ReadFile(mapPlan)
	if err != nil {
		t.Fatal(err)
	}
	claimed := strings.NewReplacer(
		"- Status: open\n- Question: Does the existing", "- Status: open\n- Claimed by: bo\n- Question: Does the existing",
		"- Status: open\n- Question: Who files", "- Status: open\n- Claimed by: bo\n- Question: Who files",
	).Replace(string(raw))
	write := func(text string) string {
		file := filepath.Join(t.TempDir(), "map.md")
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return file
	}
	for name, tc := range map[string]struct{ text, want string }{
		"free tickets in file order":    {string(raw), "frontier: T3 job-queue (prototype)\nfrontier: T7 bucket-access (work)\n"},
		"open tickets and none free":    {claimed, "frontier: none\n"},
		"no open ticket and none unset": {clearMap, "map: clear\n"},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := execute(t, connect(nil, new(bool)), subCommand, flagFile, write(tc.text))

			if err != nil || out != tc.want {
				t.Fatalf("err = %v, out = %q, want %q", err, out, tc.want)
			}
		})
	}
}

func TestAdvisorLineNamesModel(t *testing.T) {
	const waves = "wave 1: task 1\nwave 2: task 2\n"
	raw, err := os.ReadFile(validPlan)
	if err != nil {
		t.Fatal(err)
	}
	light := strings.NewReplacer("Track: full", "Track: light", "domain/", "hooks/", "application/query/pingquery/", "service/pingq/pingquery/",
		"└── application/", "└── service/", "└── query/", "└── pingq/").Replace(string(raw))
	lightFile := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(lightFile, []byte(light), 0o600); err != nil {
		t.Fatal(err)
	}
	off := strings.ReplaceAll(light, "hooks/", "model/")
	offFile := filepath.Join(t.TempDir(), "off.md")
	if err := os.WriteFile(offFile, []byte(off), 0o600); err != nil {
		t.Fatal(err)
	}
	withModels := func(models ...string) cli.Connect {
		inner := connect(nil, new(bool))
		return func(ctx context.Context) (cli.Handlers, func() error, error) {
			handlers, closer, err := inner(ctx)
			handlers.AdvisorModels = models
			return handlers, closer, err
		}
	}
	for name, tc := range map[string]struct {
		models []string
		file   string
		want   string
	}{
		"full track uses the first model":       {[]string{"fable", "opus"}, validPlan, "advisor: on, model fable\n"},
		"path trigger uses the second model":    {[]string{"fable", "opus"}, lightFile, "advisor: on, model opus (light track, risky path)\n"},
		"one model serves both triggers (full)": {[]string{"fable"}, validPlan, "advisor: on, model fable\n"},
		"one model serves both triggers (path)": {[]string{"fable"}, lightFile, "advisor: on, model fable (light track, risky path)\n"},
		"off names no model":                    {[]string{"fable", "opus"}, offFile, "advisor: off (light track)\n"},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := execute(t, withModels(tc.models...), subCommand, flagFile, tc.file)

			if err != nil || out != tc.want+waves {
				t.Fatalf("err = %v, out = %q, want %q", err, out, tc.want+waves)
			}
		})
	}
}

func TestCheckPrintsAdvisor(t *testing.T) {
	const programPlan = "../../domain/testdata/program.md"
	const waves = "wave 1: task 1\nwave 2: task 2\n"
	broken := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(broken, []byte(brokenPlan), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("a full track plan", func(t *testing.T) {
		out, err := execute(t, connect(nil, new(bool)), subCommand, flagFile, validPlan)

		if err != nil || out != "advisor: on\n"+waves {
			t.Fatalf("err = %v, out = %q, want the bare on line", err, out)
		}
	})

	lightPlan := func(folder string) string {
		raw, err := os.ReadFile(validPlan)
		if err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(t.TempDir(), "plan.md")
		light := strings.NewReplacer("Track: full", "Track: light", "domain/", folder, "application/query/pingquery/", "service/pingq/pingquery/",
			"└── application/", "└── service/", "└── query/", "└── pingq/").Replace(string(raw))
		if err := os.WriteFile(file, []byte(light), 0o600); err != nil {
			t.Fatal(err)
		}
		return file
	}

	t.Run("a light plan with a hook folder", func(t *testing.T) {
		out, err := execute(t, connect(nil, new(bool)), subCommand, flagFile, lightPlan("hooks/"))

		if err != nil || out != "advisor: on (light track, risky path)\n"+waves {
			t.Fatalf("err = %v, out = %q, want the bare on line", err, out)
		}
	})

	t.Run("a light plan with no risky path", func(t *testing.T) {
		out, err := execute(t, connect(nil, new(bool)), subCommand, flagFile, lightPlan("model/"))

		if err != nil || out != "advisor: off (light track)\n"+waves {
			t.Fatalf("err = %v, out = %q, want the light track line", err, out)
		}
	})

	t.Run("a multi-run plan and a plan with a problem print no advisor line", func(t *testing.T) {
		for _, file := range []string{programPlan, broken} {
			out, _ := execute(t, connect(nil, new(bool)), subCommand, flagFile, file)

			if strings.Contains(out, "advisor:") || strings.Contains(out, "wave ") {
				t.Errorf("%s: out = %q, want no advisor line and no wave line", file, out)
			}
		}
	})
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

		if err != nil || strings.Contains(out, "plan.md:") {
			t.Fatalf("err = %v, out = %q, want nil and no problem", err, out)
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

		if err != nil || strings.Contains(out, "plan.md:") {
			t.Fatalf("err = %v, out = %q, want nil and no problem", err, out)
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

func TestCheckReportsAnUnclosedFenceInAMultiRunPlan(t *testing.T) {
	const heading = "### R4 spec-quality\n"
	raw, err := os.ReadFile("../../domain/testdata/program.md")
	if err != nil {
		t.Fatal(err)
	}
	fenced := strings.Replace(string(raw), heading, "```\n"+heading, 1)
	file := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(file, []byte(fenced), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := execute(t, connect(nil, new(bool)), subCommand, flagFile, file)

	if !errors.Is(err, domain.ErrPlanHasProblems) {
		t.Fatalf("err = %v, want ErrPlanHasProblems", err)
	}
	if want := "plan.md:44: run-field: code fence is never closed"; !strings.Contains(out, want) {
		t.Errorf("out = %q, want the line %q", out, want)
	}
}

func TestCheckNamesTheModelOfATask(t *testing.T) {
	const dependsLine = "- Depends on: task 1\n"
	valid, err := os.ReadFile(validPlan)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "plan.md")
	withModel := strings.Replace(string(valid), dependsLine, dependsLine+"- Model: haiku\n", 1)
	if err := os.WriteFile(file, []byte(withModel), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := execute(t, connect(nil, new(bool)), subCommand, flagFile, file)

	if want := "wave 1: task 1\nwave 2: task 2 (haiku)\n"; err != nil || !strings.HasSuffix(out, want) {
		t.Fatalf("err = %v, out = %q, want the suffix %q", err, out, want)
	}
}

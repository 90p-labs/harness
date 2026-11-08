package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/run/domain"
)

const (
	testBuildVersion   = "1.2.3-test"
	envOTLPEndpoint    = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envOTLPTraces      = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	envOTLPMetrics     = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
	invalidOTLPAddress = "localhost:4318"
	telemetryPrefix    = "telemetry: "
	harnessEnvPrefix   = "HARNESS_"
)

func clearOTLPEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{envOTLPEndpoint, envOTLPTraces, envOTLPMetrics} {
		t.Setenv(name, "")
	}
}

func execute(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return executeWithStdin(t, "", args...)
}

func executeWithStdin(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	previous := version
	version = testBuildVersion
	t.Cleanup(func() { version = previous })

	var out, errOut bytes.Buffer
	code = run(context.Background(), args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestDiagramCommand(t *testing.T) {
	const (
		defNew   = "    classDef new stroke:#00b34e,stroke-width:3px,color:#00b34e\n"
		unmarked = "flowchart TD\n    a --> b\n"
	)
	clearOTLPEnv(t)
	clearHarnessEnv(t)
	t.Chdir(t.TempDir())

	for _, tc := range []struct{ name, input, fenced string }{
		{
			"flowchart",
			"flowchart TD\n    a[start] --> b[end] +\n",
			"```mermaid\nflowchart TD\n" + defNew + "    a[start] -->|\"(new)\"| b[end]\n```\n",
		},
		{
			"state diagram through the parser",
			"stateDiagram-v2\n    [*] --> Ready\n    Ready --> Done: ping +\n    state Done +\n",
			"```mermaid\nstateDiagram-v2\n" + defNew + "    [*] --> Ready\n    Ready --> Done: ping (new)\n    state Done\n    class Done new\n```\n",
		},
		{
			"class diagram through the parser",
			"classDiagram\n    class Pong +\n    Handler --> Pong +\n",
			"```mermaid\nclassDiagram\n" + defNew + "    class Pong\n    Handler --> Pong : (new)\n    cssClass \"Pong\" new\n```\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := executeWithStdin(t, tc.input, commandDiagram)

			if code != exitOK || stdout != tc.fenced || stderr != "" {
				t.Fatalf("code = %d, stdout = %q, stderr = %q, want %d and %q", code, stdout, stderr, exitOK, tc.fenced)
			}
		})
	}

	t.Run("refuses an unmarked diagram", func(t *testing.T) {
		code, stdout, stderr := executeWithStdin(t, unmarked, commandDiagram)

		if code != exitUsage || stdout != "" || !strings.Contains(stderr, "error: diagram refused: line 1: no mark") {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want %d and the refusal reason", code, stdout, stderr, exitUsage)
		}
	})
}

func TestVersion(t *testing.T) {
	clearOTLPEnv(t)
	code, stdout, stderr := execute(t, "version")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stdout != "harness 1.2.3-test\n" {
		t.Errorf("stdout = %q, want %q", stdout, "harness 1.2.3-test\n")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestUsageError(t *testing.T) {
	processArgs := os.Args
	os.Args = []string{processArgs[0], "stray"}
	t.Cleanup(func() { os.Args = processArgs })

	cases := []struct {
		name      string
		args      []string
		wantError string
	}{
		{name: "no command", args: nil, wantError: "error: no command given\n"},
		{name: "unknown command", args: []string{"nope"}, wantError: "error: unknown command \"nope\" for \"harness\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearOTLPEnv(t)
			code, stdout, stderr := execute(t, tc.args...)

			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, tc.wantError) {
				t.Errorf("stderr = %q, want prefix %q", stderr, tc.wantError)
			}
			for _, want := range []string{"Usage:", "harness [command]", "version"} {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr, want)
				}
			}
		})
	}
}

func clearHarnessEnv(t *testing.T) {
	t.Helper()
	clearOTLPEnv(t)
	for _, entry := range os.Environ() {
		if name, _, _ := strings.Cut(entry, "="); strings.HasPrefix(name, harnessEnvPrefix) {
			t.Setenv(name, "")
		}
	}
}

func TestExitCodes(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		outside   bool
		wantCode  int
		wantError string
	}{
		{name: "unknown flag", args: []string{commandVersion, "--nope"}, wantCode: exitUsage, wantError: "error: unknown flag: --nope\n"},
		{name: "extra argument", args: []string{commandVersion, "extra"}, wantCode: exitUsage, wantError: "error: "},
		{name: "extra plan check argument", args: []string{"plan", "check", "extra"}, wantCode: exitUsage, wantError: "error: "},
		{name: "no completion command", args: []string{"completion", "bash", "extra"}, wantCode: exitUsage, wantError: "error: unknown command \"completion\""},
		{name: "outside a git repo", args: []string{commandConfig}, outside: true, wantCode: exitRuntime, wantError: "error: not in a git repository\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearHarnessEnv(t)
			if tc.outside {
				t.Chdir(t.TempDir())
			}
			code, _, stderr := execute(t, tc.args...)

			if code != tc.wantCode || !strings.HasPrefix(stderr, tc.wantError) {
				t.Fatalf("code = %d, stderr = %q, want %d and prefix %q", code, stderr, tc.wantCode, tc.wantError)
			}
			if hasUsage := strings.Contains(stderr, "Usage:"); hasUsage != (tc.wantCode == exitUsage) {
				t.Fatalf("stderr = %q, usage shown = %v", stderr, hasUsage)
			}
		})
	}
}

func TestConfigCommand(t *testing.T) {
	clearHarnessEnv(t)
	file := filepath.Join(t.TempDir(), "harness.yaml")
	body := "plugins:\n  - name: guard\n    command: [guard]\n    events: [tool.before]\n  - name: slow\n    command: [slow]\n    events: [tool.after]\n    timeout: 750ms\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := execute(t, commandConfig, "--config", file)

	if code != exitOK || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"dsn: sqlite:.harness/state.db", "max_review_rounds: 3", "name: guard", "timeout: 5s", "timeout: 750ms", "worker:", "claude: sonnet"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

func TestConfigFlagMissingFile(t *testing.T) {
	clearHarnessEnv(t)
	missing := filepath.Join(t.TempDir(), "missing.yaml")

	code, _, stderr := execute(t, commandConfig, "--config", missing)

	if code != exitRuntime || !strings.Contains(stderr, missing) {
		t.Fatalf("code = %d, stderr = %q, want %d and the file name", code, stderr, exitRuntime)
	}
}

func TestTelemetryErrorKeepsExitCode(t *testing.T) {
	clearOTLPEnv(t)
	t.Setenv(envOTLPEndpoint, invalidOTLPAddress)
	code, stdout, stderr := execute(t, "version")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stdout != "harness 1.2.3-test\n" {
		t.Errorf("stdout = %q, want %q", stdout, "harness 1.2.3-test\n")
	}
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], telemetryPrefix) {
		t.Errorf("stderr = %q, want one line starting with %q", stderr, telemetryPrefix)
	}
}

const (
	commandRun = "run"
	gitBinary  = "git"
	stateFile  = ".harness/state.db"
	activeNo   = "active: no\n"

	validPlanFixture = "../../internal/plan/domain/testdata/valid.md"
	brokenPlan       = "# Plan\n"
	commandPlan      = "plan"
	commandDiagram   = "diagram"
	commandCheck     = "check"
)

func validPlanFile(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(validPlanFixture)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func startedRunID(t *testing.T, stdout string) string {
	t.Helper()
	fields := strings.Fields(strings.SplitN(stdout, "\n", 2)[0])
	if len(fields) < 2 || fields[0] != commandRun {
		t.Fatalf("start stdout = %q, want a run line", stdout)
	}
	return fields[1]
}

func writeRunPlan(t *testing.T, dir, runID, body string) {
	t.Helper()
	folder := filepath.Join(dir, ".harness", "runs", runID)
	if err := os.MkdirAll(folder, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "plan.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func validPlan(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(validPlanFile(t))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command(gitBinary, append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "init", "--quiet")
	t.Chdir(dir)
	return dir
}

func TestRunLifecycle(t *testing.T) {
	clearHarnessEnv(t)
	valid := validPlan(t)
	dir := initRepo(t)

	steps := []struct {
		args      []string
		wantPhase domain.Phase
	}{
		{[]string{"start", "Add the login page"}, domain.PhasePlanning},
		{[]string{"plan"}, domain.PhaseAwaitingApproval},
		{[]string{"approve"}, domain.PhaseWorking},
		{[]string{"review"}, domain.PhaseReviewing},
		{[]string{"ship"}, domain.PhaseShipping},
		{[]string{"done"}, domain.PhaseDone},
	}
	for _, step := range steps {
		code, stdout, stderr := execute(t, append([]string{commandRun}, step.args...)...)
		if code != exitOK || stderr != "" {
			t.Fatalf("%v: code = %d, stderr = %q", step.args, code, stderr)
		}
		for _, want := range []string{" phase=" + string(step.wantPhase) + " ", "-add-the-login-page ", "\nnext: "} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("%v: stdout = %q, want it to contain %q", step.args, stdout, want)
			}
		}
		if step.args[0] == "start" {
			writeRunPlan(t, dir, startedRunID(t, stdout), valid)
		}
	}

	code, stdout, stderr := execute(t, commandRun, "status")
	if code != exitOK || stdout != activeNo || stderr != "" {
		t.Fatalf("status: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestStateInGitCommonDir(t *testing.T) {
	clearHarnessEnv(t)
	main := initRepo(t)
	gitIn(t, main, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--quiet", "--allow-empty", "-m", "init")
	linked := filepath.Join(filepath.Dir(main), filepath.Base(main)+"-linked")
	gitIn(t, main, "worktree", "add", "--quiet", linked)
	t.Chdir(linked)

	code, _, stderr := execute(t, commandRun, "start", "linked work")
	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(main, ".git", stateFile)); err != nil {
		t.Errorf("state file in the common dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(linked, ".harness")); !os.IsNotExist(err) {
		t.Errorf("worktree has a .harness folder: %v", err)
	}
}

func TestRunExitCodes(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantCode  int
		wantError string
		wantUsage string
	}{
		{name: "extra argument", args: []string{commandRun, "plan", "extra"}, wantCode: exitUsage, wantError: "error: ", wantUsage: "harness run plan"},
		{name: "missing findings", args: []string{commandRun, "rework"}, wantCode: exitUsage, wantError: "error: ", wantUsage: "--findings"},
		{name: "missing reason", args: []string{commandRun, "block"}, wantCode: exitUsage, wantError: "error: ", wantUsage: "--reason"},
		{name: "no subcommand", args: []string{commandRun}, wantCode: exitUsage, wantError: "error: no command given\n", wantUsage: "harness run [command]"},
		{name: "no active run", args: []string{commandRun, "plan"}, wantCode: exitRuntime, wantError: "error: run not found\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearHarnessEnv(t)
			initRepo(t)
			code, _, stderr := execute(t, tc.args...)
			if code != tc.wantCode || !strings.HasPrefix(stderr, tc.wantError) {
				t.Fatalf("code = %d, stderr = %q, want %d and prefix %q", code, stderr, tc.wantCode, tc.wantError)
			}
			if tc.wantUsage != "" && !strings.Contains(stderr, tc.wantUsage) {
				t.Errorf("stderr = %q, want usage with %q", stderr, tc.wantUsage)
			}
			if hasUsage := strings.Contains(stderr, "Usage:"); hasUsage != (tc.wantCode == exitUsage) {
				t.Errorf("stderr = %q, usage shown = %v", stderr, hasUsage)
			}
		})
	}
}

func TestRunVetoExitsOne(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	deny := filepath.Join(dir, "deny.sh")
	script := "#!/bin/sh\necho '{\"verdict\":\"deny\",\"reason\":\"not now\"}'\n"
	if err := os.WriteFile(deny, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := "plugins:\n  - name: gate\n    command: [" + deny + "]\n    events: [run.transition.before]\n"
	if err := os.WriteFile(filepath.Join(dir, ".harness.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := execute(t, commandRun, "start", "vetoed")
	if code != exitRuntime || !strings.Contains(stderr, "not now") {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	code, stdout, _ := execute(t, commandRun, "status")
	if code != exitOK || stdout != activeNo {
		t.Fatalf("after veto: code = %d, stdout = %q", code, stdout)
	}
}

func TestRunPlanChecksThePlan(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	_, stdout, _ := execute(t, commandRun, "start", "Add the login page")
	runID := startedRunID(t, stdout)
	writeRunPlan(t, dir, runID, brokenPlan)

	code, _, stderr := execute(t, commandRun, commandPlan)

	if code != exitRuntime || !strings.Contains(stderr, "plan.md:1: heading-missing") {
		t.Fatalf("code = %d, stderr = %q, want %d and the problem lines", code, stderr, exitRuntime)
	}
	_, stdout, _ = execute(t, commandRun, "status")
	if want := " phase=" + string(domain.PhasePlanning) + " "; !strings.Contains(stdout, want) {
		t.Errorf("status = %q, want it to contain %q", stdout, want)
	}
}

func TestPlanCheckNoRun(t *testing.T) {
	clearHarnessEnv(t)
	initRepo(t)

	code, stdout, stderr := execute(t, commandPlan, commandCheck)

	if code != exitUsage || stdout != "" || !strings.HasPrefix(stderr, "error: no plan file") {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want %d and the no plan file error", code, stdout, stderr, exitUsage)
	}
}

func TestPlanCheckRefusesABadDiagram(t *testing.T) {
	clearHarnessEnv(t)
	broken := strings.Replace(validPlan(t), "Handler --> Pong", "Handler -> Pong", 1)
	file := filepath.Join(initRepo(t), "plan.md")
	if err := os.WriteFile(file, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := execute(t, commandPlan, commandCheck, "--file", file)

	if code != exitRuntime || !strings.Contains(stdout, ": diagram: ") {
		t.Fatalf("code = %d, stdout = %q, want %d and a diagram problem", code, stdout, exitRuntime)
	}
}

func TestHookFailsClosedOutsideGit(t *testing.T) {
	clearHarnessEnv(t)
	t.Chdir(t.TempDir())
	const input = `{"cwd":"/work","tool_name":"Bash","tool_input":{"command":"ls"}}`

	code, stdout, stderr := executeWithStdin(t, input, "hook", "--host", "claude", "pre-tool-use")

	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q, want %d", code, stderr, exitOK)
	}
	for _, want := range []string{`"permissionDecision":"deny"`, "harness error"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
}

func TestHookRefusalIsNotABlock(t *testing.T) {
	clearHarnessEnv(t)
	cases := []struct {
		name string
		args []string
	}{
		{"unknown host", []string{"hook", "--host", "vim", "pre-tool-use"}},
		{"unknown hook", []string{"hook", "--host", "claude", "PreToolUse"}},
		{"no host", []string{"hook", "pre-tool-use"}},
		{"two hooks", []string{"hook", "--host", "claude", "stop", "stop"}},
		{"unknown flag", []string{"hook", "--hots", "claude", "stop"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := executeWithStdin(t, "{}", tc.args...)

			if code != exitRuntime || stdout != "" || !strings.HasPrefix(stderr, "error: ") || strings.Contains(stderr, "Usage:") {
				t.Fatalf("code = %d, stdout = %q, stderr = %q, want exit %d with one error line", code, stdout, stderr, exitRuntime)
			}
		})
	}
}

func TestHostCapabilitiesOutsideGit(t *testing.T) {
	clearHarnessEnv(t)
	t.Chdir(t.TempDir())

	code, stdout, stderr := executeWithStdin(t, "", "host", "capabilities", "--host", "codex")

	if code != exitOK || strings.Count(stdout, " yes\n") != 7 {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func TestPlanCheckValidFixture(t *testing.T) {
	clearHarnessEnv(t)
	file := validPlanFile(t)
	initRepo(t)

	code, stdout, stderr := execute(t, commandPlan, commandCheck, "--file", file)

	if code != exitOK || stdout != "" || stderr != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want a clean plan", code, stdout, stderr)
	}
}

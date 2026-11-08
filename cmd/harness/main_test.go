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

		const want = "error: diagram refused: line 1: no mark: end a line with + (new), * (changed) or - (removed)\nhint: harness diagram --help\n"
		if code != exitUsage || stdout != "" || stderr != want {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want %d and %q", code, stdout, stderr, exitUsage, want)
		}
		if strings.Contains(stderr, "Usage:") {
			t.Errorf("stderr = %q, want no usage text", stderr)
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
	body := "plugins:\n  - name: guard\n    command: [guard]\n    events: [tool.before]\n  - name: slow\n    command: [slow]\n    events: [tool.after]\n    timeout: 750ms\nguard:\n  disabled: [writing-style]\ngit:\n  default_branch: develop\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := execute(t, commandConfig, "--config", file)

	if code != exitOK || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"dsn: sqlite:.harness/state.db", "max_review_rounds: 3", "name: guard", "timeout: 5s", "timeout: 750ms", "worker:", "claude: sonnet", "- writing-style", "default_branch: develop"} {
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
	commandVerify    = "verify"
	commandGates     = "gates"
	commandInit      = "init"

	pingBranch      = "feat/ping-query"
	strayFile       = "stray.txt"
	configFile      = ".harness.yaml"
	commitsChecked  = "commits-checked"
	gitUserName     = "user.name=t"
	gitUserEmail    = "user.email=t@example.com"
	trueProof       = "`true`"
	noGateMessage   = "no gate is set"
	goModuleFile    = "module example.com/x\n\ngo 1.22\n"
	pushInput       = `{"cwd":".","tool_name":"Bash","tool_input":{"command":"git push origin feat/x"}}`
	pongSource      = "internal/ping/domain/pong.go"
	pongTest        = "internal/ping/domain/pong_test.go"
	queryHandler    = "internal/ping/application/query/pingquery/handler.go"
	queryHandlerTst = "internal/ping/application/query/pingquery/handler_test.go"
)

func commitFiles(t *testing.T, dir, subject string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		gitIn(t, dir, "add", name)
	}
	gitIn(t, dir, "-c", gitUserName, "-c", gitUserEmail, "commit", "--quiet", "--allow-empty", "-m", subject)
}

func tracedRepo(t *testing.T) (dir, plan string) {
	t.Helper()
	plan = validPlan(t)
	dir = initRepo(t)
	gitIn(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFiles(t, dir, "chore: start the repo", nil)
	gitIn(t, dir, "checkout", "--quiet", "-b", pingBranch)
	commitFiles(t, dir, "feat: add the pong domain", map[string]string{
		pongSource: "package domain\n",
		pongTest:   "package domain\n\nfunc TestPong(t *testing.T) {}\n",
	})
	commitFiles(t, dir, "feat: add the ping query", map[string]string{
		queryHandler:    "package pingquery\n",
		queryHandlerTst: "package pingquery\n\nfunc TestPingQuery(t *testing.T) {}\n",
	})
	commitFiles(t, dir, "docs: describe the ping flow", map[string]string{pongSource: "package domain\n\n// Pong is the word.\n"})

	for _, proof := range []string{
		"`go test ./internal/ping/domain -run TestPong -count=1`",
		"`go test ./internal/ping/application/query/pingquery -run TestPingQuery -count=1`",
		"`make check`",
	} {
		plan = strings.Replace(plan, proof, trueProof, 1)
	}
	return dir, plan
}

func verifyPasses(t *testing.T) {
	t.Helper()
	code, stdout, stderr := execute(t, commandVerify)
	if code != exitOK || stderr != "" || !strings.Contains(stdout, "criterion-3") {
		t.Fatalf("verify: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

func startWithPlan(t *testing.T, dir, plan string) (runID string) {
	t.Helper()
	_, stdout, _ := execute(t, commandRun, "start", "Add the login page")
	runID = startedRunID(t, stdout)
	writeRunPlan(t, dir, runID, plan)
	for _, step := range []string{commandPlan, "approve"} {
		if code, _, stderr := execute(t, commandRun, step); code != exitOK {
			t.Fatalf("run %s: code = %d, stderr = %q", step, code, stderr)
		}
	}
	return runID
}

func TestReviewVerifiesTrace(t *testing.T) {
	clearHarnessEnv(t)
	dir, plan := tracedRepo(t)
	commitFiles(t, dir, "chore: add a stray file", map[string]string{strayFile: "stray\n"})
	startWithPlan(t, dir, plan)
	verifyPasses(t)

	code, _, stderr := execute(t, commandRun, "review")

	if code != exitRuntime || !strings.Contains(stderr, "file-untracked") || !strings.Contains(stderr, strayFile) {
		t.Fatalf("review with a stray file: code = %d, stderr = %q, want %d and the file-untracked problem", code, stderr, exitRuntime)
	}
	_, stdout, _ := execute(t, commandRun, "status")
	if want := " phase=" + string(domain.PhaseWorking) + " "; !strings.Contains(stdout, want) {
		t.Fatalf("status = %q, want it to contain %q", stdout, want)
	}

	gitIn(t, dir, "rm", "--quiet", strayFile)
	gitIn(t, dir, "-c", gitUserName, "-c", gitUserEmail, "commit", "--quiet", "-m", "chore: drop the stray file")
	verifyPasses(t)
	code, stdout, stderr = execute(t, commandRun, "review")

	if want := " phase=" + string(domain.PhaseReviewing) + " "; code != exitOK || !strings.Contains(stdout, want) {
		t.Fatalf("review after the fix: code = %d, stdout = %q, stderr = %q, want %q", code, stdout, stderr, want)
	}
}

func TestReviewNeedsProofEvidence(t *testing.T) {
	clearHarnessEnv(t)
	dir, plan := tracedRepo(t)
	startWithPlan(t, dir, plan)

	code, _, stderr := execute(t, commandRun, "review")

	if code != exitRuntime || !strings.Contains(stderr, "proof-missing") {
		t.Fatalf("review before verify: code = %d, stderr = %q, want %d and the proof-missing problem", code, stderr, exitRuntime)
	}
}

func TestVerifyCriterionGateHashesTheWholeTree(t *testing.T) {
	clearHarnessEnv(t)
	dir, plan := tracedRepo(t)
	startWithPlan(t, dir, strings.Replace(plan, trueProof, "`go version`", 1))
	verifyPasses(t)

	commitFiles(t, dir, "test: add a fixture", map[string]string{"internal/ping/domain/testdata/pong.txt": "pong\n"})
	_, stdout, _ := execute(t, commandVerify)

	if !strings.Contains(stdout, "PASS criterion-1 ") {
		t.Fatalf("stdout = %q, want criterion-1 to run again after a testdata change", stdout)
	}
}

func TestVerifyRefusesAPlanWithProblems(t *testing.T) {
	clearHarnessEnv(t)
	dir, plan := tracedRepo(t)
	writeRunPlan(t, dir, startWithPlan(t, dir, plan), strings.Replace(plan, "| 1 |", "| one |", 1))

	code, _, stderr := execute(t, commandVerify)

	if code != exitRuntime || !strings.Contains(stderr, `"one"`) {
		t.Fatalf("code = %d, stderr = %q, want %d and the parse problem", code, stderr, exitRuntime)
	}
}

func TestVerifyExitCodes(t *testing.T) {
	t.Run("no gate is set", func(t *testing.T) {
		clearHarnessEnv(t)
		initRepo(t)

		code, stdout, stderr := execute(t, commandVerify)

		if code != exitOK || !strings.Contains(stdout, noGateMessage) || stderr != "" {
			t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
		}
	})

	t.Run("a failed gate", func(t *testing.T) {
		clearHarnessEnv(t)
		dir := initRepo(t)
		cfg := "gates:\n  - id: fine\n    run: \"true\"\n  - id: broken\n    run: \"echo boom; exit 3\"\n"
		if err := os.WriteFile(filepath.Join(dir, configFile), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}

		code, stdout, stderr := execute(t, commandVerify)

		if code != exitRuntime || !strings.Contains(stdout, "PASS fine ") || !strings.Contains(stdout, "FAIL broken ") || !strings.Contains(stdout, "boom") {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want %d and the gate lines", code, stdout, stderr, exitRuntime)
		}

		if err := os.WriteFile(filepath.Join(dir, configFile), []byte("gates:\n  - id: fine\n    run: \"true\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		code, stdout, _ = execute(t, commandVerify)
		if code != exitOK || !strings.Contains(stdout, "PASS fine ") {
			t.Fatalf("passing gate: code = %d, stdout = %q, want %d", code, stdout, exitOK)
		}
	})
}

func TestVerifyCommitsAllowsAPush(t *testing.T) {
	clearHarnessEnv(t)
	dir, _ := tracedRepo(t)
	cfg := "git:\n  default_branch: main\ngates:\n  - id: fine\n    run: \"true\"\n"
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	denied := func() (int, string) {
		code, stdout, _ := executeWithStdin(t, pushInput, "hook", "--host", "claude", "pre-tool-use")
		return code, stdout
	}

	if code, stdout := denied(); code != exitOK || !strings.Contains(stdout, "guard."+commitsChecked) {
		t.Fatalf("push before the check: code = %d, stdout = %q, want a commits-checked deny", code, stdout)
	}
	code, stdout, stderr := execute(t, commandVerify, "--commits", "main")
	if code != exitOK || !strings.Contains(stdout, "PASS ") || stderr != "" {
		t.Fatalf("verify --commits: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if code, stdout := denied(); code != exitOK || strings.Contains(stdout, "deny") {
		t.Fatalf("push after the check: code = %d, stdout = %q, want an allow", code, stdout)
	}

	other := t.TempDir()
	gitIn(t, other, "init", "--quiet")
	commitFiles(t, other, "feat: start another repo", nil)
	otherPush := strings.Replace(pushInput, `"cwd":"."`, `"cwd":"`+other+`"`, 1)
	if code, stdout, _ := executeWithStdin(t, otherPush, "hook", "--host", "claude", "pre-tool-use"); code != exitOK || !strings.Contains(stdout, "guard."+commitsChecked) {
		t.Fatalf("push from an unchecked repo: code = %d, stdout = %q, want a commits-checked deny", code, stdout)
	}
}

func TestHookCommitsCheckedWithoutHead(t *testing.T) {
	clearHarnessEnv(t)
	initRepo(t)

	code, stdout, stderr := executeWithStdin(t, pushInput, "hook", "--host", "claude", "pre-tool-use")

	for _, want := range []string{`"permissionDecision":"deny"`, "guard." + commitsChecked, "commit, then run harness verify --commits"} {
		if code != exitOK || !strings.Contains(stdout, want) {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want %q", code, stdout, stderr, want)
		}
	}
}

func TestGatesInit(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModuleFile), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := execute(t, commandGates, commandInit)

	if code != exitOK || !strings.Contains(stdout, "go test") || stderr != "" {
		t.Fatalf("first init: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	code, _, stderr = execute(t, commandGates, commandInit)
	if code != exitRuntime || !strings.Contains(stderr, "gates exist") {
		t.Fatalf("second init: code = %d, stderr = %q, want %d and the gates exist error", code, stderr, exitRuntime)
	}
}

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
	dir, valid := tracedRepo(t)

	steps := []struct {
		args      []string
		wantPhase domain.Phase
	}{
		{[]string{"start", "Add the login page"}, domain.PhasePlanning},
		{[]string{"plan"}, domain.PhaseAwaitingApproval},
		{[]string{"approve"}, domain.PhaseWorking},
		{[]string{"review"}, domain.PhaseReviewing},
		{[]string{"ship"}, domain.PhaseShipping},
		{[]string{"done", "--pr", "https://github.com/o/r/pull/1"}, domain.PhaseDone},
	}
	for _, step := range steps {
		if step.args[0] == "review" {
			verifyPasses(t)
		}
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
		{name: "missing pr", args: []string{commandRun, "done"}, wantCode: exitUsage, wantError: "error: ", wantUsage: "--pr"},
		{name: "bad pr url", args: []string{commandRun, "done", "--pr", "https://example.com/1"}, wantCode: exitUsage, wantError: "error: "},
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

func TestPlanViewWiring(t *testing.T) {
	clearHarnessEnv(t)
	body := validPlan(t)
	dir := initRepo(t)
	plan := filepath.Join(dir, "plan.md")
	if err := os.WriteFile(plan, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := execute(t, commandPlan, "view", "--file", plan)

	page := filepath.Join(dir, "plan.html")
	if code != exitOK || strings.TrimSpace(stdout) != page || stderr != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want %q", code, stdout, stderr, page)
	}
	html, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(html), "<!doctype html>") || !strings.Contains(string(html), "ping") {
		t.Errorf("the page is not the plan page: %.80q", html)
	}
}

func TestHookGuardEndToEnd(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	cfg := "guard:\n  disabled: [" + commitsChecked + "]\n"
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	const input = `{"cwd":".","tool_name":"Bash","tool_input":{"command":"git push --force origin feat/x"}}`

	code, stdout, stderr := executeWithStdin(t, input, "hook", "--host", "claude", "pre-tool-use")

	if code != exitOK {
		t.Fatalf("code = %d, stderr = %q, want %d", code, stderr, exitOK)
	}
	for _, want := range []string{`"permissionDecision":"deny"`, "guard.git-no-force: "} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	}
}

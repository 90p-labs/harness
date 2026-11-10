package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
)

const (
	fileName    = ".harness.yaml"
	envPrefix   = "HARNESS_"
	envStateDSN = "HARNESS_STATE_DSN"
)

func TestMain(m *testing.M) {
	for _, entry := range os.Environ() {
		if name, _, _ := strings.Cut(entry, "="); strings.HasPrefix(name, envPrefix) {
			os.Unsetenv(name)
		}
	}
	m.Run()
}

func flagSet(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String(config.FlagConfig, "", "")
	flags.String(config.FlagStateDSN, "", "")
	if err := flags.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return flags
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

func load(t *testing.T, content string, args ...string) (config.Config, error) {
	t.Helper()
	return config.Load(flagSet(t, args...), writeConfig(t, content))
}

func TestLoadDefaults(t *testing.T) {
	got, err := config.Load(flagSet(t), t.TempDir())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got.State.DSN != "sqlite:.harness/state.db" {
		t.Errorf("state.dsn = %q", got.State.DSN)
	}
	wantRun := config.Run{Approval: true, GoLive: false, MaxReviewRounds: 3, MaxLiveRounds: 3, MaxStopBlocks: 3}
	if got.Run != wantRun {
		t.Errorf("run = %+v, want %+v", got.Run, wantRun)
	}
	if len(got.Plugins) != 0 {
		t.Errorf("plugins = %+v, want none", got.Plugins)
	}
	wantRoles := map[string]config.RoleHost{
		"explorer": {"claude": {"haiku"}, "codex": {"gpt-6-luna"}},
		"worker":   {"claude": {"sonnet", "haiku"}, "codex": {"gpt-6.1-sol", "gpt-6-luna"}},
		"reviewer": {"claude": {"sonnet", "haiku", "opus"}, "codex": {"gpt-6.1-sol", "gpt-6.1-terra", "gpt-6-astra"}},
		"advisor":  {"claude": {"fable", "opus"}, "codex": {"gpt-6-astra"}},
		"lead":     {"claude": {"opus"}, "codex": {"gpt-6-astra"}},
	}
	if !reflect.DeepEqual(got.Roles, wantRoles) {
		t.Errorf("roles = %+v, want %+v", got.Roles, wantRoles)
	}
}

func TestLoadPrecedence(t *testing.T) {
	const content = "state:\n  dsn: file-dsn\n"

	cases := []struct {
		name string
		file string
		env  string
		args []string
		want string
	}{
		{name: "default", want: "sqlite:.harness/state.db"},
		{name: "file over default", file: content, want: "file-dsn"},
		{name: "env over file", file: content, env: "env-dsn", want: "env-dsn"},
		{name: "flag over env", file: content, env: "env-dsn", args: []string{"--state-dsn", "flag-dsn"}, want: "flag-dsn"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(envStateDSN, tc.env)
			got, err := load(t, tc.file, tc.args...)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got.State.DSN != tc.want {
				t.Errorf("state.dsn = %q, want %q", got.State.DSN, tc.want)
			}
		})
	}
}

func TestLoadEmptyStateDSN(t *testing.T) {
	_, err := load(t, "state:\n  dsn: \"\"\n")
	if err == nil {
		t.Fatal("want an error for an empty state.dsn")
	}
	if !strings.Contains(err.Error(), "state.dsn") {
		t.Errorf("error %q does not name the key", err)
	}
}

func TestLoadUnknownKey(t *testing.T) {
	_, err := load(t, "state:\n  dsn: x\nbogus_key: 1\n")
	if err == nil {
		t.Fatal("want an error for an unknown key")
	}
	if !strings.Contains(err.Error(), "bogus_key") {
		t.Errorf("error %q does not name the key", err)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := writeConfig(t, "state: [unclosed\n")
	_, err := config.Load(flagSet(t), dir)
	if err == nil {
		t.Fatal("want an error for invalid yaml")
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, fileName)) {
		t.Errorf("error %q does not name the file", err)
	}
}

func TestLoadConfigFlag(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	_, err := config.Load(flagSet(t, "--config", missing), t.TempDir())
	if err == nil {
		t.Fatal("want an error for a missing --config file")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not name the file", err)
	}

	other := filepath.Join(t.TempDir(), "other.yaml")
	if err := os.WriteFile(other, []byte("state:\n  dsn: other-dsn\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(flagSet(t, "--config", other), writeConfig(t, "state:\n  dsn: top-dsn\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.State.DSN != "other-dsn" {
		t.Errorf("state.dsn = %q, want other-dsn", got.State.DSN)
	}
}

func TestLoadConfigFlagNoExtension(t *testing.T) {
	file := filepath.Join(t.TempDir(), "harness")
	if err := os.WriteFile(file, []byte("state:\n  dsn: plain-dsn\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := config.Load(flagSet(t, "--config", file), t.TempDir())

	if err != nil || got.State.DSN != "plain-dsn" {
		t.Fatalf("load = %q, %v, want plain-dsn", got.State.DSN, err)
	}
}

func TestLoadPluginTimeout(t *testing.T) {
	got, err := load(t, `plugins:
  - name: slow
    command: [slow-bin]
    events: [tool.before]
    timeout: 750ms
  - name: plain
    command: [plain-bin]
    events: [tool.before]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Plugins) != 2 {
		t.Fatalf("plugins = %+v", got.Plugins)
	}
	if got.Plugins[0].Timeout != 750*time.Millisecond {
		t.Errorf("slow timeout = %v", got.Plugins[0].Timeout)
	}
	if got.Plugins[1].Timeout != config.DefaultPluginTimeout || config.DefaultPluginTimeout != 5*time.Second {
		t.Errorf("plain timeout = %v", got.Plugins[1].Timeout)
	}
}

func TestLoadGates(t *testing.T) {
	got, err := load(t, `gates:
  - id: go-test
    run: go test ./...
    paths: ["**/*.go", go.mod]
    timeout: 90s
    baseline: .harness-lint/baseline.json
  - id: plain
    run: "true"
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := []config.Gate{
		{ID: "go-test", Run: "go test ./...", Paths: []string{"**/*.go", "go.mod"}, Timeout: 90 * time.Second, Baseline: ".harness-lint/baseline.json"},
		{ID: "plain", Run: "true"},
	}
	if !reflect.DeepEqual(got.Gates, want) {
		t.Errorf("gates = %+v, want %+v", got.Gates, want)
	}
}

func TestLoadGateConfig(t *testing.T) {
	defaults, err := config.Load(flagSet(t), t.TempDir())
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if defaults.Verify.Parallel != 1 {
		t.Errorf("default verify.parallel = %d, want 1", defaults.Verify.Parallel)
	}

	got, err := load(t, `verify:
  parallel: 3
gates:
  - id: lint
    run: pnpm lint
  - id: test
    run: pnpm test
    needs: [lint]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := []config.Gate{
		{ID: "lint", Run: "pnpm lint"},
		{ID: "test", Run: "pnpm test", Needs: []string{"lint"}},
	}
	if got.Verify.Parallel != 3 || !reflect.DeepEqual(got.Gates, want) {
		t.Errorf("verify = %+v, gates = %+v, want parallel 3 and %+v", got.Verify, got.Gates, want)
	}

	_, err = load(t, "verify:\n  parallel: 0\n")
	if err == nil || !strings.Contains(err.Error(), "verify.parallel") {
		t.Errorf("parallel 0 error = %v, want one that names verify.parallel", err)
	}
}

func TestLoadLint(t *testing.T) {
	defaults := config.Lint{
		Disabled:      []string{},
		DDD:           config.LintDDD{Root: "contexts", SrcFiles: []string{"main.ts"}},
		Observability: config.LintObservability{Chart: "deploy/chart", Rules: "deploy/**/*.rules.yaml", Dashboards: "deploy/**/dashboards/*.json"},
	}
	cases := []struct {
		name string
		file string
		want func(config.Lint) config.Lint
	}{
		{name: "defaults", want: func(l config.Lint) config.Lint { return l }},
		{
			name: "file over defaults",
			file: `lint:
  packs: [ddd, shadcn]
  disabled: [oxlint]
  ddd:
    root: domains
    domain_allow: [zod]
  shadcn:
    ui_allow: [clsx]
  observability:
    chart: charts/app
`,
			want: func(l config.Lint) config.Lint {
				l.Packs = []string{"ddd", "shadcn"}
				l.Disabled = []string{"oxlint"}
				l.DDD = config.LintDDD{Root: "domains", DomainAllow: []string{"zod"}, SrcFiles: []string{"main.ts"}}
				l.Shadcn.UIAllow = []string{"clsx"}
				l.Observability.Chart = "charts/app"
				return l
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := load(t, tc.file)

			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if want := tc.want(defaults); !reflect.DeepEqual(got.Lint, want) {
				t.Errorf("lint = %+v, want %+v", got.Lint, want)
			}
		})
	}

	if _, err := load(t, "lint:\n  ddd:\n    rot: x\n"); err == nil {
		t.Fatal("want an error for the unknown lint key")
	}
}

func TestLoadLive(t *testing.T) {
	cases := []struct {
		name string
		file string
		want config.Live
	}{
		{name: "defaults", want: config.Live{Environment: "production"}},
		{
			name: "file over defaults",
			file: "live:\n  environment: staging\n  rollback: helm rollback api\n",
			want: config.Live{Environment: "staging", Rollback: "helm rollback api"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := load(t, tc.file)

			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got.Live != tc.want {
				t.Errorf("live = %+v, want %+v", got.Live, tc.want)
			}
		})
	}

	if _, err := load(t, "live:\n  rot: x\n"); err == nil {
		t.Fatal("want an error for the unknown live key")
	}
}

func TestLoadSetupConfig(t *testing.T) {
	got, err := load(t, "setup:\n  - id: deps\n    run: composer install\n    paths: [composer.lock]\n    creates: vendor/autoload.php\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := []config.Setup{{ID: "deps", Run: "composer install", Paths: []string{"composer.lock"}, Creates: "vendor/autoload.php"}}
	if !reflect.DeepEqual(got.Setup, want) {
		t.Errorf("setup = %+v, want %+v", got.Setup, want)
	}
	if _, err := load(t, "setup:\n  - {id: deps, run: x, needs: [lint]}\n"); err == nil || !strings.Contains(err.Error(), "needs") {
		t.Errorf("needs err = %v, want the unknown key needs refused", err)
	}
}

func TestLoadGateUnknownKey(t *testing.T) {
	if _, err := load(t, "gates:\n  - {id: a, run: x, cmd: y}\n"); err == nil {
		t.Fatal("want an error for the unknown gate key")
	}
}

func TestLoadInvalidTimeout(t *testing.T) {
	for name, timeout := range map[string]string{"bare number": "5", "negative": "-1s"} {
		t.Run(name, func(t *testing.T) {
			_, err := load(t, "plugins:\n  - {name: p, command: [a], events: [tool.before], timeout: "+timeout+"}\n")
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), "timeout") {
				t.Errorf("error %q does not name the timeout", err)
			}
		})
	}
}

func TestLoadRoles(t *testing.T) {
	got, err := load(t, "roles:\n  lead:\n    claude: sonnet\n  reviewer:\n    claude: opus\n  worker:\n    claude: [opus, sonnet]\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	want := map[string]config.RoleHost{
		"lead":     {"claude": {"sonnet"}, "codex": {"gpt-6-astra"}},
		"reviewer": {"claude": {"opus"}, "codex": {"gpt-6.1-sol", "gpt-6.1-terra", "gpt-6-astra"}},
		"worker":   {"claude": {"opus", "sonnet"}, "codex": {"gpt-6.1-sol", "gpt-6-luna"}},
		"explorer": {"claude": {"haiku"}, "codex": {"gpt-6-luna"}},
		"advisor":  {"claude": {"fable", "opus"}, "codex": {"gpt-6-astra"}},
	}
	if !reflect.DeepEqual(got.Roles, want) {
		t.Errorf("roles = %+v, want %+v", got.Roles, want)
	}
}

func TestLoadLeadEnv(t *testing.T) {
	t.Setenv("HARNESS_ROLES_LEAD_CLAUDE", "sonnet")
	got, err := load(t, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if want := []string{"sonnet"}; !reflect.DeepEqual(got.Roles["lead"]["claude"], want) {
		t.Errorf("lead.claude = %+v, want %+v", got.Roles["lead"]["claude"], want)
	}
}

func TestLoadCodexOverride(t *testing.T) {
	got, err := load(t, "roles:\n  reviewer:\n    codex: [gpt-6-luna, gpt-6.1-sol]\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if want := []string{"gpt-6-luna", "gpt-6.1-sol"}; !reflect.DeepEqual(got.Roles["reviewer"]["codex"], want) {
		t.Errorf("reviewer.codex = %+v, want %+v", got.Roles["reviewer"]["codex"], want)
	}
	if want := []string{"sonnet", "haiku", "opus"}; !reflect.DeepEqual(got.Roles["reviewer"]["claude"], want) {
		t.Errorf("reviewer.claude = %+v, want %+v", got.Roles["reviewer"]["claude"], want)
	}
}

func TestLoadCodexEnv(t *testing.T) {
	t.Setenv("HARNESS_ROLES_WORKER_CODEX", "gpt-6.1-terra")
	got, err := load(t, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if want := []string{"gpt-6.1-terra"}; !reflect.DeepEqual(got.Roles["worker"]["codex"], want) {
		t.Errorf("worker.codex = %+v, want %+v", got.Roles["worker"]["codex"], want)
	}
}

func TestLoadAdvisorRole(t *testing.T) {
	if _, err := load(t, "roles:\n  advisor:\n    claude: fable\n"); err != nil {
		t.Fatalf("load: %v", err)
	}
}

func TestLoadUnknownRole(t *testing.T) {
	cases := map[string]string{
		"unknown role": "roles:\n  janitor:\n    claude: haiku\n",
		"unknown host": "roles:\n  worker:\n    gemini: pro\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := load(t, content); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestLoadInvalidRounds(t *testing.T) {
	for _, key := range []string{"max_review_rounds", "max_live_rounds", "max_stop_blocks"} {
		t.Run(key, func(t *testing.T) {
			_, err := load(t, "run:\n  "+key+": 0\n")
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("error %q does not name the key", err)
			}
		})
	}
}

func TestLoadInvalidPlugin(t *testing.T) {
	cases := map[string]string{
		"duplicate": `plugins:
  - {name: dup, command: [a], events: [tool.before]}
  - {name: dup, command: [b], events: [tool.before]}
`,
		"empty command": `plugins:
  - {name: nocmd, command: [], events: [tool.before]}
`,
		"no event": `plugins:
  - {name: noevent, command: [a], events: []}
`,
		"no name": `plugins:
  - {command: [a], events: [tool.before]}
  - {command: [b], events: [tool.before]}
`,
	}
	names := map[string]string{"duplicate": "dup", "empty command": "nocmd", "no event": "noevent", "no name": "plugins[0] has no name"}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := load(t, content)
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), names[name]) {
				t.Errorf("error %q does not name the plugin", err)
			}
		})
	}
}

func TestDeployDisabled(t *testing.T) {
	defaults, err := config.Load(flagSet(t), t.TempDir())
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if defaults.Deploy.Disabled == nil || len(defaults.Deploy.Disabled) != 0 {
		t.Errorf("default deploy.disabled = %#v, want an empty list", defaults.Deploy.Disabled)
	}

	got, err := load(t, "deploy:\n  disabled: [helm]\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got.Deploy.Disabled, []string{"helm"}) {
		t.Errorf("deploy.disabled = %v", got.Deploy.Disabled)
	}
}

func TestGuardConfig(t *testing.T) {
	defaults, err := config.Load(flagSet(t), t.TempDir())
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if len(defaults.Guard.Disabled) != 0 || defaults.Git.DefaultBranch != "" {
		t.Errorf("defaults = %+v and %+v, want none", defaults.Guard, defaults.Git)
	}

	got, err := load(t, "guard:\n  disabled: [git-no-force, writing-style]\ngit:\n  default_branch: develop\n")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(got.Guard.Disabled, []string{"git-no-force", "writing-style"}) {
		t.Errorf("guard.disabled = %v", got.Guard.Disabled)
	}
	if got.Git.DefaultBranch != "develop" {
		t.Errorf("git.default_branch = %q", got.Git.DefaultBranch)
	}
}

func TestLoadPluginFilter(t *testing.T) {
	got, err := load(t, `plugins:
  - name: notify
    command: [notify]
    events: [run.transition.after]
    transition: [approve-plan, complete]
    from: [awaiting_approval]
    to: [working]
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	plugin := got.Plugins[0]
	if len(plugin.Transition) != 2 || plugin.From[0] != shareddomain.PhaseAwaitingApproval || plugin.To[0] != shareddomain.PhaseWorking {
		t.Fatalf("plugin = %+v", plugin)
	}
}

func TestLoadRejectsBadFilterName(t *testing.T) {
	cases := map[string]struct{ key, value, valid string }{
		"from":       {"from", shareddomain.TransitionStart, shareddomain.PhaseWorking},
		"to":         {"to", "nowhere", shareddomain.PhaseWorking},
		"transition": {"transition", "jump", shareddomain.TransitionApprovePlan},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := load(t, "plugins:\n  - {name: bad, command: [a], events: [run.transition.after], "+tc.key+": ["+tc.value+"]}\n")
			if err == nil {
				t.Fatal("want an error")
			}
			for _, want := range []string{"bad", tc.key, tc.value, tc.valid} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q lacks %q", err, want)
				}
			}
		})
	}
}

func TestStrictMinor(t *testing.T) {
	got, err := load(t, "")
	if err != nil || got.Run.StrictMinor {
		t.Fatalf("default: got %v, %v, want false", got.Run.StrictMinor, err)
	}

	got, err = load(t, "run:\n  strict_minor: true\n")
	if err != nil || !got.Run.StrictMinor {
		t.Fatalf("file: got %v, %v, want true", got.Run.StrictMinor, err)
	}

	t.Setenv("HARNESS_RUN_STRICT_MINOR", "true")
	got, err = load(t, "")
	if err != nil || !got.Run.StrictMinor {
		t.Fatalf("env: got %v, %v, want true", got.Run.StrictMinor, err)
	}
}

func TestLoadRejectsFilterOnOtherEvent(t *testing.T) {
	_, err := load(t, `plugins:
  - {name: mixed, command: [a], events: [run.transition.after, tool.before], to: [working]}
`)

	if err == nil || !strings.Contains(err.Error(), "mixed") {
		t.Fatalf("err = %v, want one that names the plugin", err)
	}
}

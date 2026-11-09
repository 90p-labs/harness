package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	FlagConfig   = "config"
	FlagStateDSN = "state-dsn"

	DefaultPluginTimeout = 5 * time.Second

	fileName   = ".harness.yaml"
	fileFormat = "yaml"
	envPrefix  = "HARNESS"

	keyStateDSN        = "state.dsn"
	keyApproval        = "run.approval"
	keyGoLive          = "run.go_live"
	keyMaxReviewRounds = "run.max_review_rounds"
	keyMaxLiveRounds   = "run.max_live_rounds"
	keyMaxStopBlocks   = "run.max_stop_blocks"
	keyPlugins         = "plugins"
	keyRoles           = "roles"
	keyGuardDisabled   = "guard.disabled"
	keyDeployDisabled  = "deploy.disabled"
	keyGitBranch       = "git.default_branch"
	keyVerifyParallel  = "verify.parallel"

	keyLiveEnvironment = "live.environment"
	keyLiveRollback    = "live.rollback"

	defaultLiveEnvironment = "production"

	keyLintDisabled      = "lint.disabled"
	keyLintDDDRoot       = "lint.ddd.root"
	keyLintDDDSrcFiles   = "lint.ddd.src_files"
	keyLintChart         = "lint.observability.chart"
	keyLintRules         = "lint.observability.rules"
	keyLintDashboards    = "lint.observability.dashboards"
	defaultLintDDDRoot   = "contexts"
	defaultLintSrcFile   = "main.ts"
	defaultLintChart     = "deploy/chart"
	defaultLintRules     = "deploy/**/*.rules.yaml"
	defaultLintDashboard = "deploy/**/dashboards/*.json"

	defaultStateDSN = "sqlite:.harness/state.db"
	defaultRounds   = 3
	defaultParallel = 1
)

type Config struct {
	State   State               `mapstructure:"state"`
	Run     Run                 `mapstructure:"run"`
	Plugins []Plugin            `mapstructure:"plugins"`
	Roles   map[string]RoleHost `mapstructure:"roles"`
	Guard   Guard               `mapstructure:"guard"`
	Deploy  Deploy              `mapstructure:"deploy"`
	Git     Git                 `mapstructure:"git"`
	Setup   []Setup             `mapstructure:"setup"`
	Gates   []Gate              `mapstructure:"gates"`
	Verify  Verify              `mapstructure:"verify"`
	Lint    Lint                `mapstructure:"lint"`
	Live    Live                `mapstructure:"live"`
}

type Live struct {
	Environment string `mapstructure:"environment"`
	Rollback    string `mapstructure:"rollback"`
}

type Lint struct {
	Packs         []string          `mapstructure:"packs"`
	Disabled      []string          `mapstructure:"disabled"`
	DDD           LintDDD           `mapstructure:"ddd"`
	Shadcn        LintShadcn        `mapstructure:"shadcn"`
	Observability LintObservability `mapstructure:"observability"`
}

type LintDDD struct {
	Root        string   `mapstructure:"root"`
	DomainAllow []string `mapstructure:"domain_allow"`
	SrcFiles    []string `mapstructure:"src_files"`
}

type LintShadcn struct {
	UIAllow []string `mapstructure:"ui_allow"`
}

type LintObservability struct {
	Chart      string `mapstructure:"chart"`
	Rules      string `mapstructure:"rules"`
	Dashboards string `mapstructure:"dashboards"`
}

type Gate struct {
	ID       string        `mapstructure:"id"`
	Run      string        `mapstructure:"run"`
	Paths    []string      `mapstructure:"paths"`
	Timeout  time.Duration `mapstructure:"timeout"`
	Baseline string        `mapstructure:"baseline"`
	Needs    []string      `mapstructure:"needs"`
}

type Setup struct {
	ID      string   `mapstructure:"id"`
	Run     string   `mapstructure:"run"`
	Paths   []string `mapstructure:"paths"`
	Creates string   `mapstructure:"creates"`
}

type Verify struct {
	Parallel int `mapstructure:"parallel"`
}

type Guard struct {
	Disabled []string `mapstructure:"disabled"`
}

type Deploy struct {
	Disabled []string `mapstructure:"disabled"`
}

type Git struct {
	DefaultBranch string `mapstructure:"default_branch"`
}

type State struct {
	DSN string `mapstructure:"dsn"`
}

type Run struct {
	Approval        bool `mapstructure:"approval"`
	GoLive          bool `mapstructure:"go_live"`
	MaxReviewRounds int  `mapstructure:"max_review_rounds"`
	MaxLiveRounds   int  `mapstructure:"max_live_rounds"`
	MaxStopBlocks   int  `mapstructure:"max_stop_blocks"`
}

type Plugin struct {
	Name    string        `mapstructure:"name"`
	Command []string      `mapstructure:"command"`
	Events  []string      `mapstructure:"events"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type RoleHost map[string][]string

var (
	knownRoles = []shareddomain.Role{
		shareddomain.RoleLead,
		shareddomain.RoleWorker,
		shareddomain.RoleReviewer,
		shareddomain.RoleExplorer,
		shareddomain.RoleAdvisor,
	}
	knownHosts = []shareddomain.Host{shareddomain.HostClaude, shareddomain.HostCodex}
)

func Load(flags *pflag.FlagSet, topLevel string) (Config, error) {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	setDefaults(v)

	if flag := flags.Lookup(FlagStateDSN); flag != nil {
		if err := v.BindPFlag(keyStateDSN, flag); err != nil {
			return Config{}, fmt.Errorf("bind flag %s: %w", FlagStateDSN, err)
		}
	}

	path, explicit := filepath.Join(topLevel, fileName), false
	if flag := flags.Lookup(FlagConfig); flag != nil && flag.Value.String() != "" {
		path, explicit = flag.Value.String(), true
	}
	v.SetConfigFile(path)
	v.SetConfigType(fileFormat)
	if err := v.ReadInConfig(); err != nil && (explicit || !errors.Is(err, fs.ErrNotExist)) {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	strict := func(dc *mapstructure.DecoderConfig) {
		dc.ErrorUnused = true
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(durationOnlyFromString, dc.DecodeHook)
	}
	if err := v.Unmarshal(&cfg, strict); err != nil {
		return Config{}, fmt.Errorf("decode config %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault(keyStateDSN, defaultStateDSN)
	v.SetDefault(keyApproval, true)
	v.SetDefault(keyGoLive, false)
	v.SetDefault(keyMaxReviewRounds, defaultRounds)
	v.SetDefault(keyMaxLiveRounds, defaultRounds)
	v.SetDefault(keyMaxStopBlocks, defaultRounds)
	v.SetDefault(keyPlugins, []any{})
	v.SetDefault(keyGuardDisabled, []any{})
	v.SetDefault(keyDeployDisabled, []any{})
	v.SetDefault(keyGitBranch, "")
	v.SetDefault(keyVerifyParallel, defaultParallel)
	v.SetDefault(keyLiveEnvironment, defaultLiveEnvironment)
	v.SetDefault(keyLiveRollback, "")
	v.SetDefault(keyLintDisabled, []any{})
	v.SetDefault(keyLintDDDRoot, defaultLintDDDRoot)
	v.SetDefault(keyLintDDDSrcFiles, []string{defaultLintSrcFile})
	v.SetDefault(keyLintChart, defaultLintChart)
	v.SetDefault(keyLintRules, defaultLintRules)
	v.SetDefault(keyLintDashboards, defaultLintDashboard)
	v.SetDefault(keyRoles, map[string]any{
		string(shareddomain.RoleLead):     map[string]any{string(shareddomain.HostClaude): []string{"opus"}, string(shareddomain.HostCodex): []string{"gpt-6-astra"}},
		string(shareddomain.RoleExplorer): map[string]any{string(shareddomain.HostClaude): []string{"haiku"}, string(shareddomain.HostCodex): []string{"gpt-5.5"}},
		string(shareddomain.RoleWorker):   map[string]any{string(shareddomain.HostClaude): []string{"sonnet", "haiku"}, string(shareddomain.HostCodex): []string{"gpt-5.6-luna", "gpt-5.5"}},
		string(shareddomain.RoleReviewer): map[string]any{string(shareddomain.HostClaude): []string{"sonnet", "haiku", "opus"}, string(shareddomain.HostCodex): []string{"gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5"}},
		string(shareddomain.RoleAdvisor):  map[string]any{string(shareddomain.HostClaude): []string{"fable"}, string(shareddomain.HostCodex): []string{"gpt-6-astra"}},
	})
}

func durationOnlyFromString(from, to reflect.Type, data any) (any, error) {
	if to != reflect.TypeFor[time.Duration]() || from.Kind() == reflect.String {
		return data, nil
	}
	return nil, fmt.Errorf("%v is not a duration string such as 5s", data)
}

func (c *Config) validate() error {
	if c.State.DSN == "" {
		return fmt.Errorf("%s is empty", keyStateDSN)
	}
	for key, value := range map[string]int{
		keyMaxReviewRounds: c.Run.MaxReviewRounds,
		keyMaxLiveRounds:   c.Run.MaxLiveRounds,
		keyMaxStopBlocks:   c.Run.MaxStopBlocks,
		keyVerifyParallel:  c.Verify.Parallel,
	} {
		if value < 1 {
			return fmt.Errorf("%s is %d, want 1 or more", key, value)
		}
	}

	seen := map[string]bool{}
	for i := range c.Plugins {
		plugin := &c.Plugins[i]
		switch {
		case plugin.Name == "":
			return fmt.Errorf("%s[%d] has no name", keyPlugins, i)
		case seen[plugin.Name]:
			return fmt.Errorf("plugin %q is duplicated", plugin.Name)
		case len(plugin.Command) == 0:
			return fmt.Errorf("plugin %q has no command", plugin.Name)
		case len(plugin.Events) == 0:
			return fmt.Errorf("plugin %q has no event", plugin.Name)
		case plugin.Timeout < 0:
			return fmt.Errorf("plugin %q has a negative timeout %s", plugin.Name, plugin.Timeout)
		}
		seen[plugin.Name] = true
		if plugin.Timeout == 0 {
			plugin.Timeout = DefaultPluginTimeout
		}
	}

	for role, hosts := range c.Roles {
		if !slices.Contains(knownRoles, shareddomain.Role(role)) {
			return fmt.Errorf("unknown role %q", role)
		}
		for host := range hosts {
			if !slices.Contains(knownHosts, shareddomain.Host(host)) {
				return fmt.Errorf("unknown host %q for role %q", host, role)
			}
		}
	}
	return nil
}

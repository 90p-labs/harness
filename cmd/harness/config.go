package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/git"
)

type configView struct {
	State   stateView                    `yaml:"state"`
	Run     runView                      `yaml:"run"`
	Plugins []pluginView                 `yaml:"plugins"`
	Roles   map[string]map[string]string `yaml:"roles"`
}

type stateView struct {
	DSN string `yaml:"dsn"`
}

type runView struct {
	Approval        bool `yaml:"approval"`
	GoLive          bool `yaml:"go_live"`
	MaxReviewRounds int  `yaml:"max_review_rounds"`
	MaxLiveRounds   int  `yaml:"max_live_rounds"`
	MaxStopBlocks   int  `yaml:"max_stop_blocks"`
}

type pluginView struct {
	Name    string   `yaml:"name"`
	Command []string `yaml:"command"`
	Events  []string `yaml:"events"`
	Timeout string   `yaml:"timeout"`
}

func newConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   commandConfig,
		Short: "Print the effective config",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			topLevel, err := git.TopLevel(cmd.Context(), cwd)
			if err != nil {
				return err
			}
			cfg, err := config.Load(cmd.Flags(), topLevel)
			if err != nil {
				return err
			}
			out, err := yaml.Marshal(viewOf(cfg))
			if err != nil {
				return fmt.Errorf("print config: %w", err)
			}
			_, err = cmd.OutOrStdout().Write(out)
			return err
		},
	}
}

func viewOf(cfg config.Config) configView {
	view := configView{
		State: stateView{DSN: cfg.State.DSN},
		Run: runView{
			Approval:        cfg.Run.Approval,
			GoLive:          cfg.Run.GoLive,
			MaxReviewRounds: cfg.Run.MaxReviewRounds,
			MaxLiveRounds:   cfg.Run.MaxLiveRounds,
			MaxStopBlocks:   cfg.Run.MaxStopBlocks,
		},
		Plugins: []pluginView{},
		Roles:   map[string]map[string]string{},
	}
	for _, p := range cfg.Plugins {
		view.Plugins = append(view.Plugins, pluginView{Name: p.Name, Command: p.Command, Events: p.Events, Timeout: p.Timeout.String()})
	}
	for role, hosts := range cfg.Roles {
		view.Roles[role] = hosts
	}
	return view
}

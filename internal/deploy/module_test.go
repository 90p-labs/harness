package deploy_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/deploy"
	"github.com/jorge07/harness/internal/deploy/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/execplugin"
)

func names(plugins []shareddomain.Plugin) []string {
	got := make([]string, len(plugins))
	for i, plugin := range plugins {
		got[i] = plugin.Name()
	}
	return got
}

func TestNewPlugins(t *testing.T) {
	tests := map[string]struct {
		disabled []string
		want     []string
	}{
		"defaults":        {want: []string{domain.ToolHelm, domain.ToolPromtool}},
		"helm turned off": {disabled: []string{domain.ToolHelm}, want: []string{domain.ToolPromtool}},
		"both turned off": {disabled: []string{domain.ToolHelm, domain.ToolPromtool}, want: []string{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := deploy.NewPlugins(tt.disabled, nil)
			if err != nil || !slices.Equal(names(got), tt.want) {
				t.Errorf("plugins = %v, err = %v, want %v", names(got), err, tt.want)
			}
		})
	}
}

func TestNewPluginsRefuses(t *testing.T) {
	renderer := execplugin.ExecPlugin{PluginName: "kustomize", Command: []string{"kustomize"}, Events: []shareddomain.EventName{shareddomain.EventChartRender}}
	tests := map[string]struct {
		disabled []string
		exec     []shareddomain.Plugin
		want     error
		name     string
	}{
		"an unknown name":                    {disabled: []string{"kubectl"}, want: domain.ErrUnknownTool, name: "kubectl"},
		"a second renderer while helm is on": {exec: []shareddomain.Plugin{renderer}, want: domain.ErrRendererConflict, name: "kustomize"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := deploy.NewPlugins(tt.disabled, tt.exec)
			if !errors.Is(err, tt.want) || !strings.Contains(err.Error(), tt.name) {
				t.Errorf("err = %v, want %v that names %q", err, tt.want, tt.name)
			}
		})
	}
}

func TestNewPluginsAllowsARendererWithHelmOff(t *testing.T) {
	renderer := execplugin.ExecPlugin{PluginName: "kustomize", Command: []string{"kustomize"}, Events: []shareddomain.EventName{shareddomain.EventChartRender}}
	if _, err := deploy.NewPlugins([]string{domain.ToolHelm}, []shareddomain.Plugin{renderer}); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

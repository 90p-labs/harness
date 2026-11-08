package deploy

import (
	"context"
	"fmt"
	"slices"

	"github.com/jorge07/harness/internal/deploy/application/query/renderchart"
	"github.com/jorge07/harness/internal/deploy/domain"
	"github.com/jorge07/harness/internal/deploy/infrastructure/toolplugin"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

func NewPlugins(disabled []string, execPlugins []shareddomain.Plugin) ([]shareddomain.Plugin, error) {
	for _, name := range disabled {
		if !slices.Contains(domain.Tools, name) {
			return nil, fmt.Errorf("deploy.disabled names %q: %w", name, domain.ErrUnknownTool)
		}
	}
	if !slices.Contains(disabled, domain.ToolHelm) {
		for _, plugin := range execPlugins {
			if slices.Contains(plugin.Subscribes(), shareddomain.EventChartRender) {
				return nil, fmt.Errorf("plugin %q: %w, add helm to deploy.disabled", plugin.Name(), domain.ErrRendererConflict)
			}
		}
	}
	plugins := []shareddomain.Plugin{}
	for _, plugin := range []toolplugin.Plugin{toolplugin.Helm(), toolplugin.Promtool()} {
		if !slices.Contains(disabled, plugin.Name()) {
			plugins = append(plugins, plugin)
		}
	}
	return plugins, nil
}

func RenderChart(bus shareddomain.Dispatcher) func(ctx context.Context, dir, chart string) ([]byte, error) {
	handler := renderchart.NewHandler(bus)
	return func(ctx context.Context, dir, chart string) ([]byte, error) {
		return handler.Handle(ctx, renderchart.Query{Dir: dir, Chart: chart})
	}
}

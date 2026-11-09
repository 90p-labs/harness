package execplugin

import (
	"fmt"

	"github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
)

func FromConfig(plugins []config.Plugin) ([]domain.Plugin, error) {
	built := make([]domain.Plugin, 0, len(plugins))
	for _, entry := range plugins {
		events := make([]domain.EventName, 0, len(entry.Events))
		for _, name := range entry.Events {
			event := domain.EventName(name)
			if _, known := domain.KindOf(event); !known {
				return nil, fmt.Errorf("plugin %q event %q: %w", entry.Name, name, domain.ErrUnknownEvent)
			}
			events = append(events, event)
		}
		built = append(built, ExecPlugin{
			PluginName: entry.Name,
			Events:     events,
			Command:    entry.Command,
			Timeout:    entry.Timeout,
			Filter:     domain.TransitionFilter{Transition: entry.Transition, From: entry.From, To: entry.To},
		})
	}
	return built, nil
}

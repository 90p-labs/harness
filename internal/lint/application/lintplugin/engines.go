package lintplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/shared/application/eventbus"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type FileLister func(ctx context.Context, dir string, globs ...string) ([]string, error)

type Engines struct {
	buses map[domain.EngineName]shareddomain.Dispatcher
	files FileLister
}

var _ domain.EngineSet = Engines{}

func NewEngines(defaults []domain.Engine, disabled []string, execs []shareddomain.Plugin, files FileLister, tracer trace.TracerProvider, meter metric.MeterProvider) (Engines, error) {
	var names []string
	for _, engine := range defaults {
		names = append(names, string(engine.Name()))
	}
	for _, name := range disabled {
		if !slices.Contains(names, name) {
			return Engines{}, fmt.Errorf("%w: lint.disabled names %q, the default lint plugins are %v", domain.ErrUnknownEngine, name, names)
		}
	}

	buses := map[domain.EngineName]shareddomain.Dispatcher{}
	for _, engine := range defaults {
		var plugins []shareddomain.Plugin
		if !slices.Contains(disabled, string(engine.Name())) {
			plugins = append(plugins, New(engine))
		}
		buses[engine.Name()] = eventbus.NewBus(plugins, tracer, meter)
	}
	for _, plugin := range execs {
		events := plugin.Subscribes()
		if !slices.Contains(events, shareddomain.EventLintCheck) {
			if slices.Contains(events, shareddomain.EventLintPrepare) {
				return Engines{}, fmt.Errorf("%w: plugin %q", domain.ErrPrepareWithoutCheck, plugin.Name())
			}
			continue
		}
		if plugin.Name() == string(domain.EngineBiomeRule) {
			return Engines{}, fmt.Errorf("%w: plugin %q, the biome-grit plugin runs the biome-rule rules", domain.ErrEngineTaken, plugin.Name())
		}
		if slices.Contains(names, plugin.Name()) && !slices.Contains(disabled, plugin.Name()) {
			return Engines{}, fmt.Errorf("%w: plugin %q, add it to lint.disabled to replace the default", domain.ErrEngineTaken, plugin.Name())
		}
		buses[domain.EngineName(plugin.Name())] = eventbus.NewBus([]shareddomain.Plugin{plugin}, tracer, meter)
	}
	return Engines{buses: buses, files: files}, nil
}

func (e Engines) Cover(byEngine map[domain.EngineName][]domain.Rule) error {
	for _, engine := range slices.Sorted(maps.Keys(byEngine)) {
		if _, ok := e.buses[engine]; !ok {
			return unknownEngine(engine, byEngine[engine])
		}
	}
	return nil
}

func (e Engines) Check(ctx context.Context, dir string, engine domain.EngineName, rules []domain.Rule) ([]shareddomain.Finding, error) {
	files, err := e.files(ctx, dir, scopes(rules)...)
	if err != nil {
		return nil, err
	}
	verdict, err := e.dispatch(ctx, shareddomain.EventLintCheck, dir, engine, domain.CheckPayload{Rules: rules, Files: append([]string{}, files...)})
	if err != nil {
		return nil, err
	}
	findings := verdict.Findings
	for i, finding := range findings {
		at := slices.IndexFunc(rules, func(r domain.Rule) bool { return r.ID == finding.Rule })
		if at < 0 {
			return nil, fmt.Errorf("%w: plugin %s sent rule %q, which is not one of its rules", domain.ErrBadFinding, engine, finding.Rule)
		}
		if filepath.IsAbs(finding.File) {
			return nil, fmt.Errorf("%w: plugin %s sent file %q, which is not relative to the repository", domain.ErrBadFinding, engine, finding.File)
		}
		findings[i].Severity = rules[at].Severity
	}
	return findings, nil
}

func (e Engines) Prepare(ctx context.Context, dir string, engine domain.EngineName, rules []domain.Rule) error {
	_, err := e.dispatch(ctx, shareddomain.EventLintPrepare, dir, engine, domain.CheckPayload{Rules: rules})
	return err
}

func (e Engines) dispatch(ctx context.Context, event shareddomain.EventName, dir string, engine domain.EngineName, payload domain.CheckPayload) (shareddomain.Verdict, error) {
	bus, ok := e.buses[engine]
	if !ok {
		return shareddomain.Verdict{}, unknownEngine(engine, payload.Rules)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	result, err := bus.Dispatch(ctx, shareddomain.Envelope{Version: shareddomain.EnvelopeVersion, Event: event, Cwd: dir, Payload: raw})
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	var failures []error
	for _, failure := range result.Failures {
		failures = append(failures, failure.Err)
	}
	if err := errors.Join(failures...); err != nil {
		return shareddomain.Verdict{}, err
	}
	if result.Verdict.Kind == shareddomain.VerdictDeny {
		return shareddomain.Verdict{}, fmt.Errorf("%w: %s", domain.ErrEngineDenied, result.Verdict.Reason)
	}
	return result.Verdict, nil
}

func unknownEngine(engine domain.EngineName, rules []domain.Rule) error {
	return fmt.Errorf("%w: rule %s names engine %q, and no lint plugin has that name", domain.ErrUnknownEngine, rules[0].ID, engine)
}

func scopes(rules []domain.Rule) []string {
	var globs []string
	for _, rule := range rules {
		if rule.Scope == "" {
			return nil
		}
		if !slices.Contains(globs, rule.Scope) {
			globs = append(globs, rule.Scope)
		}
	}
	return globs
}

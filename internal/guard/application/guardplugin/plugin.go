package guardplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/90p-labs/harness/internal/guard/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	namePrefix   = "guard."
	metricDenies = "harness.guard.denies"
	attrRule     = attribute.Key("harness.rule")
)

type Deps struct {
	ReadFile      func(path string) ([]byte, error)
	DefaultBranch func(ctx context.Context, dir string) (string, error)
	Stop          domain.StopDecider
	CommitChecked func(ctx context.Context, dir string) (bool, error)
	ChangedFiles  func(ctx context.Context, dir, base string) ([]string, error)
	PatchID       func(ctx context.Context, dir, base string) (string, error)
	SaveReview    func(ctx context.Context, dir string, review domain.Review) error
	Reviews       func(ctx context.Context, dir string, scope domain.ReviewScope) ([]domain.Review, error)
	ReworkReasons func(ctx context.Context, dir, runID string) ([]string, error)
	Models        map[shareddomain.Host]map[string][]string
	Worktree      string
	StrictMinor   bool
	Panel         PanelSource
	Meter         metric.Meter
}

func Plugins(disabled []string, deps Deps) ([]shareddomain.Plugin, error) {
	rules := domain.Rules()
	if err := domain.CheckRuleIDs(disabled); err != nil {
		return nil, err
	}
	denies, err := deps.Meter.Int64Counter(metricDenies)
	if err != nil {
		return nil, fmt.Errorf("create counter %s: %w", metricDenies, err)
	}

	var plugins []shareddomain.Plugin
	for _, rule := range rules {
		if !slices.Contains(disabled, rule.ID) {
			plugins = append(plugins, toolPlugin{rule: rule, deps: deps, denies: denies})
		}
	}
	if !slices.Contains(disabled, domain.RuleStopHoldsTurn) {
		plugins = append(plugins, stopPlugin{stop: deps.Stop, denies: denies})
	}
	if !slices.Contains(disabled, domain.RuleReviewModels) {
		plugins = append(plugins, reviewRecorder{deps: deps}, reviewShip{deps: deps, denies: denies})
	}
	if !slices.Contains(disabled, domain.RuleReviewFindings) {
		plugins = append(plugins, findingsShip{deps: deps, denies: denies})
	}
	return plugins, nil
}

func countDeny(ctx context.Context, denies metric.Int64Counter, rule string) {
	denies.Add(ctx, 1, metric.WithAttributes(attrRule.String(rule)))
}

func deny(reason string) shareddomain.Verdict {
	return shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: reason}
}

type toolPlugin struct {
	rule   domain.Rule
	deps   Deps
	denies metric.Int64Counter
}

func (p toolPlugin) Name() string { return namePrefix + p.rule.ID }

func (p toolPlugin) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventToolBefore}
}

func (p toolPlugin) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	var payload shareddomain.ToolPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return shareddomain.Verdict{}, fmt.Errorf("decode tool payload: %w", err)
	}
	call, err := domain.ToolCallOf(payload)
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	var textErr error
	call.Texts, textErr = p.readTexts(envelope.Cwd, call.Command)

	reason, denied := p.rule.Denies(call, domain.RuleContext{
		Cwd:           envelope.Cwd,
		Worktree:      p.deps.Worktree,
		AgentModels:   p.deps.Models[envelope.Host],
		ModelOptional: envelope.Host == shareddomain.HostCodex,
		TextErr:       textErr,
		DefaultBranch: func() (string, error) {
			return p.deps.DefaultBranch(ctx, envelope.Cwd)
		},
		CommitChecked: func() (bool, error) {
			return p.deps.CommitChecked(ctx, envelope.Cwd)
		},
	})
	if !denied {
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
	}
	countDeny(ctx, p.denies, p.rule.ID)
	return deny(reason), nil
}

func (p toolPlugin) readTexts(cwd, line string) ([][]string, error) {
	files := domain.TextFiles(line)
	texts := make([][]string, len(files))
	var readErr error
	for i, names := range files {
		for _, name := range names {
			if !filepath.IsAbs(name) {
				name = filepath.Join(cwd, name)
			}
			body, err := p.deps.ReadFile(name)
			if err != nil {
				readErr = err
				continue
			}
			texts[i] = append(texts[i], string(body))
		}
	}
	return texts, readErr
}

type stopPlugin struct {
	stop   domain.StopDecider
	denies metric.Int64Counter
}

func (p stopPlugin) Name() string { return namePrefix + domain.RuleStopHoldsTurn }

func (p stopPlugin) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventTurnStopping}
}

func (p stopPlugin) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	var payload shareddomain.StopPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return shareddomain.Verdict{}, fmt.Errorf("decode stop payload: %w", err)
	}
	denied, reason, err := p.stop.DecideStop(ctx, payload.BackgroundAgents > 0)
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	if !denied {
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
	}
	countDeny(ctx, p.denies, domain.RuleStopHoldsTurn)
	return deny(reason + ". " + domain.StopHoldsTurnNextStep), nil
}

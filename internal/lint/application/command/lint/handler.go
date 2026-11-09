package lint

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/lint/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	spanLintRun       = "lint.run"
	spanLintEngine    = "lint.engine"
	eventFindings     = "lint.findings"
	metricFindings    = "harness.lint.findings"
	staleFormat       = "%w (have %q, want %q)"
	engineErrorFormat = "lint engine %s: %w"
)

const (
	attrRule     = attribute.Key("harness.lint.rule")
	attrSeverity = attribute.Key("harness.lint.severity")
	attrEngine   = attribute.Key("harness.lint.engine")
	attrRules    = attribute.Key("harness.lint.rules")
	attrFindings = attribute.Key("harness.lint.findings")
	attrCount    = attribute.Key("harness.lint.count")
)

type ruleSeverity struct {
	rule     string
	severity shareddomain.Severity
}

func compareRuleSeverity(a, b ruleSeverity) int {
	return cmp.Or(cmp.Compare(a.rule, b.rule), cmp.Compare(a.severity, b.severity))
}

type Command struct {
	Dir string
}

type Lookup func(domain.PackName) (domain.Pack, error)

type Handler struct {
	packs    []domain.PackName
	lookup   Lookup
	engines  domain.EngineSet
	version  string
	tracer   trace.Tracer
	findings metric.Int64Counter
}

func NewHandler(packs []domain.PackName, lookup Lookup, engines domain.EngineSet, version string, tracer trace.Tracer, meter metric.Meter) Handler {
	findings, _ := meter.Int64Counter(metricFindings)
	return Handler{packs: packs, lookup: lookup, engines: engines, version: version, tracer: tracer, findings: findings}
}

func (h Handler) Handle(ctx context.Context, cmd Command) (report shareddomain.Report, err error) {
	ctx, span := h.tracer.Start(ctx, spanLintRun)
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	report = shareddomain.Report{Version: shareddomain.ReportVersion, Findings: []shareddomain.Finding{}}
	if len(h.packs) == 0 {
		return report, nil
	}
	if err := h.CheckVersion(cmd.Dir); err != nil {
		return shareddomain.Report{}, err
	}
	rules, err := domain.RulesByEngine(h.packs, h.lookup)
	if err != nil {
		return shareddomain.Report{}, err
	}
	for _, engine := range slices.Sorted(maps.Keys(rules)) {
		engineCtx, engineSpan := h.tracer.Start(ctx, spanLintEngine)
		found, err := h.engines.Check(engineCtx, cmd.Dir, engine, rules[engine])
		engineSpan.SetAttributes(
			attrEngine.String(string(engine)),
			attrRules.Int(len(rules[engine])),
			attrFindings.Int(len(found)),
		)
		if err != nil {
			engineSpan.RecordError(err)
			engineSpan.SetStatus(codes.Error, err.Error())
			engineSpan.End()
			return shareddomain.Report{}, fmt.Errorf(engineErrorFormat, engine, err)
		}
		counts := map[ruleSeverity]int{}
		for _, f := range found {
			counts[ruleSeverity{f.Rule, f.Severity}]++
			h.findings.Add(ctx, 1, metric.WithAttributes(attrRule.String(f.Rule), attrSeverity.String(string(f.Severity))))
		}
		for _, key := range slices.SortedFunc(maps.Keys(counts), compareRuleSeverity) {
			engineSpan.AddEvent(eventFindings, trace.WithAttributes(
				attrRule.String(key.rule),
				attrSeverity.String(string(key.severity)),
				attrCount.Int(counts[key]),
			))
		}
		engineSpan.End()
		report.Findings = append(report.Findings, found...)
	}
	slices.SortFunc(report.Findings, func(a, b shareddomain.Finding) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Rule, b.Rule))
	})
	return report, nil
}

func (h Handler) CheckVersion(dir string) error {
	raw, err := os.ReadFile(filepath.Join(dir, domain.LintDir, domain.VersionFile))
	have := strings.TrimSpace(string(raw))
	if errors.Is(err, fs.ErrNotExist) || (err == nil && have != h.version) {
		return fmt.Errorf(staleFormat, domain.ErrStaleLintFiles, have, h.version)
	}
	return err
}

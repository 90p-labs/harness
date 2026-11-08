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

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	spanLintRun       = "lint.run"
	metricFindings    = "harness.lint.findings"
	staleFormat       = "%w (have %q, want %q)"
	engineErrorFormat = "lint engine %s: %w"
)

const (
	attrRule     = attribute.Key("harness.lint.rule")
	attrSeverity = attribute.Key("harness.lint.severity")
)

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
		found, err := h.engines.Check(ctx, cmd.Dir, engine, rules[engine])
		if err != nil {
			return shareddomain.Report{}, fmt.Errorf(engineErrorFormat, engine, err)
		}
		for _, f := range found {
			h.findings.Add(ctx, 1, metric.WithAttributes(attrRule.String(f.Rule), attrSeverity.String(string(f.Severity))))
		}
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

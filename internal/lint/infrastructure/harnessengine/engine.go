package harnessengine

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/git"
)

const (
	nextRulePrefix = "next/"
	firstLine      = 1
	recursiveGlob  = "**"
)

var (
	nextSrcFiles  = []string{"instrumentation.ts", "middleware.ts"}
	ruleFileRules = []string{domain.RuleRuleTests, domain.RuleSLOAlert, domain.RuleRunbook}
)

var checks = map[string]domain.Check{
	domain.RuleSrcRoots:             domain.SrcRoots,
	domain.RuleContextLayers:        domain.ContextLayers,
	domain.RuleParentNoCode:         domain.ParentNoCode,
	domain.RuleSharedLayers:         domain.SharedLayers,
	domain.RuleSharedBaseOnly:       domain.SharedBaseOnly,
	domain.RuleFileSuffix:           domain.FileSuffix,
	domain.RuleCQRSFolders:          domain.CQRSFolders,
	domain.RuleHandlerPair:          domain.HandlerPair,
	domain.RuleModulePerContext:     domain.ModulePerContext,
	domain.RuleOneKernel:            domain.OneKernel,
	domain.RuleDomainPurity:         domain.DomainPurity,
	domain.RuleLayerDirection:       domain.LayerDirection,
	domain.RuleCrossContext:         domain.CrossContext,
	domain.RuleRepositoryPort:       domain.RepositoryPort,
	domain.RuleMapperLocation:       domain.MapperLocation,
	domain.RuleEventPastTense:       domain.EventPastTense,
	domain.RuleClientServerBoundary: domain.ClientServerBoundary,
	domain.RuleAppRouterFiles:       domain.AppRouterFiles,
	domain.RuleThinRouteHandler:     domain.ThinRouteHandler,
	domain.RuleUIImports:            domain.UIImports,
	domain.RuleComponentsJSON:       domain.ComponentsJSON,
	domain.RuleComponentsNoBusiness: domain.ComponentsNoBusiness,
	domain.RuleAppThroughKernel:     domain.AppThroughKernel,
	domain.RuleUtilsLocation:        domain.UtilsLocation,
	domain.RuleApplyHandler:         domain.ApplyHandler,
	domain.RuleProjectionReadOnly:   domain.ProjectionReadOnly,
	domain.RuleOtelSDK:              domain.OtelSDK,
	domain.RuleBusInstrumented:      domain.BusInstrumented,
	domain.RuleRuleTests:            ruleTests,
	domain.RuleSLOAlert:             sloAlert,
	domain.RuleRunbook:              runbook,
	domain.RuleDashboardSeries:      dashboardSeries,
}

var _ domain.Engine = (*Engine)(nil)

type Engine struct {
	settings domain.Settings
}

func New(settings domain.Settings) *Engine {
	return &Engine{settings: settings}
}

func (*Engine) Name() domain.EngineName {
	return domain.EngineHarness
}

func (*Engine) Prepare(string, []domain.Rule) error {
	return nil
}

func (e *Engine) Run(ctx context.Context, dir string, rules []domain.Rule) ([]shareddomain.Finding, error) {
	tree, err := loadTree(ctx, dir)
	if err != nil {
		return nil, err
	}
	scopes := make([]string, 0, len(rules))
	for _, rule := range rules {
		scopes = append(scopes, rule.Scope)
	}
	graph, err := importGraph(tree, scopes)
	if err != nil {
		return nil, err
	}
	settings := e.settingsFor(rules)
	resources, err := renderChart(ctx, dir, tree, settings.Observability.Chart, rules)
	if err != nil {
		return nil, err
	}

	var findings []shareddomain.Finding
	for _, rule := range rules {
		var violations []domain.Violation
		if check, ok := checks[rule.ID]; ok {
			violations = check(tree, graph, settings)
		} else if chartCheck, ok := chartChecks[rule.ID]; ok {
			violations = chartCheck(resources)
		}
		for _, violation := range violations {
			if !git.MatchesAny([]string{scopeOf(rule, settings.Observability)}, violation.File) {
				continue
			}
			findings = append(findings, shareddomain.Finding{
				Rule:     rule.ID,
				Severity: rule.Severity,
				File:     violation.File,
				Line:     max(violation.Line, firstLine),
				Message:  rule.Message,
			})
		}
	}
	return findings, nil
}

func scopeOf(rule domain.Rule, paths domain.Observability) string {
	switch {
	case chartChecks[rule.ID] != nil:
		return path.Join(paths.Chart, recursiveGlob)
	case slices.Contains(ruleFileRules, rule.ID):
		return paths.Rules
	case rule.ID == domain.RuleDashboardSeries:
		return paths.Dashboards
	}
	return rule.Scope
}

func (e *Engine) settingsFor(rules []domain.Rule) domain.Settings {
	settings := e.settings
	settings.Observability.Chart = path.Clean(settings.Observability.Chart)
	if slices.ContainsFunc(rules, func(rule domain.Rule) bool { return strings.HasPrefix(rule.ID, nextRulePrefix) }) {
		settings.DDD.SrcFiles = slices.Concat(settings.DDD.SrcFiles, nextSrcFiles)
	}
	return settings
}

type sourceReader struct {
	dir   string
	cache map[string][]byte
}

func (r *sourceReader) ReadSource(file string) ([]byte, error) {
	if source, ok := r.cache[file]; ok {
		return source, nil
	}
	source, err := os.ReadFile(filepath.Join(r.dir, filepath.FromSlash(file)))
	if err != nil {
		return nil, err
	}
	r.cache[file] = source
	return source, nil
}

func loadTree(ctx context.Context, dir string) (domain.Tree, error) {
	files, err := git.Files(ctx, dir)
	if err != nil {
		return domain.Tree{}, err
	}
	return domain.Tree{Files: files, Reader: &sourceReader{dir: dir, cache: map[string][]byte{}}}, nil
}

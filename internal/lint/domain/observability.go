package domain

import (
	"bytes"
	"encoding/json"
	"path"
	"regexp"
	"slices"
)

const (
	RuleOtelSDK             = "obs/otel-sdk"
	RuleBusInstrumented     = "obs/bus-instrumented"
	RuleProbes              = "obs/probes"
	RuleLivenessProcessOnly = "obs/liveness-process-only"
	RuleImmutableTags       = "obs/immutable-tags"
	RuleHPATarget           = "obs/hpa-target"
	RuleRuleTests           = "obs/rule-tests"
	RuleSLOAlert            = "obs/slo-alert"
	RuleRunbook             = "obs/runbook"
	RuleDashboardSeries     = "obs/dashboard-series"
	packageFile             = "package.json"
	suffixMiddleware        = "middleware"
	otelAPIPackage          = "@opentelemetry/api"
	otelSDKPackage          = "@opentelemetry/sdk-node"
	nodeSDKStart            = "new NodeSDK("
)

var otelAPIImport = regexp.MustCompile(`['"]` + regexp.QuoteMeta(otelAPIPackage) + `['"]`)

func OtelSDK(tree Tree, _ Graph, _ Settings) []Violation {
	if !slices.Contains(tree.Files, packageFile) {
		return nil
	}
	manifest, err := tree.Source(packageFile)
	if err != nil {
		return nil
	}
	var parsed struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	_ = json.Unmarshal(manifest, &parsed)
	_, hasAPI := parsed.Dependencies[otelAPIPackage]
	_, hasSDK := parsed.Dependencies[otelSDKPackage]
	if hasAPI && hasSDK && startsNodeSDK(tree) {
		return nil
	}
	return []Violation{{File: packageFile}}
}

func startsNodeSDK(tree Tree) bool {
	return slices.ContainsFunc(tree.Files, func(file string) bool {
		if !slices.Contains(scriptExts, path.Ext(file)) {
			return false
		}
		source, err := tree.Source(file)
		return err == nil && bytes.Contains(source, []byte(nodeSDKStart))
	})
}

func BusInstrumented(tree Tree, graph Graph, settings Settings) []Violation {
	kernel := path.Join(businessRoot(settings), kernelFile)
	if !slices.Contains(tree.Files, kernel) {
		return nil
	}
	for _, edge := range graph.Edges {
		if edge.From != kernel || edge.Package || fileSuffix(edge.To) != suffixMiddleware {
			continue
		}
		if source, err := tree.Source(edge.To); err == nil && otelAPIImport.Match(source) {
			return nil
		}
	}
	return []Violation{{File: kernel}}
}

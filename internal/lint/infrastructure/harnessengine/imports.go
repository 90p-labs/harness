package harnessengine

import (
	"bytes"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/90p-labs/harness/internal/lint/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

const (
	indexName    = "index"
	typeOnlyHead = 1
	specifierTag = 2
)

var (
	importPattern = regexp.MustCompile(`(?m)(?:^[ \t]*(?:import|export)\s+(type\s+)?(?:(?:[\w$*, \t]|\{[^}]*\})*?\bfrom\s*)?|\b(?:import|require)\s*\(\s*)['"]([^'"]+)['"]`)

	sourceExtensions = []string{".ts", ".tsx", ".js", ".jsx"}

	newline = []byte("\n")
)

func importGraph(tree domain.Tree, scopes []string) (domain.Graph, error) {
	paths, err := loadAliases(tree)
	if err != nil {
		return domain.Graph{}, err
	}
	known := make(map[string]bool, len(tree.Files))
	for _, file := range tree.Files {
		known[file] = true
	}
	var graph domain.Graph
	for _, file := range tree.Files {
		if !slices.Contains(sourceExtensions, path.Ext(file)) || !git.MatchesAny(scopes, file) {
			continue
		}
		source, err := tree.Source(file)
		if err != nil {
			return domain.Graph{}, err
		}
		for _, match := range importPattern.FindAllSubmatchIndex(source, -1) {
			edge := resolveImport(known, paths, file, string(source[match[2*specifierTag]:match[2*specifierTag+1]]), match[2*typeOnlyHead] >= 0)
			edge.Line = 1 + bytes.Count(source[:match[0]], newline)
			graph.Edges = append(graph.Edges, edge)
		}
	}
	return graph, nil
}

func resolveImport(files map[string]bool, paths aliases, from, specifier string, typeOnly bool) domain.Import {
	edge := domain.Import{From: from, Specifier: specifier, TypeOnly: typeOnly}
	var candidates []string
	if strings.HasPrefix(specifier, currentFolder) {
		candidates = []string{path.Join(path.Dir(from), specifier)}
	} else if aliased, ok := paths.resolve(specifier); ok {
		candidates = aliased
	} else {
		edge.Package = true
		return edge
	}
	edge.To = candidates[0]
	for _, candidate := range candidates {
		for _, variant := range fileVariants(candidate) {
			if files[variant] {
				edge.To = variant
				return edge
			}
		}
	}
	return edge
}

func fileVariants(candidate string) []string {
	variants := []string{candidate}
	for _, ext := range sourceExtensions {
		variants = append(variants, candidate+ext)
	}
	for _, ext := range sourceExtensions {
		variants = append(variants, path.Join(candidate, indexName+ext))
	}
	return variants
}

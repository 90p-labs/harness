package domain

import (
	"bytes"
	"regexp"
	"strings"
)

const (
	RuleApplyHandler       = "ddd-es/apply-handler"
	RuleProjectionReadOnly = "ddd-es/projection-read-only"
	applyPrefix            = "apply"
	raiseNameGroup         = 1
)

var (
	raisePattern = regexp.MustCompile(`raise\(\s*new\s+(\w+)\s*\(`)

	lineBreak = []byte("\n")
)

func ApplyHandler(tree Tree, _ Graph, _ Settings) []Violation {
	var found []Violation
	for _, file := range tree.Files {
		if fileSuffix(file) != suffixAggregate {
			continue
		}
		source, err := tree.Source(file)
		if err != nil {
			continue
		}
		for _, match := range raisePattern.FindAllSubmatchIndex(source, -1) {
			event := string(source[match[2*raiseNameGroup]:match[2*raiseNameGroup+1]])
			if !regexp.MustCompile(`\b` + applyPrefix + event + `\(`).Match(source) {
				found = append(found, Violation{File: file, Line: 1 + bytes.Count(source[:match[0]], lineBreak)})
			}
		}
	}
	return found
}

func ProjectionReadOnly(_ Tree, graph Graph, _ Settings) []Violation {
	commandsPath := pathSeparator + commandsFolder + pathSeparator
	var found []Violation
	for _, edge := range graph.Edges {
		if fileSuffix(edge.From) != suffixProjection || edge.Package {
			continue
		}
		suffix := fileSuffix(edge.To)
		if suffix == suffixRepository || suffix == suffixAggregate || strings.Contains(pathSeparator+edge.To, commandsPath) {
			found = append(found, Violation{File: edge.From, Line: edge.Line})
		}
	}
	return found
}

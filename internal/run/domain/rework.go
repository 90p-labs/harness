package domain

import (
	"strings"
	"unicode"
)

type ReworkClass string

const (
	ReworkImplementation ReworkClass = "implementation"
	ReworkSpecGap        ReworkClass = "spec-gap"
	ReworkGateCheckable  ReworkClass = "gate-checkable"
	ReworkMinor          ReworkClass = "minor"
)

func (c ReworkClass) Known() bool {
	switch c {
	case ReworkImplementation, ReworkSpecGap, ReworkGateCheckable, ReworkMinor:
		return true
	}
	return false
}

type ReviewSeverity string

const (
	SeverityBlocker ReviewSeverity = "blocker"
	SeverityMajor   ReviewSeverity = "major"
	SeverityMinor   ReviewSeverity = "minor"
)

var severityRank = map[ReviewSeverity]int{SeverityMinor: 1, SeverityMajor: 2, SeverityBlocker: 3}

func (s ReviewSeverity) Known() bool { return severityRank[s] > 0 }

func HighestSeverity(findings string) ReviewSeverity {
	var highest ReviewSeverity
	for _, finding := range strings.Split(findings, ";") {
		words := strings.FieldsFunc(finding, func(r rune) bool { return !unicode.IsLetter(r) })
		for _, word := range words {
			severity := ReviewSeverity(strings.ToLower(word))
			if !severity.Known() {
				continue
			}
			if severityRank[severity] > severityRank[highest] {
				highest = severity
			}
			break
		}
	}
	return highest
}

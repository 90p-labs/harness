package domain

import (
	"strings"
	"unicode"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
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

var severityRank = map[shareddomain.ReviewSeverity]int{
	shareddomain.SeverityMinor: 1, shareddomain.SeverityMajor: 2, shareddomain.SeverityBlocker: 3,
}

func HighestSeverity(findings string) shareddomain.ReviewSeverity {
	var highest shareddomain.ReviewSeverity
	for _, finding := range strings.Split(findings, ";") {
		words := strings.FieldsFunc(finding, func(r rune) bool { return !unicode.IsLetter(r) })
		for _, word := range words {
			severity := shareddomain.ReviewSeverity(strings.ToLower(word))
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

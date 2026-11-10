package domain

import (
	"errors"
	"fmt"
	"strings"
)

type ReviewSeverity string

const (
	SeverityBlocker ReviewSeverity = "blocker"
	SeverityMajor   ReviewSeverity = "major"
	SeverityMinor   ReviewSeverity = "minor"
)

func (s ReviewSeverity) Known() bool {
	switch s {
	case SeverityBlocker, SeverityMajor, SeverityMinor:
		return true
	}
	return false
}

type FindingVerdict string

const (
	FindingReal    FindingVerdict = "real"
	FindingNotReal FindingVerdict = "not real"
)

func (v FindingVerdict) Known() bool {
	switch v {
	case FindingReal, FindingNotReal:
		return true
	}
	return false
}

type FindingStatus string

const (
	FindingOpen   FindingStatus = "open"
	FindingClosed FindingStatus = "closed"
)

func (s FindingStatus) Known() bool {
	switch s {
	case FindingOpen, FindingClosed:
		return true
	}
	return false
}

type ReviewFinding struct {
	ID       string
	Severity ReviewSeverity
	Verdict  FindingVerdict
	Status   FindingStatus
}

const (
	columnID       = "id"
	columnSeverity = "severity"
	columnVerdict  = "verdict"
	columnStatus   = "status"
	tableBar       = "|"
)

var requiredColumns = []string{columnID, columnSeverity, columnVerdict, columnStatus}

var ErrNoFindingsTable = errors.New("no findings table with the columns ID, Severity, Verdict and Status")

func ParseReviewFindings(text string) ([]ReviewFinding, error) {
	var (
		findings []ReviewFinding
		columns  map[string]int
		seen     = map[string]bool{}
		found    bool
		inTable  bool
	)
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, tableBar) {
			columns, inTable = nil, false
			continue
		}
		cells := tableCells(line)
		switch {
		case columns != nil && isSeparator(cells):
			continue
		case columns != nil:
			finding, err := findingOf(cells, columns)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			if seen[finding.ID] {
				return nil, fmt.Errorf("line %d: duplicate ID %q", i+1, finding.ID)
			}
			seen[finding.ID] = true
			findings = append(findings, finding)
		case !inTable:
			inTable = true
			var err error
			if columns, err = headerColumns(cells); err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			found = found || columns != nil
		}
	}
	if !found {
		return nil, ErrNoFindingsTable
	}
	return findings, nil
}

func NamesFinding(reason, id string) bool {
	for from := 0; from < len(reason); {
		at := strings.Index(reason[from:], id)
		if at < 0 {
			return false
		}
		start := from + at
		end := start + len(id)
		if !isIDRune(reason, start-1) && !isIDRune(reason, end) {
			return true
		}
		from = start + 1
	}
	return false
}

func isIDRune(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	c := s[i]
	return c == '_' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func tableCells(line string) []string {
	cells := strings.Split(strings.Trim(line, tableBar), tableBar)
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells
}

func isSeparator(cells []string) bool {
	for _, cell := range cells {
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return true
}

func headerColumns(cells []string) (map[string]int, error) {
	columns := map[string]int{}
	for i, cell := range cells {
		columns[strings.ToLower(cell)] = i
	}
	var missing []string
	for _, name := range requiredColumns {
		if _, ok := columns[name]; !ok {
			missing = append(missing, name)
		}
	}
	hasAllColumns := len(missing) == 0
	hasNoRequiredColumn := len(missing) == len(requiredColumns)
	if hasAllColumns {
		return columns, nil
	}
	if hasNoRequiredColumn {
		return nil, nil
	}
	return nil, fmt.Errorf("missing the columns %s", strings.Join(missing, ", "))
}

func findingOf(cells []string, columns map[string]int) (ReviewFinding, error) {
	cell := func(name string) (string, error) {
		if columns[name] >= len(cells) {
			return "", fmt.Errorf("the row has no %s cell", name)
		}
		return strings.ToLower(cells[columns[name]]), nil
	}
	var values [4]string
	for i, name := range requiredColumns {
		value, err := cell(name)
		if err != nil {
			return ReviewFinding{}, err
		}
		values[i] = value
	}
	finding := ReviewFinding{
		ID:       cells[columns[columnID]],
		Severity: ReviewSeverity(values[1]),
		Verdict:  FindingVerdict(values[2]),
		Status:   FindingStatus(values[3]),
	}
	switch {
	case finding.ID == "":
		return ReviewFinding{}, errors.New("empty ID")
	case !finding.Severity.Known():
		return ReviewFinding{}, fmt.Errorf("unknown severity %q", finding.Severity)
	case !finding.Verdict.Known():
		return ReviewFinding{}, fmt.Errorf("unknown verdict %q", finding.Verdict)
	case !finding.Status.Known():
		return ReviewFinding{}, fmt.Errorf("unknown status %q", finding.Status)
	}
	return finding, nil
}

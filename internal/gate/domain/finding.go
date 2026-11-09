package domain

import shareddomain "github.com/90p-labs/harness/internal/shared/domain"

func NewFindings(found, baseline []shareddomain.Finding) []shareddomain.Finding {
	known := make(map[shareddomain.FindingKey]struct{}, len(baseline))
	for _, f := range baseline {
		known[f.Key()] = struct{}{}
	}
	var fresh []shareddomain.Finding
	for _, f := range found {
		if f.Severity != shareddomain.SeverityError {
			continue
		}
		if _, ok := known[f.Key()]; !ok {
			fresh = append(fresh, f)
		}
	}
	return fresh
}

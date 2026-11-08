package domain

const (
	ReportVersion = 1
	SeverityError = "error"
)

type Finding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

type Report struct {
	Version  int       `json:"version"`
	Findings []Finding `json:"findings"`
}

type findingKey struct {
	rule, file, message string
}

func keyOf(f Finding) findingKey {
	return findingKey{rule: f.Rule, file: f.File, message: f.Message}
}

func NewFindings(found, baseline []Finding) []Finding {
	known := make(map[findingKey]struct{}, len(baseline))
	for _, f := range baseline {
		known[keyOf(f)] = struct{}{}
	}
	var fresh []Finding
	for _, f := range found {
		if f.Severity != SeverityError {
			continue
		}
		if _, ok := known[keyOf(f)]; !ok {
			fresh = append(fresh, f)
		}
	}
	return fresh
}

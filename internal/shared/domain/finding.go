package domain

const ReportVersion = 1

type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Message  string   `json:"message"`
}

type Report struct {
	Version  int       `json:"version"`
	Findings []Finding `json:"findings"`
}

type FindingKey struct {
	Rule, File, Message string
}

func (f Finding) Key() FindingKey {
	return FindingKey{Rule: f.Rule, File: f.File, Message: f.Message}
}

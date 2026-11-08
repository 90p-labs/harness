package domain

type VerdictKind string

const (
	VerdictAllow   VerdictKind = "allow"
	VerdictDeny    VerdictKind = "deny"
	VerdictContext VerdictKind = "context"
)

func (k VerdictKind) Known() bool {
	switch k {
	case VerdictAllow, VerdictDeny, VerdictContext:
		return true
	}
	return false
}

type Verdict struct {
	Kind     VerdictKind `json:"verdict"`
	Reason   string      `json:"reason,omitempty"`
	Context  string      `json:"context,omitempty"`
	Findings []Finding   `json:"findings,omitempty"`
}

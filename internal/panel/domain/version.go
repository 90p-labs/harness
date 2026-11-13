package domain

import "strings"

// Version has a plan part and a run part, written <plan>.<run>.
type Version struct {
	Plan string
	Run  string
}

func (v Version) String() string { return v.Plan + "." + v.Run }

// ParseVersion splits <plan>.<run>. A malformed value has empty parts, so it
// never matches a card version.
func ParseVersion(s string) Version {
	plan, run, ok := strings.Cut(s, ".")
	if !ok || plan == "" || run == "" || strings.Contains(run, ".") {
		return Version{}
	}
	return Version{Plan: plan, Run: run}
}

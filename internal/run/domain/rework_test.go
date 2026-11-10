package domain_test

import (
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestHighestSeverity(t *testing.T) {
	cases := []struct {
		name     string
		findings string
		want     shareddomain.ReviewSeverity
	}{
		{"first severity word of each finding", "F1, major, a.go:12 no blocker test; F2 minor b.go:3", shareddomain.SeverityMajor},
		{"blocker wins", "F1 minor a.go:1; F2, Blocker, b.go:2", shareddomain.SeverityBlocker},
		{"none", "F1 a.go:1 broken", ""},
		{"empty", "", ""},
		{"word inside another word does not count", "F1 majority a.go:1", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.HighestSeverity(c.findings); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestReworkClassKnown(t *testing.T) {
	for _, class := range []domain.ReworkClass{domain.ReworkImplementation, domain.ReworkSpecGap, domain.ReworkGateCheckable, domain.ReworkMinor} {
		if !class.Known() {
			t.Errorf("%q is not known", class)
		}
	}
	if domain.ReworkClass("other").Known() || domain.ReworkClass("").Known() {
		t.Error("an unknown class is known")
	}
	if !shareddomain.SeverityBlocker.Known() || shareddomain.ReviewSeverity("x").Known() {
		t.Error("severity Known is wrong")
	}
}

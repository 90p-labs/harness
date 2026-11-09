package domain_test

import (
	"testing"

	"github.com/90p-labs/harness/internal/shared/domain"
)

func TestStricterTrack(t *testing.T) {
	const none domain.Track = ""
	cases := []struct {
		name string
		a, b domain.Track
		want domain.Track
	}{
		{"empty and empty", none, none, none},
		{"empty and light", none, domain.TrackLight, domain.TrackLight},
		{"light and empty", domain.TrackLight, none, domain.TrackLight},
		{"empty and full", none, domain.TrackFull, domain.TrackFull},
		{"light and light", domain.TrackLight, domain.TrackLight, domain.TrackLight},
		{"light and full", domain.TrackLight, domain.TrackFull, domain.TrackFull},
		{"full and light", domain.TrackFull, domain.TrackLight, domain.TrackFull},
		{"full and empty", domain.TrackFull, none, domain.TrackFull},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.Stricter(c.a, c.b); got != c.want {
				t.Fatalf("Stricter(%q, %q) = %q, want %q", c.a, c.b, got, c.want)
			}
		})
	}
}

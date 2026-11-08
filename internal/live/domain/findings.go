package domain

import (
	"fmt"
	"strings"
	"time"
)

const rollbackNotConfigured = "not configured, set live.rollback"

type Findings struct {
	Release       Release
	RunID         string
	Round         int
	Breaches      []SignalBreach
	DeployFailure string
	Rollback      string
}

func (f Findings) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Regression in live round %d of run %s: PR %s, sha %s, %s.\n",
		f.Round, f.RunID, f.Release.PRURL, f.Release.MergeSHA, f.Release.Environment)
	if f.DeployFailure != "" {
		fmt.Fprintf(&b, "- %s\n", f.DeployFailure)
	}
	for _, br := range f.Breaches {
		fmt.Fprintf(&b, "- %s, window %s", br.Reason, shortDuration(br.Threshold.Window))
		if br.Threshold.Panel != "" {
			fmt.Fprintf(&b, ", panel %s", br.Threshold.Panel)
		}
		b.WriteString("\n")
	}
	rollback := f.Rollback
	if rollback == "" {
		rollback = rollbackNotConfigured
	}
	fmt.Fprintf(&b, "Rollback: %s\n", rollback)
	return b.String()
}

func shortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

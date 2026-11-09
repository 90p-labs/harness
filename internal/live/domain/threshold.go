package domain

import (
	"time"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type Threshold struct {
	Signal string
	Source string
	Query  string
	Breach shareddomain.Breach
	Window time.Duration
	Panel  string
}

func WatchWindow(thresholds []Threshold) time.Duration {
	var longest time.Duration
	for _, t := range thresholds {
		longest = max(longest, t.Window)
	}
	return longest
}

type BreachCheck struct {
	Op    shareddomain.Operator `json:"op"`
	Value float64               `json:"value"`
}

type CheckWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type SignalCheck struct {
	Signal      string      `json:"signal"`
	Source      string      `json:"source"`
	Query       string      `json:"query"`
	Breach      BreachCheck `json:"breach"`
	Window      CheckWindow `json:"window"`
	SHA         string      `json:"sha"`
	Environment string      `json:"environment"`
}

func NewSignalCheck(t Threshold, release Release, now time.Time) SignalCheck {
	to := release.DeployedAt.Add(t.Window)
	if now.Before(to) {
		to = now
	}
	return SignalCheck{
		Signal:      t.Signal,
		Source:      t.Source,
		Query:       t.Query,
		Breach:      BreachCheck{Op: t.Breach.Op, Value: t.Breach.Value},
		Window:      CheckWindow{From: release.DeployedAt.UTC().Format(time.RFC3339), To: to.UTC().Format(time.RFC3339)},
		SHA:         release.MergeSHA,
		Environment: release.Environment,
	}
}

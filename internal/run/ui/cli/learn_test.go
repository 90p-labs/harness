package cli_test

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/90p-labs/harness/internal/run/application/query/learn"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	learnRunID    = "20261113-004016-turn-repeated-corrections-into-durable-p"
	learnNextWant = "next: add the user corrections and the lessons that cite a line above, then ask the Conventions question; with nothing to ask, start pr-open (references/learning.md)"
)

type learnFindings struct {
	findings []shareddomain.ReviewFinding
}

func (f learnFindings) Read(context.Context, domain.Run) ([]shareddomain.ReviewFinding, error) {
	return f.findings, nil
}

func runLearn(t *testing.T, findings []shareddomain.ReviewFinding, args ...string) []string {
	t.Helper()
	store := &reportStore{run: &domain.Run{ID: learnRunID, Key: reportKey, Phase: domain.PhaseShipping}}
	return runLearnStore(t, store, findings, args...)
}

func runLearnStore(t *testing.T, store *reportStore, findings []shareddomain.ReviewFinding, args ...string) []string {
	t.Helper()
	connect := func(context.Context) (cli.Handlers, func() error, error) {
		return cli.Handlers{
			Key:   reportKey,
			Learn: learn.NewHandler(store, store, learnFindings{findings: findings}, noop.NewTracerProvider().Tracer("test")),
		}, func() error { return nil }, nil
	}
	cmd := cli.NewCommand(connect)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append([]string{"learn"}, args...))
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
}

func TestLearnCommand(t *testing.T) {
	t.Run("prints one candidate line each and the next line", func(t *testing.T) {
		got := runLearn(t, []shareddomain.ReviewFinding{
			{ID: "S1", Verdict: shareddomain.FindingReal, Lens: "standards", Location: "a.go:3", Text: "name the constant"},
			{ID: "S2", Verdict: shareddomain.FindingReal, Lens: "standards", Text: "no location here"},
		})

		want := []string{
			"run: " + learnRunID,
			"candidate: standards S1 a.go:3: name the constant",
			"candidate: standards S2: no location here",
			learnNextWant,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("takes a run id", func(t *testing.T) {
		got := runLearn(t, nil, learnRunID)

		if got[0] != "run: "+learnRunID {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("the short text names the run evidence", func(t *testing.T) {
		cmd := cli.NewCommand(func(context.Context) (cli.Handlers, func() error, error) {
			return cli.Handlers{}, func() error { return nil }, nil
		})
		sub, _, err := cmd.Find([]string{"learn"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(sub.Short, "run evidence") {
			t.Fatalf("Short %q does not name the run evidence", sub.Short)
		}
	})

	t.Run("a spec finding is no candidate", func(t *testing.T) {
		got := runLearn(t, []shareddomain.ReviewFinding{
			{ID: "P1", Verdict: shareddomain.FindingReal, Lens: "spec", Text: "one-off"},
		})

		want := []string{
			"run: " + learnRunID,
			"learn: no convention candidates and no evidence",
			"next: with no user correction, start pr-open",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestLearnCommandEvidence(t *testing.T) {
	newStore := func(stopBlocks int, rows ...domain.Transitioned) *reportStore {
		return &reportStore{
			run:  &domain.Run{ID: learnRunID, Key: reportKey, Phase: domain.PhaseShipping, StopBlocksTotal: stopBlocks},
			rows: rows,
		}
	}
	candidate := []shareddomain.ReviewFinding{
		{ID: "S1", Verdict: shareddomain.FindingReal, Lens: "standards", Location: "a.go:3", Text: "name the constant"},
	}
	runLine := "run: " + learnRunID

	cases := []struct {
		name     string
		store    *reportStore
		findings []shareddomain.ReviewFinding
		want     []string
	}{
		{
			name: "all four kinds with a candidate",
			store: newStore(1,
				domain.Transitioned{Event: domain.EventRequestRework, Class: domain.ReworkImplementation, Reason: "F1 major a.go:3"},
				domain.Transitioned{Event: domain.EventBlock, Reason: "wait for the token"},
				domain.Transitioned{Event: domain.EventAmend},
				domain.Transitioned{Event: domain.EventAmend},
			),
			findings: candidate,
			want: []string{
				runLine,
				"candidate: standards S1 a.go:3: name the constant",
				"evidence: rework-1: implementation: F1 major a.go:3",
				"evidence: block-1: wait for the token",
				"evidence: amends: 2",
				"evidence: stop-blocks: 1",
				learnNextWant,
			},
		},
		{
			name:  "evidence with no candidate",
			store: newStore(0, domain.Transitioned{Event: domain.EventBlock, Reason: "no access"}),
			want: []string{
				runLine,
				"learn: no convention candidates",
				"evidence: block-1: no access",
				learnNextWant,
			},
		},
		{
			name:     "a candidate with no evidence",
			store:    newStore(0),
			findings: candidate,
			want: []string{
				runLine,
				"candidate: standards S1 a.go:3: name the constant",
				learnNextWant,
			},
		},
		{
			name:  "an empty text prints no colon",
			store: newStore(0, domain.Transitioned{Event: domain.EventRequestRework}),
			want: []string{
				runLine,
				"learn: no convention candidates",
				"evidence: rework-1",
				learnNextWant,
			},
		},
		{
			name:  "no candidate and no evidence",
			store: newStore(0),
			want: []string{
				runLine,
				"learn: no convention candidates and no evidence",
				"next: with no user correction, start pr-open",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runLearnStore(t, tc.store, tc.findings)

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

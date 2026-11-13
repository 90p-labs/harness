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

const learnRunID = "20261113-004016-turn-repeated-corrections-into-durable-p"

type learnFindings struct {
	findings []shareddomain.ReviewFinding
}

func (f learnFindings) Read(context.Context, domain.Run) ([]shareddomain.ReviewFinding, error) {
	return f.findings, nil
}

func runLearn(t *testing.T, findings []shareddomain.ReviewFinding, args ...string) []string {
	t.Helper()
	store := &reportStore{run: &domain.Run{ID: learnRunID, Key: reportKey, Phase: domain.PhaseShipping}}
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
			"next: add the user corrections, then ask the Conventions question (references/learning.md)",
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

	t.Run("with no candidate it says so", func(t *testing.T) {
		got := runLearn(t, []shareddomain.ReviewFinding{
			{ID: "P1", Verdict: shareddomain.FindingReal, Lens: "spec", Text: "one-off"},
		})

		want := []string{
			"run: " + learnRunID,
			"learn: no convention candidates",
			"next: with no user correction, start pr-open",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

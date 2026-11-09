package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/ui/cli"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	flagJSON      = "--json"
	subInit       = "init"
	subBaseline   = "baseline"
	subCatalogue  = "catalogue"
	subCanaries   = "canaries"
	flagProof     = "--proof"
	proofFolder   = "lint/proof"
	errorLine     = "error ddd/src-roots contexts/a.ts:3 move the file\n"
	warnLine      = "warn next/img src/app/page.tsx:12 use next/image\n"
	missingEngine = "biome is missing"
	noPacksLine   = "no lint pack is set: set lint.packs in .harness.yaml\n"
)

var errEngine = errors.New(missingEngine)

func connectWith(handlers cli.Handlers) cli.Connect {
	return func(context.Context) (cli.Handlers, error) { return handlers, nil }
}

var catalogue = domain.RuleCatalogue{Packs: []domain.CataloguePack{{
	Name: domain.PackDDD,
	Rules: []domain.CatalogueRule{
		{ID: "ddd/domain-purity", Description: "The domain depends only on itself."},
		{ID: "ddd/layer-direction", Description: "The application never depends on the infrastructure."},
	},
}}}

func loadCatalogue() (domain.RuleCatalogue, error) { return catalogue, nil }

func run(t *testing.T, handlers cli.Handlers, args ...string) (string, error) {
	t.Helper()
	return runWith(t, connectWith(handlers), loadCatalogue, args...)
}

func runWith(t *testing.T, connect cli.Connect, load func() (domain.RuleCatalogue, error), args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := cli.NewLintCommand(connect, load)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func reportOf(findings ...shareddomain.Finding) cli.Handlers {
	return cli.Handlers{Lint: func(context.Context) (shareddomain.Report, error) {
		return shareddomain.Report{Version: shareddomain.ReportVersion, Findings: findings}, nil
	}}
}

var (
	errorFinding = shareddomain.Finding{Rule: "ddd/src-roots", Severity: shareddomain.SeverityError, File: "contexts/a.ts", Line: 3, Message: "move the file"}
	warnFinding  = shareddomain.Finding{Rule: "next/img", Severity: shareddomain.SeverityWarn, File: "src/app/page.tsx", Line: 12, Message: "use next/image"}
)

func TestLintCommandReport(t *testing.T) {
	t.Run("json prints the version 1 report and exits 0 with an error finding", func(t *testing.T) {
		out, err := run(t, reportOf(errorFinding), flagJSON)

		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		var got shareddomain.Report
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("output %q is not JSON: %v", out, err)
		}
		if got.Version != 1 || len(got.Findings) != 1 || got.Findings[0] != errorFinding {
			t.Fatalf("report = %+v", got)
		}
	})

	t.Run("an error finding prints its line and exits 1", func(t *testing.T) {
		out, err := run(t, reportOf(warnFinding, errorFinding))

		if !errors.Is(err, cli.ErrFindings) {
			t.Fatalf("err = %v, want ErrFindings", err)
		}
		if out != warnLine+errorLine {
			t.Fatalf("output = %q", out)
		}
	})

	t.Run("a warn finding alone exits 0", func(t *testing.T) {
		out, err := run(t, reportOf(warnFinding))

		if err != nil || out != warnLine {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})

	t.Run("no pack prints that no pack is set and exits 0", func(t *testing.T) {
		handlers := reportOf()
		handlers.NoPacks = true

		out, err := run(t, handlers)

		if err != nil || out != noPacksLine {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})

	noPacks := cli.Handlers{
		Init:     func(context.Context) error { return nil },
		Baseline: func(context.Context) error { return nil },
		NoPacks:  true,
	}
	for _, tc := range []struct{ sub, want string }{{subInit, noPacksLine}, {subBaseline, ""}} {
		t.Run("with no pack, "+tc.sub+" prints its line", func(t *testing.T) {
			out, err := run(t, noPacks, tc.sub)

			if err != nil || out != tc.want {
				t.Fatalf("out = %q, err = %v, want %q", out, err, tc.want)
			}
		})
	}

	t.Run("an engine error is returned with and without json", func(t *testing.T) {
		failing := cli.Handlers{Lint: func(context.Context) (shareddomain.Report, error) {
			return shareddomain.Report{}, errEngine
		}}
		for _, args := range [][]string{nil, {flagJSON}} {
			if _, err := run(t, failing, args...); !errors.Is(err, errEngine) {
				t.Fatalf("args %v: err = %v, want the engine error", args, err)
			}
		}
	})

	t.Run("init and baseline call their handlers", func(t *testing.T) {
		var called []string
		handlers := cli.Handlers{
			Init:     func(context.Context) error { called = append(called, subInit); return nil },
			Baseline: func(context.Context) error { called = append(called, subBaseline); return nil },
		}
		for _, sub := range []string{subInit, subBaseline} {
			if _, err := run(t, handlers, sub); err != nil {
				t.Fatalf("%s: %v", sub, err)
			}
		}
		if len(called) != 2 || called[0] != subInit || called[1] != subBaseline {
			t.Fatalf("called = %v", called)
		}
	})
}

func TestCatalogueCommand(t *testing.T) {
	noConnect := func(context.Context) (cli.Handlers, error) {
		t.Fatal("the catalogue command called connect")
		return cli.Handlers{}, nil
	}

	t.Run("json prints the catalogue", func(t *testing.T) {
		out, err := runWith(t, noConnect, loadCatalogue, subCatalogue, flagJSON)

		var got domain.RuleCatalogue
		if err != nil || json.Unmarshal([]byte(out), &got) != nil || !reflect.DeepEqual(got, catalogue) {
			t.Fatalf("out = %q, err = %v, want the catalogue as JSON", out, err)
		}
		if !strings.Contains(out, `"description":`) || !strings.Contains(out, `"name":"ddd"`) {
			t.Fatalf("out = %q, want the lower case keys", out)
		}
	})

	t.Run("text prints one line for each rule", func(t *testing.T) {
		out, err := runWith(t, noConnect, loadCatalogue, subCatalogue)

		want := "ddd/domain-purity The domain depends only on itself.\n" +
			"ddd/layer-direction The application never depends on the infrastructure.\n"
		if err != nil || out != want {
			t.Fatalf("out = %q, err = %v, want %q", out, err, want)
		}
	})

	t.Run("a load error is returned", func(t *testing.T) {
		failing := func() (domain.RuleCatalogue, error) { return domain.RuleCatalogue{}, errEngine }

		if _, err := runWith(t, noConnect, failing, subCatalogue); !errors.Is(err, errEngine) {
			t.Fatalf("err = %v, want the load error", err)
		}
	})
}

func canariesOf(results []domain.CanaryResult, err error) cli.Handlers {
	return cli.Handlers{Canaries: func(_ context.Context, proof string) ([]domain.CanaryResult, error) {
		if proof != proofFolder {
			return nil, fmt.Errorf("proof = %q, want %q", proof, proofFolder)
		}
		return results, err
	}}
}

func TestCanariesCommand(t *testing.T) {
	pass := domain.CanaryResult{Rule: "ddd/domain-purity", Canary: "ddd-domain-purity", Status: domain.CanaryPass}
	waived := domain.CanaryResult{Rule: "ddd/file-suffix", Status: domain.CanaryWaived, Detail: "deptrac checks dependencies"}
	passLine := "PASS ddd/domain-purity ddd-domain-purity\n"
	waivedLine := "WAIVED ddd/file-suffix: deptrac checks dependencies\n"

	t.Run("passes with each canary caught or waived", func(t *testing.T) {
		out, err := run(t, canariesOf([]domain.CanaryResult{pass, waived}, nil), subCanaries, flagProof, proofFolder)

		if err != nil || out != passLine+waivedLine {
			t.Fatalf("out = %q, err = %v, want %q", out, err, passLine+waivedLine)
		}
	})

	t.Run("fails with a FAIL line", func(t *testing.T) {
		missed := domain.CanaryResult{Rule: "ddd/layer-direction", Canary: "ddd-layer-direction", Status: domain.CanaryFail, Detail: "the engine deptrac gave no ddd/layer-direction finding"}
		absent := domain.CanaryResult{Rule: "ddd/cross-context", Status: domain.CanaryFail, Detail: "not in rules.yaml"}

		out, err := run(t, canariesOf([]domain.CanaryResult{pass, missed, absent, waived}, nil), subCanaries, flagProof, proofFolder)

		want := passLine +
			"FAIL ddd/layer-direction ddd-layer-direction: the engine deptrac gave no ddd/layer-direction finding\n" +
			"FAIL ddd/cross-context: not in rules.yaml\n" +
			waivedLine
		if !errors.Is(err, cli.ErrCanariesFailed) || out != want {
			t.Fatalf("out = %q, err = %v, want %q and ErrCanariesFailed", out, err, want)
		}
	})

	t.Run("an engine entry names its engine", func(t *testing.T) {
		onDeptrac := domain.CanaryResult{Rule: "ddd/domain-purity", Engine: "deptrac", Canary: "php-domain-purity", Status: domain.CanaryPass}
		onHarness := domain.CanaryResult{Rule: "ddd/domain-purity", Engine: domain.EngineHarness, Status: domain.CanaryWaived, Detail: "the binary fixtures prove it"}

		out, err := run(t, canariesOf([]domain.CanaryResult{onDeptrac, onHarness}, nil), subCanaries, flagProof, proofFolder)

		want := "PASS ddd/domain-purity deptrac php-domain-purity\n" +
			"WAIVED ddd/domain-purity harness: the binary fixtures prove it\n"
		if err != nil || out != want {
			t.Fatalf("out = %q, err = %v, want %q", out, err, want)
		}
	})

	t.Run("a config error is returned", func(t *testing.T) {
		if _, err := run(t, canariesOf(nil, errEngine), subCanaries, flagProof, proofFolder); !errors.Is(err, errEngine) {
			t.Fatalf("err = %v, want the handler error", err)
		}
	})

	t.Run("an empty proof folder is refused", func(t *testing.T) {
		for _, args := range [][]string{{subCanaries}, {subCanaries, flagProof, ""}} {
			if _, err := run(t, canariesOf(nil, nil), args...); !errors.Is(err, cli.ErrProofRequired) {
				t.Fatalf("run %v: err = %v, want ErrProofRequired", args, err)
			}
		}
	})
}

func TestBuiltInNoCanaryLine(t *testing.T) {
	out, err := run(t, canariesOf(nil, nil), subCanaries, flagProof, proofFolder)

	want := "lint canaries: only built-in rules are active, so there is no canary to run\n"
	if err != nil || out != want {
		t.Fatalf("out = %q, err = %v, want %q", out, err, want)
	}
}

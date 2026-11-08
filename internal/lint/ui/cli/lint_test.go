package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jorge07/harness/internal/lint/ui/cli"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	flagJSON      = "--json"
	subInit       = "init"
	subBaseline   = "baseline"
	errorLine     = "error ddd/src-roots contexts/a.ts:3 move the file\n"
	warnLine      = "warn next/img src/app/page.tsx:12 use next/image\n"
	missingEngine = "biome is missing"
	noPacksLine   = "no lint pack is set\n"
)

var errEngine = errors.New(missingEngine)

func connectWith(handlers cli.Handlers) cli.Connect {
	return func(context.Context) (cli.Handlers, error) { return handlers, nil }
}

func run(t *testing.T, handlers cli.Handlers, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := cli.NewLintCommand(connectWith(handlers))
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

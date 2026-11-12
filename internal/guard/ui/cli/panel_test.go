package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/90p-labs/harness/internal/guard/domain"
)

func TestListPrintsRecords(t *testing.T) {
	run := func(source ListSource, args ...string) (string, error) {
		var out bytes.Buffer
		cmd := NewReviewCommand(nil, source)
		cmd.SilenceUsage = true
		cmd.SetArgs(args)
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		return out.String(), err
	}

	t.Run("prints head, patch id and each record", func(t *testing.T) {
		source := func(context.Context) (ReviewList, error) {
			return ReviewList{Head: "abc123", PatchID: "p1", Reviews: []domain.Review{
				{Model: "opus", PatchID: "p1", RunID: "r1", FilesRead: []string{"a.go", "b.go"}},
				{Model: "sonnet"},
			}}, nil
		}
		out, err := run(source, "list")
		want := "head abc123, patch id p1\n" +
			"opus\trun r1\tpatch p1\tread a.go, b.go\n" +
			"sonnet\trun -\tpatch -\tread nothing\n"
		if err != nil || out != want {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})
	t.Run("no records and no patch id", func(t *testing.T) {
		source := func(context.Context) (ReviewList, error) { return ReviewList{Head: "abc123"}, nil }
		out, err := run(source, "list")
		if err != nil || out != "head abc123, patch id -\nno reviews recorded\n" {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})
	t.Run("source error", func(t *testing.T) {
		boom := errors.New("boom")
		source := func(context.Context) (ReviewList, error) { return ReviewList{}, boom }
		if _, err := run(source, "list"); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPanelPrintsModels(t *testing.T) {
	run := func(source PanelSource, args ...string) (string, error) {
		var out bytes.Buffer
		cmd := NewReviewCommand(source, nil)
		cmd.SilenceUsage = true
		cmd.SetArgs(args)
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		return out.String(), err
	}

	t.Run("prints the models and the reason", func(t *testing.T) {
		source := func(context.Context) ([]string, string, error) {
			return []string{"opus", "sonnet"}, "small fixup", nil
		}
		out, err := run(source, "panel")
		if err != nil || out != "panel: opus, sonnet (small fixup)\n" {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})
	t.Run("source error", func(t *testing.T) {
		boom := errors.New("boom")
		source := func(context.Context) ([]string, string, error) { return nil, "", boom }
		if _, err := run(source, "panel"); !errors.Is(err, boom) {
			t.Fatalf("err = %v", err)
		}
	})
}

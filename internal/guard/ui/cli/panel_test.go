package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestPanelPrintsModels(t *testing.T) {
	run := func(source PanelSource, args ...string) (string, error) {
		var out bytes.Buffer
		cmd := NewReviewCommand(source)
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

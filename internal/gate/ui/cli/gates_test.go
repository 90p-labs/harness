package cli_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jorge07/harness/internal/gate/application/command/initgates"
	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/gate/ui/cli"
)

func TestGatesInitCommand(t *testing.T) {
	t.Run("writes the setup steps and the detected gates", func(t *testing.T) {
		var closed bool
		cmd := cli.NewGatesCommand(connectWith(cli.Handlers{
			Init: func(context.Context) (initgates.Result, error) {
				return initgates.Result{
					Setup: []domain.Setup{{Gate: domain.Gate{ID: "deps", Run: "go mod download"}}},
					Gates: []domain.Gate{
						{ID: "vet", Run: "go vet ./...", Timeout: time.Minute},
						{ID: "test", Run: "go test ./..."},
					},
				}, nil
			},
		}, &closed))

		stdout, err := run(t, cmd, "init")

		if err != nil || stdout != "setup deps: go mod download\nvet: go vet ./...\ntest: go test ./...\n" || !closed {
			t.Fatalf("err = %v, stdout = %q, closed = %v", err, stdout, closed)
		}
	})

	t.Run("refuses when the key exists", func(t *testing.T) {
		var closed bool
		cmd := cli.NewGatesCommand(connectWith(cli.Handlers{
			Init: func(context.Context) (initgates.Result, error) { return initgates.Result{}, domain.ErrGatesExist },
		}, &closed))

		stdout, err := run(t, cmd, "init")

		if !errors.Is(err, domain.ErrGatesExist) || stdout != "" {
			t.Fatalf("err = %v, stdout = %q, want ErrGatesExist and no output", err, stdout)
		}
	})
}

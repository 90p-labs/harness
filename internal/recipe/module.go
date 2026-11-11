package recipe

import (
	"context"

	"github.com/90p-labs/harness/examples"
	"github.com/90p-labs/harness/internal/recipe/domain"
	"github.com/90p-labs/harness/internal/recipe/infrastructure/catalogue"
	"github.com/90p-labs/harness/internal/recipe/infrastructure/workspace"
	"github.com/90p-labs/harness/internal/recipe/ui/cli"
	"github.com/spf13/cobra"
)

// NewCommand wires `harness recipe` over the embedded examples and the disk.
// target returns the folder that apply writes in.
func NewCommand(target func(ctx context.Context) (string, error)) *cobra.Command {
	return cli.NewCommand(cli.Deps{
		Catalogue: func() (domain.Catalogue, error) {
			c, err := catalogue.New(examples.FS)
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		Open:   workspace.Open,
		Target: target,
	})
}

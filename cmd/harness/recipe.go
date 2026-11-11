package main

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/recipe"
	recipedomain "github.com/90p-labs/harness/internal/recipe/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

func newRecipeCommand() *cobra.Command {
	cmd := recipe.NewCommand(recipeTarget)
	for _, sub := range cmd.Commands() {
		run := sub.RunE
		sub.RunE = func(cmd *cobra.Command, args []string) error {
			err := run(cmd, args)
			if errors.Is(err, recipedomain.ErrUnknownRecipe) {
				return usageError{err}
			}
			return err
		}
	}
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

// recipeTarget is the git top level, or the current folder outside git. It
// needs no .harness.yaml.
func recipeTarget(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	top, err := git.TopLevel(ctx, cwd)
	switch {
	case errors.Is(err, git.ErrNotRepository):
		return cwd, nil
	case err != nil:
		return "", err
	}
	return top, nil
}

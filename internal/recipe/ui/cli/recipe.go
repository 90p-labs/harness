package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/recipe/application/command/apply"
	"github.com/90p-labs/harness/internal/recipe/application/query/recipes"
	"github.com/90p-labs/harness/internal/recipe/domain"
)

const (
	commandRecipe = "recipe"
	commandList   = "list"
	commandShow   = "show"
	commandApply  = "apply"
	flagDryRun    = "dry-run"

	nextPrefix = "next: "
	nameGap    = 2
)

// Deps are the ports of the recipe command. Catalogue is lazy, so a broken
// catalogue fails only the commands that use it.
type Deps struct {
	Catalogue func() (domain.Catalogue, error)
	Open      domain.Opener
	// Target returns the folder that apply writes in.
	Target func(ctx context.Context) (string, error)
}

func NewCommand(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandRecipe,
		Short: "List, show and apply the setup recipes",
	}
	cmd.AddCommand(listCommand(deps), showCommand(deps), applyCommand(deps))
	return cmd
}

func listCommand(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   commandList,
		Short: "Print each recipe name and summary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cat, err := deps.Catalogue()
			if err != nil {
				return err
			}
			list := recipes.NewHandler(cat).Handle()
			width := 0
			for _, r := range list {
				width = max(width, len(r.Name))
			}
			out := cmd.OutOrStdout()
			for _, r := range list {
				fmt.Fprintf(out, "%-*s%s\n", width+nameGap, r.Name, r.Summary)
			}
			return nil
		},
	}
}

func showCommand(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   commandShow + " <name>",
		Short: "Print the summary, files, scaffold, next steps and guide of a recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := deps.Catalogue()
			if err != nil {
				return err
			}
			recipe, err := cat.Get(domain.Name(args[0]))
			if err != nil {
				return unknown(cat, err)
			}
			printRecipe(cmd.OutOrStdout(), recipe)
			return nil
		},
	}
}

func printRecipe(out io.Writer, r domain.Recipe) {
	fmt.Fprintf(out, "%s: %s\n", r.Name, r.Summary)
	fmt.Fprintln(out, "files:")
	for _, file := range r.Files {
		fmt.Fprintf(out, "  %s\n", file)
	}
	if len(r.Scaffold) > 0 {
		fmt.Fprintln(out, "scaffold (printed, never run; run them in order in one shell):")
		for _, line := range r.Scaffold {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}
	for _, step := range r.Next {
		fmt.Fprintf(out, "%s%s\n", nextPrefix, step)
	}
	if r.Guide != "" {
		fmt.Fprintf(out, "\n%s", r.Guide)
		if !strings.HasSuffix(r.Guide, "\n") {
			fmt.Fprintln(out)
		}
	}
}

func applyCommand(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandApply + " <name> [--dry-run]",
		Short: "Write the missing files of a recipe at the git top level; never change a file that exists",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cat, err := deps.Catalogue()
			if err != nil {
				return err
			}
			target, err := deps.Target(cmd.Context())
			if err != nil {
				return err
			}
			dryRun, _ := cmd.Flags().GetBool(flagDryRun)
			result, err := apply.NewHandler(cat, deps.Open).Handle(apply.Command{Name: domain.Name(args[0]), Target: target, DryRun: dryRun})
			out := cmd.OutOrStdout()
			// On a write error the result holds the files already written.
			for _, change := range result.Changes {
				fmt.Fprintf(out, "%s %s\n", verb(change.Action, dryRun), change.Path)
			}
			if err != nil {
				return unknown(cat, err)
			}
			for _, step := range result.Recipe.Next {
				fmt.Fprintf(out, "%s%s\n", nextPrefix, step)
			}
			return nil
		},
	}
	cmd.Flags().Bool(flagDryRun, false, "print what apply would do and write nothing")
	return cmd
}

func verb(action domain.Action, dryRun bool) string {
	if !dryRun {
		return string(action)
	}
	switch action {
	case domain.Wrote:
		return "would write"
	case domain.Kept:
		return "would keep"
	}
	return string(action)
}

// unknown adds the known names to an unknown recipe error.
func unknown(cat domain.Catalogue, err error) error {
	if !errors.Is(err, domain.ErrUnknownRecipe) {
		return err
	}
	var names []string
	for _, r := range recipes.NewHandler(cat).Handle() {
		names = append(names, r.Name.String())
	}
	return fmt.Errorf("%w; known recipes: %s", err, strings.Join(names, ", "))
}

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/plan/application/command/renderview"
)

const (
	commandView = "view"
	flagOut     = "out"
	flagOpen    = "open"
)

func viewCommand(connect Connect) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandView,
		Short: "Write the plan as one HTML page",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			handlers, closeStore, err := connect(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, closeStore()) }()
			command, err := viewFor(cmd, handlers)
			if err != nil {
				return err
			}
			page, err := handlers.View.Handle(cmd.Context(), command)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), relativeToTop(handlers.RepoTop, page))
			if open, _ := cmd.Flags().GetBool(flagOpen); open {
				return handlers.Open(cmd.Context(), page)
			}
			return nil
		},
	}
	cmd.Flags().String(flagFile, "", "the plan file, default the plan of the active run")
	cmd.Flags().String(flagOut, "", "the page file, default plan.html beside the plan")
	cmd.Flags().Bool(flagOpen, false, "open the page in the browser")
	return cmd
}

func viewFor(cmd *cobra.Command, handlers Handlers) (renderview.Command, error) {
	command := renderview.Command{}
	if path, _ := cmd.Flags().GetString(flagFile); path != "" {
		command.Path = path
	} else {
		var err error
		if command, err = handlers.DefaultView(cmd.Context()); err != nil {
			return renderview.Command{}, err
		}
	}
	if out, _ := cmd.Flags().GetString(flagOut); out != "" {
		command.Out = out
	}
	return command, nil
}

// relativeToTop shortens the page path to the repository top, and keeps it when there is no top or the page lies outside.
func relativeToTop(top, page string) string {
	if top == "" {
		return page
	}
	abs, err := filepath.Abs(page)
	if err != nil {
		return page
	}
	rel, err := filepath.Rel(top, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return page
	}
	return rel
}

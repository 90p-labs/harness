package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	commandReview = "review"
	commandPanel  = "panel"
)

// PanelSource returns the models of the final review panel and the reason for that panel.
type PanelSource func(ctx context.Context) (models []string, reason string, err error)

func NewReviewCommand(source PanelSource) *cobra.Command {
	review := &cobra.Command{Use: commandReview, Short: "Show facts of the review round"}
	review.AddCommand(&cobra.Command{
		Use:   commandPanel,
		Short: "Print the models of the final review panel and the reason",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			models, reason, err := source(cmd.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "panel: %s (%s)\n", strings.Join(models, ", "), reason)
			return err
		},
	})
	return review
}

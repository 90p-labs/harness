package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/guard/domain"
)

const (
	commandReview = "review"
	commandPanel  = "panel"
	commandList   = "list"
	noValue       = "-"
	readNothing   = "nothing"
)

// PanelSource returns the models of the final review panel and the reason for that panel.
type PanelSource func(ctx context.Context) (models []string, reason string, err error)

// ReviewList is HEAD, its patch id against the default branch, and the reviews recorded for either.
type ReviewList struct {
	Head    string
	PatchID string
	Reviews []domain.Review
}

// ListSource returns the reviews recorded for HEAD or its patch id.
type ListSource func(ctx context.Context) (ReviewList, error)

func NewReviewCommand(panel PanelSource, list ListSource) *cobra.Command {
	review := &cobra.Command{Use: commandReview, Short: "Show facts of the review round"}
	review.AddCommand(&cobra.Command{
		Use:   commandPanel,
		Short: "Print the models of the final review panel and the reason",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			models, reason, err := panel(cmd.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "panel: %s (%s)\n", strings.Join(models, ", "), reason)
			return err
		},
	})
	review.AddCommand(&cobra.Command{
		Use:   commandList,
		Short: "Print the reviews recorded for HEAD and its patch id",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			reviews, err := list(cmd.Context())
			if err != nil {
				return err
			}
			return printReviews(cmd.OutOrStdout(), reviews)
		},
	})
	return review
}

func printReviews(out io.Writer, list ReviewList) error {
	var b strings.Builder
	fmt.Fprintf(&b, "head %s, patch id %s\n", list.Head, orNoValue(list.PatchID))
	if len(list.Reviews) == 0 {
		b.WriteString("no reviews recorded\n")
	}
	for _, review := range list.Reviews {
		read := readNothing
		if len(review.FilesRead) > 0 {
			read = strings.Join(review.FilesRead, ", ")
		}
		fmt.Fprintf(&b, "%s\trun %s\tpatch %s\tread %s\n", review.Model, orNoValue(review.RunID), orNoValue(review.PatchID), read)
	}
	_, err := io.WriteString(out, b.String())
	return err
}

func orNoValue(value string) string {
	if value == "" {
		return noValue
	}
	return value
}

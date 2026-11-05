package marketplace

import (
	"context"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newReviewQueueCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "review-queue", Short: "Review submissions", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newReviewQueueListCommand(f), newReviewQueueGetCommand(f))
	return cmd
}

func newReviewQueueListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	var filter string
	cmd := &cobra.Command{Use: "list", Short: "List the review queue", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum submissions to return; 0 returns every page")
	cmd.Flags().StringVar(&filter, "filter", "", "AIP-160 filter")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runReviewQueueList(cmd.Context(), f, limit, filter)
	}
	return cmd
}

func newReviewQueueGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get SUBMISSION_ID", Short: "Get a review queue submission", Args: cobra.ExactArgs(1), SilenceUsage: true, SilenceErrors: true}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runReviewQueueGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func runReviewQueueList(ctx context.Context, f *cli.Factory, limit int, filter string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	rows, raw, err := listPages[submission](ctx, f, "/v1/reviewQueue", limit, filter, "", "submissions")
	if err != nil {
		return err
	}
	return printRows(f, raw, rows, submissionColumns())
}

func runReviewQueueGet(ctx context.Context, f *cli.Factory, submissionID string) error {
	if err := requireUser(f); err != nil {
		return err
	}
	submissionID, err := requireID(submissionID, "submission ID")
	if err != nil {
		return err
	}
	raw, err := callRaw(ctx, f, http.MethodGet, "/v1/reviewQueue/submissions/"+url.PathEscape(submissionID), nil, nil)
	if err != nil {
		return err
	}
	detail, err := decodeObject[reviewDetail](raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []submission{detail.Submission}, submissionColumns())
}

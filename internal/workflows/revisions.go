package workflows

import (
	"context"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newRevisionsCommand(f *cli.Factory) *cobra.Command {
	var workflowID string
	cmd := &cobra.Command{Use: "revisions", Short: "Manage immutable workflow revisions", SilenceUsage: true, SilenceErrors: true, Args: groupArgs, RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() }}
	cmd.PersistentFlags().StringVar(&workflowID, "workflow", "", "Parent workflow ID")
	cmd.AddCommand(newRevisionsCreateCommand(f, &workflowID), newRevisionsListCommand(f, &workflowID), newRevisionsGetCommand(f, &workflowID))
	return cmd
}

func newRevisionsCreateCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	var source, revisionID string
	cmd := &cobra.Command{Use: "create --workflow ID --input FILE", Short: "Create an immutable workflow revision", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&source, "input", "", "Revision JSON body file, or - for stdin")
	cmd.Flags().StringVar(&revisionID, "revision-id", "", "Caller-chosen revision ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRevisionsCreate(cmd.Context(), f, *workflowID, source, revisionID, cmd.Flags().Changed("revision-id"))
	}
	return cmd
}

func newRevisionsListCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "list --workflow ID", Short: "List workflow revisions", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum revisions to return; 0 follows every page")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRevisionsList(cmd.Context(), f, *workflowID, limit)
	}
	return cmd
}

func newRevisionsGetCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "get REVISION_ID --workflow ID", Short: "Get a workflow revision", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runRevisionsGet(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func runRevisionsCreate(ctx context.Context, f *cli.Factory, workflowID, source, revisionID string, revisionIDSet bool) error {
	workflowID, err := requireFlag("workflow", workflowID)
	if err != nil {
		return err
	}
	body, err := requiredJSONObject(f.IO.In, source)
	if err != nil {
		return err
	}
	query := url.Values{}
	if revisionIDSet {
		query.Set("revisionId", revisionID)
	}
	resp, err := doRequest(ctx, f, http.MethodPost, workflowSubpath(workflowID, "revisions"), query, body)
	if err != nil {
		return err
	}
	return printRevision(f, resp.Body)
}

func runRevisionsList(ctx context.Context, f *cli.Factory, workflowID string, limit int) error {
	workflowID, err := requireFlag("workflow", workflowID)
	if err != nil {
		return err
	}
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	items, raw, err := listPages[Revision](ctx, f, workflowSubpath(workflowID, "revisions"), limit, maxPageSize, "revisions", nil)
	if err != nil {
		return err
	}
	return printRows(f, raw, items, revisionColumns())
}

func runRevisionsGet(ctx context.Context, f *cli.Factory, workflowID, revisionID string) error {
	path, err := revisionPath(workflowID, revisionID)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	return printRevision(f, resp.Body)
}

func revisionPath(workflowID, revisionID string) (string, error) {
	workflowID, err := requireFlag("workflow", workflowID)
	if err != nil {
		return "", err
	}
	revisionID, err = requirePositional("revision ID", revisionID)
	if err != nil {
		return "", err
	}
	return workflowSubpath(workflowID, "revisions", revisionID), nil
}

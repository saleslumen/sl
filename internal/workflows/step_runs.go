package workflows

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newStepRunsCommand(f *cli.Factory) *cobra.Command {
	var workflowID, executionID string
	cmd := &cobra.Command{Use: "step-runs", Short: "Inspect workflow step runs", SilenceUsage: true, SilenceErrors: true, Args: groupArgs, RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() }}
	cmd.PersistentFlags().StringVar(&workflowID, "workflow", "", "Parent workflow ID")
	cmd.PersistentFlags().StringVar(&executionID, "execution", "", "Parent execution ID")
	cmd.AddCommand(newStepRunsListCommand(f, &workflowID, &executionID), newStepRunsGetCommand(f, &workflowID, &executionID))
	return cmd
}

func newStepRunsListCommand(f *cli.Factory, workflowID, executionID *string) *cobra.Command {
	var limit int
	var filter string
	cmd := &cobra.Command{Use: "list --workflow ID --execution ID", Short: "List execution step runs", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum step runs to return; 0 follows every page")
	cmd.Flags().StringVar(&filter, "filter", "", "AIP-160 filter; the only accepted form is `cycle = N`")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runStepRunsList(cmd.Context(), f, *workflowID, *executionID, limit, filter)
	}
	return cmd
}

func newStepRunsGetCommand(f *cli.Factory, workflowID, executionID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "get SEQUENCE --workflow ID --execution ID", Short: "Get an execution step run", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runStepRunsGet(cmd.Context(), f, *workflowID, *executionID, args[0])
	}
	return cmd
}

func stepRunsPath(workflowID, executionID string) (string, error) {
	path, err := executionPath(workflowID, executionID)
	if err != nil {
		return "", err
	}
	return path + "/stepRuns", nil
}

func runStepRunsList(ctx context.Context, f *cli.Factory, workflowID, executionID string, limit int, filter string) error {
	path, err := stepRunsPath(workflowID, executionID)
	if err != nil {
		return err
	}
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	query := url.Values{}
	if filter != "" {
		query.Set("filter", filter)
	}
	items, raw, err := listPages[StepRun](ctx, f, path, limit, maxPageSize, "stepRuns", query)
	if err != nil {
		return err
	}
	return printRows(f, raw, items, stepRunColumns())
}

func runStepRunsGet(ctx context.Context, f *cli.Factory, workflowID, executionID, sequence string) error {
	path, err := stepRunsPath(workflowID, executionID)
	if err != nil {
		return err
	}
	sequence, err = requirePositional("step-run sequence", sequence)
	if err != nil {
		return err
	}
	value, err := strconv.ParseInt(sequence, 10, 64)
	if err != nil || value < 1 {
		return &cli.UsageError{Msg: "step-run sequence must be a positive integer"}
	}
	resp, err := doRequest(ctx, f, http.MethodGet, path+"/"+strconv.FormatInt(value, 10), nil, nil)
	if err != nil {
		return err
	}
	return printStepRun(f, resp.Body)
}

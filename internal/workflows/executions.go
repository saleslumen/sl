package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newExecutionsCommand(f *cli.Factory) *cobra.Command {
	var workflowID string
	cmd := &cobra.Command{Use: "executions", Short: "Manage workflow executions", SilenceUsage: true, SilenceErrors: true}
	cmd.PersistentFlags().StringVar(&workflowID, "workflow", "", "Parent workflow ID")
	cmd.AddCommand(newExecutionsListCommand(f, &workflowID))
	cmd.AddCommand(newExecutionsGetCommand(f, &workflowID))
	cmd.AddCommand(newExecutionsCancelCommand(f, &workflowID))
	return cmd
}

func newExecutionsListCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	limit := defaultPageSize
	cmd := &cobra.Command{Use: "list --workflow ID", Short: "List workflow executions", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum executions to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runExecutionsList(cmd.Context(), f, *workflowID, limit)
	}
	return cmd
}

func newExecutionsGetCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "get EXECUTION_ID --workflow ID", Short: "Get a workflow execution", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runExecutionGet(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func newExecutionsCancelCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "cancel EXECUTION_ID --workflow ID", Short: "Cancel a workflow execution", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runExecutionCancel(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func runExecutionsList(ctx context.Context, f *cli.Factory, workflowID string, limit int) error {
	workflowID, err := requireFlag("workflow", workflowID)
	if err != nil {
		return err
	}
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	items, raw, err := listPages[Execution](ctx, f, workflowSubpath(workflowID, "executions"), limit, 0, "executions")
	if err != nil {
		return err
	}
	return printRows(f, raw, items, executionColumns())
}

func runExecutionGet(ctx context.Context, f *cli.Factory, workflowID, executionID string) error {
	path, err := executionResourcePath(workflowID, executionID)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	return printExecution(f, resp.Body)
}

func runExecutionCancel(ctx context.Context, f *cli.Factory, workflowID, executionID string) error {
	path, err := executionResourcePath(workflowID, executionID)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, path+":cancel", nil, emptyObject())
	if err != nil {
		return err
	}
	return printExecution(f, resp.Body)
}

func executionResourcePath(workflowID, executionID string) (string, error) {
	workflowID, err := requireFlag("workflow", workflowID)
	if err != nil {
		return "", err
	}
	executionID, err = requirePositional("execution ID", executionID)
	if err != nil {
		return "", err
	}
	return workflowSubpath(workflowID, "executions", executionID), nil
}

func printExecution(f *cli.Factory, raw []byte) error {
	row, err := decodeExecutionEnvelope(raw)
	if err != nil {
		return err
	}
	return printRows(f, raw, []Execution{row}, executionColumns())
}

func decodeExecutionEnvelope(raw []byte) (Execution, error) {
	var payload struct {
		Execution Execution `json:"execution"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Execution{}, fmt.Errorf("workflows: decode execution: %w", err)
	}
	return payload.Execution, nil
}

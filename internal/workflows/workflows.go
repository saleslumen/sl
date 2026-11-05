package workflows

import (
	"context"
	"net/http"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newWorkflowCommands(f *cli.Factory) []*cobra.Command {
	return []*cobra.Command{
		newListCommand(f),
		newGetCommand(f),
		newCreateCommand(f),
		newUpdateCommand(f),
		newDeleteCommand(f),
		newPublishCommand(f),
		newActivateCommand(f),
		newDeactivateCommand(f),
		newHistoryCommand(f),
	}
}

func newListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "list", Short: "List workflows (pin plus open draft)", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum workflows to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runList(cmd.Context(), f, limit)
	}
	return cmd
}

func newGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get WORKFLOW_ID", Short: "Get a workflow (pin plus open draft)", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newCreateCommand(f *cli.Factory) *cobra.Command {
	var input, name string
	cmd := &cobra.Command{Use: "create [--name NAME | --input FILE]", Short: "Create an unpublished workflow draft", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&name, "name", "", "Workflow name")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCreate(cmd.Context(), f, input, name, cmd.Flags().Changed("name"))
	}
	return cmd
}

func newUpdateCommand(f *cli.Factory) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "update WORKFLOW_ID --input FILE", Short: "Update a workflow draft", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runUpdate(cmd.Context(), f, args[0], input)
	}
	return cmd
}

func newDeleteCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "delete WORKFLOW_ID", Short: "Delete a workflow", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDelete(cmd.Context(), f, args[0])
	}
	return cmd
}

func newPublishCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "publish WORKFLOW_ID", Short: "Publish a workflow draft without activating", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCustom(cmd.Context(), f, args[0], "publish")
	}
	return cmd
}

func newActivateCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "activate WORKFLOW_ID", Short: "Activate a published workflow", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCustom(cmd.Context(), f, args[0], "activate")
	}
	return cmd
}

func newDeactivateCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "deactivate WORKFLOW_ID", Short: "Deactivate a workflow without changing the pin", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCustom(cmd.Context(), f, args[0], "deactivate")
	}
	return cmd
}

func newHistoryCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "history WORKFLOW_ID", Short: "Show version history (isLive pin, isDraft unpublished)", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runHistory(cmd.Context(), f, args[0])
	}
	return cmd
}

func runList(ctx context.Context, f *cli.Factory, limit int) error {
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	items, raw, err := listPages[Workflow](ctx, f, workflowsPath, limit, 0, "workflows")
	if err != nil {
		return err
	}
	return printRows(f, raw, items, workflowColumns())
}

func runGet(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requirePositional("WORKFLOW_ID", id)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodGet, workflowPath(id), nil, nil)
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func runCreate(ctx context.Context, f *cli.Factory, input, name string, nameChanged bool) error {
	body, err := createWorkflowBody(f.IO.In, input, name, nameChanged)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, workflowsPath, nil, body)
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func runUpdate(ctx context.Context, f *cli.Factory, id, input string) error {
	id, err := requirePositional("WORKFLOW_ID", id)
	if err != nil {
		return err
	}
	body, err := requiredJSONObject(f.IO.In, input)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPut, workflowPath(id), nil, body)
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func runDelete(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requirePositional("WORKFLOW_ID", id)
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete workflow " + id + "?"); err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodDelete, workflowPath(id), nil, nil)
	if err != nil {
		return err
	}
	return printJSON(f, resp.Body)
}

func runCustom(ctx context.Context, f *cli.Factory, id, verb string) error {
	id, err := requirePositional("WORKFLOW_ID", id)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, workflowCustomPath(id, verb), nil, emptyObject())
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func runHistory(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requirePositional("WORKFLOW_ID", id)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodGet, workflowSubpath(id, "history"), nil, nil)
	if err != nil {
		return err
	}
	versions, _, err := apiclient.DecodePage[WorkflowVersion](resp.Body, "versions", "nextPageToken")
	if err != nil {
		return err
	}
	return printVersions(f, resp.Body, versions)
}

package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

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
		newDeployCommand(f),
		newUndeployCommand(f),
	}
}

func newListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	var query string
	cmd := &cobra.Command{Use: "list", Short: "List workflows", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum workflows to return; 0 follows every page")
	cmd.Flags().StringVar(&query, "query", "", "Search workflow display names")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runList(cmd.Context(), f, limit, query)
	}
	return cmd
}

func newGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get ID", Short: "Get a workflow", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runGet(cmd.Context(), f, args[0]) }
	return cmd
}

func newCreateCommand(f *cli.Factory) *cobra.Command {
	var source, displayName, workflowID string
	cmd := &cobra.Command{Use: "create [--display-name NAME | --input FILE]", Short: "Create a workflow", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&source, "input", "", "Workflow JSON body file, or - for stdin")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Workflow display name")
	cmd.Flags().StringVar(&workflowID, "workflow-id", "", "Caller-chosen workflow ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCreate(cmd.Context(), f, source, displayName, workflowID, cmd.Flags().Changed("display-name"), cmd.Flags().Changed("workflow-id"))
	}
	return cmd
}

func newUpdateCommand(f *cli.Factory) *cobra.Command {
	var source, displayName, updateMask string
	cmd := &cobra.Command{Use: "update ID [--display-name NAME | --input FILE]", Short: "Update a workflow", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&source, "input", "", "Workflow update JSON body file, or - for stdin")
	cmd.Flags().StringVar(&displayName, "display-name", "", "New workflow display name")
	cmd.Flags().StringVar(&updateMask, "update-mask", "", "Optional comma-separated field paths")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runUpdate(cmd.Context(), f, args[0], source, displayName, updateMask, cmd.Flags().Changed("display-name"), cmd.Flags().Changed("update-mask"))
	}
	return cmd
}

func newDeleteCommand(f *cli.Factory) *cobra.Command {
	var etag string
	var force bool
	cmd := &cobra.Command{Use: "delete ID", Short: "Delete a workflow", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&etag, "etag", "", "Optional workflow concurrency token")
	cmd.Flags().BoolVar(&force, "force", false, "Delete child resources when no execution is live")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDelete(cmd.Context(), f, args[0], etag, force)
	}
	return cmd
}

func newDeployCommand(f *cli.Factory) *cobra.Command {
	var revision, etag string
	cmd := &cobra.Command{Use: "deploy ID --revision REVISION_NAME", Short: "Deploy a workflow revision", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&revision, "revision", "", "Revision resource name to serve")
	cmd.Flags().StringVar(&etag, "etag", "", "Optional workflow concurrency token")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDeploy(cmd.Context(), f, args[0], revision, etag)
	}
	return cmd
}

func newUndeployCommand(f *cli.Factory) *cobra.Command {
	var etag string
	cmd := &cobra.Command{Use: "undeploy ID", Short: "Clear a workflow's serving revision", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&etag, "etag", "", "Optional workflow concurrency token")
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runUndeploy(cmd.Context(), f, args[0], etag) }
	return cmd
}

func runList(ctx context.Context, f *cli.Factory, limit int, searchQuery string) error {
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	if searchQuery != "" {
		query := pageQuery(min(pageSizeForLimit(limit, maxPageSize), maxSearchPageSize), "")
		query.Set("query", searchQuery)
		resp, err := doRequest(ctx, f, http.MethodGet, workflowsPath, query, nil)
		if err != nil {
			return err
		}
		items, _, err := apiclient.DecodePage[json.RawMessage](resp.Body, "workflows", "nextPageToken")
		if err != nil {
			return err
		}
		rows, err := decodeRows[Workflow](items)
		if err != nil {
			return err
		}
		return printRows(f, resp.Body, rows, workflowColumns())
	}
	items, raw, err := listPages[Workflow](ctx, f, workflowsPath, limit, maxPageSize, "workflows", nil)
	if err != nil {
		return err
	}
	return printRows(f, raw, items, workflowColumns())
}

func runGet(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requirePositional("workflow ID", id)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodGet, workflowPath(id), nil, nil)
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func runCreate(ctx context.Context, f *cli.Factory, source, displayName, workflowID string, displayNameSet, workflowIDSet bool) error {
	body, err := readWorkflowObject(f.IO.In, source, "displayName", "display-name", displayName, displayNameSet)
	if err != nil {
		return err
	}
	query := url.Values{}
	if workflowIDSet {
		query.Set("workflowId", workflowID)
	}
	resp, err := doRequest(ctx, f, http.MethodPost, workflowsPath, query, body)
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func runUpdate(ctx context.Context, f *cli.Factory, id, source, displayName, updateMask string, displayNameSet, updateMaskSet bool) error {
	id, err := requirePositional("workflow ID", id)
	if err != nil {
		return err
	}
	body, err := readWorkflowObject(f.IO.In, source, "displayName", "display-name", displayName, displayNameSet)
	if err != nil {
		return err
	}
	query := url.Values{}
	if updateMaskSet {
		query.Set("updateMask", updateMask)
	}
	resp, err := doRequest(ctx, f, http.MethodPatch, workflowPath(id), query, body)
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func runDelete(ctx context.Context, f *cli.Factory, id, etag string, force bool) error {
	id, err := requirePositional("workflow ID", id)
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete workflow " + id + "?"); err != nil {
		return err
	}
	query := url.Values{}
	if etag != "" {
		query.Set("etag", etag)
	}
	if force {
		query.Set("force", "true")
	}
	resp, err := doRequest(ctx, f, http.MethodDelete, workflowPath(id), query, nil)
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func runDeploy(ctx context.Context, f *cli.Factory, id, revision, etag string) error {
	id, err := requirePositional("workflow ID", id)
	if err != nil {
		return err
	}
	revision, err = requireFlag("revision", revision)
	if err != nil {
		return err
	}
	body := map[string]string{"revision": revision}
	if etag != "" {
		body["etag"] = etag
	}
	resp, err := doRequest(ctx, f, http.MethodPost, workflowCustomPath(id, "deploy"), nil, body)
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func runUndeploy(ctx context.Context, f *cli.Factory, id, etag string) error {
	id, err := requirePositional("workflow ID", id)
	if err != nil {
		return err
	}
	body := map[string]string{}
	if etag != "" {
		body["etag"] = etag
	}
	resp, err := doRequest(ctx, f, http.MethodPost, workflowCustomPath(id, "undeploy"), nil, body)
	if err != nil {
		return err
	}
	return printWorkflow(f, resp.Body)
}

func decodeRows[T any](items []json.RawMessage) ([]T, error) {
	rows := make([]T, 0, len(items))
	for _, item := range items {
		row, err := decodeRow[T](item)
		if err != nil {
			return nil, fmt.Errorf("workflows: decode list item: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

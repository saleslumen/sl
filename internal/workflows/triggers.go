package workflows

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newTriggersCommand(f *cli.Factory) *cobra.Command {
	var workflowID string
	cmd := &cobra.Command{Use: "triggers", Short: "Manage workflow triggers", SilenceUsage: true, SilenceErrors: true, Args: groupArgs, RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() }}
	cmd.PersistentFlags().StringVar(&workflowID, "workflow", "", "Parent workflow ID")
	cmd.AddCommand(newTriggersCreateCommand(f, &workflowID), newTriggersListCommand(f, &workflowID), newTriggersGetCommand(f, &workflowID), newTriggersUpdateCommand(f, &workflowID), newTriggersDeleteCommand(f, &workflowID), newTriggersRotateCommand(f, &workflowID))
	return cmd
}

func newEventTypesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "event-types", Short: "Inspect trigger event types", SilenceUsage: true, SilenceErrors: true, Args: groupArgs, RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() }}
	cmd.AddCommand(newEventTypesListCommand(f))
	return cmd
}

func newTriggersCreateCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	var source, triggerID string
	cmd := &cobra.Command{Use: "create --workflow ID --input FILE", Short: "Create a workflow trigger", Long: "Create a workflow trigger. This command requires user OAuth (`sl auth login`).", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&source, "input", "", "Trigger JSON body file, or - for stdin")
	cmd.Flags().StringVar(&triggerID, "trigger-id", "", "Caller-chosen trigger ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTriggersCreate(cmd.Context(), f, *workflowID, source, triggerID, cmd.Flags().Changed("trigger-id"))
	}
	return cmd
}

func newTriggersListCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	var limit int
	var showDeleted bool
	cmd := &cobra.Command{Use: "list --workflow ID", Short: "List workflow triggers", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum triggers to return; 0 follows every page")
	cmd.Flags().BoolVar(&showDeleted, "show-deleted", false, "Include tombstoned triggers")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTriggersList(cmd.Context(), f, *workflowID, limit, showDeleted)
	}
	return cmd
}

func newTriggersGetCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "get TRIGGER_ID --workflow ID", Short: "Get a workflow trigger", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTriggersGet(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func newTriggersUpdateCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	var source, updateMask string
	cmd := &cobra.Command{Use: "update TRIGGER_ID --workflow ID --input FILE", Short: "Update a workflow trigger", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&source, "input", "", "Trigger JSON body file, or - for stdin")
	cmd.Flags().StringVar(&updateMask, "update-mask", "", "Optional comma-separated trigger field paths")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTriggersUpdate(cmd.Context(), f, *workflowID, args[0], source, updateMask, cmd.Flags().Changed("update-mask"))
	}
	return cmd
}

func newTriggersDeleteCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "delete TRIGGER_ID --workflow ID", Short: "Delete a workflow trigger", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTriggersDelete(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func newTriggersRotateCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	cmd := &cobra.Command{Use: "rotate-webhook-token TRIGGER_ID --workflow ID", Short: "Rotate a webhook trigger token", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTriggersRotate(cmd.Context(), f, *workflowID, args[0])
	}
	return cmd
}

func newEventTypesListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "list", Short: "List supported trigger event types", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum event types to return; 0 follows every page")
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runEventTypesList(cmd.Context(), f, limit) }
	return cmd
}

func runTriggersList(ctx context.Context, f *cli.Factory, workflow string, limit int, showDeleted bool) error {
	workflow, err := requireFlag("workflow", workflow)
	if err != nil {
		return err
	}
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	query := url.Values{}
	if showDeleted {
		query.Set("showDeleted", "true")
	}
	items, raw, err := listPages[Trigger](ctx, f, workflowSubpath(workflow, "triggers"), limit, maxPageSize, "triggers", query)
	if err != nil {
		return err
	}
	return printRows(f, raw, items, triggerColumns())
}

func runTriggersCreate(ctx context.Context, f *cli.Factory, workflow, source, triggerID string, triggerIDSet bool) error {
	workflow, err := requireFlag("workflow", workflow)
	if err != nil {
		return err
	}
	body, err := requiredJSONObject(f.IO.In, source)
	if err != nil {
		return err
	}
	if err := f.RequireUserOAuth(); err != nil {
		return err
	}
	query := url.Values{}
	if triggerIDSet {
		query.Set("triggerId", triggerID)
	}
	resp, err := doRequest(ctx, f, http.MethodPost, workflowSubpath(workflow, "triggers"), query, body)
	if err != nil {
		return err
	}
	return printTrigger(f, resp.Body)
}

func runTriggersGet(ctx context.Context, f *cli.Factory, workflow, triggerID string) error {
	path, err := triggerPath(workflow, triggerID)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	return printTrigger(f, resp.Body)
}

func runTriggersUpdate(ctx context.Context, f *cli.Factory, workflow, triggerID, source, updateMask string, updateMaskSet bool) error {
	path, err := triggerPath(workflow, triggerID)
	if err != nil {
		return err
	}
	body, err := requiredJSONObject(f.IO.In, source)
	if err != nil {
		return err
	}
	query := url.Values{}
	if updateMaskSet {
		mask, err := parseUpdateMask(updateMask)
		if err != nil {
			return err
		}
		query.Set("updateMask", mask)
	}
	resp, err := doRequest(ctx, f, http.MethodPatch, path, query, body)
	if err != nil {
		return err
	}
	return printTrigger(f, resp.Body)
}

func runTriggersDelete(ctx context.Context, f *cli.Factory, workflow, triggerID string) error {
	path, err := triggerPath(workflow, triggerID)
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete trigger " + triggerID + "?"); err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	return printTrigger(f, resp.Body)
}

func runTriggersRotate(ctx context.Context, f *cli.Factory, workflow, triggerID string) error {
	path, err := triggerPath(workflow, triggerID)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, path+":rotateWebhookToken", nil, map[string]any{})
	if err != nil {
		return err
	}
	return printTrigger(f, resp.Body)
}

func runEventTypesList(ctx context.Context, f *cli.Factory, limit int) error {
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	items, raw, err := listPages[TriggerEventType](ctx, f, triggerEventTypesPath, limit, maxPageSize, "triggerEventTypes", nil)
	if err != nil {
		return err
	}
	return printRows(f, raw, items, eventTypeColumns())
}

func triggerPath(workflow, triggerID string) (string, error) {
	workflow, err := requireFlag("workflow", workflow)
	if err != nil {
		return "", err
	}
	triggerID, err = requirePositional("trigger ID", triggerID)
	if err != nil {
		return "", err
	}
	return workflowSubpath(workflow, "triggers", triggerID), nil
}

func parseUpdateMask(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", &cli.UsageError{Msg: "--update-mask must be nonempty when supplied"}
	}
	seen := map[string]struct{}{}
	parts := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		field := strings.TrimSpace(part)
		if field == "" {
			return "", &cli.UsageError{Msg: "--update-mask entries must be nonempty"}
		}
		if _, ok := seen[field]; ok {
			return "", &cli.UsageError{Msg: fmt.Sprintf("--update-mask has duplicate %s", field)}
		}
		seen[field] = struct{}{}
		parts = append(parts, field)
	}
	return strings.Join(parts, ","), nil
}

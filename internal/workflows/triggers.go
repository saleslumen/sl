package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

func newTriggersCommand(f *cli.Factory) *cobra.Command {
	var workflowID string
	cmd := &cobra.Command{Use: "triggers", Short: "Manage workflow triggers", SilenceUsage: true, SilenceErrors: true}
	cmd.PersistentFlags().StringVar(&workflowID, "workflow", "", "Parent workflow ID")
	cmd.AddCommand(newTriggersCreateCommand(f, &workflowID))
	cmd.AddCommand(newTriggersListCommand(f, &workflowID))
	cmd.AddCommand(newTriggersGetCommand(f, &workflowID))
	cmd.AddCommand(newTriggersUpdateCommand(f, &workflowID))
	cmd.AddCommand(newTriggersDeleteCommand(f, &workflowID))
	cmd.AddCommand(newTriggersRotateCommand(f, &workflowID))
	return cmd
}

func newEventTypesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "event-types", Short: "Inspect trigger event types", SilenceUsage: true, SilenceErrors: true}
	cmd.AddCommand(newEventTypesListCommand(f))
	return cmd
}

func newTriggersCreateCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "create --workflow ID --input FILE", Short: "Create a workflow trigger", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&source, "input", "", "JSON trigger body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTriggersCreate(cmd.Context(), f, *workflowID, source)
	}
	return cmd
}

func newTriggersListCommand(f *cli.Factory, workflowID *string) *cobra.Command {
	limit := defaultPageSize
	cmd := &cobra.Command{Use: "list --workflow ID", Short: "List workflow triggers", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultPageSize, "Maximum triggers to return (0 for all)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTriggersList(cmd.Context(), f, *workflowID, limit)
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
	var input, updateMask string
	cmd := &cobra.Command{Use: "update TRIGGER_ID --workflow ID --update-mask FIELDS --input FILE", Short: "Update a workflow trigger", SilenceUsage: true, SilenceErrors: true, Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&input, "input", "", "JSON trigger body file, or - for stdin")
	cmd.Flags().StringVar(&updateMask, "update-mask", "", "Comma-separated trigger field paths")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runTriggersUpdate(cmd.Context(), f, *workflowID, args[0], input, updateMask)
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
	return &cobra.Command{Use: "list", Short: "List supported trigger event types", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return runEventTypesList(cmd.Context(), f)
	}}
}

func runTriggersList(ctx context.Context, f *cli.Factory, workflow string, limit int) error {
	workflow, err := requireFlag("workflow", workflow)
	if err != nil {
		return err
	}
	if limit < 0 {
		return &cli.UsageError{Msg: "--limit must be >= 0"}
	}
	items, raw, err := listPages[WorkflowTrigger](ctx, f, workflowSubpath(workflow, "triggers"), limit, maxTriggerPageSize, "triggers")
	if err != nil {
		return err
	}
	return printRows(f, raw, items, triggerColumns())
}

func runTriggersCreate(ctx context.Context, f *cli.Factory, workflow, source string) error {
	if err := f.RequireUserOAuth(); err != nil {
		return err
	}
	workflow, err := requireFlag("workflow", workflow)
	if err != nil {
		return err
	}
	body, err := requiredJSONObject(f.IO.In, source)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, workflowSubpath(workflow, "triggers"), nil, body)
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

func runTriggersUpdate(ctx context.Context, f *cli.Factory, workflow, triggerID, input, updateMask string) error {
	path, err := triggerPath(workflow, triggerID)
	if err != nil {
		return err
	}
	mask, err := parseUpdateMask(updateMask)
	if err != nil {
		return err
	}
	body, err := requiredJSONObject(f.IO.In, input)
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("updateMask", mask)
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
	return printJSON(f, resp.Body)
}

func runTriggersRotate(ctx context.Context, f *cli.Factory, workflow, triggerID string) error {
	path, err := triggerPath(workflow, triggerID)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, path+":rotateWebhookToken", nil, emptyObject())
	if err != nil {
		return err
	}
	return printTrigger(f, resp.Body)
}

func runEventTypesList(ctx context.Context, f *cli.Factory) error {
	resp, err := doRequest(ctx, f, http.MethodGet, triggerEventTypesPath, nil, nil)
	if err != nil {
		return err
	}
	items, _, err := apiclient.DecodePage[TriggerEventType](resp.Body, "eventTypes", "nextPageToken")
	if err != nil {
		return err
	}
	return printRows(f, resp.Body, items, eventTypeColumns())
}

func printTrigger(f *cli.Factory, raw []byte) error {
	var envelope struct {
		Trigger WorkflowTrigger `json:"trigger"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("workflows: decode trigger: %w", err)
	}
	return printRows(f, raw, []WorkflowTrigger{envelope.Trigger}, triggerColumns())
}

func triggerPath(workflow, triggerID string) (string, error) {
	workflow, err := requireFlag("workflow", workflow)
	if err != nil {
		return "", err
	}
	triggerID, err = requirePositional("TRIGGER_ID", triggerID)
	if err != nil {
		return "", err
	}
	return workflowSubpath(workflow, "triggers", triggerID), nil
}

func parseUpdateMask(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", &cli.UsageError{Msg: "--update-mask is required"}
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

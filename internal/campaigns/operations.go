package campaigns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type operation struct {
	Name                           string          `json:"name"`
	Campaign                       string          `json:"campaign"`
	State                          string          `json:"state"`
	CommandType                    string          `json:"command_type"`
	CancelledDeliveryCount         json.RawMessage `json:"cancelled_delivery_count"`
	AlreadyAttemptingDeliveryCount json.RawMessage `json:"already_attempting_delivery_count"`
	UpdateTime                     string          `json:"update_time"`
}

func newOperationsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "operations", Short: "Inspect campaign operations", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	cmd.AddCommand(newOperationsGetCommand(f), newOperationsCancelCommand(f))
	return cmd
}

func newOperationsGetCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	cmd := &cobra.Command{Use: "get ID [--campaign ID]", Short: "Get an operation", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("operation id")}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID for an import or enrichment operation")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if campaign != "" {
			return runScopedOperation(cmd.Context(), f, campaign, args[0], false)
		}
		return runOperationsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func runOperationsGet(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requireArg([]string{id}, "operation id")
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodGet, "/v1/operations/"+url.PathEscape(id), nil, nil)
	if err != nil {
		return err
	}
	return printOperation(f, resp.Body)
}

func decodeOperation(raw []byte) (operation, error) {
	var row operation
	if err := json.Unmarshal(raw, &row); err != nil {
		return operation{}, fmt.Errorf("campaigns: decode operation: %w", err)
	}
	return row, nil
}

func formatNullableCount(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	return formatScalar(raw)
}

func operationColumns() []output.Column[operation] {
	return []output.Column[operation]{
		{Header: "ID", Value: func(row operation) string { return resourceID(row.Name) }},
		{Header: "CAMPAIGN ID", Value: func(row operation) string { return resourceID(row.Campaign) }},
		{Header: "STATE", Value: func(row operation) string { return row.State }},
		{Header: "COMMAND", Value: func(row operation) string { return row.CommandType }},
		{Header: "CANCELLED", Value: func(row operation) string { return formatNullableCount(row.CancelledDeliveryCount) }},
		{Header: "ALREADY ATTEMPTING", Value: func(row operation) string { return formatNullableCount(row.AlreadyAttemptingDeliveryCount) }},
		{Header: "UPDATED", Value: func(row operation) string { return row.UpdateTime }},
	}
}

func printOperation(f *cli.Factory, raw []byte) error {
	row, err := decodeOperation(raw)
	if err != nil {
		return err
	}
	if err := output.Table(f.Printer(), raw, []operation{row}, operationColumns()); err != nil {
		return fmt.Errorf("campaigns: print operation: %w", err)
	}
	return nil
}

func newOperationsCancelCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	cmd := &cobra.Command{Use: "cancel ID --campaign ID", Short: "Cancel a campaign operation", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("operation id")}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runScopedOperation(cmd.Context(), f, campaign, args[0], true)
	}
	return cmd
}

func runScopedOperation(ctx context.Context, f *cli.Factory, campaign, id string, cancel bool) error {
	campaignID, err := requireCampaign(campaign)
	if err != nil {
		return err
	}
	id, err = requireArg([]string{id}, "operation id")
	if err != nil {
		return err
	}
	path := "/v1/campaigns/" + campaignID + "/operations/" + url.PathEscape(resourceID(id))
	method := http.MethodGet
	if cancel {
		method = http.MethodPost
		path += ":cancel"
	}
	response, err := do(ctx, f, method, path, nil, nil)
	if err != nil {
		return err
	}
	return f.Printer().Object(response.Body)
}

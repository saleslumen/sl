package campaigns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type deliveryRecord struct {
	Name            string  `json:"name"`
	Person          string  `json:"person"`
	Step            string  `json:"step"`
	SenderAccountID string  `json:"sender_account_id"`
	Status          string  `json:"status"`
	ScheduledAt     *string `json:"scheduled_at"`
	UpdateTime      string  `json:"update_time"`
}

func newDeliveriesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "deliveries", Short: "Inspect campaign deliveries", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	cmd.AddCommand(newDeliveriesListCommand(f), newDeliveriesGetCommand(f), newDeliveriesResolveUnknownCommand(f))
	return cmd
}

func newDeliveriesListCommand(f *cli.Factory) *cobra.Command {
	var campaign, personID, status, stepID, senderAccountID, failureCode, scheduledAfter, scheduledBefore string
	limit := 50
	cmd := &cobra.Command{Use: "list --campaign ID", Short: "List campaign deliveries", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum deliveries to return")
	cmd.Flags().StringVar(&personID, "person-id", "", "Filter by person ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by delivery status (ALLOCATING, PLANNED, SUBMITTING, ACCEPTED, ATTEMPTING, SENT, UNKNOWN, CANCELLED, FAILED)")
	cmd.Flags().StringVar(&stepID, "step-id", "", "Filter by step ID")
	cmd.Flags().StringVar(&senderAccountID, "sender-account-id", "", "Filter by sender account ID")
	cmd.Flags().StringVar(&failureCode, "failure-code", "", "Filter by failure code")
	cmd.Flags().StringVar(&scheduledAfter, "scheduled-after", "", "Filter deliveries scheduled after RFC3339")
	cmd.Flags().StringVar(&scheduledBefore, "scheduled-before", "", "Filter deliveries scheduled before RFC3339")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDeliveriesList(cmd.Context(), f, campaign, limit, personID, status, stepID, senderAccountID, failureCode, scheduledAfter, scheduledBefore)
	}
	return cmd
}

func newDeliveriesGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get ID", Short: "Get a delivery", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("delivery id")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDeliveriesGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newDeliveriesResolveUnknownCommand(f *cli.Factory) *cobra.Command {
	var source, requestID string
	cmd := &cobra.Command{Use: "resolve-unknown ID --input FILE", Short: "Resolve an unknown delivery", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("delivery id")}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDeliveriesResolveUnknown(cmd.Context(), f, args[0], source, requestID, cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func runDeliveriesList(ctx context.Context, f *cli.Factory, campaign string, limit int, personID, status, stepID, senderAccountID, failureCode, scheduledAfter, scheduledBefore string) error {
	path, err := campaignDeliveriesPath(campaign)
	if err != nil {
		return err
	}
	if limit < 1 {
		return &cli.UsageError{Msg: "--limit must be at least 1"}
	}
	items, raw, err := listDeliveries(ctx, f, path, limit, personID, status, stepID, senderAccountID, failureCode, scheduledAfter, scheduledBefore)
	if err != nil {
		return err
	}
	rows, err := decodeDeliveryRecords(items)
	if err != nil {
		return err
	}
	return printDeliveryTable(f, raw, rows)
}

func runDeliveriesGet(ctx context.Context, f *cli.Factory, id string) error {
	path, err := deliveryResourcePath(id)
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	return printDelivery(f, resp.Body)
}

func runDeliveriesResolveUnknown(ctx context.Context, f *cli.Factory, id, source, requestID string, requestIDSet bool) error {
	path, err := deliveryResourcePath(id)
	if err != nil {
		return err
	}
	obj, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, path+":resolveUnknown", nil, obj)
	if err != nil {
		return err
	}
	return printDelivery(f, resp.Body)
}

func campaignDeliveriesPath(campaign string) (string, error) {
	id, err := requireCampaign(campaign)
	if err != nil {
		return "", err
	}
	return "/v1/campaigns/" + id + "/deliveries", nil
}

func deliveryResourcePath(id string) (string, error) {
	value, err := requireArg([]string{id}, "delivery id")
	if err != nil {
		return "", err
	}
	return "/v1/deliveries/" + resourceID(value), nil
}

func listDeliveries(ctx context.Context, f *cli.Factory, path string, limit int, personID, status, stepID, senderAccountID, failureCode, scheduledAfter, scheduledBefore string) ([]json.RawMessage, []byte, error) {
	remaining := limit
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		resp, err := do(ctx, f, http.MethodGet, path, deliveryListQuery(remaining, pageToken, personID, status, stepID, senderAccountID, failureCode, scheduledAfter, scheduledBefore), nil)
		if err != nil {
			return nil, "", err
		}
		page, next, err := apiclient.DecodePage[json.RawMessage](resp.Body, "deliveries", "next_page_token")
		if err != nil {
			return nil, "", err
		}
		remaining -= len(page)
		return page, next, nil
	})
	if err != nil {
		return nil, nil, err
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	envelope := input.Object{}
	if err := envelope.Set("deliveries", items); err != nil {
		return nil, nil, fmt.Errorf("campaigns: encode deliveries: %w", err)
	}
	raw, err := envelope.Encode()
	if err != nil {
		return nil, nil, fmt.Errorf("campaigns: encode deliveries: %w", err)
	}
	return items, raw, nil
}

func deliveryListQuery(limit int, pageToken, personID, status, stepID, senderAccountID, failureCode, scheduledAfter, scheduledBefore string) url.Values {
	query := listQuery(limit, pageToken)
	if personID != "" {
		query.Set("person_id", personID)
	}
	if status != "" {
		query.Set("status", status)
	}
	if stepID != "" {
		query.Set("step_id", stepID)
	}
	if senderAccountID != "" {
		query.Set("sender_account_id", senderAccountID)
	}
	if failureCode != "" {
		query.Set("failure_code", failureCode)
	}
	if scheduledAfter != "" {
		query.Set("scheduled_after", scheduledAfter)
	}
	if scheduledBefore != "" {
		query.Set("scheduled_before", scheduledBefore)
	}
	return query
}

func decodeDeliveryRecords(items []json.RawMessage) ([]deliveryRecord, error) {
	rows := make([]deliveryRecord, 0, len(items))
	for _, item := range items {
		var row deliveryRecord
		if err := json.Unmarshal(item, &row); err != nil {
			return nil, fmt.Errorf("campaigns: decode delivery: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func printDelivery(f *cli.Factory, raw []byte) error {
	var row deliveryRecord
	if err := json.Unmarshal(raw, &row); err != nil {
		return fmt.Errorf("campaigns: decode delivery: %w", err)
	}
	return printDeliveryTable(f, raw, []deliveryRecord{row})
}

func printDeliveryTable(f *cli.Factory, raw []byte, rows []deliveryRecord) error {
	if err := output.Table(f.Printer(), raw, rows, deliveryColumns()); err != nil {
		return fmt.Errorf("campaigns: print deliveries: %w", err)
	}
	return nil
}

func deliveryScheduledAt(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func deliveryColumns() []output.Column[deliveryRecord] {
	return []output.Column[deliveryRecord]{
		{Header: "ID", Value: func(row deliveryRecord) string { return resourceID(row.Name) }},
		{Header: "PERSON ID", Value: func(row deliveryRecord) string { return resourceID(row.Person) }},
		{Header: "STEP ID", Value: func(row deliveryRecord) string { return resourceID(row.Step) }},
		{Header: "SENDER ACCOUNT ID", Value: func(row deliveryRecord) string { return row.SenderAccountID }},
		{Header: "STATUS", Value: func(row deliveryRecord) string { return row.Status }},
		{Header: "SCHEDULED AT", Value: func(row deliveryRecord) string { return deliveryScheduledAt(row.ScheduledAt) }},
		{Header: "UPDATED", Value: func(row deliveryRecord) string { return row.UpdateTime }},
	}
}

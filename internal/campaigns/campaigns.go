package campaigns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type campaignRecord struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	State       string `json:"state"`
	Schedule    string `json:"schedule"`
	Etag        string `json:"etag"`
	UpdateTime  string `json:"update_time"`
}

func newCampaignCommands(f *cli.Factory) []*cobra.Command {
	return []*cobra.Command{
		newCampaignsListCommand(f),
		newCampaignsGetCommand(f),
		newCampaignsCreateCommand(f),
		newCampaignsUpdateCommand(f),
		newCampaignsDeleteCommand(f),
		newCampaignsCustomCommand(f, "activate ID", "Activate a campaign", "activate", ""),
		newCampaignsCustomCommand(f, "pause ID", "Pause a campaign", "pause", ""),
		newCampaignsCustomCommand(f, "resume ID", "Resume a campaign", "resume", ""),
		newCampaignsCustomCommand(f, "complete ID", "Complete a campaign", "complete", ""),
		newCampaignsCustomCommand(f, "archive ID", "Archive a campaign", "archive", "Archive campaign %s?"),
		newCampaignsCustomCommand(f, "unarchive ID", "Unarchive a campaign", "unarchive", ""),
		newCampaignsSetVariablesCommand(f),
		newCampaignsSetSenderAccountsCommand(f),
	}
}

func newCampaignsListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	var visibility string
	cmd := &cobra.Command{Use: "list", Short: "List campaigns", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum campaigns to return")
	cmd.Flags().StringVar(&visibility, "visibility", "", "Archive visibility filter")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCampaignsList(cmd.Context(), f, limit, visibility)
	}
	return cmd
}

func newCampaignsGetCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "get ID", Short: "Get a campaign", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("campaign id")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCampaignsGet(cmd.Context(), f, args[0])
	}
	return cmd
}

func newCampaignsCreateCommand(f *cli.Factory) *cobra.Command {
	var source, displayName, requestID string
	cmd := &cobra.Command{Use: "create [--display-name NAME | --input FILE]", Short: "Create a campaign", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Campaign display name")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID; generated when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCampaignsCreate(cmd.Context(), f, source, displayName, requestID, cmd.Flags().Changed("display-name"), cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newCampaignsUpdateCommand(f *cli.Factory) *cobra.Command {
	var source, requestID, etag string
	cmd := &cobra.Command{Use: "update ID --input FILE", Short: "Update a campaign", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("campaign id")}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID; generated when omitted")
	cmd.Flags().StringVar(&etag, "etag", "", "Campaign etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCampaignsUpdate(cmd.Context(), f, args[0], source, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newCampaignsDeleteCommand(f *cli.Factory) *cobra.Command {
	var requestID, etag string
	cmd := &cobra.Command{Use: "delete ID", Short: "Delete a campaign", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("campaign id")}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID; generated when omitted")
	cmd.Flags().StringVar(&etag, "etag", "", "Campaign etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCampaignsDelete(cmd.Context(), f, args[0], requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newCampaignsCustomCommand(f *cli.Factory, use, short, verb, prompt string) *cobra.Command {
	var requestID, etag string
	cmd := &cobra.Command{Use: use, Short: short, SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("campaign id")}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID; generated when omitted")
	cmd.Flags().StringVar(&etag, "etag", "", "Campaign etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCampaignsCustom(cmd.Context(), f, args[0], verb, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"), prompt)
	}
	return cmd
}

func newCampaignsSetVariablesCommand(f *cli.Factory) *cobra.Command {
	var source, requestID, etag string
	cmd := &cobra.Command{Use: "set-variables ID --input FILE", Short: "Set campaign variables", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("campaign id")}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID; generated when omitted")
	cmd.Flags().StringVar(&etag, "etag", "", "Campaign etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCampaignsSetVariables(cmd.Context(), f, args[0], source, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newCampaignsSetSenderAccountsCommand(f *cli.Factory) *cobra.Command {
	var source, requestID, etag string
	cmd := &cobra.Command{Use: "set-sender-accounts ID --input FILE", Short: "Set campaign sender accounts", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("campaign id")}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID; generated when omitted")
	cmd.Flags().StringVar(&etag, "etag", "", "Campaign etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runCampaignsSetSenderAccounts(cmd.Context(), f, args[0], source, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func runCampaignsList(ctx context.Context, f *cli.Factory, limit int, visibility string) error {
	if limit < 1 {
		return &cli.UsageError{Msg: "--limit must be at least 1"}
	}
	items, raw, err := listCampaigns(ctx, f, limit, visibility)
	if err != nil {
		return err
	}
	return printCampaignRows(f, raw, items)
}

func runCampaignsGet(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requireArg([]string{id}, "campaign id")
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodGet, campaignPath(id), nil, nil)
	if err != nil {
		return err
	}
	return printCampaign(f, resp.Body)
}

func runCampaignsCreate(ctx context.Context, f *cli.Factory, source, displayName, requestID string, displayNameSet, requestIDSet bool) error {
	if strings.TrimSpace(source) == "" && !displayNameSet {
		return &cli.UsageError{Msg: "--input or --display-name is required"}
	}
	obj, err := input.ReadObject(f.IO.In, source)
	if err != nil {
		return err
	}
	if err := obj.MergeFlag("display_name", "display-name", displayName, displayNameSet); err != nil {
		return err
	}
	if err := ensureOrganization(f, obj); err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, "/v1/campaigns", nil, obj)
	if err != nil {
		return err
	}
	return printCampaign(f, resp.Body)
}

func runCampaignsUpdate(ctx context.Context, f *cli.Factory, id, source, requestID, etag string, requestIDSet, etagSet bool) error {
	id, err := requireArg([]string{id}, "campaign id")
	if err != nil {
		return err
	}
	obj, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureCampaignOrganization(f, obj); err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, obj, etag, etagSet, campaignPath(id)); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPatch, campaignPath(id), nil, obj)
	if err != nil {
		return err
	}
	return printCampaign(f, resp.Body)
}

func runCampaignsDelete(ctx context.Context, f *cli.Factory, id, requestID, etag string, requestIDSet, etagSet bool) error {
	id, err := requireArg([]string{id}, "campaign id")
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete campaign " + id + "?"); err != nil {
		return err
	}
	obj, err := campaignControlBody(ctx, f, requestID, etag, requestIDSet, etagSet, campaignPath(id))
	if err != nil {
		return err
	}
	query := url.Values{}
	query.Set("request_id", obj.String("request_id"))
	query.Set("etag", obj.String("etag"))
	if _, err := do(ctx, f, http.MethodDelete, campaignPath(id), query, nil); err != nil {
		return err
	}
	return nil
}

func runCampaignsCustom(ctx context.Context, f *cli.Factory, id, verb, requestID, etag string, requestIDSet, etagSet bool, prompt string) error {
	id, err := requireArg([]string{id}, "campaign id")
	if err != nil {
		return err
	}
	if prompt != "" {
		if err := f.Confirm(fmt.Sprintf(prompt, id)); err != nil {
			return err
		}
	}
	path := campaignCustomPath(id, verb)
	obj, err := campaignControlBody(ctx, f, requestID, etag, requestIDSet, etagSet, campaignPath(id))
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, path, nil, obj)
	if err != nil {
		return err
	}
	if verb == "pause" {
		return printOperation(f, resp.Body)
	}
	return printCampaign(f, resp.Body)
}

func runCampaignsSetVariables(ctx context.Context, f *cli.Factory, id, source, requestID, etag string, requestIDSet, etagSet bool) error {
	id, err := requireArg([]string{id}, "campaign id")
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
	if err := ensureEtag(ctx, f, obj, etag, etagSet, campaignPath(id)); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, campaignCustomPath(id, "setVariables"), nil, obj)
	if err != nil {
		return err
	}
	return printCampaign(f, resp.Body)
}

func runCampaignsSetSenderAccounts(ctx context.Context, f *cli.Factory, id, source, requestID, etag string, requestIDSet, etagSet bool) error {
	id, err := requireArg([]string{id}, "campaign id")
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
	if err := ensureEtag(ctx, f, obj, etag, etagSet, campaignPath(id)); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, campaignCustomPath(id, "setSenderAccounts"), nil, obj)
	if err != nil {
		return err
	}
	return printCampaignJSON(f, resp.Body)
}

func listCampaigns(ctx context.Context, f *cli.Factory, limit int, visibility string) ([]campaignRecord, []byte, error) {
	remaining := limit
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		query := listQuery(remaining, pageToken)
		if visibility != "" {
			query.Set("visibility", visibility)
		}
		resp, err := do(ctx, f, http.MethodGet, "/v1/campaigns", query, nil)
		if err != nil {
			return nil, "", err
		}
		page, next, err := apiclient.DecodePage[json.RawMessage](resp.Body, "campaigns", "next_page_token")
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
	rows, err := decodeCampaignRecords(items)
	if err != nil {
		return nil, nil, err
	}
	envelope := input.Object{}
	if err := envelope.Set("campaigns", items); err != nil {
		return nil, nil, fmt.Errorf("campaigns: encode campaigns: %w", err)
	}
	raw, err := envelope.Encode()
	if err != nil {
		return nil, nil, fmt.Errorf("campaigns: encode campaigns: %w", err)
	}
	return rows, raw, nil
}

func campaignPath(id string) string {
	return "/v1/campaigns/" + url.PathEscape(resourceID(id))
}

func campaignCustomPath(id, verb string) string {
	return campaignPath(id) + ":" + verb
}

func campaignControlBody(ctx context.Context, f *cli.Factory, requestID, etag string, requestIDSet, etagSet bool, getPath string) (input.Object, error) {
	obj := input.Object{}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return nil, err
	}
	if err := ensureEtag(ctx, f, obj, etag, etagSet, getPath); err != nil {
		return nil, err
	}
	return obj, nil
}

func ensureCampaignOrganization(f *cli.Factory, obj input.Object) error {
	if !obj.Has("campaign") {
		return nil
	}
	var campaign input.Object
	if err := json.Unmarshal(obj["campaign"], &campaign); err != nil || campaign == nil {
		return nil
	}
	if err := ensureOrganization(f, campaign); err != nil {
		return err
	}
	if err := obj.Set("campaign", campaign); err != nil {
		return fmt.Errorf("campaigns: encode campaign: %w", err)
	}
	return nil
}

func decodeCampaignRecords(items []json.RawMessage) ([]campaignRecord, error) {
	rows := make([]campaignRecord, 0, len(items))
	for _, raw := range items {
		row, err := decodeCampaignRecord(raw)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func decodeCampaignRecord(raw []byte) (campaignRecord, error) {
	obj := input.Object{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return campaignRecord{}, fmt.Errorf("campaigns: decode campaign: %w", err)
	}
	payload := json.RawMessage(raw)
	if obj.Has("campaign") {
		payload = obj["campaign"]
	}
	var row campaignRecord
	if err := json.Unmarshal(payload, &row); err != nil {
		return campaignRecord{}, fmt.Errorf("campaigns: decode campaign: %w", err)
	}
	return row, nil
}

func campaignColumns() []output.Column[campaignRecord] {
	return []output.Column[campaignRecord]{
		{Header: "ID", Value: func(row campaignRecord) string { return resourceID(row.Name) }},
		{Header: "DISPLAY NAME", Value: func(row campaignRecord) string { return row.DisplayName }},
		{Header: "STATE", Value: func(row campaignRecord) string { return row.State }},
		{Header: "SCHEDULE", Value: func(row campaignRecord) string { return row.Schedule }},
		{Header: "ETAG", Value: func(row campaignRecord) string { return row.Etag }},
		{Header: "UPDATED", Value: func(row campaignRecord) string { return row.UpdateTime }},
	}
}

func printCampaignRows(f *cli.Factory, raw []byte, rows []campaignRecord) error {
	if err := output.Table(f.Printer(), raw, rows, campaignColumns()); err != nil {
		return fmt.Errorf("campaigns: print: %w", err)
	}
	return nil
}

func printCampaign(f *cli.Factory, raw []byte) error {
	row, err := decodeCampaignRecord(raw)
	if err != nil {
		return err
	}
	return printCampaignRows(f, raw, []campaignRecord{row})
}

func printCampaignJSON(f *cli.Factory, raw []byte) error {
	if err := f.Printer().Object(raw); err != nil {
		return fmt.Errorf("campaigns: print: %w", err)
	}
	return nil
}

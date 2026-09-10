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

type sequence struct {
	Name        string            `json:"name"`
	Kind        string            `json:"kind"`
	DisplayName string            `json:"display_name"`
	Steps       []json.RawMessage `json:"steps"`
	Etag        string            `json:"etag"`
	UpdateTime  string            `json:"update_time"`
}

type sequencePreview struct {
	Subject               string   `json:"subject"`
	ContentMode           string   `json:"content_mode"`
	TrackOpens            bool     `json:"track_opens"`
	TrackClicks           bool     `json:"track_clicks"`
	UnsubscribePolicy     string   `json:"unsubscribe_policy"`
	Warnings              []string `json:"warnings"`
	OperationalURLsDiffer bool     `json:"operational_urls_differ"`
}

func newSequencesCommand(f *cli.Factory) *cobra.Command {
	var campaign string
	cmd := &cobra.Command{Use: "sequences", Short: "Manage campaign sequences", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	cmd.PersistentFlags().StringVar(&campaign, "campaign", "", "Parent campaign ID")
	cmd.AddCommand(newSequencesListCommand(f, &campaign), newSequencesGetCommand(f, &campaign), newSequencesCreateCommand(f, &campaign), newSequencesUpdateCommand(f, &campaign), newSequencesDeleteCommand(f, &campaign), newSequencesPreviewCommand(f, &campaign))
	return cmd
}

func newSequencesListCommand(f *cli.Factory, campaign *string) *cobra.Command {
	limit := 50
	cmd := &cobra.Command{Use: "list --campaign ID", Short: "List campaign sequences", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum sequences to return")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSequencesList(cmd.Context(), f, *campaign, limit)
	}
	return cmd
}

func newSequencesGetCommand(f *cli.Factory, campaign *string) *cobra.Command {
	cmd := &cobra.Command{Use: "get ID --campaign ID", Short: "Get a campaign sequence", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("sequence id")}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSequencesGet(cmd.Context(), f, *campaign, args)
	}
	return cmd
}

func newSequencesCreateCommand(f *cli.Factory, campaign *string) *cobra.Command {
	var source, requestID string
	cmd := &cobra.Command{Use: "create --campaign ID --input FILE", Short: "Create a triggered sequence", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSequencesCreate(cmd.Context(), f, *campaign, source, requestID, cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newSequencesUpdateCommand(f *cli.Factory, campaign *string) *cobra.Command {
	var source, requestID, etag string
	cmd := &cobra.Command{Use: "update ID --campaign ID --input FILE", Short: "Replace a campaign sequence", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("sequence id")}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.Flags().StringVar(&etag, "etag", "", "Sequence etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSequencesUpdate(cmd.Context(), f, *campaign, args, source, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newSequencesDeleteCommand(f *cli.Factory, campaign *string) *cobra.Command {
	var requestID, etag string
	cmd := &cobra.Command{Use: "delete ID --campaign ID", Short: "Delete a triggered sequence", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("sequence id")}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.Flags().StringVar(&etag, "etag", "", "Sequence etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSequencesDelete(cmd.Context(), f, *campaign, args, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newSequencesPreviewCommand(f *cli.Factory, campaign *string) *cobra.Command {
	var source string
	cmd := &cobra.Command{Use: "preview ID --campaign ID --input FILE", Short: "Preview a sequence variant", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("sequence id")}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSequencesPreview(cmd.Context(), f, *campaign, args, source)
	}
	return cmd
}

func runSequencesList(ctx context.Context, f *cli.Factory, campaign string, limit int) error {
	campaign, err := requireCampaign(campaign)
	if err != nil {
		return err
	}
	if limit < 1 {
		return &cli.UsageError{Msg: "--limit must be at least 1"}
	}
	path := sequencesCollectionPath(campaign)
	remaining := limit
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		resp, err := do(ctx, f, http.MethodGet, path, listQuery(remaining, pageToken), nil)
		if err != nil {
			return nil, "", err
		}
		page, next, err := apiclient.DecodePage[json.RawMessage](resp.Body, "sequences", "next_page_token")
		if err != nil {
			return nil, "", err
		}
		remaining -= len(page)
		return page, next, nil
	})
	if err != nil {
		return err
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	rows, err := decodeSequences(items)
	if err != nil {
		return err
	}
	envelope := input.Object{}
	if err := envelope.Set("sequences", items); err != nil {
		return err
	}
	raw, err := envelope.Encode()
	if err != nil {
		return err
	}
	return printSequenceRows(f, raw, rows)
}

func runSequencesGet(ctx context.Context, f *cli.Factory, campaign string, args []string) error {
	path, _, err := sequenceResourcePath(campaign, args)
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	return printSequence(f, resp.Body)
}

func runSequencesCreate(ctx context.Context, f *cli.Factory, campaign, source, requestID string, requestIDSet bool) error {
	campaign, err := requireCampaign(campaign)
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureRequestID(f, body, requestID, requestIDSet); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, sequencesCollectionPath(campaign), nil, body)
	if err != nil {
		return err
	}
	return printSequence(f, resp.Body)
}

func runSequencesUpdate(ctx context.Context, f *cli.Factory, campaign string, args []string, source, requestID, etag string, requestIDSet, etagSet bool) error {
	path, _, err := sequenceResourcePath(campaign, args)
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureRequestID(f, body, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, body, etag, etagSet, path); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPatch, path, nil, body)
	if err != nil {
		return err
	}
	return printSequence(f, resp.Body)
}

func runSequencesDelete(ctx context.Context, f *cli.Factory, campaign string, args []string, requestID, etag string, requestIDSet, etagSet bool) error {
	path, id, err := sequenceResourcePath(campaign, args)
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete sequence " + id + "?"); err != nil {
		return err
	}
	controls := input.Object{}
	if err := ensureRequestID(f, controls, requestID, requestIDSet); err != nil {
		return err
	}
	if err := ensureEtag(ctx, f, controls, etag, etagSet, path); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("request_id", controls.String("request_id"))
	query.Set("etag", controls.String("etag"))
	if _, err := do(ctx, f, http.MethodDelete, path, query, nil); err != nil {
		return err
	}
	return nil
}

func runSequencesPreview(ctx context.Context, f *cli.Factory, campaign string, args []string, source string) error {
	path, _, err := sequenceResourcePath(campaign, args)
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, path+":preview", nil, body)
	if err != nil {
		return err
	}
	return printSequencePreview(f, resp.Body)
}

func sequencesCollectionPath(campaign string) string {
	return "/v1/campaigns/" + campaign + "/sequences"
}

func sequenceResourcePath(campaign string, args []string) (string, string, error) {
	campaign, err := requireCampaign(campaign)
	if err != nil {
		return "", "", err
	}
	id, err := requireArg(args, "sequence id")
	if err != nil {
		return "", "", err
	}
	return sequencesCollectionPath(campaign) + "/" + id, id, nil
}

func decodeSequences(items []json.RawMessage) ([]sequence, error) {
	rows := make([]sequence, 0, len(items))
	for _, raw := range items {
		var row sequence
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, fmt.Errorf("campaigns: decode sequence: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func sequenceColumns() []output.Column[sequence] {
	return []output.Column[sequence]{
		{Header: "ID", Value: func(s sequence) string { return resourceID(s.Name) }},
		{Header: "DISPLAY NAME", Value: func(s sequence) string { return s.DisplayName }},
		{Header: "KIND", Value: func(s sequence) string { return s.Kind }},
		{Header: "STEPS", Value: func(s sequence) string { return formatCount(len(s.Steps)) }},
		{Header: "ETAG", Value: func(s sequence) string { return s.Etag }},
		{Header: "UPDATED", Value: func(s sequence) string { return s.UpdateTime }},
	}
}

func sequencePreviewColumns() []output.Column[sequencePreview] {
	return []output.Column[sequencePreview]{
		{Header: "SUBJECT", Value: func(p sequencePreview) string { return p.Subject }},
		{Header: "CONTENT MODE", Value: func(p sequencePreview) string { return p.ContentMode }},
		{Header: "TRACK OPENS", Value: func(p sequencePreview) string { return formatScalar(p.TrackOpens) }},
		{Header: "TRACK CLICKS", Value: func(p sequencePreview) string { return formatScalar(p.TrackClicks) }},
		{Header: "UNSUBSCRIBE POLICY", Value: func(p sequencePreview) string { return p.UnsubscribePolicy }},
		{Header: "WARNINGS", Value: func(p sequencePreview) string { return strings.Join(p.Warnings, ",") }},
		{Header: "OPERATIONAL URLS DIFFER", Value: func(p sequencePreview) string { return formatScalar(p.OperationalURLsDiffer) }},
	}
}

func printSequenceRows(f *cli.Factory, raw []byte, rows []sequence) error {
	return output.Table(f.Printer(), raw, rows, sequenceColumns())
}

func printSequence(f *cli.Factory, raw []byte) error {
	var row sequence
	if err := json.Unmarshal(raw, &row); err != nil {
		return fmt.Errorf("campaigns: decode sequence: %w", err)
	}
	return printSequenceRows(f, raw, []sequence{row})
}

func printSequencePreview(f *cli.Factory, raw []byte) error {
	var row sequencePreview
	if err := json.Unmarshal(raw, &row); err != nil {
		return fmt.Errorf("campaigns: decode sequence preview: %w", err)
	}
	return output.Table(f.Printer(), raw, []sequencePreview{row}, sequencePreviewColumns())
}

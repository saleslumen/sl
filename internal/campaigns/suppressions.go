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

type suppression struct {
	Name         string `json:"name"`
	EmailAddress string `json:"email_address"`
	Reason       string `json:"reason"`
	Etag         string `json:"etag"`
	CreateTime   string `json:"create_time"`
}

func newSuppressionsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "suppressions", Short: "Manage organization suppressions", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}}
	cmd.AddCommand(newSuppressionsListCommand(f), newSuppressionsCreateCommand(f), newSuppressionsDeleteCommand(f))
	return cmd
}

func newSuppressionsListCommand(f *cli.Factory) *cobra.Command {
	limit := 50
	var emailAddress string
	cmd := &cobra.Command{Use: "list", Short: "List suppressions", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum suppressions to return")
	cmd.Flags().StringVar(&emailAddress, "email-address", "", "Filter by email address")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSuppressionsList(cmd.Context(), f, limit, emailAddress)
	}
	return cmd
}

func newSuppressionsCreateCommand(f *cli.Factory) *cobra.Command {
	var source, requestID string
	cmd := &cobra.Command{Use: "create --input FILE", Short: "Create a suppression", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: cobra.NoArgs}
	cmd.Flags().StringVar(&source, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSuppressionsCreate(cmd.Context(), f, source, requestID, cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newSuppressionsDeleteCommand(f *cli.Factory) *cobra.Command {
	var etag, requestID string
	cmd := &cobra.Command{Use: "delete ID --etag ETAG", Short: "Delete a suppression", SilenceUsage: true, SilenceErrors: true, DisableAutoGenTag: true, Args: exactArg("suppression id")}
	cmd.Flags().StringVar(&etag, "etag", "", "Suppression etag (required; no GET exists)")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runSuppressionsDelete(cmd.Context(), f, args[0], etag, requestID, cmd.Flags().Changed("etag"), cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func runSuppressionsList(ctx context.Context, f *cli.Factory, limit int, emailAddress string) error {
	if limit < 1 {
		return &cli.UsageError{Msg: "--limit must be at least 1"}
	}
	fetched := 0
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		query := listQuery(limit-fetched, pageToken)
		if emailAddress != "" {
			query.Set("email_address", emailAddress)
		}
		resp, err := do(ctx, f, http.MethodGet, "/v1/suppressions", query, nil)
		if err != nil {
			return nil, "", err
		}
		page, next, err := apiclient.DecodePage[json.RawMessage](resp.Body, "suppressions", "next_page_token")
		if err != nil {
			return nil, "", err
		}
		fetched += len(page)
		return page, next, nil
	})
	if err != nil {
		return err
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	rows := make([]suppression, 0, len(items))
	for _, raw := range items {
		row, err := decodeSuppression(raw)
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	raw, err := json.Marshal(struct {
		Suppressions []json.RawMessage `json:"suppressions"`
	}{Suppressions: items})
	if err != nil {
		return fmt.Errorf("campaigns: encode suppressions: %w", err)
	}
	return printSuppressions(f, raw, rows)
}

func runSuppressionsCreate(ctx context.Context, f *cli.Factory, source, requestID string, requestIDSet bool) error {
	obj, err := readRequiredObject(f, source)
	if err != nil {
		return err
	}
	if err := ensureRequestID(f, obj, requestID, requestIDSet); err != nil {
		return err
	}
	resp, err := do(ctx, f, http.MethodPost, "/v1/suppressions", nil, obj)
	if err != nil {
		return err
	}
	return printSuppression(f, resp.Body)
}

func runSuppressionsDelete(ctx context.Context, f *cli.Factory, id, etag, requestID string, etagSet, requestIDSet bool) error {
	id, err := requireArg([]string{id}, "suppression id")
	if err != nil {
		return err
	}
	if !etagSet || etag == "" {
		return &cli.UsageError{Msg: "--etag is required"}
	}
	if err := f.Confirm(fmt.Sprintf("Delete suppression %s?", id)); err != nil {
		return err
	}
	controls := input.Object{}
	if err := ensureRequestID(f, controls, requestID, requestIDSet); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("request_id", controls.String("request_id"))
	query.Set("etag", etag)
	if _, err := do(ctx, f, http.MethodDelete, "/v1/suppressions/"+url.PathEscape(id), query, nil); err != nil {
		return err
	}
	return nil
}

func decodeSuppression(raw []byte) (suppression, error) {
	var row suppression
	if err := json.Unmarshal(raw, &row); err != nil {
		return suppression{}, fmt.Errorf("campaigns: decode suppression: %w", err)
	}
	return row, nil
}

func suppressionColumns() []output.Column[suppression] {
	return []output.Column[suppression]{
		{Header: "ID", Value: func(row suppression) string { return resourceID(row.Name) }},
		{Header: "EMAIL", Value: func(row suppression) string { return row.EmailAddress }},
		{Header: "REASON", Value: func(row suppression) string { return row.Reason }},
		{Header: "ETAG", Value: func(row suppression) string { return row.Etag }},
		{Header: "CREATED", Value: func(row suppression) string { return row.CreateTime }},
	}
}

func printSuppression(f *cli.Factory, raw []byte) error {
	row, err := decodeSuppression(raw)
	if err != nil {
		return err
	}
	return printSuppressions(f, raw, []suppression{row})
}

func printSuppressions(f *cli.Factory, raw []byte, rows []suppression) error {
	if err := output.Table(f.Printer(), raw, rows, suppressionColumns()); err != nil {
		return fmt.Errorf("campaigns: print suppressions: %w", err)
	}
	return nil
}

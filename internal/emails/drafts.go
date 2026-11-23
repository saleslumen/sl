package emails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type mailboxDraft struct {
	ID        string
	MessageID string
	AccountID string
	ETag      string
	Snippet   string
}

func newDraftsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "drafts", Short: "Manage mailbox drafts"}
	cmd.AddCommand(newDraftsListCommand(f))
	cmd.AddCommand(newDraftsGetCommand(f))
	cmd.AddCommand(newDraftsCreateCommand(f))
	cmd.AddCommand(newDraftsUpdateCommand(f))
	cmd.AddCommand(newDraftsDeleteCommand(f))
	cmd.AddCommand(newDraftsSendCommand(f))
	return cmd
}

func newDraftsListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	var query string
	var includeSpamTrash bool
	cmd := &cobra.Command{Use: "list", Short: "List mailbox drafts", Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "Maximum drafts to return")
	cmd.Flags().StringVar(&query, "query", "", "Search query")
	cmd.Flags().BoolVar(&includeSpamTrash, "include-spam-trash", false, "Include SPAM and TRASH drafts")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDraftsList(cmd.Context(), f, limit, query, includeSpamTrash)
	}
	return cmd
}

func newDraftsGetCommand(f *cli.Factory) *cobra.Command {
	var format string
	cmd := &cobra.Command{Use: "get ID", Short: "Get a mailbox draft", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&format, "format", "FULL", "Message format (MINIMAL, METADATA, FULL, RAW)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDraftsGet(cmd.Context(), f, args[0], format)
	}
	return cmd
}

func newDraftsCreateCommand(f *cli.Factory) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "create --input FILE", Short: "Create a mailbox draft", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDraftsCreate(cmd.Context(), f, input)
	}
	return cmd
}

func newDraftsUpdateCommand(f *cli.Factory) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "update ID --input FILE", Short: "Update a mailbox draft", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDraftsUpdate(cmd.Context(), f, args[0], input)
	}
	return cmd
}

func newDraftsSendCommand(f *cli.Factory) *cobra.Command {
	var input, requestID string
	cmd := &cobra.Command{Use: "send --input FILE", Short: "Send a mailbox draft", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID (generated if omitted)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDraftsSend(cmd.Context(), f, input, requestID, cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newDraftsDeleteCommand(f *cli.Factory) *cobra.Command {
	var requestID, etag string
	cmd := &cobra.Command{Use: "delete ID", Short: "Permanently delete a mailbox draft", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.Flags().StringVar(&etag, "etag", "", "Current draft message etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDraftsDelete(cmd.Context(), f, args[0], requestID, etag)
	}
	return cmd
}

func runDraftsList(ctx context.Context, f *cli.Factory, limit int, query string, includeSpamTrash bool) error {
	if err := validateLimit(limit); err != nil {
		return err
	}
	var fetched int
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		page, next, err := fetchDraftPage(ctx, f, pageSize(limit, fetched), pageToken, query, includeSpamTrash)
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
	rows, err := mailboxDraftsFrom(items)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(struct {
		Drafts []json.RawMessage `json:"drafts"`
	}{Drafts: items})
	if err != nil {
		return fmt.Errorf("emails: encode drafts: %w", err)
	}
	return printTable(f, raw, rows, draftColumns())
}

func runDraftsGet(ctx context.Context, f *cli.Factory, id, format string) error {
	id, err := requirePathID(id, "draft ID")
	if err != nil {
		return err
	}
	if err := validateMessageFormat(format); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("format", format)
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath("/v1/drafts", id), query, nil)
	if err != nil {
		return err
	}
	return printDraft(f, resp.Body)
}

func runDraftsCreate(ctx context.Context, f *cli.Factory, input string) error {
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, "/v1/drafts", nil, raw)
	if err != nil {
		return err
	}
	return printDraft(f, resp.Body)
}

func runDraftsUpdate(ctx context.Context, f *cli.Factory, id, input string) error {
	id, err := requirePathID(id, "draft ID")
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPut, itemPath("/v1/drafts", id), nil, raw)
	if err != nil {
		return err
	}
	return printDraft(f, resp.Body)
}

func runDraftsSend(ctx context.Context, f *cli.Factory, input, requestIDValue string, requestIDSet bool) error {
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	if err := mergeGeneratedRequestID(body, f, requestIDValue, requestIDSet); err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, "/v1/drafts:send", nil, raw)
	if err != nil {
		return err
	}
	return printMessage(f, resp.Body)
}

func runDraftsDelete(ctx context.Context, f *cli.Factory, id, requestIDValue, etag string) error {
	id, err := requirePathID(id, "draft ID")
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete draft " + id + "?"); err != nil {
		return err
	}
	query, err := draftMutationQuery(ctx, f, id, requestIDValue, etag)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodDelete, itemPath("/v1/drafts", id), query, nil)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return nil
	}
	return printObject(f, resp.Body)
}

func fetchDraftPage(ctx context.Context, f *cli.Factory, pageSize int, pageToken, query string, includeSpamTrash bool) ([]json.RawMessage, string, error) {
	values := url.Values{}
	values.Set("maxResults", strconv.Itoa(pageSize))
	if pageToken != "" {
		values.Set("pageToken", pageToken)
	}
	if query != "" {
		values.Set("q", query)
	}
	if includeSpamTrash {
		values.Set("includeSpamTrash", "true")
	}
	resp, err := doRequest(ctx, f, http.MethodGet, "/v1/drafts", values, nil)
	if err != nil {
		return nil, "", err
	}
	return apiclient.DecodePage[json.RawMessage](resp.Body, "drafts", "nextPageToken")
}

func draftMutationQuery(ctx context.Context, f *cli.Factory, id, requestIDValue, etag string) (url.Values, error) {
	requestIDValue, err := requestID(f, requestIDValue)
	if err != nil {
		return nil, err
	}
	if etag == "" {
		raw, err := fetchDraftEtag(ctx, f, id)
		if err != nil {
			return nil, err
		}
		etag = jsonScalarText(raw)
	}
	query := url.Values{}
	query.Set("requestId", requestIDValue)
	query.Set("etag", etag)
	return query, nil
}

func fetchDraftEtag(ctx context.Context, f *cli.Factory, id string) (json.RawMessage, error) {
	query := url.Values{}
	query.Set("format", "MINIMAL")
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath("/v1/drafts", id), query, nil)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(resp.Body, &wrap); err != nil {
		return nil, fmt.Errorf("emails: decode draft %s: %w", id, err)
	}
	etag, err := resourceEtag(wrap.Message)
	if err != nil {
		return nil, fmt.Errorf("emails: draft %s: %w", id, err)
	}
	return etag, nil
}

func mailboxDraftsFrom(items []json.RawMessage) ([]mailboxDraft, error) {
	rows := make([]mailboxDraft, 0, len(items))
	for _, item := range items {
		row, err := mailboxDraftFrom(item)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func mailboxDraftFrom(raw json.RawMessage) (mailboxDraft, error) {
	var wire struct {
		ID      string `json:"id"`
		Message struct {
			ID        string          `json:"id"`
			AccountID string          `json:"accountId"`
			ETag      json.RawMessage `json:"etag"`
			Snippet   string          `json:"snippet"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return mailboxDraft{}, fmt.Errorf("emails: decode draft: %w", err)
	}
	return mailboxDraft{ID: wire.ID, MessageID: wire.Message.ID, AccountID: wire.Message.AccountID, ETag: jsonScalarText(wire.Message.ETag), Snippet: wire.Message.Snippet}, nil
}

func printDraft(f *cli.Factory, raw []byte) error {
	row, err := mailboxDraftFrom(raw)
	if err != nil {
		return err
	}
	return printTable(f, raw, []mailboxDraft{row}, draftColumns())
}

func draftColumns() []output.Column[mailboxDraft] {
	return []output.Column[mailboxDraft]{
		{Header: "ID", Value: func(row mailboxDraft) string { return row.ID }},
		{Header: "MESSAGE_ID", Value: func(row mailboxDraft) string { return row.MessageID }},
		{Header: "ACCOUNT", Value: func(row mailboxDraft) string { return row.AccountID }},
		{Header: "ETAG", Value: func(row mailboxDraft) string { return row.ETag }},
		{Header: "SNIPPET", Value: func(row mailboxDraft) string { return row.Snippet }},
	}
}

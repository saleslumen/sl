package emails

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	foundationinput "github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type mailboxMessage struct {
	ID           string
	ThreadID     string
	AccountID    string
	InternalDate string
	LabelIDs     []string
	SendState    string
	ETag         string
	Snippet      string
}

func newMessagesCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "messages", Short: "Manage mailbox messages"}
	cmd.AddCommand(newMessagesListCommand(f))
	cmd.AddCommand(newMessagesGetCommand(f))
	cmd.AddCommand(newMessagesSendCommand(f))
	cmd.AddCommand(newMessagesModifyCommand(f))
	cmd.AddCommand(newMessagesTrashCommand(f))
	cmd.AddCommand(newMessagesUntrashCommand(f))
	cmd.AddCommand(newMessagesDeleteCommand(f))
	cmd.AddCommand(newMessagesBatchModifyCommand(f))
	cmd.AddCommand(newMessagesBatchDeleteCommand(f))
	cmd.AddCommand(newMessagesGenerateQuotedCommand(f))
	return cmd
}

func newMessagesListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	var query string
	var labels, accounts []string
	var includeSpamTrash bool
	cmd := &cobra.Command{Use: "list", Short: "List mailbox messages", Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "Maximum messages to return")
	cmd.Flags().StringVar(&query, "query", "", "Search query")
	cmd.Flags().StringArrayVar(&labels, "label", nil, "Label ID that messages must include")
	cmd.Flags().BoolVar(&includeSpamTrash, "include-spam-trash", false, "Include SPAM and TRASH messages")
	cmd.Flags().StringArrayVar(&accounts, "account", nil, "Account ID to search")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesList(cmd.Context(), f, limit, query, labels, includeSpamTrash, accounts)
	}
	return cmd
}

func newMessagesGetCommand(f *cli.Factory) *cobra.Command {
	var format string
	var metadataHeaders []string
	cmd := &cobra.Command{Use: "get ID", Short: "Get a mailbox message", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&format, "format", "FULL", "Message format (MINIMAL, METADATA, FULL, RAW)")
	cmd.Flags().StringArrayVar(&metadataHeaders, "metadata-header", nil, "Header to include when format is METADATA")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesGet(cmd.Context(), f, args[0], format, metadataHeaders)
	}
	return cmd
}

func newMessagesSendCommand(f *cli.Factory) *cobra.Command {
	var input, requestID string
	cmd := &cobra.Command{Use: "send --input FILE", Short: "Send a mailbox message", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID (generated if omitted)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesSend(cmd.Context(), f, input, requestID, cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newMessagesModifyCommand(f *cli.Factory) *cobra.Command {
	var input, requestID, etag string
	cmd := &cobra.Command{Use: "modify ID --input FILE", Short: "Modify message labels or schedule", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID when scheduling or trashing")
	cmd.Flags().StringVar(&etag, "etag", "", "Message etag when scheduling or trashing (fetched if omitted)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesModify(cmd.Context(), f, args[0], input, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newMessagesTrashCommand(f *cli.Factory) *cobra.Command {
	var requestID, etag string
	cmd := &cobra.Command{Use: "trash ID", Short: "Move a message to trash", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID (generated if omitted)")
	cmd.Flags().StringVar(&etag, "etag", "", "Current message etag (fetched if omitted)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesTrash(cmd.Context(), f, args[0], requestID, etag)
	}
	return cmd
}

func newMessagesUntrashCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "untrash ID", Short: "Remove a message from trash", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesUntrash(cmd.Context(), f, args[0])
	}
	return cmd
}

func newMessagesDeleteCommand(f *cli.Factory) *cobra.Command {
	var requestID, etag string
	cmd := &cobra.Command{Use: "delete ID", Short: "Permanently delete a mailbox message", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID")
	cmd.Flags().StringVar(&etag, "etag", "", "Current message etag; fetched when omitted")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesDelete(cmd.Context(), f, args[0], requestID, etag)
	}
	return cmd
}

func newMessagesBatchModifyCommand(f *cli.Factory) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "batch-modify --input FILE", Short: "Modify messages in batch", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesBatchModify(cmd.Context(), f, input)
	}
	return cmd
}

func newMessagesBatchDeleteCommand(f *cli.Factory) *cobra.Command {
	var input, requestID string
	cmd := &cobra.Command{Use: "batch-delete --input FILE", Short: "Permanently delete messages in batch", Args: cobra.NoArgs}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency request ID (generated if omitted)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesBatchDelete(cmd.Context(), f, input, requestID, cmd.Flags().Changed("request-id"))
	}
	return cmd
}

func newMessagesGenerateQuotedCommand(f *cli.Factory) *cobra.Command {
	var input string
	cmd := &cobra.Command{Use: "generate-quoted-content ID --input FILE", Short: "Generate quoted reply or forward content", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMessagesGenerateQuoted(cmd.Context(), f, args[0], input)
	}
	return cmd
}

func runMessagesList(ctx context.Context, f *cli.Factory, limit int, query string, labels []string, includeSpamTrash bool, accounts []string) error {
	if err := validateLimit(limit); err != nil {
		return err
	}
	var fetched int
	items, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		page, next, err := fetchMessagePage(ctx, f, pageSize(limit, fetched), pageToken, query, labels, includeSpamTrash, accounts)
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
	rows, err := mailboxMessagesFrom(items)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(struct {
		Messages []json.RawMessage `json:"messages"`
	}{Messages: items})
	if err != nil {
		return fmt.Errorf("emails: encode messages: %w", err)
	}
	return printTable(f, raw, rows, messageColumns())
}

func runMessagesGet(ctx context.Context, f *cli.Factory, id, format string, metadataHeaders []string) error {
	id, err := requirePathID(id, "message ID")
	if err != nil {
		return err
	}
	if err := validateMessageFormat(format); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("format", format)
	for _, header := range metadataHeaders {
		query.Add("metadataHeaders", header)
	}
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath("/v1/messages", id), query, nil)
	if err != nil {
		return err
	}
	row, err := mailboxMessageFrom(resp.Body)
	if err != nil {
		return err
	}
	return printTable(f, resp.Body, []mailboxMessage{row}, messageColumns())
}

func runMessagesSend(ctx context.Context, f *cli.Factory, input, requestIDValue string, requestIDSet bool) error {
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
	resp, err := doRequest(ctx, f, http.MethodPost, "/v1/messages:send", nil, raw)
	if err != nil {
		return err
	}
	return printMessage(f, resp.Body)
}

func runMessagesModify(ctx context.Context, f *cli.Factory, id, input, requestIDValue, etag string, requestIDSet, etagSet bool) error {
	id, err := requirePathID(id, "message ID")
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	if messageModifyNeedsControls(body) {
		if err := body.MergeFlag("requestId", "request-id", requestIDValue, requestIDSet); err != nil {
			return err
		}
		if err := setAbsentGeneratedRequestID(body, f); err != nil {
			return err
		}
		if err := body.MergeFlag("etag", "etag", etag, etagSet); err != nil {
			return err
		}
		if err := setAbsentFetchedEtag(body, func() (json.RawMessage, error) { return fetchMessageEtag(ctx, f, id) }); err != nil {
			return err
		}
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, itemPath("/v1/messages", id)+":modify", nil, raw)
	if err != nil {
		return err
	}
	return printMessage(f, resp.Body)
}

func runMessagesTrash(ctx context.Context, f *cli.Factory, id, requestIDValue, etag string) error {
	id, err := requirePathID(id, "message ID")
	if err != nil {
		return err
	}
	if err := f.Confirm("Trash message " + id + "?"); err != nil {
		return err
	}
	query, err := messageMutationQuery(ctx, f, id, requestIDValue, etag)
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, itemPath("/v1/messages", id)+":trash", query, nil)
	if err != nil {
		return err
	}
	return printMessage(f, resp.Body)
}

func runMessagesUntrash(ctx context.Context, f *cli.Factory, id string) error {
	id, err := requirePathID(id, "message ID")
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, itemPath("/v1/messages", id)+":untrash", nil, nil)
	if err != nil {
		return err
	}
	return printMessage(f, resp.Body)
}

func runMessagesDelete(ctx context.Context, f *cli.Factory, id, requestIDValue, etag string) error {
	id, err := requirePathID(id, "message ID")
	if err != nil {
		return err
	}
	if err := f.Confirm("Delete message " + id + "?"); err != nil {
		return err
	}
	query, err := messageMutationQuery(ctx, f, id, requestIDValue, etag)
	if err != nil {
		return err
	}
	// DeleteMessage proto bindings have no body; requestId and etag are query parameters.
	resp, err := doRequest(ctx, f, http.MethodDelete, itemPath("/v1/messages", id), query, nil)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return nil
	}
	return printObject(f, resp.Body)
}

func runMessagesBatchModify(ctx context.Context, f *cli.Factory, input string) error {
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, "/v1/messages:batchModify", nil, raw)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}

func runMessagesBatchDelete(ctx context.Context, f *cli.Factory, input, requestIDValue string, requestIDSet bool) error {
	if err := f.Confirm("Delete messages in batch?"); err != nil {
		return err
	}
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
	resp, err := doRequest(ctx, f, http.MethodPost, "/v1/messages:batchDelete", nil, raw)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}

func runMessagesGenerateQuoted(ctx context.Context, f *cli.Factory, id, input string) error {
	id, err := requirePathID(id, "message ID")
	if err != nil {
		return err
	}
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	if err := injectReplyToMessageID(body, id); err != nil {
		return err
	}
	encoded, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, "/v1/messages:generateQuotedContent", nil, encoded)
	if err != nil {
		return err
	}
	return printObject(f, resp.Body)
}

func fetchMessagePage(ctx context.Context, f *cli.Factory, pageSize int, pageToken, query string, labels []string, includeSpamTrash bool, accounts []string) ([]json.RawMessage, string, error) {
	values := url.Values{}
	values.Set("maxResults", strconv.Itoa(pageSize))
	if pageToken != "" {
		values.Set("pageToken", pageToken)
	}
	if query != "" {
		values.Set("q", query)
	}
	for _, label := range labels {
		values.Add("labelIds", label)
	}
	if includeSpamTrash {
		values.Set("includeSpamTrash", "true")
	}
	for _, account := range accounts {
		values.Add("accountIds", account)
	}
	resp, err := doRequest(ctx, f, http.MethodGet, "/v1/messages", values, nil)
	if err != nil {
		return nil, "", err
	}
	return apiclient.DecodePage[json.RawMessage](resp.Body, "messages", "nextPageToken")
}

func messageMutationQuery(ctx context.Context, f *cli.Factory, id, requestIDValue, etag string) (url.Values, error) {
	requestIDValue, err := requestID(f, requestIDValue)
	if err != nil {
		return nil, err
	}
	if etag == "" {
		raw, err := fetchMessageEtag(ctx, f, id)
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

func fetchMessageEtag(ctx context.Context, f *cli.Factory, id string) (json.RawMessage, error) {
	query := url.Values{}
	query.Set("format", "MINIMAL")
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath("/v1/messages", id), query, nil)
	if err != nil {
		return nil, err
	}
	etag, err := resourceEtag(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("emails: message %s: %w", id, err)
	}
	return etag, nil
}

func mergeGeneratedRequestID(body foundationinput.Object, f *cli.Factory, value string, changed bool) error {
	if err := body.MergeFlag("requestId", "request-id", value, changed); err != nil {
		return err
	}
	return setAbsentGeneratedRequestID(body, f)
}

func messageModifyNeedsControls(body foundationinput.Object) bool {
	return body.Has("scheduleTime") || addsTrashLabel(body)
}

func injectReplyToMessageID(body foundationinput.Object, id string) error {
	if !body.Has("replyToMessageId") {
		return body.SetAbsent("replyToMessageId", id)
	}
	if body.String("replyToMessageId") != id {
		return &cli.UsageError{Msg: "replyToMessageId conflicts with positional message ID"}
	}
	return nil
}

func validateMessageFormat(format string) error {
	switch format {
	case "MINIMAL", "METADATA", "FULL", "RAW":
		return nil
	default:
		return &cli.UsageError{Msg: "format must be MINIMAL, METADATA, FULL, or RAW"}
	}
}

func mailboxMessagesFrom(items []json.RawMessage) ([]mailboxMessage, error) {
	rows := make([]mailboxMessage, 0, len(items))
	for _, item := range items {
		row, err := mailboxMessageFrom(item)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func mailboxMessageFrom(raw json.RawMessage) (mailboxMessage, error) {
	var wire struct {
		ID           string          `json:"id"`
		ThreadID     string          `json:"threadId"`
		AccountID    string          `json:"accountId"`
		InternalDate string          `json:"internalDate"`
		LabelIDs     []string        `json:"labelIds"`
		SendState    string          `json:"sendState"`
		ETag         json.RawMessage `json:"etag"`
		Snippet      string          `json:"snippet"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return mailboxMessage{}, fmt.Errorf("emails: decode message: %w", err)
	}
	return mailboxMessage{ID: wire.ID, ThreadID: wire.ThreadID, AccountID: wire.AccountID, InternalDate: wire.InternalDate, LabelIDs: wire.LabelIDs, SendState: wire.SendState, ETag: jsonScalarText(wire.ETag), Snippet: wire.Snippet}, nil
}

func printMessage(f *cli.Factory, raw []byte) error {
	row, err := mailboxMessageFrom(raw)
	if err != nil {
		return err
	}
	return printTable(f, raw, []mailboxMessage{row}, messageColumns())
}

func messageColumns() []output.Column[mailboxMessage] {
	return []output.Column[mailboxMessage]{
		{Header: "ID", Value: func(row mailboxMessage) string { return row.ID }},
		{Header: "THREAD", Value: func(row mailboxMessage) string { return row.ThreadID }},
		{Header: "ACCOUNT", Value: func(row mailboxMessage) string { return row.AccountID }},
		{Header: "DATE", Value: func(row mailboxMessage) string { return row.InternalDate }},
		{Header: "LABELS", Value: func(row mailboxMessage) string { return strings.Join(row.LabelIDs, ",") }},
		{Header: "SEND_STATE", Value: func(row mailboxMessage) string { return row.SendState }},
		{Header: "ETAG", Value: func(row mailboxMessage) string { return row.ETag }},
		{Header: "SNIPPET", Value: func(row mailboxMessage) string { return row.Snippet }},
	}
}

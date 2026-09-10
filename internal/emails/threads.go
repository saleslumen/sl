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
	foundationinput "github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

const threadTrashLabel = "TRASH"

type thread struct {
	ID        string            `json:"id"`
	Snippet   string            `json:"snippet"`
	HistoryID string            `json:"historyId"`
	Messages  []json.RawMessage `json:"messages"`
	Etag      json.RawMessage   `json:"etag"`
}

func newThreadsCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "threads", Short: "Manage conversation threads"}
	cmd.AddCommand(newThreadsListCommand(f), newThreadsGetCommand(f), newThreadsModifyCommand(f), newThreadsDeleteCommand(f), newThreadsTrashCommand(f), newThreadsUntrashCommand(f))
	return cmd
}

func newThreadsListCommand(f *cli.Factory) *cobra.Command {
	var limit int
	var query string
	var labels, accounts []string
	var includeSpamTrash bool
	cmd := &cobra.Command{Use: "list", Short: "List conversation threads", Args: cobra.NoArgs}
	cmd.Flags().IntVar(&limit, "limit", defaultListLimit, "Maximum threads to collect")
	cmd.Flags().StringVar(&query, "query", "", "Search query")
	cmd.Flags().StringArrayVar(&labels, "label", nil, "Required label ID")
	cmd.Flags().BoolVar(&includeSpamTrash, "include-spam-trash", false, "Include SPAM and TRASH threads")
	cmd.Flags().StringArrayVar(&accounts, "account", nil, "Restrict to account ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runThreadsList(cmd.Context(), f, limit, query, labels, accounts, includeSpamTrash)
	}
	return cmd
}

func newThreadsGetCommand(f *cli.Factory) *cobra.Command {
	var format string
	var metadataHeaders []string
	cmd := &cobra.Command{Use: "get ID", Short: "Get a thread", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&format, "format", "FULL", "Embedded message format")
	cmd.Flags().StringArrayVar(&metadataHeaders, "metadata-header", nil, "Header to include when format is METADATA")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runThreadsGet(cmd.Context(), f, args[0], format, metadataHeaders)
	}
	return cmd
}

func newThreadsModifyCommand(f *cli.Factory) *cobra.Command {
	var input, requestID, etag string
	cmd := &cobra.Command{Use: "modify ID --input FILE", Short: "Modify thread labels", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&input, "input", "", "JSON request body file, or - for stdin")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency key when adding TRASH (generated if omitted)")
	cmd.Flags().StringVar(&etag, "etag", "", "Thread etag when adding TRASH (fetched if omitted)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runThreadsModify(cmd.Context(), f, args[0], input, requestID, etag, cmd.Flags().Changed("request-id"), cmd.Flags().Changed("etag"))
	}
	return cmd
}

func newThreadsDeleteCommand(f *cli.Factory) *cobra.Command {
	var requestID, etag string
	cmd := &cobra.Command{Use: "delete ID", Short: "Delete a thread", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency key (generated if omitted)")
	cmd.Flags().StringVar(&etag, "etag", "", "Thread etag (fetched if omitted)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runThreadsQueryMutation(cmd.Context(), f, http.MethodDelete, itemPath("/v1/threads", args[0]), args[0], fmt.Sprintf("Delete thread %s?", args[0]), requestID, etag)
	}
	return cmd
}

func newThreadsTrashCommand(f *cli.Factory) *cobra.Command {
	var requestID, etag string
	cmd := &cobra.Command{Use: "trash ID", Short: "Move a thread to trash", Args: cobra.ExactArgs(1)}
	cmd.Flags().StringVar(&requestID, "request-id", "", "Idempotency key (generated if omitted)")
	cmd.Flags().StringVar(&etag, "etag", "", "Thread etag (fetched if omitted)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runThreadsQueryMutation(cmd.Context(), f, http.MethodPost, itemPath("/v1/threads", args[0])+":trash", args[0], fmt.Sprintf("Trash thread %s?", args[0]), requestID, etag)
	}
	return cmd
}

func newThreadsUntrashCommand(f *cli.Factory) *cobra.Command {
	cmd := &cobra.Command{Use: "untrash ID", Short: "Remove a thread from trash", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		resp, err := doRequest(cmd.Context(), f, http.MethodPost, itemPath("/v1/threads", args[0])+":untrash", nil, nil)
		if err != nil {
			return err
		}
		return printThread(f, resp.Body)
	}
	return cmd
}

func runThreadsList(ctx context.Context, f *cli.Factory, limit int, query string, labels, accounts []string, includeSpamTrash bool) error {
	if err := validateLimit(limit); err != nil {
		return err
	}
	collected := 0
	raws, err := apiclient.ListAll(ctx, limit, func(ctx context.Context, pageToken string) ([]json.RawMessage, string, error) {
		resp, err := doRequest(ctx, f, http.MethodGet, "/v1/threads", threadListQuery(pageToken, pageSize(limit, collected), query, labels, accounts, includeSpamTrash), nil)
		if err != nil {
			return nil, "", err
		}
		threads, next, err := apiclient.DecodePage[json.RawMessage](resp.Body, "threads", "nextPageToken")
		if err != nil {
			return nil, "", err
		}
		collected += len(threads)
		return threads, next, nil
	})
	if err != nil {
		return err
	}
	if raws == nil {
		raws = []json.RawMessage{}
	}
	return printThreadList(f, raws)
}

func runThreadsGet(ctx context.Context, f *cli.Factory, id, format string, metadataHeaders []string) error {
	if err := validateMessageFormat(format); err != nil {
		return err
	}
	query := url.Values{}
	query.Set("format", format)
	for _, header := range metadataHeaders {
		query.Add("metadataHeaders", header)
	}
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath("/v1/threads", id), query, nil)
	if err != nil {
		return err
	}
	return printThread(f, resp.Body)
}

func runThreadsModify(ctx context.Context, f *cli.Factory, id, input, requestID, etag string, requestIDSet, etagSet bool) error {
	body, err := readRequiredObject(f, input)
	if err != nil {
		return err
	}
	if addsTrashLabel(body) {
		if err := body.MergeFlag("requestId", "request-id", requestID, requestIDSet); err != nil {
			return err
		}
		if err := setAbsentGeneratedRequestID(body, f); err != nil {
			return err
		}
		if err := body.MergeFlag("etag", "etag", etag, etagSet); err != nil {
			return err
		}
		if err := setAbsentFetchedEtag(body, func() (json.RawMessage, error) { return fetchThreadEtag(ctx, f, id) }); err != nil {
			return err
		}
	}
	raw, err := body.Encode()
	if err != nil {
		return err
	}
	resp, err := doRequest(ctx, f, http.MethodPost, itemPath("/v1/threads", id)+":modify", nil, raw)
	if err != nil {
		return err
	}
	return printThread(f, resp.Body)
}

func runThreadsQueryMutation(ctx context.Context, f *cli.Factory, method, path, id, prompt, requestIDValue, etag string) error {
	if err := f.Confirm(prompt); err != nil {
		return err
	}
	rid, err := requestID(f, requestIDValue)
	if err != nil {
		return err
	}
	if etag == "" {
		raw, err := fetchThreadEtag(ctx, f, id)
		if err != nil {
			return err
		}
		etag = jsonScalarText(raw)
	}
	query := url.Values{}
	query.Set("requestId", rid)
	query.Set("etag", etag)
	resp, err := doRequest(ctx, f, method, path, query, nil)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return printObject(f, resp.Body)
	}
	return printThread(f, resp.Body)
}

func threadColumns() []output.Column[thread] {
	return []output.Column[thread]{
		{Header: "ID", Value: func(row thread) string { return row.ID }},
		{Header: "MESSAGES", Value: func(row thread) string { return strconv.Itoa(len(row.Messages)) }},
		{Header: "HISTORY_ID", Value: func(row thread) string { return row.HistoryID }},
		{Header: "ETAG", Value: func(row thread) string { return jsonScalarText(row.Etag) }},
		{Header: "SNIPPET", Value: func(row thread) string { return row.Snippet }},
	}
}

func printThread(f *cli.Factory, raw []byte) error {
	var row thread
	if err := json.Unmarshal(raw, &row); err != nil {
		return fmt.Errorf("emails: decode thread: %w", err)
	}
	return printTable(f, raw, []thread{row}, threadColumns())
}

func printThreadList(f *cli.Factory, raws []json.RawMessage) error {
	rows := make([]thread, 0, len(raws))
	for _, raw := range raws {
		var row thread
		if err := json.Unmarshal(raw, &row); err != nil {
			return fmt.Errorf("emails: decode thread: %w", err)
		}
		rows = append(rows, row)
	}
	out, err := json.Marshal(struct {
		Threads []json.RawMessage `json:"threads"`
	}{Threads: raws})
	if err != nil {
		return fmt.Errorf("emails: encode threads: %w", err)
	}
	return printTable(f, out, rows, threadColumns())
}

func threadListQuery(pageToken string, pageSize int, query string, labels, accounts []string, includeSpamTrash bool) url.Values {
	values := url.Values{}
	values.Set("maxResults", strconv.Itoa(pageSize))
	if pageToken != "" {
		values.Set("pageToken", pageToken)
	}
	if query != "" {
		values.Set("q", query)
	}
	for _, id := range labels {
		values.Add("labelIds", id)
	}
	for _, id := range accounts {
		values.Add("accountIds", id)
	}
	if includeSpamTrash {
		values.Set("includeSpamTrash", "true")
	}
	return values
}

func fetchThreadEtag(ctx context.Context, f *cli.Factory, id string) (json.RawMessage, error) {
	query := url.Values{}
	query.Set("format", "MINIMAL")
	resp, err := doRequest(ctx, f, http.MethodGet, itemPath("/v1/threads", id), query, nil)
	if err != nil {
		return nil, err
	}
	etag, err := resourceEtag(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("emails: thread %s: %w", id, err)
	}
	return etag, nil
}

func addsTrashLabel(body foundationinput.Object) bool {
	raw, ok := body["addLabelIds"]
	if !ok {
		return false
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return false
	}
	for _, id := range ids {
		if id == threadTrashLabel {
			return true
		}
	}
	return false
}

func setAbsentGeneratedRequestID(body foundationinput.Object, f *cli.Factory) error {
	if body.Has("requestId") {
		return nil
	}
	id, err := requestID(f, "")
	if err != nil {
		return err
	}
	return body.SetAbsent("requestId", id)
}

func setAbsentFetchedEtag(body foundationinput.Object, fetch func() (json.RawMessage, error)) error {
	if body.Has("etag") {
		return nil
	}
	etag, err := fetch()
	if err != nil {
		return err
	}
	return body.SetAbsent("etag", etag)
}

func resourceEtag(raw []byte) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decode etag: %w", err)
	}
	etag, ok := obj["etag"]
	if !ok || len(etag) == 0 || string(etag) == "null" {
		return nil, fmt.Errorf("missing etag")
	}
	return etag, nil
}

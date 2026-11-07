package emails

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
)

const messagesTestRequestID = "00000000-0000-4000-8000-000000000001"

type messagesRecorded struct {
	method, host, path, rawQuery, body string
	headers                            http.Header
}

func recordMessages(t *testing.T, status int, response []byte) (*httptest.Server, *[]messagesRecorded) {
	t.Helper()
	var got []messagesRecorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		got = append(got, messagesRecorded{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		w.WriteHeader(status)
		_, _ = w.Write(response)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func assertMessagesWire(t *testing.T, got messagesRecorded, method, path, rawQuery, body, namespace string, contentType bool) {
	t.Helper()
	if got.method != method || got.host != "emails.example.test" || got.path != path || got.rawQuery != rawQuery || got.body != body {
		t.Fatalf("request %s %s %s?%s body=%q", got.method, got.host, got.path, got.rawQuery, got.body)
	}
	assertCLIHeaders(t, got.headers, namespace)
	if got.headers.Get("User-Agent") != "sl/test" {
		t.Fatalf("User-Agent=%q", got.headers.Get("User-Agent"))
	}
	if got.headers.Get("Accept") != "application/json" {
		t.Fatalf("Accept=%q", got.headers.Get("Accept"))
	}
	if contentType {
		if got.headers.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type=%q", got.headers.Get("Content-Type"))
		}
		return
	}
	if got.headers.Get("Content-Type") != "" {
		t.Fatalf("Content-Type=%q", got.headers.Get("Content-Type"))
	}
}

func TestMessagesHelpStatesUserCredentialAndExcludesCommands(t *testing.T) {
	f := &cli.Factory{IO: &cli.IOStreams{Out: io.Discard, Err: io.Discard, In: bytes.NewReader(nil)}}
	cmd := newMessagesCommand(f)
	if strings.Contains(cmd.Long, "user credential") {
		t.Fatalf("long=%q", cmd.Long)
	}
	names := map[string]bool{}
	for _, child := range cmd.Commands() {
		names[child.Name()] = true
	}
	for _, name := range []string{"list", "get", "send", "modify", "trash", "untrash", "delete", "batch-modify", "batch-delete", "generate-quoted-content"} {
		if !names[name] {
			t.Fatalf("missing %s in %v", name, names)
		}
	}
	if names["drafts"] {
		t.Fatal("drafts must not be a messages child")
	}
	product := NewCommand(f)
	registered := map[string]bool{}
	for _, child := range product.Commands() {
		registered[child.Name()] = true
	}
	if !registered["messages"] || !registered["attachments"] || !registered["drafts"] {
		t.Fatalf("NewCommand missing messages/attachments/drafts: %v", registered)
	}
	drafts := map[string]bool{}
	for _, child := range product.Commands() {
		if child.Name() != "drafts" {
			continue
		}
		for _, verb := range child.Commands() {
			drafts[verb.Name()] = true
		}
	}
	for _, name := range []string{"list", "get", "create", "update", "delete", "send"} {
		if !drafts[name] {
			t.Fatalf("missing drafts %s in %v", name, drafts)
		}
	}
}

func TestMessagesListSendsQueryHeadersAndTable(t *testing.T) {
	raw := `{"messages":[{"id":"m1","threadId":"t1","accountId":"a@x.com","internalDate":"1710000000000","labelIds":["INBOX","UNREAD"],"sendState":"MESSAGE_SEND_STATE_SENT","etag":"3","snippet":"Hello","payload":{"mimeType":"text/plain"}}],"nextPageToken":""}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "messages", "list", "--query", "from:me", "--label", "INBOX", "--label", "UNREAD", "--include-spam-trash", "--account", "acc1", "--account", "acc2")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodGet, "/v1/messages", "accountIds=acc1&accountIds=acc2&includeSpamTrash=true&labelIds=INBOX&labelIds=UNREAD&maxResults=10&q=from%3Ame", "", "", false)
	if out != "m1\tt1\ta@x.com\t1710000000000\tINBOX,UNREAD\tMESSAGE_SEND_STATE_SENT\t3\tHello\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMessagesListPaginatesDecreasingMaxResults(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/messages" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		assertCLIHeaders(t, r.Header, "ns-1")
		queries = append(queries, r.URL.RawQuery)
		switch r.URL.Query().Get("pageToken") {
		case "":
			_, _ = w.Write([]byte(`{"messages":[{"id":"m1","threadId":"t1","accountId":"a","internalDate":"1","labelIds":["INBOX"],"sendState":"MESSAGE_SEND_STATE_SENT","etag":"1","snippet":"one"},{"id":"m2","threadId":"t1","accountId":"a","internalDate":"2","labelIds":["INBOX"],"sendState":"MESSAGE_SEND_STATE_QUEUED","etag":"2","snippet":"two"}],"nextPageToken":"p2"}`))
		case "p2":
			_, _ = w.Write([]byte(`{"messages":[{"id":"m3","threadId":"t2","accountId":"b","internalDate":"3","labelIds":["SENT"],"sendState":"MESSAGE_SEND_STATE_FAILED","etag":"3","snippet":"three"},{"id":"m4","threadId":"t2","accountId":"b","internalDate":"4","labelIds":["SENT"],"sendState":"MESSAGE_SEND_STATE_SENT","etag":"4","snippet":"four"}]}`))
		default:
			t.Errorf("token %q", r.URL.Query().Get("pageToken"))
		}
	}))
	t.Cleanup(srv.Close)
	out, err := executeEmails(t, testClient(t, srv, "ns-1"), nil, nil, "emails", "messages", "list", "--limit", "3")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(queries) != 2 || queries[0] != "maxResults=3" || queries[1] != "maxResults=1&pageToken=p2" {
		t.Fatalf("queries %#v", queries)
	}
	want := "m1\tt1\ta\t1\tINBOX\tMESSAGE_SEND_STATE_SENT\t1\tone\nm2\tt1\ta\t2\tINBOX\tMESSAGE_SEND_STATE_QUEUED\t2\ttwo\nm3\tt2\tb\t3\tSENT\tMESSAGE_SEND_STATE_FAILED\t3\tthree\n"
	if out != want {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMessagesListJSONUsesCollectedMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"messages":[{"id":"m1","payload":{"raw":"omit-in-table"}}],"nextPageToken":"n2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"messages":[{"id":"m2"}]}`))
	}))
	t.Cleanup(srv.Close)
	out, err := executeEmailsJSON(t, testClient(t, srv, ""), nil, nil, true, "emails", "messages", "list", "--limit", "2")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != `{"messages":[{"id":"m1","payload":{"raw":"omit-in-table"}},{"id":"m2"}]}` {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMessagesListRejectsNegativeLimit(t *testing.T) {
	_, err := executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, nil, "emails", "messages", "list", "--limit", "-1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--limit") {
		t.Fatalf("err=%v", err)
	}
}

func TestMessagesGetFormatMetadataHeadersAndTable(t *testing.T) {
	raw := `{"id":"m/1","threadId":"t1","accountId":"a@x.com","internalDate":"9","labelIds":["INBOX"],"sendState":"MESSAGE_SEND_STATE_SENT","etag":4,"snippet":"Hi","payload":{"parts":[{"mimeType":"text/plain"}]}}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	out, err := executeEmails(t, testClient(t, srv, "ns-2"), nil, nil, "emails", "messages", "get", "m/1", "--format", "METADATA", "--metadata-header", "From", "--metadata-header", "To")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodGet, "/v1/messages/m%2F1", "format=METADATA&metadataHeaders=From&metadataHeaders=To", "", "ns-2", false)
	if out != "m/1\tt1\ta@x.com\t9\tINBOX\tMESSAGE_SEND_STATE_SENT\t4\tHi\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMessagesGetDefaultFormatFULLAndJSON(t *testing.T) {
	raw := []byte(`{"id":"m1","threadId":"t1","snippet":"Hi","payload":{"mimeType":"multipart/mixed"}}`)
	srv, got := recordMessages(t, http.StatusOK, raw)
	out, err := executeEmailsJSON(t, testClient(t, srv, ""), nil, nil, true, "emails", "messages", "get", "m1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodGet, "/v1/messages/m1", "format=FULL", "", "", false)
	if out != string(raw) {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMessagesGetRejectsInvalidFormat(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	_, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "messages", "get", "m1", "--format", "minimal")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "MINIMAL") || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func TestMessagesDeleteFetchesETagThenDeletes(t *testing.T) {
	var got []messagesRecorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = append(got, messagesRecorded{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"m1","etag":"7"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	var prompt string
	out, err := executeEmails(t, testClient(t, srv, ""), nil, func(got string) error {
		prompt = got
		return nil
	}, "emails", "messages", "delete", "m1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete message m1?" || out != "" || len(got) != 2 {
		t.Fatalf("prompt=%q out=%q hits=%d", prompt, out, len(got))
	}
	assertMessagesWire(t, got[0], http.MethodGet, "/v1/messages/m1", "format=MINIMAL", "", "", false)
	assertMessagesWire(t, got[1], http.MethodDelete, "/v1/messages/m1", "etag=7&requestId="+messagesTestRequestID, "", "", false)
}

func TestMessagesDeleteUsesProvidedETagAndRequestID(t *testing.T) {
	srv, got := recordMessages(t, http.StatusOK, nil)
	out, err := executeEmails(t, testClient(t, srv, "ns-3"), nil, nil, "emails", "messages", "delete", "m1", "--etag", "9", "--request-id", "rid-1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != "" || len(*got) != 1 {
		t.Fatalf("out=%q hits=%d", out, len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodDelete, "/v1/messages/m1", "etag=9&requestId=rid-1", "", "ns-3", false)
}

func TestMessagesDeleteConfirmDeclinedAndMissingYes(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	_, err := executeEmails(t, testClient(t, srv, ""), nil, func(string) error {
		return cli.ErrCancelled
	}, "emails", "messages", "delete", "m1")
	if !errors.Is(err, cli.ErrCancelled) || hits != 0 {
		t.Fatalf("declined err=%v hits=%d", err, hits)
	}
	_, err = executeEmails(t, testClient(t, srv, ""), nil, func(string) error {
		return &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	}, "emails", "messages", "delete", "m1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || hits != 0 {
		t.Fatalf("non-tty err=%v hits=%d", err, hits)
	}
}

func TestMessagesGenerateQuotedInjectsReplyTo(t *testing.T) {
	srv, got := recordMessages(t, http.StatusOK, []byte(`{"quotedBody":"quoted"}`))
	input := writeInput(t, `{"quoteType":"reply","userBody":"thanks"}`)
	out, err := executeEmailsJSON(t, testClient(t, srv, ""), nil, nil, true, "emails", "messages", "generate-quoted-content", "m1", "--input", input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != `{"quotedBody":"quoted"}` || len(*got) != 1 {
		t.Fatalf("out=%q hits=%d", out, len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodPost, "/v1/messages:generateQuotedContent", "", `{"quoteType":"reply","replyToMessageId":"m1","userBody":"thanks"}`, "", true)
}

func TestMessagesGenerateQuotedReadsStdinAndKeepsMatchingID(t *testing.T) {
	srv, got := recordMessages(t, http.StatusOK, []byte(`{"quotedBody":"ok"}`))
	out, err := executeEmailsJSON(t, testClient(t, srv, ""), strings.NewReader(`{"quoteType":"forward","replyToMessageId":"m1"}`), nil, true, "emails", "messages", "generate-quoted-content", "m1", "--input", "-")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != `{"quotedBody":"ok"}` || len(*got) != 1 {
		t.Fatalf("out=%q hits=%d", out, len(*got))
	}
	var body map[string]any
	if err := json.Unmarshal([]byte((*got)[0].body), &body); err != nil {
		t.Fatal(err)
	}
	if body["quoteType"] != "forward" || body["replyToMessageId"] != "m1" {
		t.Fatalf("body=%s", (*got)[0].body)
	}
}

func TestMessagesGenerateQuotedRejectsConflictAndRequiresInput(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	input := writeInput(t, `{"replyToMessageId":"other","quoteType":"reply"}`)
	_, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "messages", "generate-quoted-content", "m1", "--input", input)
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "replyToMessageId") || hits != 0 {
		t.Fatalf("conflict err=%v hits=%d", err, hits)
	}
	_, err = executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "messages", "generate-quoted-content", "m1")
	if !errors.As(err, &usage) || usage.Msg != "--input is required" || hits != 0 {
		t.Fatalf("missing input err=%v hits=%d", err, hits)
	}
}

func TestAttachmentsGetNestedPathAndTable(t *testing.T) {
	raw := []byte(`{"attachmentId":"att/1","size":12,"data":"dGVzdA=="}`)
	srv, got := recordMessages(t, http.StatusOK, raw)
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "attachments", "get", "att/1", "--message", "m/1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodGet, "/v1/messages/m%2F1/attachments/att%2F1", "", "", "", false)
	if out != "att/1\t12\tdGVzdA==\n" {
		t.Fatalf("stdout=%q", out)
	}
	jsonOut, err := executeEmailsJSON(t, testClient(t, srv, "ns-4"), nil, nil, true, "emails", "attachments", "get", "att/1", "--message", "m/1")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if jsonOut != string(raw) {
		t.Fatalf("json=%q", jsonOut)
	}
	assertMessagesWire(t, (*got)[1], http.MethodGet, "/v1/messages/m%2F1/attachments/att%2F1", "", "", "ns-4", false)
}

func TestAttachmentsGetRequiresMessage(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	_, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "attachments", "get", "att-1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || usage.Msg != "--message is required" || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func TestMessagesSendMergesGeneratedRequestID(t *testing.T) {
	raw := `{"id":"m1","threadId":"t1","accountId":"a@x.com","internalDate":"1","labelIds":["SENT"],"sendState":"MESSAGE_SEND_STATE_QUEUED","etag":"1","snippet":"Hi"}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	input := writeInput(t, `{"message":{"accountId":"a@x.com","raw":"YQ"}}`)
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "messages", "send", "--input", input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodPost, "/v1/messages:send", "", `{"message":{"accountId":"a@x.com","raw":"YQ"},"requestId":"`+messagesTestRequestID+`"}`, "", true)
	if out != "m1\tt1\ta@x.com\t1\tSENT\tMESSAGE_SEND_STATE_QUEUED\t1\tHi\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMessagesSendFlagInputConflictAndRequiresInput(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	_, err := executeEmails(t, testClient(t, srv, ""), strings.NewReader(`{"message":{"raw":"YQ"},"requestId":"from-input"}`), nil, "emails", "messages", "send", "--input", "-", "--request-id", "rid-1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--request-id") || !strings.Contains(usage.Msg, "requestId") || hits != 0 {
		t.Fatalf("conflict err=%v hits=%d", err, hits)
	}
	_, err = executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "messages", "send")
	if !errors.As(err, &usage) || usage.Msg != "--input is required" || hits != 0 {
		t.Fatalf("missing input err=%v hits=%d", err, hits)
	}
}

func TestMessagesModifyLabelsSkipsEtagFetch(t *testing.T) {
	raw := `{"id":"m/1","threadId":"t1","accountId":"a","internalDate":"1","labelIds":["STARRED"],"sendState":"MESSAGE_SEND_STATE_SENT","etag":"2","snippet":"Hi"}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	input := writeInput(t, `{"addLabelIds":["STARRED"]}`)
	out, err := executeEmails(t, testClient(t, srv, "ns-5"), nil, nil, "emails", "messages", "modify", "m/1", "--input", input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodPost, "/v1/messages/m%2F1:modify", "", `{"addLabelIds":["STARRED"]}`, "ns-5", true)
	if out != "m/1\tt1\ta\t1\tSTARRED\tMESSAGE_SEND_STATE_SENT\t2\tHi\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMessagesModifyScheduleTimeFetchesEtagAndRequestID(t *testing.T) {
	var got []messagesRecorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = append(got, messagesRecorded{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"m1","etag":"7"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"m1","threadId":"t1","accountId":"a","internalDate":"1","labelIds":["SCHEDULED"],"sendState":"MESSAGE_SEND_STATE_QUEUED","etag":"8","snippet":"later"}`))
	}))
	t.Cleanup(srv.Close)
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "messages", "modify", "m1", "--input", writeInput(t, `{"scheduleTime":"1767225600000"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("hits=%d", len(got))
	}
	assertMessagesWire(t, got[0], http.MethodGet, "/v1/messages/m1", "format=MINIMAL", "", "", false)
	assertMessagesWire(t, got[1], http.MethodPost, "/v1/messages/m1:modify", "", `{"etag":"7","requestId":"`+messagesTestRequestID+`","scheduleTime":"1767225600000"}`, "", true)
	if out != "m1\tt1\ta\t1\tSCHEDULED\tMESSAGE_SEND_STATE_QUEUED\t8\tlater\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMessagesTrashConfirmQueryAndDecline(t *testing.T) {
	var got []messagesRecorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = append(got, messagesRecorded{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"m1","etag":"7"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"m1","threadId":"t1","accountId":"a","internalDate":"1","labelIds":["TRASH"],"sendState":"MESSAGE_SEND_STATE_SENT","etag":"8","snippet":"bye"}`))
	}))
	t.Cleanup(srv.Close)
	var prompt string
	out, err := executeEmails(t, testClient(t, srv, ""), nil, func(got string) error {
		prompt = got
		return nil
	}, "emails", "messages", "trash", "m1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Trash message m1?" || len(got) != 2 {
		t.Fatalf("prompt=%q hits=%d", prompt, len(got))
	}
	assertMessagesWire(t, got[0], http.MethodGet, "/v1/messages/m1", "format=MINIMAL", "", "", false)
	assertMessagesWire(t, got[1], http.MethodPost, "/v1/messages/m1:trash", "etag=7&requestId="+messagesTestRequestID, "", "", false)
	if out != "m1\tt1\ta\t1\tTRASH\tMESSAGE_SEND_STATE_SENT\t8\tbye\n" {
		t.Fatalf("stdout=%q", out)
	}
	hits := 0
	deny := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(deny.Close)
	_, err = executeEmails(t, testClient(t, deny, ""), nil, func(string) error {
		return cli.ErrCancelled
	}, "emails", "messages", "trash", "m1")
	if !errors.Is(err, cli.ErrCancelled) || hits != 0 {
		t.Fatalf("declined err=%v hits=%d", err, hits)
	}
}

func TestMessagesUntrashPostsNoBody(t *testing.T) {
	raw := `{"id":"m/1","threadId":"t1","accountId":"a","internalDate":"1","labelIds":["INBOX"],"sendState":"MESSAGE_SEND_STATE_SENT","etag":"2","snippet":"back"}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "messages", "untrash", "m/1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodPost, "/v1/messages/m%2F1:untrash", "", "", "", false)
	if out != "m/1\tt1\ta\t1\tINBOX\tMESSAGE_SEND_STATE_SENT\t2\tback\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMessagesBatchModifyPassesInput(t *testing.T) {
	srv, got := recordMessages(t, http.StatusOK, []byte(`{}`))
	input := writeInput(t, `{"ids":["m1","m2"],"addLabelIds":["STARRED"]}`)
	out, err := executeEmailsJSON(t, testClient(t, srv, ""), nil, nil, true, "emails", "messages", "batch-modify", "--input", input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != `{}` || len(*got) != 1 {
		t.Fatalf("out=%q hits=%d", out, len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodPost, "/v1/messages:batchModify", "", `{"addLabelIds":["STARRED"],"ids":["m1","m2"]}`, "", true)
}

func TestMessagesBatchDeleteConfirmAndGeneratedRequestID(t *testing.T) {
	srv, got := recordMessages(t, http.StatusOK, []byte(`{}`))
	var prompt string
	out, err := executeEmailsJSON(t, testClient(t, srv, ""), nil, func(got string) error {
		prompt = got
		return nil
	}, true, "emails", "messages", "batch-delete", "--input", writeInput(t, `{"ids":["m1","m2"]}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete messages in batch?" || out != `{}` || len(*got) != 1 {
		t.Fatalf("prompt=%q out=%q hits=%d", prompt, out, len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodPost, "/v1/messages:batchDelete", "", `{"ids":["m1","m2"],"requestId":"`+messagesTestRequestID+`"}`, "", true)
	hits := 0
	deny := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(deny.Close)
	_, err = executeEmails(t, testClient(t, deny, ""), nil, func(string) error {
		return cli.ErrCancelled
	}, "emails", "messages", "batch-delete", "--input", writeInput(t, `{"ids":["m1"]}`))
	if !errors.Is(err, cli.ErrCancelled) || hits != 0 {
		t.Fatalf("declined err=%v hits=%d", err, hits)
	}
}

package emails

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
)

func TestDraftsListSendsQueryHeadersAndTable(t *testing.T) {
	raw := `{"drafts":[{"id":"d1","message":{"id":"m1","accountId":"a@x.com","etag":"3","snippet":"Hello"}}],"nextPageToken":""}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "drafts", "list", "--query", "from:me", "--include-spam-trash")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodGet, "/v1/drafts", "includeSpamTrash=true&maxResults=10&q=from%3Ame", "", "", false)
	if out != "d1\tm1\ta@x.com\t3\tHello\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDraftsListPaginatesAndOmitsAccountIds(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/drafts" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if _, ok := r.URL.Query()["accountIds"]; ok {
			t.Errorf("accountIds present: %s", r.URL.RawQuery)
		}
		queries = append(queries, r.URL.RawQuery)
		switch r.URL.Query().Get("pageToken") {
		case "":
			_, _ = w.Write([]byte(`{"drafts":[{"id":"d1","message":{"id":"m1","accountId":"a","etag":"1","snippet":"one"}},{"id":"d2","message":{"id":"m2","accountId":"a","etag":"2","snippet":"two"}}],"nextPageToken":"p2"}`))
		case "p2":
			_, _ = w.Write([]byte(`{"drafts":[{"id":"d3","message":{"id":"m3","accountId":"b","etag":"3","snippet":"three"}},{"id":"d4","message":{"id":"m4","accountId":"b","etag":"4","snippet":"four"}}]}`))
		default:
			t.Errorf("token %q", r.URL.Query().Get("pageToken"))
		}
	}))
	t.Cleanup(srv.Close)
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "drafts", "list", "--limit", "3")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(queries) != 2 || queries[0] != "maxResults=3" || queries[1] != "maxResults=1&pageToken=p2" {
		t.Fatalf("queries %#v", queries)
	}
	want := "d1\tm1\ta\t1\tone\nd2\tm2\ta\t2\ttwo\nd3\tm3\tb\t3\tthree\n"
	if out != want {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDraftsListJSONUsesCollectedDrafts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"drafts":[{"id":"d1","message":{"raw":"omit"}}],"nextPageToken":"n2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"drafts":[{"id":"d2"}]}`))
	}))
	t.Cleanup(srv.Close)
	out, err := executeEmailsJSON(t, testClient(t, srv, ""), nil, nil, true, "emails", "drafts", "list", "--limit", "2")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != `{"drafts":[{"id":"d1","message":{"raw":"omit"}},{"id":"d2"}]}` {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDraftsListRejectsNegativeLimit(t *testing.T) {
	_, err := executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, nil, "emails", "drafts", "list", "--limit", "-1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--limit") {
		t.Fatalf("err=%v", err)
	}
}

func TestDraftsGetFormatPathEscapeAndTable(t *testing.T) {
	raw := `{"id":"d/1","message":{"id":"m/1","accountId":"a@x.com","etag":4,"snippet":"Hi"}}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	out, err := executeEmails(t, testClient(t, srv, "ns-2"), nil, nil, "emails", "drafts", "get", "d/1", "--format", "METADATA")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodGet, "/v1/drafts/d%2F1", "format=METADATA", "", "ns-2", false)
	if out != "d/1\tm/1\ta@x.com\t4\tHi\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDraftsGetDefaultFormatFULLAndJSON(t *testing.T) {
	raw := []byte(`{"id":"d1","message":{"id":"m1","snippet":"Hi","payload":{"mimeType":"text/plain"}}}`)
	srv, got := recordMessages(t, http.StatusOK, raw)
	out, err := executeEmailsJSON(t, testClient(t, srv, ""), nil, nil, true, "emails", "drafts", "get", "d1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodGet, "/v1/drafts/d1", "format=FULL", "", "", false)
	if out != string(raw) {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDraftsGetRejectsInvalidFormat(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	_, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "drafts", "get", "d1", "--format", "minimal")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "MINIMAL") || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func TestDraftsCreatePassesRequestObject(t *testing.T) {
	raw := `{"id":"d1","message":{"id":"m1","accountId":"a@x.com","etag":"1","snippet":"draft"}}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	input := writeInput(t, `{"draft":{"message":{"accountId":"a@x.com","raw":"YQ"}}}`)
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "drafts", "create", "--input", input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodPost, "/v1/drafts", "", `{"draft":{"message":{"accountId":"a@x.com","raw":"YQ"}}}`, "", true)
	if out != "d1\tm1\ta@x.com\t1\tdraft\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDraftsUpdatePathEscapeAndInput(t *testing.T) {
	raw := `{"id":"d/1","message":{"id":"m1","accountId":"a","etag":"2","snippet":"upd"}}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	out, err := executeEmails(t, testClient(t, srv, ""), strings.NewReader(`{"draft":{"message":{"raw":"YWI"}}}`), nil, "emails", "drafts", "update", "d/1", "--input", "-")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodPut, "/v1/drafts/d%2F1", "", `{"draft":{"message":{"raw":"YWI"}}}`, "", true)
	if out != "d/1\tm1\ta\t2\tupd\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDraftsSendMergesGeneratedRequestIDAndPrintsMessage(t *testing.T) {
	raw := `{"id":"m1","threadId":"t1","accountId":"a@x.com","internalDate":"9","labelIds":["SENT"],"sendState":"MESSAGE_SEND_STATE_QUEUED","etag":"3","snippet":"sent"}`
	srv, got := recordMessages(t, http.StatusOK, []byte(raw))
	input := writeInput(t, `{"draft":{"id":"d1"}}`)
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "drafts", "send", "--input", input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodPost, "/v1/drafts:send", "", `{"draft":{"id":"d1"},"requestId":"`+messagesTestRequestID+`"}`, "", true)
	if out != "m1\tt1\ta@x.com\t9\tSENT\tMESSAGE_SEND_STATE_QUEUED\t3\tsent\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDraftsSendFlagInputConflict(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	_, err := executeEmails(t, testClient(t, srv, ""), strings.NewReader(`{"draft":{"id":"d1"},"requestId":"from-input"}`), nil, "emails", "drafts", "send", "--input", "-", "--request-id", "rid-1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--request-id") || !strings.Contains(usage.Msg, "requestId") || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func TestDraftsCreateUpdateSendRequireInput(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	client := testClient(t, srv, "")
	for _, args := range [][]string{
		{"emails", "drafts", "create"},
		{"emails", "drafts", "update", "d1"},
		{"emails", "drafts", "send"},
	} {
		_, err := executeEmails(t, client, nil, nil, args...)
		var usage *cli.UsageError
		if !errors.As(err, &usage) || usage.Msg != "--input is required" || hits != 0 {
			t.Fatalf("%v err=%v hits=%d", args, err, hits)
		}
	}
}

func TestDraftsDeleteFetchesNestedEtagAndQuery(t *testing.T) {
	var got []messagesRecorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = append(got, messagesRecorded{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"d/1","message":{"id":"m1","accountId":"a","etag":"7","snippet":"x"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	var prompt string
	out, err := executeEmails(t, testClient(t, srv, ""), nil, func(got string) error {
		prompt = got
		return nil
	}, "emails", "drafts", "delete", "d/1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete draft d/1?" || out != "" || len(got) != 2 {
		t.Fatalf("prompt=%q out=%q hits=%d", prompt, out, len(got))
	}
	assertMessagesWire(t, got[0], http.MethodGet, "/v1/drafts/d%2F1", "format=MINIMAL", "", "", false)
	assertMessagesWire(t, got[1], http.MethodDelete, "/v1/drafts/d%2F1", "etag=7&requestId="+messagesTestRequestID, "", "", false)
}

func TestDraftsDeleteUsesProvidedETagAndRequestID(t *testing.T) {
	srv, got := recordMessages(t, http.StatusOK, nil)
	out, err := executeEmails(t, testClient(t, srv, "ns-3"), nil, nil, "emails", "drafts", "delete", "d1", "--etag", "9", "--request-id", "rid-1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != "" || len(*got) != 1 {
		t.Fatalf("out=%q hits=%d", out, len(*got))
	}
	assertMessagesWire(t, (*got)[0], http.MethodDelete, "/v1/drafts/d1", "etag=9&requestId=rid-1", "", "ns-3", false)
}

func TestDraftsDeleteConfirmDeclined(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	_, err := executeEmails(t, testClient(t, srv, ""), nil, func(string) error {
		return cli.ErrCancelled
	}, "emails", "drafts", "delete", "d1")
	if !errors.Is(err, cli.ErrCancelled) || hits != 0 {
		t.Fatalf("declined err=%v hits=%d", err, hits)
	}
}

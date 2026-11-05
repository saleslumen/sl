package emails

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/spf13/cobra"
)

type tlCall struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
	Header http.Header
}

func tlRecord(t *testing.T, handler func(http.ResponseWriter, *http.Request, int)) (*[]tlCall, *httptest.Server) {
	t.Helper()
	calls := &[]tlCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		*calls = append(*calls, tlCall{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: string(raw), Header: r.Header.Clone()})
		handler(w, r, len(*calls)-1)
	}))
	t.Cleanup(srv.Close)
	return calls, srv
}

func tlWriteOK(w http.ResponseWriter, body string) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func tlAssertJSON(t *testing.T, got, want string) {
	t.Helper()
	if strings.TrimSpace(got) == "" && strings.TrimSpace(want) == "" {
		return
	}
	var a, b any
	if err := json.Unmarshal([]byte(got), &a); err != nil {
		t.Fatalf("got body %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &b); err != nil {
		t.Fatalf("want body %q: %v", want, err)
	}
	if !tlJSONEqual(a, b) {
		t.Fatalf("body %s want %s", got, want)
	}
}

func tlJSONEqual(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

func tlAssertQuery(t *testing.T, got url.Values, want url.Values) {
	t.Helper()
	if got.Encode() != want.Encode() {
		t.Fatalf("query %q want %q", got.Encode(), want.Encode())
	}
}

func TestThreadsAndLabelsRegistered(t *testing.T) {
	cmd := NewCommand(&cli.Factory{IO: &cli.IOStreams{Out: io.Discard}})
	names := map[string]*cobra.Command{}
	for _, child := range cmd.Commands() {
		names[child.Name()] = child
	}
	if names["threads"] == nil || names["labels"] == nil {
		t.Fatalf("commands=%v", names)
	}
	threads := map[string]bool{}
	for _, child := range names["threads"].Commands() {
		threads[child.Name()] = true
	}
	for _, name := range []string{"list", "get", "modify", "delete", "trash", "untrash"} {
		if !threads[name] {
			t.Fatalf("missing threads %s", name)
		}
	}
	labels := map[string]bool{}
	for _, child := range names["labels"].Commands() {
		labels[child.Name()] = true
	}
	for _, name := range []string{"list", "get", "create", "update", "delete"} {
		if !labels[name] {
			t.Fatalf("missing labels %s", name)
		}
	}
}

func TestThreadsListQueryHeadersAndTable(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"threads":[{"id":"t1","snippet":"hello","historyId":"h1","etag":"3","messages":[{},{}]}]}`)
	})
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "threads", "list", "--query", "from:ada", "--label", "INBOX", "--label", "STARRED", "--account", "acc-1", "--include-spam-trash")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("calls=%d", len(*calls))
	}
	if (*calls)[0].Method != http.MethodGet || (*calls)[0].Path != "/v1/threads" {
		t.Fatalf("request %s %s", (*calls)[0].Method, (*calls)[0].Path)
	}
	tlAssertQuery(t, (*calls)[0].Query, url.Values{"maxResults": []string{"10"}, "q": []string{"from:ada"}, "labelIds": []string{"INBOX", "STARRED"}, "accountIds": []string{"acc-1"}, "includeSpamTrash": []string{"true"}})
	assertCLIHeaders(t, (*calls)[0].Header, "")
	if (*calls)[0].Body != "" {
		t.Fatalf("body=%q", (*calls)[0].Body)
	}
	if out != "t1\t2\th1\t3\thello\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestThreadsListPaginationDecreasesMaxResults(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, i int) {
		switch i {
		case 0:
			tlWriteOK(w, `{"threads":[{"id":"t1","snippet":"a","historyId":"h1","etag":1,"messages":[]},{"id":"t2","snippet":"b","historyId":"h2","etag":2,"messages":[{}]}],"nextPageToken":"p2"}`)
		default:
			tlWriteOK(w, `{"threads":[{"id":"t3","snippet":"c","historyId":"h3","etag":3,"messages":[]},{"id":"t4","snippet":"d","historyId":"h4","etag":4,"messages":[]}]}`)
		}
	})
	out, err := executeEmails(t, testClient(t, srv, "ns-1"), nil, nil, "emails", "threads", "list", "--limit", "3")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("calls=%d", len(*calls))
	}
	tlAssertQuery(t, (*calls)[0].Query, url.Values{"maxResults": []string{"3"}})
	tlAssertQuery(t, (*calls)[1].Query, url.Values{"maxResults": []string{"1"}, "pageToken": []string{"p2"}})
	assertCLIHeaders(t, (*calls)[0].Header, "ns-1")
	assertCLIHeaders(t, (*calls)[1].Header, "ns-1")
	if out != "t1\t0\th1\t1\ta\nt2\t1\th2\t2\tb\nt3\t0\th3\t3\tc\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestThreadsListJSONUsesCollectedThreads(t *testing.T) {
	_, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"threads":[{"id":"t1","snippet":"a","historyId":"h1","etag":"1","messages":[]}],"nextPageToken":""}`)
	})
	out, err := executeEmailsJSON(t, testClient(t, srv, ""), nil, nil, true, "emails", "threads", "list")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	tlAssertJSON(t, out, `{"threads":[{"id":"t1","snippet":"a","historyId":"h1","etag":"1","messages":[]}]}`)
}

func TestThreadsGetFormatAndMetadataHeaders(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"id":"t1","snippet":"hello","historyId":"h1","etag":"3","messages":[{},{}]}`)
	})
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "threads", "get", "t1", "--format", "METADATA", "--metadata-header", "From", "--metadata-header", "To")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if (*calls)[0].Method != http.MethodGet || (*calls)[0].Path != "/v1/threads/t1" {
		t.Fatalf("request %s %s", (*calls)[0].Method, (*calls)[0].Path)
	}
	tlAssertQuery(t, (*calls)[0].Query, url.Values{"format": []string{"METADATA"}, "metadataHeaders": []string{"From", "To"}})
	assertCLIHeaders(t, (*calls)[0].Header, "")
	if out != "t1\t2\th1\t3\thello\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestThreadsGetRejectsInvalidFormat(t *testing.T) {
	_, err := executeEmails(t, testClient(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request")
	})), ""), nil, nil, "emails", "threads", "get", "t1", "--format", "WIDE")
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("usage: %v", err)
	}
}

func TestThreadsModifyPassThroughBody(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"id":"t1","snippet":"hello","historyId":"h1","etag":"5","messages":[]}`)
	})
	out, err := executeEmails(t, testClient(t, srv, ""), nil, func(string) error {
		t.Fatal("confirm")
		return nil
	}, "emails", "threads", "modify", "t1", "--input", writeInput(t, `{"addLabelIds":["STARRED"],"removeLabelIds":["UNREAD"]}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0].Method != http.MethodPost || (*calls)[0].Path != "/v1/threads/t1:modify" {
		t.Fatalf("request %#v", calls)
	}
	if len((*calls)[0].Query) != 0 {
		t.Fatalf("query=%v", (*calls)[0].Query)
	}
	tlAssertJSON(t, (*calls)[0].Body, `{"addLabelIds":["STARRED"],"removeLabelIds":["UNREAD"]}`)
	if (*calls)[0].Header.Get("Content-Type") != "application/json" {
		t.Fatalf("content-type=%q", (*calls)[0].Header.Get("Content-Type"))
	}
	if out != "t1\t0\th1\t5\thello\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestThreadsModifyTrashFetchesEtagAndGeneratesRequestID(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, i int) {
		switch i {
		case 0:
			if r.Method != http.MethodGet || r.URL.Path != "/v1/threads/t1" {
				t.Errorf("fetch %s %s", r.Method, r.URL.Path)
			}
			tlWriteOK(w, `{"id":"t1","etag":"4"}`)
		default:
			tlWriteOK(w, `{"id":"t1","snippet":"gone","historyId":"h9","etag":"5","messages":[]}`)
		}
	})
	confirmed := false
	out, err := executeEmails(t, testClient(t, srv, ""), nil, func(string) error {
		confirmed = true
		return nil
	}, "emails", "threads", "modify", "t1", "--input", writeInput(t, `{"addLabelIds":["TRASH"]}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if confirmed {
		t.Fatal("modify confirmed")
	}
	if len(*calls) != 2 {
		t.Fatalf("calls=%d", len(*calls))
	}
	tlAssertQuery(t, (*calls)[0].Query, url.Values{"format": []string{"MINIMAL"}})
	if (*calls)[1].Method != http.MethodPost || (*calls)[1].Path != "/v1/threads/t1:modify" {
		t.Fatalf("mutation %s %s", (*calls)[1].Method, (*calls)[1].Path)
	}
	tlAssertJSON(t, (*calls)[1].Body, `{"addLabelIds":["TRASH"],"requestId":"00000000-0000-4000-8000-000000000001","etag":"4"}`)
	if out != "t1\t0\th9\t5\tgone\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestThreadsModifyTrashUsesExplicitControlsWithoutFetch(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method == http.MethodGet {
			t.Error("unexpected get")
		}
		tlWriteOK(w, `{"id":"t1","snippet":"x","historyId":"h","etag":"9","messages":[]}`)
	})
	_, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "threads", "modify", "t1", "--request-id", "req-1", "--etag", "8", "--input", writeInput(t, `{"addLabelIds":["TRASH"]}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("calls=%d", len(*calls))
	}
	tlAssertJSON(t, (*calls)[0].Body, `{"addLabelIds":["TRASH"],"requestId":"req-1","etag":"8"}`)
}

func TestThreadsModifyTrashKeepsInputControls(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method == http.MethodGet {
			t.Error("unexpected get")
		}
		tlWriteOK(w, `{"id":"t1","snippet":"x","historyId":"h","etag":"9","messages":[]}`)
	})
	_, err := executeEmails(t, testClient(t, srv, ""), strings.NewReader(`{"addLabelIds":["TRASH"],"requestId":"from-input","etag":7}`), nil, "emails", "threads", "modify", "t1", "--input", "-")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	tlAssertJSON(t, (*calls)[0].Body, `{"addLabelIds":["TRASH"],"requestId":"from-input","etag":7}`)
}

func TestThreadsModifyTrashRejectsDuplicateRequestID(t *testing.T) {
	_, err := executeEmails(t, testClient(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request")
	})), ""), nil, nil, "emails", "threads", "modify", "t1", "--request-id", "req-1", "--input", writeInput(t, `{"addLabelIds":["TRASH"],"requestId":"other"}`))
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("usage: %v", err)
	}
}

func TestThreadsDeleteFetchesEtagAndSendsQuery(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, i int) {
		if i == 0 {
			tlWriteOK(w, `{"id":"t1","etag":4}`)
			return
		}
		tlWriteOK(w, "")
	})
	var prompts []string
	out, err := executeEmails(t, testClient(t, srv, ""), nil, func(prompt string) error {
		prompts = append(prompts, prompt)
		return nil
	}, "emails", "threads", "delete", "t1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(prompts) != 1 || prompts[0] != "Delete thread t1?" {
		t.Fatalf("prompts=%v", prompts)
	}
	if len(*calls) != 2 {
		t.Fatalf("calls=%d", len(*calls))
	}
	if (*calls)[0].Method != http.MethodGet || (*calls)[0].Path != "/v1/threads/t1" {
		t.Fatalf("fetch %s %s", (*calls)[0].Method, (*calls)[0].Path)
	}
	tlAssertQuery(t, (*calls)[0].Query, url.Values{"format": []string{"MINIMAL"}})
	if (*calls)[1].Method != http.MethodDelete || (*calls)[1].Path != "/v1/threads/t1" {
		t.Fatalf("delete %s %s", (*calls)[1].Method, (*calls)[1].Path)
	}
	tlAssertQuery(t, (*calls)[1].Query, url.Values{"requestId": []string{"00000000-0000-4000-8000-000000000001"}, "etag": []string{"4"}})
	if (*calls)[1].Body != "" || (*calls)[1].Header.Get("Content-Type") != "" {
		t.Fatalf("delete body=%q content-type=%q", (*calls)[1].Body, (*calls)[1].Header.Get("Content-Type"))
	}
	assertCLIHeaders(t, (*calls)[1].Header, "")
	if out != "" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestThreadsDeleteUsesFlagEtagWithoutFetch(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method == http.MethodGet {
			t.Error("unexpected get")
		}
		tlWriteOK(w, "")
	})
	_, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "threads", "delete", "t1", "--request-id", "req-9", "--etag", "6")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("calls=%d", len(*calls))
	}
	tlAssertQuery(t, (*calls)[0].Query, url.Values{"requestId": []string{"req-9"}, "etag": []string{"6"}})
}

func TestThreadsDeleteDeclinedSkipsRequest(t *testing.T) {
	called := false
	_, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		called = true
		tlWriteOK(w, "")
	})
	_, err := executeEmails(t, testClient(t, srv, ""), nil, func(string) error {
		return cli.ErrCancelled
	}, "emails", "threads", "delete", "t1", "--etag", "1")
	if !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("err=%v", err)
	}
	if called {
		t.Fatal("request after decline")
	}
}

func TestThreadsTrashQueryMutation(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"id":"t1","snippet":"trashed","historyId":"h","etag":"2","messages":[]}`)
	})
	var prompts []string
	out, err := executeEmails(t, testClient(t, srv, "ns-2"), nil, func(prompt string) error {
		prompts = append(prompts, prompt)
		return nil
	}, "emails", "threads", "trash", "t1", "--request-id", "req-t", "--etag", "1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(prompts) != 1 || prompts[0] != "Trash thread t1?" {
		t.Fatalf("prompts=%v", prompts)
	}
	if (*calls)[0].Method != http.MethodPost || (*calls)[0].Path != "/v1/threads/t1:trash" {
		t.Fatalf("request %s %s", (*calls)[0].Method, (*calls)[0].Path)
	}
	tlAssertQuery(t, (*calls)[0].Query, url.Values{"requestId": []string{"req-t"}, "etag": []string{"1"}})
	if (*calls)[0].Body != "" {
		t.Fatalf("body=%q", (*calls)[0].Body)
	}
	assertCLIHeaders(t, (*calls)[0].Header, "ns-2")
	if out != "t1\t0\th\t2\ttrashed\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestThreadsUntrashHasNoBody(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"id":"t1","snippet":"back","historyId":"h","etag":"3","messages":[]}`)
	})
	out, err := executeEmails(t, testClient(t, srv, ""), nil, func(string) error {
		t.Fatal("confirm")
		return nil
	}, "emails", "threads", "untrash", "t1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if (*calls)[0].Method != http.MethodPost || (*calls)[0].Path != "/v1/threads/t1:untrash" {
		t.Fatalf("request %s %s", (*calls)[0].Method, (*calls)[0].Path)
	}
	if len((*calls)[0].Query) != 0 || (*calls)[0].Body != "" || (*calls)[0].Header.Get("Content-Type") != "" {
		t.Fatalf("query=%v body=%q content-type=%q", (*calls)[0].Query, (*calls)[0].Body, (*calls)[0].Header.Get("Content-Type"))
	}
	if out != "t1\t0\th\t3\tback\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestLabelsListReturnsAllLabels(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"labels":[{"id":"INBOX","name":"INBOX","type":"SYSTEM"},{"id":"u1","name":"Q4","type":"USER","etag":"1"},{"id":"u2","name":"Q5","type":"USER","etag":"2"}]}`)
	})
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "labels", "list")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0].Method != http.MethodGet || (*calls)[0].Path != "/v1/labels" || len((*calls)[0].Query) != 0 {
		t.Fatalf("request %#v", calls)
	}
	assertCLIHeaders(t, (*calls)[0].Header, "")
	if out != "INBOX\tINBOX\tSYSTEM\t\nu1\tQ4\tUSER\t1\nu2\tQ5\tUSER\t2\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestLabelsGetTable(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"id":"INBOX","name":"INBOX","type":"SYSTEM"}`)
	})
	out, err := executeEmails(t, testClient(t, srv, "ns-9"), nil, nil, "emails", "labels", "get", "INBOX")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if (*calls)[0].Method != http.MethodGet || (*calls)[0].Path != "/v1/labels/INBOX" {
		t.Fatalf("request %s %s", (*calls)[0].Method, (*calls)[0].Path)
	}
	assertCLIHeaders(t, (*calls)[0].Header, "ns-9")
	if out != "INBOX\tINBOX\tSYSTEM\t\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestLabelsCreateNameAndInputConflict(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"id":"u1","name":"Q4 pipeline","type":"USER","etag":"1"}`)
	})
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "labels", "create", "--name", "Q4 pipeline")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if (*calls)[0].Method != http.MethodPost || (*calls)[0].Path != "/v1/labels" {
		t.Fatalf("request %s %s", (*calls)[0].Method, (*calls)[0].Path)
	}
	tlAssertJSON(t, (*calls)[0].Body, `{"label":{"name":"Q4 pipeline"}}`)
	if out != "u1\tQ4 pipeline\tUSER\t1\n" {
		t.Fatalf("stdout=%q", out)
	}
	_, err = executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "labels", "create", "--name", "Other", "--input", writeInput(t, `{"label":{"name":"Q4 pipeline"}}`))
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("conflict: %v", err)
	}
}

func TestLabelsCreateInputOnly(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, `{"id":"u1","name":"Q4","type":"USER","etag":"1"}`)
	})
	_, err := executeEmails(t, testClient(t, srv, ""), strings.NewReader(`{"label":{"name":"Q4"}}`), nil, "emails", "labels", "create", "--input", "-")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	tlAssertJSON(t, (*calls)[0].Body, `{"label":{"name":"Q4"}}`)
}

func TestLabelsUpdateFetchesStringEtag(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, i int) {
		if i == 0 {
			tlWriteOK(w, `{"id":"u1","name":"Q4","type":"USER","etag":"1"}`)
			return
		}
		tlWriteOK(w, `{"id":"u1","name":"Q5","type":"USER","etag":"2"}`)
	})
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "labels", "update", "u1", "--input", writeInput(t, `{"name":"Q5"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if (*calls)[0].Method != http.MethodGet || (*calls)[0].Path != "/v1/labels/u1" {
		t.Fatalf("fetch %s %s", (*calls)[0].Method, (*calls)[0].Path)
	}
	if (*calls)[1].Method != http.MethodPatch || (*calls)[1].Path != "/v1/labels/u1" {
		t.Fatalf("patch %s %s", (*calls)[1].Method, (*calls)[1].Path)
	}
	tlAssertQuery(t, (*calls)[1].Query, url.Values{"updateMask": []string{"name"}})
	tlAssertJSON(t, (*calls)[1].Body, `{"name":"Q5","etag":"1"}`)
	if out != "u1\tQ5\tUSER\t2\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestLabelsUpdateExplicitEtagAndMask(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		if r.Method == http.MethodGet {
			t.Error("unexpected get")
		}
		tlWriteOK(w, `{"id":"u1","name":"Q5","type":"USER","etag":"3"}`)
	})
	_, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "labels", "update", "u1", "--etag", "2", "--update-mask", "name", "--input", writeInput(t, `{"name":"Q5"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("calls=%d", len(*calls))
	}
	tlAssertQuery(t, (*calls)[0].Query, url.Values{"updateMask": []string{"name"}})
	tlAssertJSON(t, (*calls)[0].Body, `{"etag":"2","name":"Q5"}`)
}

func TestLabelsUpdateRejectsDuplicateEtag(t *testing.T) {
	_, err := executeEmails(t, testClient(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request")
	})), ""), nil, nil, "emails", "labels", "update", "u1", "--etag", "2", "--input", writeInput(t, `{"name":"Q5","etag":"1"}`))
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("usage: %v", err)
	}
}

func TestLabelsDeleteConfirms(t *testing.T) {
	calls, srv := tlRecord(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		tlWriteOK(w, "")
	})
	var prompts []string
	out, err := executeEmails(t, testClient(t, srv, ""), nil, func(prompt string) error {
		prompts = append(prompts, prompt)
		return nil
	}, "emails", "labels", "delete", "u1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(prompts) != 1 || prompts[0] != "Delete label u1?" {
		t.Fatalf("prompts=%v", prompts)
	}
	if (*calls)[0].Method != http.MethodDelete || (*calls)[0].Path != "/v1/labels/u1" || (*calls)[0].Body != "" {
		t.Fatalf("request %#v", (*calls)[0])
	}
	assertCLIHeaders(t, (*calls)[0].Header, "")
	if out != "" {
		t.Fatalf("stdout=%q", out)
	}
}

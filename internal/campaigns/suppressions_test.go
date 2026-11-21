package campaigns

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

type recordedRequest struct {
	method, host, path, rawQuery, body string
	headers                            http.Header
}

func recordCampaigns(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	var got []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		got = append(got, recordedRequest{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func suppressionsFactory(t *testing.T, srv *httptest.Server, namespace string, stdin io.Reader, stdout io.Writer, jsonOut bool) *cli.Factory {
	t.Helper()
	f := testFactory(stdin, stdout)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, namespace), nil }
	f.Printer = func() *output.Printer { return output.New(stdout, output.Options{JSON: jsonOut}) }
	return f
}

func executeNoun(t *testing.T, cmd *cobra.Command, args ...string) error {
	t.Helper()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func wantHeaders(namespace string, contentType bool) map[string]string {
	headers := map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"}
	if namespace != "" {
		headers["Sl-Namespace-Id"] = namespace
	}
	if contentType {
		headers["Content-Type"] = "application/json"
	}
	return headers
}

func TestSuppressionsHelpIsImperativeAndLazy(t *testing.T) {
	var stdout bytes.Buffer
	called := false
	f := testFactory(nil, &stdout)
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	cmd := newSuppressionsCommand(f)
	if cmd.Use != "suppressions" || cmd.Short != "Manage organization suppressions" || strings.Contains(cmd.Short, "\n") {
		t.Fatalf("use=%q short=%q", cmd.Use, cmd.Short)
	}
	names := map[string]bool{}
	for _, child := range cmd.Commands() {
		if strings.Contains(child.Short, "\n") {
			t.Fatalf("child %s short has newline: %q", child.Name(), child.Short)
		}
		names[child.Name()] = true
		switch child.Name() {
		case "create":
			if child.Use != "create --input FILE" {
				t.Fatalf("create use=%q", child.Use)
			}
		case "delete":
			if child.Use != "delete ID --etag ETAG" {
				t.Fatalf("delete use=%q", child.Use)
			}
		}
	}
	for _, name := range []string{"list", "create", "delete"} {
		if !names[name] {
			t.Fatalf("missing %s in %#v", name, names)
		}
	}
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	if called {
		t.Fatal("help resolved a lazy dependency")
	}
	help := stdout.String()
	if !strings.Contains(help, "Manage organization suppressions") || !strings.Contains(help, "list") || !strings.Contains(help, "create") || !strings.Contains(help, "delete") {
		t.Fatalf("help=%q", help)
	}
}

func TestSuppressionsListDefaultLimitHeadersAndTable(t *testing.T) {
	raw := `{"suppressions":[{"name":"suppressions/s1","email_address":"ada@example.com","reason":"MANUAL_BLOCK","etag":"3","create_time":"2026-01-01T00:00:00Z","extra":true}],"next_page_token":""}`
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(raw))
	})
	var stdout bytes.Buffer
	f := suppressionsFactory(t, srv, "", nil, &stdout, false)
	if err := executeNoun(t, newSuppressionsCommand(f), "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	rec := (*got)[0]
	if rec.method != http.MethodGet || rec.host != "campaigns.example.test" || rec.path != "/v1/suppressions" || rec.rawQuery != "page_size=50" || rec.body != "" {
		t.Fatalf("request %s %s %s?%s body=%q", rec.method, rec.host, rec.path, rec.rawQuery, rec.body)
	}
	assertApplicationHeaders(t, rec.headers, wantHeaders("", false))
	if rec.headers.Get("sl-organization-id") != "" || rec.headers.Get("Authorization") != "" {
		t.Fatalf("forbidden headers %#v", applicationHeaders(rec.headers))
	}
	if stdout.String() != "s1\tada@example.com\tMANUAL_BLOCK\t3\t2026-01-01T00:00:00Z\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestSuppressionsListEmailFilterNamespaceAndJSON(t *testing.T) {
	page := `{"suppressions":[{"name":"suppressions/s2","email_address":"ada@example.com","reason":"UNSUBSCRIBED","etag":"1","create_time":"2026-02-02T00:00:00Z"}]}`
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(page))
	})
	var stdout bytes.Buffer
	f := suppressionsFactory(t, srv, "ns-1", nil, &stdout, true)
	if err := executeNoun(t, newSuppressionsCommand(f), "list", "--email-address", "ada@example.com", "--limit", "50"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	rec := (*got)[0]
	if rec.method != http.MethodGet || rec.path != "/v1/suppressions" || rec.rawQuery != "email_address=ada%40example.com&page_size=50" {
		t.Fatalf("request %s %s?%s", rec.method, rec.path, rec.rawQuery)
	}
	assertApplicationHeaders(t, rec.headers, wantHeaders("ns-1", false))
	if stdout.String() != `{"suppressions":[{"name":"suppressions/s2","email_address":"ada@example.com","reason":"UNSUBSCRIBED","etag":"1","create_time":"2026-02-02T00:00:00Z"}]}` {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestSuppressionsListPaginatesRemainingPageSize(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/suppressions" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		assertApplicationHeaders(t, r.Header, wantHeaders("", false))
		queries = append(queries, r.URL.RawQuery)
		switch r.URL.Query().Get("page_token") {
		case "":
			_, _ = w.Write([]byte(`{"suppressions":[{"name":"suppressions/a","email_address":"a@x.com","reason":"HARD_BOUNCE","etag":"1","create_time":"t1"},{"name":"suppressions/b","email_address":"b@x.com","reason":"MANUAL_BLOCK","etag":"2","create_time":"t2"}],"next_page_token":"p2"}`))
		case "p2":
			_, _ = w.Write([]byte(`{"suppressions":[{"name":"suppressions/c","email_address":"c@x.com","reason":"UNSUBSCRIBED","etag":"3","create_time":"t3"},{"name":"suppressions/d","email_address":"d@x.com","reason":"MANUAL_BLOCK","etag":"4","create_time":"t4"}]}`))
		default:
			t.Errorf("token %q", r.URL.Query().Get("page_token"))
		}
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := suppressionsFactory(t, srv, "", nil, &stdout, false)
	if err := executeNoun(t, newSuppressionsCommand(f), "list", "--limit", "3"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(queries) != 2 || queries[0] != "page_size=3" || queries[1] != "page_size=1&page_token=p2" {
		t.Fatalf("queries %#v", queries)
	}
	if stdout.String() != "a\ta@x.com\tHARD_BOUNCE\t1\tt1\nb\tb@x.com\tMANUAL_BLOCK\t2\tt2\nc\tc@x.com\tUNSUBSCRIBED\t3\tt3\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestSuppressionsListJSONUsesCollectedItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page_token") == "" {
			_, _ = w.Write([]byte(`{"suppressions":[{"name":"suppressions/a","email_address":"a@x.com","reason":"MANUAL_BLOCK","etag":"1","create_time":"t1","keep":true}],"next_page_token":"n2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"suppressions":[{"name":"suppressions/b","email_address":"b@x.com","reason":"HARD_BOUNCE","etag":"2","create_time":"t2"}]}`))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := suppressionsFactory(t, srv, "", nil, &stdout, true)
	if err := executeNoun(t, newSuppressionsCommand(f), "list", "--limit", "2"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if stdout.String() != `{"suppressions":[{"name":"suppressions/a","email_address":"a@x.com","reason":"MANUAL_BLOCK","etag":"1","create_time":"t1","keep":true},{"name":"suppressions/b","email_address":"b@x.com","reason":"HARD_BOUNCE","etag":"2","create_time":"t2"}]}` {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestSuppressionsListRejectsInvalidLimit(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	f := suppressionsFactory(t, srv, "", nil, io.Discard, false)
	for _, limit := range []string{"0", "-1"} {
		err := executeNoun(t, newSuppressionsCommand(f), "list", "--limit", limit)
		var usage *cli.UsageError
		if !errors.As(err, &usage) || usage.Msg != "--limit must be at least 1" || hits != 0 {
			t.Fatalf("limit=%s err=%v hits=%d", limit, err, hits)
		}
	}
}

func TestSuppressionsListTTYHeaders(t *testing.T) {
	srv, _ := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"suppressions":[{"name":"suppressions/s1","email_address":"ada@example.com","reason":"MANUAL_BLOCK","etag":"9","create_time":"2026-01-01T00:00:00Z"}]}`))
	})
	var stdout bytes.Buffer
	f := suppressionsFactory(t, srv, "", nil, &stdout, false)
	f.Printer = func() *output.Printer { return output.New(&stdout, output.Options{TTY: true}) }
	if err := executeNoun(t, newSuppressionsCommand(f), "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := stdout.String()
	if !strings.HasPrefix(got, "ID  EMAIL            REASON        ETAG  CREATED") || !strings.Contains(got, "s1  ada@example.com  MANUAL_BLOCK  9     2026-01-01T00:00:00Z") {
		t.Fatalf("stdout=%q", got)
	}
}

func TestSuppressionsCreateInputRequestIDHeadersBodyAndTable(t *testing.T) {
	created := `{"name":"suppressions/s1","email_address":"ada@example.com","reason":"MANUAL_BLOCK","etag":"1","create_time":"2026-01-01T00:00:00Z"}`
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(created))
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "body.json")
	if err := os.WriteFile(path, []byte(`{"email_address":"ada@example.com","reason":"MANUAL_BLOCK","keep":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	f := suppressionsFactory(t, srv, "ns-9", nil, &stdout, false)
	orgCalled := false
	f.Organization = func() (string, error) {
		orgCalled = true
		return "org-should-not-run", nil
	}
	if err := executeNoun(t, newSuppressionsCommand(f), "create", "--input", path); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if orgCalled {
		t.Fatal("organization resolver ran")
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	rec := (*got)[0]
	if rec.method != http.MethodPost || rec.host != "campaigns.example.test" || rec.path != "/v1/suppressions" || rec.rawQuery != "" {
		t.Fatalf("request %s %s %s?%s", rec.method, rec.host, rec.path, rec.rawQuery)
	}
	assertApplicationHeaders(t, rec.headers, wantHeaders("ns-9", true))
	if rec.headers.Get("sl-organization-id") != "" {
		t.Fatalf("organization header %q", rec.headers.Get("sl-organization-id"))
	}
	var body input.Object
	if err := json.Unmarshal([]byte(rec.body), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	if body.String("email_address") != "ada@example.com" || body.String("reason") != "MANUAL_BLOCK" || body.String("request_id") != "00000000-0000-4000-8000-000000000001" || string(body["keep"]) != "true" {
		t.Fatalf("body=%s", rec.body)
	}
	if stdout.String() != "s1\tada@example.com\tMANUAL_BLOCK\t1\t2026-01-01T00:00:00Z\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestSuppressionsCreatePreservesInputRequestIDAndJSON(t *testing.T) {
	created := `{"name":"suppressions/s9","email_address":"b@x.com","reason":"HARD_BOUNCE","etag":"4","create_time":"t4"}`
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(created))
	})
	var stdout bytes.Buffer
	f := suppressionsFactory(t, srv, "", strings.NewReader(`{"email_address":"b@x.com","reason":"HARD_BOUNCE","request_id":"input-id"}`), &stdout, true)
	if err := executeNoun(t, newSuppressionsCommand(f), "create", "--input", "-"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 || (*got)[0].body != `{"email_address":"b@x.com","reason":"HARD_BOUNCE","request_id":"input-id"}` {
		t.Fatalf("body=%q", (*got)[0].body)
	}
	if stdout.String() != created {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestSuppressionsCreateFlagRequestID(t *testing.T) {
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"suppressions/s3","email_address":"c@x.com","reason":"UNSUBSCRIBED","etag":"2","create_time":"t2"}`))
	})
	f := suppressionsFactory(t, srv, "", strings.NewReader(`{"email_address":"c@x.com","reason":"UNSUBSCRIBED"}`), io.Discard, false)
	if err := executeNoun(t, newSuppressionsCommand(f), "create", "--input", "-", "--request-id", "flag-id"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 || (*got)[0].body != `{"email_address":"c@x.com","reason":"UNSUBSCRIBED","request_id":"flag-id"}` {
		t.Fatalf("body=%q", (*got)[0].body)
	}
}

func TestSuppressionsCreateRequestIDConflict(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	f := suppressionsFactory(t, srv, "", strings.NewReader(`{"email_address":"c@x.com","reason":"UNSUBSCRIBED","request_id":"input-id"}`), io.Discard, false)
	err := executeNoun(t, newSuppressionsCommand(f), "create", "--input", "-", "--request-id", "flag-id")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--request-id") || !strings.Contains(usage.Msg, "request_id") || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func TestSuppressionsCreateRequiresInput(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	f := suppressionsFactory(t, srv, "", nil, io.Discard, false)
	err := executeNoun(t, newSuppressionsCommand(f), "create")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || usage.Msg != "--input is required" || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func TestSuppressionsCreateConflictStatus(t *testing.T) {
	srv, _ := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"ABORTED","message":"idempotency conflict"}`))
	})
	f := suppressionsFactory(t, srv, "", strings.NewReader(`{"email_address":"c@x.com","reason":"UNSUBSCRIBED"}`), io.Discard, false)
	err := executeNoun(t, newSuppressionsCommand(f), "create", "--input", "-")
	var apiErr *apiclient.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Code != "ABORTED" || apiErr.Path != "/v1/suppressions" || apiErr.Method != http.MethodPost {
		t.Fatalf("err=%v", err)
	}
}

func TestSuppressionsDeleteQueryConfirmNoPrefetch(t *testing.T) {
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	var stdout bytes.Buffer
	var prompt string
	f := suppressionsFactory(t, srv, "ns-2", nil, &stdout, false)
	f.Confirm = func(got string) error {
		prompt = got
		return nil
	}
	if err := executeNoun(t, newSuppressionsCommand(f), "delete", "s1", "--etag", "77"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete suppression s1?" || stdout.String() != "" || len(*got) != 1 {
		t.Fatalf("prompt=%q out=%q hits=%d", prompt, stdout.String(), len(*got))
	}
	rec := (*got)[0]
	if rec.method != http.MethodDelete || rec.host != "campaigns.example.test" || rec.path != "/v1/suppressions/s1" || rec.rawQuery != "etag=77&request_id=00000000-0000-4000-8000-000000000001" || rec.body != "" {
		t.Fatalf("request %s %s %s?%s body=%q", rec.method, rec.host, rec.path, rec.rawQuery, rec.body)
	}
	assertApplicationHeaders(t, rec.headers, wantHeaders("ns-2", false))
	if rec.headers.Get("sl-organization-id") != "" {
		t.Fatalf("organization header %q", rec.headers.Get("sl-organization-id"))
	}
}

func TestSuppressionsDeleteFlagRequestIDAndEscapedPath(t *testing.T) {
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	f := suppressionsFactory(t, srv, "", nil, io.Discard, false)
	if err := executeNoun(t, newSuppressionsCommand(f), "delete", "s/1", "--etag", `W/"abc"`, "--request-id", "flag-id"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	rec := (*got)[0]
	if rec.method != http.MethodDelete || rec.path != "/v1/suppressions/s%2F1" || rec.rawQuery != "etag=W%2F%22abc%22&request_id=flag-id" {
		t.Fatalf("request %s %s?%s", rec.method, rec.path, rec.rawQuery)
	}
}

func TestSuppressionsDeleteRequiresEtagWithoutPrefetch(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	prompted := false
	f := suppressionsFactory(t, srv, "", nil, io.Discard, false)
	f.Confirm = func(string) error {
		prompted = true
		return nil
	}
	err := executeNoun(t, newSuppressionsCommand(f), "delete", "s1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--etag") || hits != 0 || prompted {
		t.Fatalf("err=%v hits=%d prompted=%v", err, hits, prompted)
	}
	err = executeNoun(t, newSuppressionsCommand(f), "delete", "s1", "--etag", "")
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--etag") || hits != 0 {
		t.Fatalf("empty etag: %v hits=%d", err, hits)
	}
}

func TestSuppressionsDeleteConfirmDeclinedAndMissingYes(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	declined := suppressionsFactory(t, srv, "", nil, io.Discard, false)
	declined.Confirm = func(prompt string) error {
		if prompt != "Delete suppression s1?" {
			t.Fatalf("prompt=%q", prompt)
		}
		return cli.ErrCancelled
	}
	if err := executeNoun(t, newSuppressionsCommand(declined), "delete", "s1", "--etag", "9"); !errors.Is(err, cli.ErrCancelled) || hits != 0 {
		t.Fatalf("declined: %v hits=%d", err, hits)
	}
	usage := &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	missing := suppressionsFactory(t, srv, "", nil, io.Discard, false)
	missing.Confirm = func(string) error { return usage }
	err := executeNoun(t, newSuppressionsCommand(missing), "delete", "s1", "--etag", "9")
	var got *cli.UsageError
	if !errors.As(err, &got) || got.Msg != usage.Msg || hits != 0 {
		t.Fatalf("yes: %v hits=%d", err, hits)
	}
}

func TestSuppressionsDeleteRequiresID(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	f := suppressionsFactory(t, srv, "", nil, io.Discard, false)
	err := executeNoun(t, newSuppressionsCommand(f), "delete")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "suppression id") || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func TestSuppressionsDeleteConflictStatus(t *testing.T) {
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"ABORTED","message":"etag mismatch"}`))
	})
	f := suppressionsFactory(t, srv, "", nil, io.Discard, false)
	err := executeNoun(t, newSuppressionsCommand(f), "delete", "s1", "--etag", "1")
	var apiErr *apiclient.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Code != "ABORTED" || apiErr.Path != "/v1/suppressions/s1" || apiErr.Method != http.MethodDelete {
		t.Fatalf("err=%v", err)
	}
	if len(*got) != 1 || (*got)[0].method != http.MethodDelete {
		t.Fatalf("prefetch? %#v", *got)
	}
}

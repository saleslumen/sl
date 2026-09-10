package emails

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/saleslumen/sl/internal/root"
	"github.com/spf13/cobra"
)

const testKey = "sl_key_test_fixture"

type hostRewrite struct {
	target *url.URL
	next   http.RoundTripper
}

func (h hostRewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = h.target.Scheme
	clone.URL.Host = h.target.Host
	return h.next.RoundTrip(clone)
}

func testClient(t *testing.T, srv *httptest.Server, namespace string) *apiclient.Client {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return apiclient.New(apiclient.Options{APIKey: testKey, NamespaceID: namespace, APIDomain: "example.test", UserAgent: "sl/test", HTTPClient: &http.Client{Transport: hostRewrite{target: u, next: http.DefaultTransport}}})
}

func executeEmails(t *testing.T, client *apiclient.Client, stdin io.Reader, confirm func(string) error, args ...string) (string, error) {
	t.Helper()
	return executeEmailsJSON(t, client, stdin, confirm, false, args...)
}

func executeEmailsJSON(t *testing.T, client *apiclient.Client, stdin io.Reader, confirm func(string) error, jsonMode bool, args ...string) (string, error) {
	t.Helper()
	if stdin == nil {
		stdin = bytes.NewReader(nil)
	}
	var out bytes.Buffer
	f := &cli.Factory{
		IO: &cli.IOStreams{In: stdin, Out: &out, Err: io.Discard},
		Client: func() (*apiclient.Client, error) {
			return client, nil
		},
		Organization: func() (string, error) {
			return "org-test", nil
		},
		Printer: func() *output.Printer {
			return output.New(&out, output.Options{JSON: jsonMode})
		},
		Confirm: testConfirm(confirm),
		NewRequestID: func() string {
			return "00000000-0000-4000-8000-000000000001"
		},
	}
	cmd := root.NewRootCommand(f, NewCommand)
	cmd.SetArgs(args)
	cmd.SetIn(io.NopCloser(stdin))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	return out.String(), err
}

func testConfirm(override func(string) error) func(string) error {
	return func(prompt string) error {
		if override == nil {
			return nil
		}
		return override(prompt)
	}
}

func assertCLIHeaders(t *testing.T, h http.Header, namespace string) {
	t.Helper()
	if h.Get("sl-api-key") != testKey {
		t.Fatalf("sl-api-key=%q", h.Get("sl-api-key"))
	}
	if got := h.Get("sl-namespace-id"); got != namespace {
		t.Fatalf("sl-namespace-id=%q want %q", got, namespace)
	}
	if h.Get("sl-organization-id") != "" {
		t.Fatal("sl-organization-id present")
	}
	if h.Get("Authorization") != "" {
		t.Fatal("Authorization present")
	}
}

func writeInput(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewCommandRegistersAccountsAndTools(t *testing.T) {
	cmd := NewCommand(&cli.Factory{IO: &cli.IOStreams{Out: io.Discard}})
	if cmd.Use != "emails" {
		t.Fatalf("use=%q", cmd.Use)
	}
	names := map[string]*cobra.Command{}
	for _, child := range cmd.Commands() {
		names[child.Name()] = child
	}
	if names["accounts"] == nil || names["tools"] == nil {
		t.Fatalf("commands=%v", names)
	}
	tools := map[string]bool{}
	for _, child := range names["tools"].Commands() {
		tools[child.Name()] = true
	}
	if !tools["discover"] || !tools["verify"] || !tools["imap"] || !tools["smtp"] {
		t.Fatalf("tools=%v", tools)
	}
	accounts := map[string]bool{}
	for _, child := range names["accounts"].Commands() {
		accounts[child.Name()] = true
	}
	for _, name := range []string{"list", "get", "create", "update", "delete", "batch"} {
		if !accounts[name] {
			t.Fatalf("missing accounts %s", name)
		}
	}
}

func TestCommandHelpContracts(t *testing.T) {
	cmd := NewCommand(&cli.Factory{IO: &cli.IOStreams{Out: io.Discard}})
	if cmd.Short != "Manage emails" {
		t.Fatalf("short=%q", cmd.Short)
	}
	cases := map[string]string{
		"accounts update": "update ID --input FILE",
		"attachments get": "get ID --message MESSAGE_ID",
		"drafts create":   "create --input FILE",
		"drafts update":   "update ID --input FILE",
		"drafts send":     "send --input FILE",
		"labels update":   "update ID --input FILE",
		"messages send":   "send --input FILE",
		"messages modify": "modify ID --input FILE",
		"threads modify":  "modify ID --input FILE",
		"tools discover":  "discover --domain DOMAIN --name NAME",
		"tools verify":    "verify --input FILE",
	}
	for path, want := range cases {
		found, _, err := cmd.Find(strings.Fields(path))
		if err != nil || found.Use != want {
			t.Fatalf("%s use=%q err=%v want=%q", path, found.Use, err, want)
		}
	}
	for _, path := range []string{"accounts list", "labels list"} {
		found, _, err := cmd.Find(strings.Fields(path))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if found.Flags().Lookup("limit") != nil {
			t.Fatalf("%s exposes --limit", path)
		}
	}
	for path, want := range map[string]string{"accounts list": "List email accounts (the API returns at most 100)", "labels list": "List labels"} {
		found, _, _ := cmd.Find(strings.Fields(path))
		if found.Short != want {
			t.Fatalf("%s short=%q want=%q", path, found.Short, want)
		}
	}
}

func TestEmailsHelpDoesNotResolveClient(t *testing.T) {
	called := false
	var out bytes.Buffer
	f := &cli.Factory{
		IO: &cli.IOStreams{In: bytes.NewReader(nil), Out: &out, Err: io.Discard},
		Client: func() (*apiclient.Client, error) {
			called = true
			return nil, cli.ErrNoCredentials
		},
		Printer: func() *output.Printer {
			return output.New(&out, output.Options{})
		},
	}
	cmd := root.NewRootCommand(f, NewCommand)
	cmd.SetArgs([]string{"emails", "--help"})
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	if called {
		t.Fatal("client resolved")
	}
	if !strings.Contains(out.String(), "accounts") || !strings.Contains(out.String(), "tools") {
		t.Fatalf("help=%s", out.String())
	}
}

func TestAccountsListSendsEmailAndReturnsAllAccounts(t *testing.T) {
	var method, path, rawQuery, host string
	var headers http.Header
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		method = r.Method
		host = r.Host
		path = r.URL.Path
		rawQuery = r.URL.RawQuery
		headers = r.Header.Clone()
		_, _ = w.Write([]byte(`[
			{"id":"1","account_id":"a1","email_address":"a@example.com","display_name":"A","namespace_id":null,"oauth2_connected":true,"imap_password_configured":true,"smtp_password_configured":false,"updated_at":"2026-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z"},
			{"id":"2","account_id":"a2","email_address":"b@example.com","display_name":"B","namespace_id":"ns-1","oauth2_connected":false,"imap_password_configured":false,"smtp_password_configured":true,"updated_at":"2026-02-01T00:00:00Z","created_at":"2025-02-01T00:00:00Z"},
			{"id":"3","account_id":"a3","email_address":"c@example.com","display_name":"C","oauth2_connected":false,"imap_password_configured":true,"smtp_password_configured":true,"updated_at":"2026-03-01T00:00:00Z","created_at":"2025-03-01T00:00:00Z"}
		]`))
	}))
	t.Cleanup(srv.Close)
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "accounts", "list", "--email", "jane")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if hits != 1 || method != http.MethodGet || host != "emails.example.test" || path != "/v1/accounts" || rawQuery != "email=jane" {
		t.Fatalf("request hits=%d %s %s %s?%s", hits, method, host, path, rawQuery)
	}
	assertCLIHeaders(t, headers, "")
	want := "1\ta@example.com\tA\t\ttrue\ttrue\tfalse\t2026-01-01T00:00:00Z\n2\tb@example.com\tB\tns-1\tfalse\tfalse\ttrue\t2026-02-01T00:00:00Z\n3\tc@example.com\tC\t\tfalse\ttrue\ttrue\t2026-03-01T00:00:00Z\n"
	if out != want {
		t.Fatalf("stdout=%q want=%q", out, want)
	}
	jsonOut, err := executeEmailsJSON(t, testClient(t, srv, ""), nil, nil, true, "emails", "accounts", "list")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if len(got) != 3 || got[0]["id"] != "1" {
		t.Fatalf("json=%s", jsonOut)
	}
}

func TestAccountsListSendsNamespaceAndOmitsOrganization(t *testing.T) {
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	if _, err := executeEmails(t, testClient(t, srv, "ns-1"), nil, nil, "emails", "accounts", "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertCLIHeaders(t, headers, "ns-1")
}

func TestAccountsGetTableAndEscapedPath(t *testing.T) {
	var method, path string
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		headers = r.Header.Clone()
		_, _ = w.Write([]byte(`{"id":"acc/1","account_id":"jane@example.com","email_address":"jane@example.com","display_name":"Jane","namespace_id":null,"oauth2_connected":true,"imap_password_configured":true,"smtp_password_configured":true,"updated_at":"2026-01-02T00:00:00Z","created_at":"2025-01-02T00:00:00Z"}`))
	}))
	t.Cleanup(srv.Close)
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "accounts", "get", "acc/1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if method != http.MethodGet || path != "/v1/accounts/acc%2F1" {
		t.Fatalf("request %s %s", method, path)
	}
	assertCLIHeaders(t, headers, "")
	want := "acc/1\tjane@example.com\tJane\t\ttrue\ttrue\ttrue\t2026-01-02T00:00:00Z\tjane@example.com\t2025-01-02T00:00:00Z\n"
	if out != want {
		t.Fatalf("stdout=%q want=%q", out, want)
	}
}

func TestAccountsCreateMergesFlagsAndInput(t *testing.T) {
	var method, path, body string
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		method = r.Method
		path = r.URL.Path
		headers = r.Header.Clone()
		body = string(raw)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message":"Account created successfully"}`))
	}))
	t.Cleanup(srv.Close)
	input := writeInput(t, `{"imap_config":{"server_host":"imap.example.com"}}`)
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "accounts", "create", "--account-id", "jane@example.com", "--email-address", "jane@example.com", "--display-name", "Jane Doe", "--input", input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if method != http.MethodPost || path != "/v1/accounts" {
		t.Fatalf("request %s %s", method, path)
	}
	assertCLIHeaders(t, headers, "")
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("body json: %v", err)
	}
	if string(got["account_id"]) != `"jane@example.com"` || string(got["email_address"]) != `"jane@example.com"` || string(got["display_name"]) != `"Jane Doe"` || string(got["imap_config"]) != `{"server_host":"imap.example.com"}` {
		t.Fatalf("body=%s", body)
	}
	if out != `{"message":"Account created successfully"}` {
		t.Fatalf("stdout=%q", out)
	}
}

func TestAccountsCreateReadsStdinAndFile(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(raw))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	t.Cleanup(srv.Close)
	wire := `{"account_id":"a@example.com","email_address":"a@example.com","display_name":"A"}`
	if _, err := executeEmails(t, testClient(t, srv, ""), strings.NewReader(wire), nil, "emails", "accounts", "create", "--input", "-"); err != nil {
		t.Fatalf("stdin: %v", err)
	}
	if _, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "accounts", "create", "--input", writeInput(t, wire)); err != nil {
		t.Fatalf("file: %v", err)
	}
	want := `{"account_id":"a@example.com","display_name":"A","email_address":"a@example.com"}`
	if len(bodies) != 2 || bodies[0] != want || bodies[1] != want {
		t.Fatalf("bodies=%v", bodies)
	}
}

func TestAccountsCreateFlagInputDuplicateIsUsageError(t *testing.T) {
	_, err := executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), strings.NewReader(`{"account_id":"a"}`), nil, "emails", "accounts", "create", "--account-id", "b", "--input", "-")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--account-id") || !strings.Contains(usage.Msg, "account_id") {
		t.Fatalf("err=%v", err)
	}
}

func TestAccountsCreateRequiresFields(t *testing.T) {
	_, err := executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, nil, "emails", "accounts", "create", "--account-id", "a")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || usage.Msg != "email_address is required" {
		t.Fatalf("err=%v", err)
	}
}

func TestAccountsCreateRejectsInvalidInput(t *testing.T) {
	_, err := executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), strings.NewReader("not-json"), nil, "emails", "accounts", "create", "--input", "-")
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err=%v", err)
	}
}

func TestAccountsUpdatePassesInput(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		method = r.Method
		path = r.URL.Path
		body = string(raw)
		_, _ = w.Write([]byte(`{"message":"Account updated successfully"}`))
	}))
	t.Cleanup(srv.Close)
	wire := `{"display_name":"Sam"}`
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "accounts", "update", "acc-1", "--input", writeInput(t, wire))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if method != http.MethodPut || path != "/v1/accounts/acc-1" || body != wire {
		t.Fatalf("request %s %s body=%s", method, path, body)
	}
	if out != `{"message":"Account updated successfully"}` {
		t.Fatalf("stdout=%q", out)
	}
}

func TestAccountsUpdateRequiresInput(t *testing.T) {
	_, err := executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, nil, "emails", "accounts", "update", "acc-1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || usage.Msg != "--input is required" {
		t.Fatalf("err=%v", err)
	}
}

func TestAccountsDeleteUsesConfirm(t *testing.T) {
	hits := 0
	var method, path string
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		method = r.Method
		path = r.URL.Path
		headers = r.Header.Clone()
		_, _ = w.Write([]byte(`{"status":"success","message":"Account deleted successfully"}`))
	}))
	t.Cleanup(srv.Close)
	var prompt string
	out, err := executeEmails(t, testClient(t, srv, ""), nil, func(got string) error {
		prompt = got
		return nil
	}, "emails", "accounts", "delete", "acc-1")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete account acc-1?" || hits != 1 || method != http.MethodDelete || path != "/v1/accounts/acc-1" {
		t.Fatalf("prompt=%q hits=%d %s %s", prompt, hits, method, path)
	}
	assertCLIHeaders(t, headers, "")
	if out != `{"status":"success","message":"Account deleted successfully"}` {
		t.Fatalf("stdout=%q", out)
	}
	hits = 0
	_, err = executeEmails(t, testClient(t, srv, ""), nil, func(string) error {
		return cli.ErrCancelled
	}, "emails", "accounts", "delete", "acc-1")
	if !errors.Is(err, cli.ErrCancelled) || hits != 0 {
		t.Fatalf("declined err=%v hits=%d", err, hits)
	}
	_, err = executeEmails(t, testClient(t, srv, ""), nil, func(string) error {
		return &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	}, "emails", "accounts", "delete", "acc-1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || hits != 0 {
		t.Fatalf("non-tty err=%v hits=%d", err, hits)
	}
}

func TestAccountsBatchPassesArray(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		method = r.Method
		path = r.URL.Path
		body = string(raw)
		_, _ = w.Write([]byte(`{"message":"Accounts created successfully"}`))
	}))
	t.Cleanup(srv.Close)
	wire := `[{"account_id":"a@example.com","email_address":"a@example.com","display_name":"A"}]`
	out, err := executeEmails(t, testClient(t, srv, ""), strings.NewReader(wire), nil, "emails", "accounts", "batch", "--input", "-")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if method != http.MethodPost || path != "/v1/accounts:batch" || body != wire {
		t.Fatalf("request %s %s body=%s", method, path, body)
	}
	if out != `{"message":"Accounts created successfully"}` {
		t.Fatalf("stdout=%q", out)
	}
}

func TestAccountsBatchRejectsObject(t *testing.T) {
	_, err := executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), strings.NewReader(`{"account_id":"a"}`), nil, "emails", "accounts", "batch", "--input", "-")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || usage.Msg != "--input must be a JSON array" {
		t.Fatalf("err=%v", err)
	}
}

func TestToolsDiscoverOneGET(t *testing.T) {
	hits := 0
	var method, path, rawQuery, host string
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		method = r.Method
		host = r.Host
		path = r.URL.Path
		rawQuery = r.URL.RawQuery
		headers = r.Header.Clone()
		_, _ = w.Write([]byte(`{"kind":"email-search","status":"pending","data":{"id":"run-1","domain":"example.com","name":"Jane Doe"}}`))
	}))
	t.Cleanup(srv.Close)
	out, err := executeEmails(t, testClient(t, srv, "ns-1"), nil, nil, "emails", "tools", "discover", "--domain", "example.com", "--name", "Jane Doe")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if hits != 1 || method != http.MethodGet || host != "emails.example.test" || path != "/v1/tools:discover" || rawQuery != "domain=example.com&name=Jane+Doe" {
		t.Fatalf("request hits=%d %s %s %s?%s", hits, method, host, path, rawQuery)
	}
	assertCLIHeaders(t, headers, "ns-1")
	if !strings.Contains(out, `"status":"pending"`) {
		t.Fatalf("stdout=%s", out)
	}
}

func TestToolsDiscoverRequiresFlags(t *testing.T) {
	_, err := executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, nil, "emails", "tools", "discover")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--domain") || !strings.Contains(usage.Msg, "--name") {
		t.Fatalf("err=%v", err)
	}
}

func TestToolsIMAPAndSMTPPassInput(t *testing.T) {
	var methods, paths, bodies []string
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		methods = append(methods, r.Method)
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, string(raw))
		headers = r.Header.Clone()
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(srv.Close)
	wire := `{"server_host":"mail.example.com","port":993,"username":"user@example.com","password":"secret"}`
	out, err := executeEmails(t, testClient(t, srv, ""), nil, nil, "emails", "tools", "imap", "--input", writeInput(t, wire))
	if err != nil {
		t.Fatalf("imap: %v", err)
	}
	if _, err := executeEmails(t, testClient(t, srv, ""), strings.NewReader(wire), nil, "emails", "tools", "smtp", "--input", "-"); err != nil {
		t.Fatalf("smtp: %v", err)
	}
	if strings.Join(methods, ",") != "POST,POST" || strings.Join(paths, ",") != "/v1/tools:imap,/v1/tools:smtp" {
		t.Fatalf("requests %v %v", methods, paths)
	}
	want := `{"password":"secret","port":993,"server_host":"mail.example.com","username":"user@example.com"}`
	if bodies[0] != want || bodies[1] != want {
		t.Fatalf("bodies=%v", bodies)
	}
	assertCLIHeaders(t, headers, "")
	if out != `{"success":true}` {
		t.Fatalf("stdout=%q", out)
	}
}

func TestToolsVerifyStreamsJSONLines(t *testing.T) {
	var body string
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		headers = r.Header.Clone()
		if r.Method != http.MethodPost || r.URL.Path != "/v1/tools:verify" {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, "\n{\"email\":\"a@example.com\",\"status\":\"valid\"}\r\n \n{\"done\":true,\"count\":1}")
	}))
	t.Cleanup(srv.Close)
	input := `{"emails":["a@example.com"],"features":["STANDARD"]}`
	out, err := executeEmails(t, testClient(t, srv, "ns-1"), strings.NewReader(input), nil, "emails", "tools", "verify", "--input", "-")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if body != input {
		t.Fatalf("body=%q", body)
	}
	assertCLIHeaders(t, headers, "ns-1")
	if out != "{\"email\":\"a@example.com\",\"status\":\"valid\"}\n{\"done\":true,\"count\":1}\n" {
		t.Fatalf("stdout=%q", out)
	}
}

func TestToolsVerifyAppliesJQPerRecord(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{\"status\":\"valid\"}\n{\"status\":\"catch_all\"}\n")
	}))
	t.Cleanup(srv.Close)
	var out bytes.Buffer
	f := &cli.Factory{
		IO:      &cli.IOStreams{In: strings.NewReader(`{"emails":["a@example.com"],"features":["STANDARD"]}`), Out: &out},
		Client:  func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil },
		Printer: func() *output.Printer { return output.New(&out, output.Options{JQ: ".status"}) },
	}
	if err := runToolsVerify(context.Background(), f, "-"); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if out.String() != "valid\ncatch_all\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestToolsConnectionRequiresObjectInput(t *testing.T) {
	_, err := executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, nil, "emails", "tools", "imap")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || usage.Msg != "--input is required" {
		t.Fatalf("err=%v", err)
	}
	_, err = executeEmails(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), strings.NewReader(`["x"]`), nil, "emails", "tools", "smtp", "--input", "-")
	if !errors.As(err, &usage) || usage.Msg != "--input must be a JSON object" {
		t.Fatalf("array err=%v", err)
	}
}

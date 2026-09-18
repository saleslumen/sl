package campaigns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/input"
	"github.com/saleslumen/sl/internal/output"
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

func testFactory(stdin io.Reader, stdout io.Writer) *cli.Factory {
	if stdin == nil {
		stdin = bytes.NewReader(nil)
	}
	if stdout == nil {
		stdout = io.Discard
	}
	return &cli.Factory{
		IO: &cli.IOStreams{In: stdin, Out: stdout, Err: io.Discard},
		Client: func() (*apiclient.Client, error) {
			return nil, cli.ErrNoCredentials
		},
		Organization: func() (string, error) {
			return "org-test", nil
		},
		Printer: func() *output.Printer {
			return output.New(stdout, output.Options{})
		},
		Confirm: func(prompt string) error {
			return nil
		},
		NewRequestID: func() string {
			return "00000000-0000-4000-8000-000000000001"
		},
	}
}

func applicationHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for name, values := range h {
		if name == "Accept-Encoding" || name == "Content-Length" {
			continue
		}
		out[name] = strings.Join(values, ",")
	}
	return out
}

func assertApplicationHeaders(t *testing.T, got http.Header, want map[string]string) {
	t.Helper()
	app := applicationHeaders(got)
	if len(app) != len(want) {
		t.Fatalf("headers %#v, want %#v", app, want)
	}
	for name, value := range want {
		if app[name] != value {
			t.Fatalf("header %s=%q, want %q in %#v", name, app[name], value, app)
		}
	}
}

func TestNewCommandCollapsesRootAndHelp(t *testing.T) {
	var stdout bytes.Buffer
	called := false
	f := testFactory(nil, &stdout)
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	f.Organization = func() (string, error) {
		called = true
		return "", nil
	}
	f.Printer = func() *output.Printer {
		called = true
		return output.New(&stdout, output.Options{})
	}
	cmd := NewCommand(f)
	if cmd.Use != "campaigns" || cmd.Short != "Manage campaigns" {
		t.Fatalf("use=%q short=%q", cmd.Use, cmd.Short)
	}
	if cmd.Name() != "campaigns" {
		t.Fatalf("name=%q", cmd.Name())
	}
	names := map[string]bool{}
	for _, child := range cmd.Commands() {
		if child.Name() == "campaigns" {
			t.Fatal("root resource must collapse onto campaigns")
		}
		names[child.Name()] = true
	}
	for _, name := range []string{"schedules", "people", "sequences", "deliveries", "metrics", "suppressions", "operations", "tasks"} {
		if !names[name] {
			t.Fatalf("missing noun %s in %#v", name, names)
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
	if !strings.Contains(help, "Manage campaigns") {
		t.Fatalf("help=%q", help)
	}
	for _, name := range []string{"schedules", "people", "sequences", "deliveries", "metrics", "suppressions", "operations", "tasks"} {
		if !strings.Contains(help, name) {
			t.Fatalf("help missing %s: %s", name, help)
		}
	}
}

func TestGeneratedControls(t *testing.T) {
	f := testFactory(nil, nil)
	obj := input.Object{"display_name": json.RawMessage(`"Acme"`)}
	if err := ensureRequestID(f, obj, "", false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if obj.String("request_id") != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("generated %#v", obj)
	}
	if string(obj["display_name"]) != `"Acme"` {
		t.Fatalf("display_name rewritten %#v", obj)
	}
	existing := input.Object{"request_id": json.RawMessage(`"input-id"`)}
	if err := ensureRequestID(f, existing, "", false); err != nil {
		t.Fatalf("keep input: %v", err)
	}
	if existing.String("request_id") != "input-id" {
		t.Fatalf("input overwritten %#v", existing)
	}
	flagged := input.Object{}
	if err := ensureRequestID(f, flagged, "flag-id", true); err != nil {
		t.Fatalf("flag: %v", err)
	}
	if flagged.String("request_id") != "flag-id" {
		t.Fatalf("flag %#v", flagged)
	}
	conflict := input.Object{"request_id": json.RawMessage(`"input-id"`)}
	var usage *cli.UsageError
	if err := ensureRequestID(f, conflict, "flag-id", true); !errors.As(err, &usage) {
		t.Fatalf("conflict: %v", err)
	}
}

func TestOrganizationFallback(t *testing.T) {
	called := false
	f := testFactory(nil, nil)
	f.Organization = func() (string, error) {
		called = true
		return "org-1", nil
	}
	obj := input.Object{"display_name": json.RawMessage(`"Acme"`)}
	if err := ensureOrganization(f, obj); err != nil {
		t.Fatalf("inject: %v", err)
	}
	if !called || obj.String("organization") != "organizations/org-1" || string(obj["display_name"]) != `"Acme"` {
		t.Fatalf("injected %#v called=%v", obj, called)
	}
	called = false
	existing := input.Object{"organization": json.RawMessage(`"organizations/from-input"`), "keep": json.RawMessage("true")}
	if err := ensureOrganization(f, existing); err != nil {
		t.Fatalf("preserve: %v", err)
	}
	if called || existing.String("organization") != "organizations/from-input" || string(existing["keep"]) != "true" {
		t.Fatalf("preserved %#v called=%v", existing, called)
	}
	missing := testFactory(nil, nil)
	missing.Organization = func() (string, error) {
		return "", &cli.UsageError{Msg: "organization required; pass --organization or run 'sl config set organization_id <id>'"}
	}
	var usage *cli.UsageError
	if err := ensureOrganization(missing, input.Object{}); !errors.As(err, &usage) {
		t.Fatalf("missing: %v", err)
	}
}

func TestEnsureEtagFetch(t *testing.T) {
	var gotMethod, gotPath string
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"campaigns/c1","etag":"77","display_name":"Kept"}`))
	}))
	t.Cleanup(srv.Close)
	f := testFactory(nil, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	obj := input.Object{"request_id": json.RawMessage(`"rid"`)}
	if err := ensureEtag(context.Background(), f, obj, "", false, "/v1/campaigns/c1"); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if hits != 1 || gotMethod != http.MethodGet || gotPath != "/v1/campaigns/c1" {
		t.Fatalf("request %s %s hits=%d", gotMethod, gotPath, hits)
	}
	if obj.String("etag") != "77" || string(obj["request_id"]) != `"rid"` {
		t.Fatalf("etag %#v", obj)
	}
	hits = 0
	existing := input.Object{"etag": json.RawMessage(`"input-etag"`)}
	if err := ensureEtag(context.Background(), f, existing, "", false, "/v1/campaigns/c1"); err != nil {
		t.Fatalf("preserve: %v", err)
	}
	if hits != 0 || existing.String("etag") != "input-etag" {
		t.Fatalf("preserved hits=%d %#v", hits, existing)
	}
	flagged := input.Object{}
	if err := ensureEtag(context.Background(), f, flagged, "flag-etag", true, "/v1/campaigns/c1"); err != nil {
		t.Fatalf("flag: %v", err)
	}
	if hits != 0 || flagged.String("etag") != "flag-etag" {
		t.Fatalf("flag hits=%d %#v", hits, flagged)
	}
	var usage *cli.UsageError
	if err := ensureEtag(context.Background(), f, input.Object{}, "", false, ""); !errors.As(err, &usage) || !strings.Contains(usage.Msg, "--etag") {
		t.Fatalf("required: %v", err)
	}
	conflict := input.Object{"etag": json.RawMessage(`"input-etag"`)}
	if err := ensureEtag(context.Background(), f, conflict, "flag-etag", true, "/v1/campaigns/c1"); !errors.As(err, &usage) {
		t.Fatalf("conflict: %v", err)
	}
}

func TestDoHeadersOmitOrganization(t *testing.T) {
	t.Setenv("SL_ORGANIZATION_ID", "org-configured")
	var method, host, path, rawQuery, body string
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		method = r.Method
		host = r.Host
		path = r.URL.Path
		rawQuery = r.URL.RawQuery
		headers = r.Header.Clone()
		body = string(raw)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	f := testFactory(nil, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-1"), nil }
	payload := input.Object{"organization": json.RawMessage(`"organizations/org-configured"`), "display_name": json.RawMessage(`"Acme"`)}
	resp, err := do(context.Background(), f, http.MethodPost, "/v1/campaigns", url.Values{"page_size": []string{"1"}}, payload)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("body: %s", resp.Body)
	}
	if method != http.MethodPost || host != "campaigns.example.test" || path != "/v1/campaigns" || rawQuery != "page_size=1" {
		t.Fatalf("request %s %s %s?%s", method, host, path, rawQuery)
	}
	assertApplicationHeaders(t, headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if got := headers.Get("sl-organization-id"); got != "" {
		t.Fatalf("sl-organization-id=%q", got)
	}
	if !strings.Contains(body, `"organization":"organizations/org-configured"`) || !strings.Contains(body, `"display_name":"Acme"`) {
		t.Fatalf("body: %s", body)
	}
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	headers = nil
	if _, err := do(context.Background(), f, http.MethodGet, "/v1/campaigns", nil, nil); err != nil {
		t.Fatalf("get: %v", err)
	}
	assertApplicationHeaders(t, headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if got := headers.Get("sl-organization-id"); got != "" {
		t.Fatalf("get sl-organization-id=%q", got)
	}
}

func TestPaginationSupport(t *testing.T) {
	if q := listQuery(3, "tok"); q.Get("page_size") != "3" || q.Get("page_token") != "tok" {
		t.Fatalf("query %#v", q)
	}
	if q := listQuery(0, ""); q.Get("page_size") != "" || q.Get("page_token") != "" {
		t.Fatalf("default query %#v", q)
	}
	if q := listQuery(500, ""); q.Get("page_size") != "200" {
		t.Fatalf("capped %#v", q)
	}
}

func TestResourceIDAndFormatting(t *testing.T) {
	if got := resourceID("campaigns/abc/people/p1"); got != "p1" {
		t.Fatalf("nested=%q", got)
	}
	if got := resourceID("campaigns/abc"); got != "abc" {
		t.Fatalf("campaign=%q", got)
	}
	if got := resourceID("abc"); got != "abc" {
		t.Fatalf("bare=%q", got)
	}
	if got := formatCount(12); got != "12" {
		t.Fatalf("count=%q", got)
	}
	if got := formatScalar(true); got != "true" {
		t.Fatalf("bool=%q", got)
	}
	if got := formatScalar(json.RawMessage(`"DRAFT"`)); got != "DRAFT" {
		t.Fatalf("raw=%q", got)
	}
	if got := formatScalar(nil); got != "" {
		t.Fatalf("nil=%q", got)
	}
}

func TestRequireArgUsageError(t *testing.T) {
	var usage *cli.UsageError
	if _, err := requireArg(nil, "campaign id"); !errors.As(err, &usage) || !strings.Contains(usage.Msg, "campaign id") {
		t.Fatalf("missing: %v", err)
	}
	id, err := requireArg([]string{"c1"}, "campaign id")
	if err != nil || id != "c1" {
		t.Fatalf("id=%q err=%v", id, err)
	}
}

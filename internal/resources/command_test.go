package resources

import (
	"bytes"
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

type recordedRequest struct {
	Method string
	Host   string
	Path   string
	Query  url.Values
	Header http.Header
	Body   []byte
}

func testClient(t *testing.T, srv *httptest.Server, namespace string) *apiclient.Client {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return apiclient.New(apiclient.Options{APIKey: testKey, NamespaceID: namespace, APIDomain: "example.test", UserAgent: "sl/test", HTTPClient: &http.Client{Transport: hostRewrite{target: u, next: http.DefaultTransport}}})
}

func testIO(in io.Reader, stdinTTY, stdoutTTY bool) (*cli.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	if in == nil {
		in = bytes.NewReader(nil)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return &cli.IOStreams{In: in, Out: stdout, Err: stderr, IsStdinTTY: stdinTTY, IsStdoutTTY: stdoutTTY}, stdout, stderr
}

type execResult struct {
	stdout string
	stderr string
	err    error
}

func execResources(t *testing.T, client *apiclient.Client, stdin io.Reader, stdinTTY bool, confirm func(string) error, organization func() (string, error), args ...string) execResult {
	t.Helper()
	ios, stdout, stderr := testIO(stdin, stdinTTY, false)
	jsonMode := false
	jq := ""
	for i, arg := range args {
		if arg == "--json" {
			jsonMode = true
		}
		if arg == "--jq" && i+1 < len(args) {
			jq = args[i+1]
			jsonMode = true
		}
		if strings.HasPrefix(arg, "--jq=") {
			jq = strings.TrimPrefix(arg, "--jq=")
			jsonMode = true
		}
	}
	if confirm == nil {
		confirm = func(string) error { return nil }
	}
	if organization == nil {
		organization = func() (string, error) {
			return "org-test", nil
		}
	}
	f := &cli.Factory{
		IO: ios,
		Client: func() (*apiclient.Client, error) {
			return client, nil
		},
		Printer: func() *output.Printer {
			return output.New(ios.Out, output.Options{JSON: jsonMode, JQ: jq, TTY: ios.IsStdoutTTY})
		},
		Confirm:      confirm,
		Organization: organization,
		NewRequestID: func() string {
			return "00000000-0000-4000-8000-000000000001"
		},
	}
	cmd := root.NewRootCommand(f, NewCommand)
	cmd.SetArgs(args)
	cmd.SetIn(ios.In)
	cmd.SetOut(ios.Out)
	cmd.SetErr(ios.Err)
	err := cmd.Execute()
	return execResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func startResourcesServer(t *testing.T, handle func(http.ResponseWriter, *http.Request, recordedRequest)) (*httptest.Server, *[]recordedRequest) {
	t.Helper()
	var recs []recordedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		rec := recordedRequest{Method: r.Method, Host: r.Host, Path: r.URL.Path, Query: r.URL.Query(), Header: r.Header.Clone(), Body: raw}
		recs = append(recs, rec)
		handle(w, r, rec)
	}))
	t.Cleanup(srv.Close)
	return srv, &recs
}

func writeInputFile(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func requireUsage(t *testing.T, err error, substr string) {
	t.Helper()
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("usage: %v", err)
	}
	if substr != "" && !strings.Contains(usage.Error(), substr) {
		t.Fatalf("usage=%q want %q", usage.Error(), substr)
	}
}

func assertResourcesHeaders(t *testing.T, h http.Header, namespace string) {
	t.Helper()
	if h.Get("sl-api-key") != testKey {
		t.Fatalf("sl-api-key=%q", h.Get("sl-api-key"))
	}
	if h.Get("sl-namespace-id") != namespace {
		t.Fatalf("sl-namespace-id=%q", h.Get("sl-namespace-id"))
	}
	if h.Get("sl-organization-id") != "" {
		t.Fatalf("sl-organization-id present")
	}
	if h.Get("Authorization") != "" {
		t.Fatalf("Authorization present")
	}
	if h.Get("Accept") != "application/json" {
		t.Fatalf("Accept=%q", h.Get("Accept"))
	}
	if h.Get("User-Agent") != "sl/test" {
		t.Fatalf("User-Agent=%q", h.Get("User-Agent"))
	}
}

func decodeJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json: %v body=%s", err, raw)
	}
	return out
}

func TestCommandUsage(t *testing.T) {
	root := NewCommand(&cli.Factory{})
	if root.Use != "resources" || root.Short != "Manage organizations and namespaces" {
		t.Fatalf("use=%q short=%q", root.Use, root.Short)
	}
	cases := []struct{ path, use string }{
		{"organizations get", "get ID"},
		{"namespaces list", "list"},
		{"namespaces create", "create --input FILE"},
		{"namespaces delete", "delete ID"},
	}
	for _, tc := range cases {
		cmd, _, err := root.Find(strings.Fields(tc.path))
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if cmd.Use != tc.use {
			t.Errorf("%s Use=%q want=%q", tc.path, cmd.Use, tc.use)
		}
	}
}

func TestOrganizationsGetPathEscapeAndTable(t *testing.T) {
	srv, recs := startResourcesServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/organizations/org%2F1" || rec.Host != "resources.example.test" {
			t.Errorf("request %s %s host=%s", rec.Method, rec.Path, rec.Host)
		}
		assertResourcesHeaders(t, rec.Header, "ns-1")
		_, _ = w.Write([]byte(`{"id":"org/1","name":"Acme","domain":"acme.example","industry":"software","employee_count":"1-10","preferred_language":"en","country":"US","updated_at":"2026-01-01T00:00:00Z"}`))
	})
	got := execResources(t, testClient(t, srv, "ns-1"), nil, false, nil, nil, "resources", "organizations", "get", "org/1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != "org/1\tAcme\tacme.example\tsoftware\t1-10\ten\tUS\t2026-01-01T00:00:00Z\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestOrganizationsGetRejectsEmptyID(t *testing.T) {
	got := execResources(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, nil, "resources", "organizations", "get", "  ")
	requireUsage(t, got.err, "organization ID is required")
}

func TestNamespacesListUsesFactoryOrganizationAndRawJSONArray(t *testing.T) {
	raw := `[{"id":"ns-1","organization_id":"org-test","name":"default","description":"Primary","updated_at":"t1"}]`
	srv, recs := startResourcesServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/organizations/org-test/namespaces" || rec.Host != "resources.example.test" {
			t.Errorf("request %s %s host=%s", rec.Method, rec.Path, rec.Host)
		}
		if len(rec.Query) != 0 {
			t.Errorf("query=%v", rec.Query)
		}
		assertResourcesHeaders(t, rec.Header, "")
		_, _ = w.Write([]byte(raw))
	})
	got := execResources(t, testClient(t, srv, ""), nil, false, nil, nil, "resources", "namespaces", "list")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != "ns-1\torg-test\tdefault\tPrimary\tt1\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	jsonGot := execResources(t, testClient(t, srv, ""), nil, false, nil, nil, "--json", "resources", "namespaces", "list")
	if jsonGot.err != nil {
		t.Fatalf("json: %v", jsonGot.err)
	}
	if strings.TrimSpace(jsonGot.stdout) != raw {
		t.Fatalf("json=%q", jsonGot.stdout)
	}
	if len(*recs) != 2 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestNamespacesListUsesOrganizationCallback(t *testing.T) {
	srv, recs := startResourcesServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Path != "/v1/organizations/org-flag/namespaces" {
			t.Errorf("path=%s", rec.Path)
		}
		assertResourcesHeaders(t, rec.Header, "")
		_, _ = w.Write([]byte(`[]`))
	})
	got := execResources(t, testClient(t, srv, ""), nil, false, nil, func() (string, error) {
		return "org-flag", nil
	}, "resources", "namespaces", "list")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != "" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestNamespacesCreateRequiresInputAndPassesBody(t *testing.T) {
	got := execResources(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, nil, "resources", "namespaces", "create")
	requireUsage(t, got.err, "--input")
	var body map[string]any
	srv, recs := startResourcesServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodPost || rec.Path != "/v1/organizations/org-test/namespaces" || rec.Host != "resources.example.test" {
			t.Errorf("request %s %s host=%s", rec.Method, rec.Path, rec.Host)
		}
		assertResourcesHeaders(t, rec.Header, "")
		if rec.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type=%q", rec.Header.Get("Content-Type"))
		}
		body = decodeJSON(t, rec.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"ns-new","organization_id":"org-test","name":"default","description":"Primary","updated_at":"t1"}`))
	})
	got = execResources(t, testClient(t, srv, ""), nil, false, nil, nil, "resources", "namespaces", "create", "--input", writeInputFile(t, `{"name":"default","description":"Primary"}`))
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if body["name"] != "default" || body["description"] != "Primary" {
		t.Fatalf("body=%#v", body)
	}
	if _, ok := body["organization_id"]; ok {
		t.Fatalf("body injected organization_id %#v", body)
	}
	if got.stdout != "ns-new\torg-test\tdefault\tPrimary\tt1\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestNamespacesDeleteConfirmDeclineAndEmptyBody(t *testing.T) {
	hits := 0
	srv, recs := startResourcesServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		hits++
		if rec.Method != http.MethodDelete || rec.Path != "/v1/organizations/org-test/namespaces/ns-1" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		assertResourcesHeaders(t, rec.Header, "")
		w.WriteHeader(http.StatusNoContent)
	})
	declined := execResources(t, testClient(t, srv, ""), nil, false, func(string) error {
		return cli.ErrCancelled
	}, nil, "resources", "namespaces", "delete", "ns-1")
	if !errors.Is(declined.err, cli.ErrCancelled) {
		t.Fatalf("err=%v", declined.err)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
	var prompt string
	got := execResources(t, testClient(t, srv, ""), nil, false, func(p string) error {
		prompt = p
		return nil
	}, nil, "resources", "namespaces", "delete", "ns-1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if prompt != "Delete namespace ns-1?" {
		t.Fatalf("prompt=%q", prompt)
	}
	if got.stdout != "" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestNamespacesDeletePathEscapeAndEmptyID(t *testing.T) {
	got := execResources(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, nil, "resources", "namespaces", "delete", "  ")
	requireUsage(t, got.err, "namespace ID is required")
	srv, recs := startResourcesServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodDelete || rec.Path != "/v1/organizations/org-test/namespaces/ns%2F1" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	got = execResources(t, testClient(t, srv, ""), nil, false, nil, nil, "resources", "namespaces", "delete", "ns/1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestNamespacesMissingOrganization(t *testing.T) {
	got := execResources(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, func() (string, error) {
		return "", nil
	}, "resources", "namespaces", "list")
	requireUsage(t, got.err, organizationRequiredMsg)
}

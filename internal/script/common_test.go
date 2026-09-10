package script

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
	"github.com/saleslumen/sl/internal/input"
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

func execScript(t *testing.T, client *apiclient.Client, stdin io.Reader, stdinTTY bool, confirm func(string) error, args ...string) execResult {
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
	f := &cli.Factory{
		IO: ios,
		Client: func() (*apiclient.Client, error) {
			return client, nil
		},
		Printer: func() *output.Printer {
			return output.New(ios.Out, output.Options{JSON: jsonMode, JQ: jq, TTY: ios.IsStdoutTTY})
		},
		Confirm: confirm,
		Organization: func() (string, error) {
			return "org-test", nil
		},
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

func startScriptServer(t *testing.T, handle func(http.ResponseWriter, *http.Request, recordedRequest)) (*httptest.Server, *[]recordedRequest) {
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

func assertScriptHeaders(t *testing.T, h http.Header, namespace string) {
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

func TestCommandUsageShowsRequiredFlags(t *testing.T) {
	root := NewCommand(&cli.Factory{})
	if root.Short != "Manage Apps Script projects" {
		t.Fatalf("short=%q", root.Short)
	}
	cases := []struct {
		path string
		use  string
	}{
		{"content get", "get --project ID"},
		{"content update", "update --project ID --input FILE"},
		{"versions compare", "compare --project ID --from N --to N"},
		{"versions restore", "restore VERSION --project ID"},
		{"deployments update", "update DEPLOYMENT_ID --project ID --input FILE"},
		{"processes list-script-processes", "list-script-processes --project ID"},
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

func decodeJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json: %v body=%s", err, raw)
	}
	return out
}

func TestListPageSizeCapsAt100(t *testing.T) {
	if got := listPageSize(50, 0); got != 50 {
		t.Fatalf("default page=%d", got)
	}
	if got := listPageSize(150, 0); got != 100 {
		t.Fatalf("first page=%d", got)
	}
	if got := listPageSize(150, 100); got != 50 {
		t.Fatalf("second page=%d", got)
	}
}

func TestApplyDraftEtagFetchesOnlyWhenAbsent(t *testing.T) {
	calls := 0
	fetch := func() (string, error) {
		calls++
		return `W/"etag"`, nil
	}
	body := input.Object{"keep": json.RawMessage("true")}
	if err := applyDraftEtag(body, "", false, fetch); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || body.String("draftEtag") != `W/"etag"` || string(body["keep"]) != "true" {
		t.Fatalf("fetched %#v calls=%d", body, calls)
	}
	calls = 0
	body = input.Object{"draftEtag": json.RawMessage(`"x"`), "keep": json.RawMessage("1")}
	err := applyDraftEtag(body, "", false, fetch)
	if err != nil || calls != 0 || body.String("draftEtag") != "x" || string(body["keep"]) != "1" {
		t.Fatalf("input etag %#v calls=%d err=%v", body, calls, err)
	}
	calls = 0
	body = input.Object{"keep": json.RawMessage(`"y"`)}
	err = applyDraftEtag(body, "flag", true, fetch)
	if err != nil || calls != 0 || body.String("draftEtag") != "flag" || body.String("keep") != "y" {
		t.Fatalf("flag etag %#v calls=%d err=%v", body, calls, err)
	}
	err = applyDraftEtag(input.Object{"draftEtag": json.RawMessage(`"x"`)}, "flag", true, fetch)
	requireUsage(t, err, "draftEtag")
}

func TestRequireLimitAndProject(t *testing.T) {
	requireUsage(t, requireLimit(0), "--limit")
	requireUsage(t, requireProject(""), "--project")
	if err := requireLimit(1); err != nil {
		t.Fatal(err)
	}
	if err := requireProject("p1"); err != nil {
		t.Fatal(err)
	}
}

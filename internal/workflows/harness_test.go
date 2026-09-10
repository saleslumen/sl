package workflows

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

type capturedRequest struct {
	method  string
	host    string
	path    string
	query   string
	headers http.Header
	body    string
}

type factoryOptions struct {
	stdin     io.Reader
	jsonMode  bool
	stdoutTTY bool
	confirm   func(string) error
}

func testClient(t *testing.T, srv *httptest.Server, namespace string) *apiclient.Client {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return apiclient.New(apiclient.Options{APIKey: testKey, NamespaceID: namespace, APIDomain: "example.test", UserAgent: "sl/test", HTTPClient: &http.Client{Transport: hostRewrite{target: u, next: http.DefaultTransport}}})
}

func testFactory(t *testing.T, client *apiclient.Client, opts factoryOptions) (*cli.Factory, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	if opts.stdin == nil {
		opts.stdin = bytes.NewReader(nil)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if opts.confirm == nil {
		opts.confirm = func(string) error { return nil }
	}
	return &cli.Factory{
		IO: &cli.IOStreams{In: opts.stdin, Out: stdout, Err: stderr, IsStdoutTTY: opts.stdoutTTY},
		Client: func() (*apiclient.Client, error) {
			return client, nil
		},
		Organization: func() (string, error) {
			return "org-configured", nil
		},
		Printer: func() *output.Printer {
			return output.New(stdout, output.Options{JSON: opts.jsonMode, TTY: opts.stdoutTTY})
		},
		Confirm: opts.confirm,
	}, stdout, stderr
}

func execute(t *testing.T, f *cli.Factory, args ...string) error {
	t.Helper()
	cmd := NewCommand(f)
	cmd.SetArgs(args)
	cmd.SetIn(f.IO.In)
	cmd.SetOut(f.IO.Out)
	cmd.SetErr(f.IO.Err)
	return cmd.Execute()
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
	if got.Get("sl-organization-id") != "" {
		t.Fatalf("sl-organization-id=%q", got.Get("sl-organization-id"))
	}
}

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("got json: %v body=%s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("want json: %v body=%s", err, want)
	}
	gotRaw, err := json.Marshal(gotValue)
	if err != nil {
		t.Fatal(err)
	}
	wantRaw, err := json.Marshal(wantValue)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("body=%s want=%s", got, want)
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

func usageError(t *testing.T, err error) string {
	t.Helper()
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("error=%v", err)
	}
	return usage.Error()
}

func captureServer(t *testing.T, status int, response string, requests *[]capturedRequest) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		*requests = append(*requests, capturedRequest{method: r.Method, host: r.Host, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone(), body: string(raw)})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	return srv
}

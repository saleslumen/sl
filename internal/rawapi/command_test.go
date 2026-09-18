package rawapi

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
	"strconv"
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

func testClient(t *testing.T, srv *httptest.Server, namespace string) *apiclient.Client {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return apiclient.New(apiclient.Options{APIKey: testKey, NamespaceID: namespace, APIDomain: "example.test", UserAgent: "sl/test", HTTPClient: &http.Client{Transport: hostRewrite{target: u, next: http.DefaultTransport}}})
}

func executeAPI(t *testing.T, client *apiclient.Client, stdin io.Reader, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	f := &cli.Factory{
		IO: &cli.IOStreams{In: stdin, Out: &out, Err: io.Discard},
		Client: func() (*apiclient.Client, error) {
			return client, nil
		},
		Printer: func() *output.Printer {
			return output.New(&out, output.Options{})
		},
	}
	cmd := NewCommand(f)
	cmd.SetArgs(args)
	cmd.SetIn(io.NopCloser(strings.NewReader("")))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	return out.String(), err
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

func TestAPISendsMethodPathQueryHeadersAndBody(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"campaigns":[]}`))
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	input := filepath.Join(dir, "body.json")
	if err := os.WriteFile(input, []byte(`{"displayName":"Acme"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := executeAPI(t, testClient(t, srv, "ns-1"), nil, "campaigns", "/v1/campaigns", "-X", "POST", "-F", "page_size=1", "--input", input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != `{"campaigns":[]}` {
		t.Fatalf("stdout: %s", out)
	}
	if method != http.MethodPost || host != "campaigns.example.test" || path != "/v1/campaigns" || rawQuery != "page_size=1" {
		t.Fatalf("request %s %s %s?%s", method, host, path, rawQuery)
	}
	assertApplicationHeaders(t, headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if body != `{"displayName":"Acme"}` {
		t.Fatalf("body: %s", body)
	}
}

func TestAPIAcceptsRawProductLabels(t *testing.T) {
	var hosts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hosts = append(hosts, r.Host)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	client := testClient(t, srv, "")
	for _, product := range []string{"campaigns", "emails", "workflows", "script", "resources"} {
		if _, err := executeAPI(t, client, nil, product, "/v1/probe"); err != nil {
			t.Fatalf("%s: %v", product, err)
		}
	}
	want := []string{"campaigns.example.test", "emails.example.test", "workflows.example.test", "script.example.test", "resources.example.test"}
	if strings.Join(hosts, ",") != strings.Join(want, ",") {
		t.Fatalf("hosts %#v", hosts)
	}
}

func TestAPIRejectsUnknownProduct(t *testing.T) {
	_, err := executeAPI(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, "management", "/v1/namespaces")
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("usage: %v", err)
	}
	if !strings.Contains(usage.Msg, "resources") || !strings.Contains(usage.Msg, "campaigns") {
		t.Fatalf("message: %s", usage.Msg)
	}
}

func TestAPIReadsStdinInput(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		if r.Header.Get("sl-namespace-id") != "" {
			t.Error("namespace present")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	out, err := executeAPI(t, testClient(t, srv, ""), strings.NewReader(`{"name":"n1"}`), "resources", "/v1/namespaces", "-X", "POST", "--input", "-")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if body != `{"name":"n1"}` || out != `{"ok":true}` {
		t.Fatalf("body=%s out=%s", body, out)
	}
}

func TestAPIRejectsInvalidInputJSON(t *testing.T) {
	_, err := executeAPI(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), strings.NewReader("not-json"), "emails", "/v1/accounts", "--input", "-")
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("usage: %v", err)
	}
}

func TestAPIRejectsEmptyInput(t *testing.T) {
	client := apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"})
	_, err := executeAPI(t, client, strings.NewReader("  \n"), "emails", "/v1/accounts", "--input", "-")
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("empty stdin: %v", err)
	}
	input := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(input, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = executeAPI(t, client, nil, "emails", "/v1/accounts", "--input", input)
	if !errors.As(err, &usage) {
		t.Fatalf("empty file: %v", err)
	}
}

func TestAPIRequiresInjectedDeps(t *testing.T) {
	t.Run("client", func(t *testing.T) {
		cmd := NewCommand(&cli.Factory{IO: &cli.IOStreams{Out: io.Discard}, Printer: func() *output.Printer {
			return output.New(io.Discard, output.Options{})
		}})
		cmd.SetArgs([]string{"campaigns", "/v1/campaigns"})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "client is required") {
			t.Fatalf("err: %v", err)
		}
	})
	t.Run("print", func(t *testing.T) {
		cmd := NewCommand(&cli.Factory{
			IO: &cli.IOStreams{Out: io.Discard},
			Client: func() (*apiclient.Client, error) {
				return apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil
			},
		})
		cmd.SetArgs([]string{"campaigns", "/v1/campaigns"})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "printer is required") {
			t.Fatalf("err: %v", err)
		}
	})
	t.Run("nil-client", func(t *testing.T) {
		cmd := NewCommand(&cli.Factory{
			IO:     &cli.IOStreams{Out: io.Discard},
			Client: func() (*apiclient.Client, error) { return nil, nil },
			Printer: func() *output.Printer {
				return output.New(io.Discard, output.Options{})
			},
		})
		cmd.SetArgs([]string{"campaigns", "/v1/campaigns"})
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "client is required") {
			t.Fatalf("err: %v", err)
		}
	})
}

func TestPaginateConcatenatesSingleArray(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens = append(tokens, r.URL.Query().Get("pageToken"))
		switch r.URL.Query().Get("pageToken") {
		case "":
			_, _ = w.Write([]byte(`{"campaigns":[{"name":"a"}],"nextPageToken":"p2","totalSize":2}`))
		case "p2":
			_, _ = w.Write([]byte(`{"campaigns":[{"name":"b"}],"nextPageToken":""}`))
		default:
			t.Errorf("unexpected token %q", r.URL.Query().Get("pageToken"))
		}
	}))
	t.Cleanup(srv.Close)
	out, err := executeAPI(t, testClient(t, srv, ""), nil, "campaigns", "/v1/campaigns", "--paginate")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(tokens) != 2 || tokens[0] != "" || tokens[1] != "p2" {
		t.Fatalf("tokens %#v", tokens)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json: %v", err)
	}
	if string(got["campaigns"]) != `[{"name":"a"},{"name":"b"}]` {
		t.Fatalf("items: %s", got["campaigns"])
	}
	if _, ok := got["nextPageToken"]; ok {
		t.Fatal("token retained")
	}
	if string(got["totalSize"]) != "2" {
		t.Fatalf("totalSize: %s", got["totalSize"])
	}
}

func TestPaginateFollowsSnakeCaseToken(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens = append(tokens, r.URL.Query().Get("page_token"))
		if r.URL.Query().Get("page_token") == "" {
			_, _ = w.Write([]byte(`{"items":[1],"next_page_token":"n2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[2]}`))
	}))
	t.Cleanup(srv.Close)
	out, err := executeAPI(t, testClient(t, srv, ""), nil, "emails", "/v1/accounts", "--paginate")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(tokens) != 2 || tokens[1] != "n2" {
		t.Fatalf("tokens %#v", tokens)
	}
	if !strings.Contains(out, `[1,2]`) {
		t.Fatalf("out: %s", out)
	}
}

func TestPaginateRequiresExactlyOneArrayField(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "none", body: `{"nextPageToken":""}`},
		{name: "two", body: `{"campaigns":[],"items":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			_, err := executeAPI(t, testClient(t, srv, ""), nil, "workflows", "/v1/workflows", "--paginate")
			if err == nil || !strings.Contains(err.Error(), "exactly one array field") {
				t.Fatalf("err: %v", err)
			}
		})
	}
}

func TestPaginateHasFiniteSafetyLimit(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"items":[1],"nextPageToken":"t` + strconv.Itoa(hits) + `"}`))
	}))
	t.Cleanup(srv.Close)
	_, err := executeAPI(t, testClient(t, srv, ""), nil, "script", "/v1/projects", "--paginate")
	if err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("err: %v", err)
	}
	if hits != maxPaginatePages {
		t.Fatalf("hits %d, want %d", hits, maxPaginatePages)
	}
}

func TestPaginateRejectsRepeatedToken(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"items":[1],"nextPageToken":"stuck"}`))
	}))
	t.Cleanup(srv.Close)
	_, err := executeAPI(t, testClient(t, srv, ""), nil, "campaigns", "/v1/campaigns", "--paginate")
	if err == nil || !strings.Contains(err.Error(), "repeated page token") {
		t.Fatalf("err: %v", err)
	}
	if hits != 2 {
		t.Fatalf("hits %d", hits)
	}
}

func TestPaginateRejectsNonGET(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	t.Cleanup(srv.Close)
	_, err := executeAPI(t, testClient(t, srv, ""), nil, "campaigns", "/v1/campaigns", "-X", "POST", "--paginate")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "GET") {
		t.Fatalf("err: %v", err)
	}
	if hits != 0 {
		t.Fatalf("hits %d", hits)
	}
}

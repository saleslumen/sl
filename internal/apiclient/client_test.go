package apiclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
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

func testOptions(t *testing.T, srv *httptest.Server) Options {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return Options{APIKey: testKey, APIDomain: "example.test", UserAgent: "sl/test", HTTPClient: &http.Client{Transport: hostRewrite{target: u, next: http.DefaultTransport}}}
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

func TestDoSendsMethodPathQueryHeadersAndBody(t *testing.T) {
	var method, path, rawQuery, body string
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		method = r.Method
		path = r.URL.Path
		rawQuery = r.URL.RawQuery
		headers = r.Header.Clone()
		body = string(raw)
		w.Header().Set("X-Request-Id", "req-1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	opts := testOptions(t, srv)
	opts.NamespaceID = "ns-1"
	client := New(opts)
	query := url.Values{"pageSize": []string{"1"}}
	resp, err := client.Do(context.Background(), Request{Product: "campaigns", Method: http.MethodPost, Path: "/v1/campaigns", Query: query, Body: map[string]string{"displayName": "Acme"}})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("status: %d", resp.Status)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("body: %s", resp.Body)
	}
	if resp.Header.Get("X-Request-Id") != "req-1" {
		t.Fatalf("response header: %q", resp.Header.Get("X-Request-Id"))
	}
	if method != http.MethodPost || path != "/v1/campaigns" || rawQuery != "pageSize=1" {
		t.Fatalf("request %s %s?%s", method, path, rawQuery)
	}
	assertApplicationHeaders(t, headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if body != `{"displayName":"Acme"}` {
		t.Fatalf("body: %s", body)
	}
}

func TestDoOmitsOptionalHeadersWithoutBody(t *testing.T) {
	t.Setenv("SL_ORGANIZATION_ID", "org-configured")
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	client := New(testOptions(t, srv))
	if _, err := client.Do(context.Background(), Request{Product: "emails", Method: http.MethodGet, Path: "/v1/accounts"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	assertApplicationHeaders(t, headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if got := headers.Get("sl-organization-id"); got != "" {
		t.Fatalf("sl-organization-id=%q", got)
	}
}

type deadlineCapture struct {
	next http.RoundTripper
	at   time.Time
	has  bool
}

func (d *deadlineCapture) RoundTrip(req *http.Request) (*http.Response, error) {
	d.at, d.has = req.Context().Deadline()
	return d.next.RoundTrip(req)
}

func TestDoAppliesTimeoutWithoutMutatingClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	opts := testOptions(t, srv)
	capture := &deadlineCapture{next: opts.HTTPClient.Transport}
	opts.HTTPClient.Transport = capture
	injected := opts.HTTPClient
	before := injected.Timeout
	client := New(opts)
	if _, err := client.Do(context.Background(), Request{Product: "workflows", Method: http.MethodGet, Path: "/v1/workflows"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if injected.Timeout != before {
		t.Fatalf("mutated timeout: %s", injected.Timeout)
	}
	if !capture.has {
		t.Fatal("missing request deadline")
	}
	remain := time.Until(capture.at)
	if remain < 29*time.Second || remain > requestTimeout {
		t.Fatalf("deadline remaining %s", remain)
	}
}

func TestDoLogsVerboseLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	var logged string
	opts := testOptions(t, srv)
	opts.Logger = func(line string) { logged = line }
	client := New(opts)
	if _, err := client.Do(context.Background(), Request{Product: "script", Method: http.MethodGet, Path: "/v1/projects"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if logged != "GET script.example.test /v1/projects → 200" {
		t.Fatalf("log: %q", logged)
	}
}

func TestDoRejectsInvalidRequest(t *testing.T) {
	client := New(Options{APIKey: testKey, UserAgent: "sl/test"})
	if _, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "/v1/campaigns"}); err == nil || !strings.Contains(err.Error(), "product") {
		t.Fatalf("product: %v", err)
	}
	if _, err := client.Do(context.Background(), Request{Product: "campaigns", Path: "/v1/campaigns"}); err == nil || !strings.Contains(err.Error(), "method") {
		t.Fatalf("method: %v", err)
	}
	if _, err := client.Do(context.Background(), Request{Product: "campaigns", Method: http.MethodGet, Path: "v1/campaigns"}); err == nil || !strings.Contains(err.Error(), "path") {
		t.Fatalf("path: %v", err)
	}
}

func TestDoAcceptsRawJSONBody(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	client := New(testOptions(t, srv))
	if _, err := client.Do(context.Background(), Request{Product: "resources", Method: http.MethodPatch, Path: "/v1/namespaces", Body: json.RawMessage(`{"displayName":"A"}`)}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if body != `{"displayName":"A"}` {
		t.Fatalf("body: %s", body)
	}
}

func TestRequestCredentialHeaders(t *testing.T) {
	const access = "access-token-fixture"
	cases := []struct {
		name        string
		apiKey      string
		accessToken string
		wantBearer  bool
		wantAPIKey  bool
		wantErr     bool
	}{
		{name: "api-key-only", apiKey: testKey, wantAPIKey: true},
		{name: "oauth-only", accessToken: access, wantBearer: true},
		{name: "both-set", apiKey: testKey, accessToken: access, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			var headers http.Header
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				headers = r.Header.Clone()
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}))
			t.Cleanup(srv.Close)
			opts := testOptions(t, srv)
			opts.APIKey = tc.apiKey
			opts.AccessToken = tc.accessToken
			_, err := New(opts).Do(context.Background(), Request{Product: "campaigns", Method: http.MethodGet, Path: "/v1/campaigns"})
			if tc.wantErr {
				if !errors.Is(err, ErrMixedCredentials) {
					t.Fatalf("err=%v", err)
				}
				if called {
					t.Fatal("mixed credentials reached the server")
				}
				return
			}
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			if tc.wantAPIKey {
				if headers.Get("sl-api-key") != testKey || headers.Get("Authorization") != "" {
					t.Fatal("api-key request headers were not exclusive")
				}
				return
			}
			if headers.Get("Authorization") != "Bearer "+access || headers.Get("sl-api-key") != "" {
				t.Fatal("oauth request headers were not exclusive")
			}
		})
	}
}

func TestStreamSendsBearerOnly(t *testing.T) {
	const access = "access-token-fixture"
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "event: finished\ndata: {}\n\n")
	}))
	t.Cleanup(srv.Close)
	opts := testOptions(t, srv)
	opts.APIKey = ""
	opts.AccessToken = access
	err := New(opts).Stream(context.Background(), Request{Product: "campaigns", Method: http.MethodGet, Path: "/v1/tasks/t1:stream"}, func(name string, data []byte) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if headers.Get("Authorization") != "Bearer "+access || headers.Get("sl-api-key") != "" {
		t.Fatal("stream headers were not exclusive")
	}
}

func TestDoSendsByteSliceBodyVerbatim(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	client := New(testOptions(t, srv))
	if _, err := client.Do(context.Background(), Request{Product: "script", Method: http.MethodPut, Path: "/v1/projects/p/content", Body: []byte(`{"content":{"files":[]}}`)}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if body != `{"content":{"files":[]}}` {
		t.Fatalf("body: %s", body)
	}
}

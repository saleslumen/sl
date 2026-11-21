package marketplace

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
)

const testAccess = "oauth-access-token-fixture"

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
	method string
	host   string
	path   string
	query  string
	body   string
}

func testTransport(t *testing.T, srv *httptest.Server) *http.Client {
	t.Helper()
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Transport: hostRewrite{target: target, next: http.DefaultTransport}}
}

func testFactory(t *testing.T, srv *httptest.Server, stdin io.Reader, oauth func() error) (*cli.Factory, *bytes.Buffer) {
	t.Helper()
	if stdin == nil {
		stdin = bytes.NewReader(nil)
	}
	stdout := &bytes.Buffer{}
	client := apiclient.New(apiclient.Options{AccessToken: testAccess, APIDomain: "example.test", UserAgent: "sl/test", HTTPClient: testTransport(t, srv)})
	return &cli.Factory{
		IO:           &cli.IOStreams{In: stdin, Out: stdout, Err: io.Discard},
		Client:       func() (*apiclient.Client, error) { return client, nil },
		Organization: func() (string, error) { return "org-1", nil },
		Printer:      func() *output.Printer { return output.New(stdout, output.Options{}) },
		Confirm:      func(string) error { return nil },
		RequireUserOAuth: func() error {
			if oauth != nil {
				return oauth()
			}
			return nil
		},
	}, stdout
}

func captureServer(t *testing.T, status int, response string, requests *[]capturedRequest) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		*requests = append(*requests, capturedRequest{method: r.Method, host: r.Host, path: r.URL.Path, query: r.URL.RawQuery, body: string(raw)})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	return srv
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

func TestListCatalogSkipsUserOAuth(t *testing.T) {
	var requests []capturedRequest
	body := `{"apps":[{"appId":"app-1","lifecycleState":"ACTIVE","nameDisplay":"Outbound"}],"nextPageToken":""}`
	f, out := testFactory(t, captureServer(t, http.StatusOK, body, &requests), nil, func() error {
		return cli.ErrUserOAuthRequired
	})
	if err := execute(t, f, "list"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].host != "marketplace.example.test" || requests[0].path != "/v1/apps" || requests[0].query != "pageSize=50" {
		t.Fatalf("request %+v", requests)
	}
	if out.String() != "app-1\tACTIVE\tOutbound\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestGetAppRequiresUserOAuth(t *testing.T) {
	var requests []capturedRequest
	f, _ := testFactory(t, captureServer(t, http.StatusOK, `{}`, &requests), nil, func() error {
		return cli.ErrUserOAuthRequired
	})
	err := execute(t, f, "get", "app-1")
	if !errors.Is(err, cli.ErrUserOAuthRequired) {
		t.Fatalf("err=%v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("requests=%d", len(requests))
	}
}

func TestListRejectsNegativeLimit(t *testing.T) {
	f, _ := testFactory(t, captureServer(t, http.StatusOK, `{}`, &[]capturedRequest{}), nil, nil)
	err := execute(t, f, "list", "--limit", "-1")
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err=%v", err)
	}
}

func TestUninstallSendsRetentionChoice(t *testing.T) {
	var requests []capturedRequest
	f, _ := testFactory(t, captureServer(t, http.StatusOK, ``, &requests), nil, nil)
	if err := execute(t, f, "installations", "uninstall", "inst-1", "--retention-choice", "RETAIN_DATA"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].method != http.MethodDelete || requests[0].path != "/v1/installations/inst-1" || requests[0].query != "retentionChoice=RETAIN_DATA" {
		t.Fatalf("request %+v", requests)
	}
}

func TestSetAPIKeyBindsWithoutPrintingTheKey(t *testing.T) {
	const secret = "super-secret-key"
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		requests = append(requests, capturedRequest{method: r.Method, host: r.Host, path: r.URL.Path, query: r.URL.RawQuery, body: string(raw)})
		w.WriteHeader(http.StatusOK)
		switch {
		case strings.HasSuffix(r.URL.Path, ":initiateApiKey"):
			_, _ = w.Write([]byte(`{"operationId":"ignored","expireTime":"2026-09-25T00:00:00Z"}`))
		case r.URL.Path == scriptBindRoute.Path:
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`{"outcome":"COMMITTED"}`))
		}
	}))
	t.Cleanup(srv.Close)
	f, out := testFactory(t, srv, strings.NewReader(secret), nil)
	if err := execute(t, f, "connections", "set-api-key", "zapmail", "--installation", "inst-1", "--api-key-file", "-"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 3 {
		t.Fatalf("requests=%d %+v", len(requests), requests)
	}
	if requests[0].host != "marketplace.example.test" || requests[0].path != "/v1/installations/inst-1/connections/zapmail:initiateApiKey" {
		t.Fatalf("initiate %+v", requests[0])
	}
	if requests[1].host != "script.example.test" || requests[1].path != scriptBindRoute.Path || !strings.Contains(requests[1].body, secret) {
		t.Fatalf("bind %+v", requests[1])
	}
	if requests[2].host != "marketplace.example.test" || requests[2].path != "/v1/installationConnectionApiKeyOperations:complete" {
		t.Fatalf("complete %+v", requests[2])
	}
	if !strings.Contains(requests[0].body, `"operationId"`) || !strings.Contains(requests[2].body, `"operationId"`) || requests[0].body != requests[2].body {
		t.Fatalf("operation ids %s %s", requests[0].body, requests[2].body)
	}
	if strings.Contains(out.String(), secret) || strings.Contains(requests[0].body, secret) || strings.Contains(requests[2].body, secret) {
		t.Fatal("api key leaked")
	}
	if out.String() != "COMMITTED\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestSetAPIKeyUsesCompletionAfterBindError(t *testing.T) {
	const secret = "super-secret-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == scriptBindRoute.Path {
			http.Error(w, "bind failed", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		if strings.HasSuffix(r.URL.Path, ":complete") {
			_, _ = w.Write([]byte(`{"outcome":"NOT_COMMITTED"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	f, out := testFactory(t, srv, strings.NewReader(secret), nil)
	if err := execute(t, f, "connections", "set-api-key", "zapmail", "--installation", "inst-1", "--api-key-file", "-"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "NOT_COMMITTED\n" || strings.Contains(out.String(), secret) {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestSetAPIKeyReportsOperationWhenCompleteFails(t *testing.T) {
	const secret = "super-secret-key"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ":complete") {
			http.Error(w, "complete failed", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	f, out := testFactory(t, srv, strings.NewReader(secret), nil)
	err := execute(t, f, "connections", "set-api-key", "zapmail", "--installation", "inst-1", "--api-key-file", "-")
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(out.String(), secret) {
		t.Fatalf("err=%v stdout=%q", err, out.String())
	}
	if !strings.Contains(err.Error(), "sl marketplace api-key-operations complete ") {
		t.Fatalf("err=%v", err)
	}
}

func TestSetAPIKeyRejectsBlankKeyBeforeHTTP(t *testing.T) {
	var requests []capturedRequest
	f, _ := testFactory(t, captureServer(t, http.StatusOK, `{}`, &requests), strings.NewReader(" \n\t"), nil)
	err := execute(t, f, "connections", "set-api-key", "zapmail", "--installation", "inst-1", "--api-key-file", "-")
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err=%v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("requests=%d", len(requests))
	}
}

func TestSetAPIKeyRequiresOAuthBeforeReadingTheKey(t *testing.T) {
	f, _ := testFactory(t, captureServer(t, http.StatusOK, `{}`, &[]capturedRequest{}), strings.NewReader("unused"), func() error {
		return cli.ErrUserOAuthRequired
	})
	err := execute(t, f, "connections", "set-api-key", "zapmail", "--installation", "inst-1", "--api-key-file", "-")
	if !errors.Is(err, cli.ErrUserOAuthRequired) {
		t.Fatalf("err=%v", err)
	}
}

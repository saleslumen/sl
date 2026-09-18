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
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

const (
	deliveryFixture = `{"name":"deliveries/d1","person":"people/p1","step":"steps/s1","sender_account_id":"sa1","status":"SENT","scheduled_at":"2026-08-01T00:00:00Z","update_time":"2026-08-02T00:00:00Z"}`
	deliveryTable   = "d1\tp1\ts1\tsa1\tSENT\t2026-08-01T00:00:00Z\t2026-08-02T00:00:00Z\n"
	deliveryReqID   = "00000000-0000-4000-8000-000000000001"
	analytics502    = `{"code":"UNAVAILABLE","message":"analytics unavailable"}`
)

type capturedRequest struct {
	Method  string
	Host    string
	Path    string
	Query   string
	Headers http.Header
	Body    string
}

func deliveriesFactory(t *testing.T, stdin io.Reader, stdout io.Writer, jsonMode bool) *cli.Factory {
	t.Helper()
	f := testFactory(stdin, stdout)
	f.Organization = func() (string, error) {
		t.Fatal("Organization must not be called")
		return "", nil
	}
	f.Printer = func() *output.Printer {
		return output.New(stdout, output.Options{JSON: jsonMode})
	}
	return f
}

func executeDeliveries(t *testing.T, f *cli.Factory, args ...string) error {
	t.Helper()
	cmd := newDeliveriesCommand(f)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func writeDeliveryInput(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func captureCampaigns(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]capturedRequest) {
	t.Helper()
	var got []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		got = append(got, capturedRequest{Method: r.Method, Host: r.Host, Path: r.URL.Path, Query: r.URL.RawQuery, Headers: r.Header.Clone(), Body: string(raw)})
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func writeOK(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}
}

func writeStatus(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func assertCampaignRequest(t *testing.T, got capturedRequest, method, path, query, body, namespace string, contentType bool) {
	t.Helper()
	if got.Method != method || got.Host != "campaigns.example.test" || got.Path != path || got.Query != query || got.Body != body {
		t.Fatalf("request %s %s %s?%s body=%q", got.Method, got.Host, got.Path, got.Query, got.Body)
	}
	want := map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"}
	if namespace != "" {
		want["Sl-Namespace-Id"] = namespace
	}
	if contentType {
		want["Content-Type"] = "application/json"
	}
	assertApplicationHeaders(t, got.Headers, want)
	if got.Headers.Get("sl-organization-id") != "" || got.Headers.Get("Authorization") != "" {
		t.Fatalf("forbidden headers %#v", applicationHeaders(got.Headers))
	}
}

func assertUsageErr(t *testing.T, err error, substr string) {
	t.Helper()
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, substr) {
		t.Fatalf("usage=%v want %q", err, substr)
	}
}

func assertUnavailable(t *testing.T, err error, method, path string) {
	t.Helper()
	var api *apiclient.Error
	if !errors.As(err, &api) || api.Status != http.StatusBadGateway || api.Code != "UNAVAILABLE" || api.Product != "campaigns" || api.Method != method || api.Path != path {
		t.Fatalf("502: %#v", err)
	}
}

func TestDeliveriesCommandWiring(t *testing.T) {
	var stdout bytes.Buffer
	called := false
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	cmd := newDeliveriesCommand(f)
	if cmd.Use != "deliveries" || cmd.Short != "Inspect campaign deliveries" || strings.Contains(cmd.Short, "\n") {
		t.Fatalf("use=%q short=%q", cmd.Use, cmd.Short)
	}
	want := map[string]string{"list": "List campaign deliveries", "get": "Get a delivery", "resolve-unknown": "Resolve an unknown delivery"}
	got := map[string]*cobra.Command{}
	for _, child := range cmd.Commands() {
		got[child.Name()] = child
		if want[child.Name()] != child.Short || strings.Contains(child.Short, "\n") {
			t.Fatalf("%s short=%q", child.Name(), child.Short)
		}
	}
	for name := range want {
		if got[name] == nil {
			t.Fatalf("missing %s in %#v", name, got)
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
	for name := range want {
		if !strings.Contains(help, name) {
			t.Fatalf("help missing %s: %s", name, help)
		}
	}
	list := got["list"]
	if list.Use != "list --campaign ID" || got["get"].Use != "get ID" || got["resolve-unknown"].Use != "resolve-unknown ID --input FILE" {
		t.Fatalf("use list=%q get=%q resolve=%q", list.Use, got["get"].Use, got["resolve-unknown"].Use)
	}
	for _, flag := range []string{"campaign", "limit", "person-id", "status", "step-id", "sender-account-id", "failure-code", "scheduled-after", "scheduled-before"} {
		if list.Flags().Lookup(flag) == nil {
			t.Fatalf("list missing --%s", flag)
		}
	}
	if got["resolve-unknown"].Flags().Lookup("input") == nil || got["resolve-unknown"].Flags().Lookup("request-id") == nil {
		t.Fatal("resolve-unknown missing control flags")
	}
	if usage := list.Flags().Lookup("status").Usage; !strings.Contains(usage, "ALLOCATING") || !strings.Contains(usage, "FAILED") {
		t.Fatalf("status usage=%q", usage)
	}
}

func TestDeliveriesListDefaultQueryHeadersAndTable(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(`{"deliveries":[`+deliveryFixture+`]}`))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeDeliveries(t, f, "list", "--campaign", "c1"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(*recs) != 1 {
		t.Fatalf("hits=%d", len(*recs))
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/campaigns/c1/deliveries", "page_size=50", "", "", false)
	if stdout.String() != deliveryTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestDeliveriesListFiltersAndNamespace(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(`{"deliveries":[`+deliveryFixture+`]}`))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-1"), nil }
	if err := executeDeliveries(t, f, "list", "--campaign", "c1", "--limit", "3", "--person-id", "p1", "--status", "FAILED", "--step-id", "s1", "--sender-account-id", "sa1", "--failure-code", "HARD_BOUNCE", "--scheduled-after", "2026-08-01T00:00:00Z", "--scheduled-before", "2026-09-01T00:00:00Z"); err != nil {
		t.Fatalf("list: %v", err)
	}
	wantQuery := "failure_code=HARD_BOUNCE&page_size=3&person_id=p1&scheduled_after=2026-08-01T00%3A00%3A00Z&scheduled_before=2026-09-01T00%3A00%3A00Z&sender_account_id=sa1&status=FAILED&step_id=s1"
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/campaigns/c1/deliveries", wantQuery, "", "ns-1", false)
}

func TestDeliveriesListStatusPassthrough(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(`{"deliveries":[]}`))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeDeliveries(t, f, "list", "--campaign", "c1", "--status", "BOGUS"); err != nil {
		t.Fatalf("list: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/campaigns/c1/deliveries", "page_size=50&status=BOGUS", "", "", false)
}

func TestDeliveriesListPaginationAndJSON(t *testing.T) {
	page1 := `{"deliveries":[{"name":"deliveries/d1","person":"people/p1","step":"steps/s1","sender_account_id":"sa1","status":"SENT","scheduled_at":"2026-08-01T00:00:00Z","update_time":"2026-08-02T00:00:00Z"}],"next_page_token":"p2"}`
	page2 := `{"deliveries":[{"name":"deliveries/d2","person":"people/p2","step":"steps/s2","sender_account_id":"sa2","status":"FAILED","scheduled_at":null,"update_time":"2026-08-03T00:00:00Z"}]}`
	hits := 0
	srv, recs := captureCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			_, _ = w.Write([]byte(page1))
			return
		}
		_, _ = w.Write([]byte(page2))
	})
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeDeliveries(t, f, "list", "--campaign", "c1", "--limit", "2"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(*recs) != 2 {
		t.Fatalf("hits=%d", len(*recs))
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/campaigns/c1/deliveries", "page_size=2", "", "", false)
	assertCampaignRequest(t, (*recs)[1], http.MethodGet, "/v1/campaigns/c1/deliveries", "page_size=1&page_token=p2", "", "", false)
	wantTable := deliveryTable + "d2\tp2\ts2\tsa2\tFAILED\t\t2026-08-03T00:00:00Z\n"
	if stdout.String() != wantTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
	stdout.Reset()
	hits = 0
	*recs = nil
	f = deliveriesFactory(t, nil, &stdout, true)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeDeliveries(t, f, "list", "--campaign", "c1", "--limit", "2"); err != nil {
		t.Fatalf("json: %v", err)
	}
	var payload map[string][]json.RawMessage
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json decode: %v body=%s", err, stdout.String())
	}
	if len(payload["deliveries"]) != 2 {
		t.Fatalf("json items=%d body=%s", len(payload["deliveries"]), stdout.String())
	}
}

func TestDeliveriesListEmptyAndCappedPageSize(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(`{"deliveries":[]}`))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, true)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeDeliveries(t, f, "list", "--campaign", "c1", "--limit", "500"); err != nil {
		t.Fatalf("list: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/campaigns/c1/deliveries", "page_size=200", "", "", false)
	if stdout.String() != `{"deliveries":[]}` {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestDeliveriesListUsageAnd502(t *testing.T) {
	f := deliveriesFactory(t, nil, io.Discard, false)
	f.Client = func() (*apiclient.Client, error) {
		t.Fatal("client")
		return nil, nil
	}
	assertUsageErr(t, executeDeliveries(t, f, "list"), "--campaign")
	assertUsageErr(t, executeDeliveries(t, f, "list", "--campaign", "c1", "--limit", "0"), "--limit")
	srv, _ := captureCampaigns(t, writeStatus(http.StatusBadGateway, analytics502))
	ok := deliveriesFactory(t, nil, io.Discard, false)
	ok.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	err := executeDeliveries(t, ok, "list", "--campaign", "c1")
	assertUnavailable(t, err, http.MethodGet, "/v1/campaigns/c1/deliveries")
}

func TestDeliveriesGetPathHeadersTableAndJSON(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(deliveryFixture))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-9"), nil }
	if err := executeDeliveries(t, f, "get", "deliveries/d1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/deliveries/d1", "", "", "ns-9", false)
	if stdout.String() != deliveryTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
	stdout.Reset()
	*recs = nil
	f = deliveriesFactory(t, nil, &stdout, true)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeDeliveries(t, f, "get", "d1"); err != nil {
		t.Fatalf("json: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodGet, "/v1/deliveries/d1", "", "", "", false)
	if stdout.String() != deliveryFixture {
		t.Fatalf("json=%q", stdout.String())
	}
}

func TestDeliveriesGetUsageDecodeAnd502(t *testing.T) {
	f := deliveriesFactory(t, nil, io.Discard, false)
	f.Client = func() (*apiclient.Client, error) {
		t.Fatal("client")
		return nil, nil
	}
	if err := executeDeliveries(t, f, "get"); err == nil {
		t.Fatal("expected args error")
	}
	assertUsageErr(t, executeDeliveries(t, f, "get", "   "), "delivery id")
	srv, _ := captureCampaigns(t, writeOK(`[]`))
	bad := deliveriesFactory(t, nil, io.Discard, false)
	bad.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeDeliveries(t, bad, "get", "d1"); err == nil || !strings.Contains(err.Error(), "decode delivery") {
		t.Fatalf("decode: %v", err)
	}
	down, _ := captureCampaigns(t, writeStatus(http.StatusBadGateway, analytics502))
	fail := deliveriesFactory(t, nil, io.Discard, false)
	fail.Client = func() (*apiclient.Client, error) { return testClient(t, down, ""), nil }
	assertUnavailable(t, executeDeliveries(t, fail, "get", "d1"), http.MethodGet, "/v1/deliveries/d1")
}

func TestDeliveriesResolveUnknownGeneratesRequestID(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(deliveryFixture))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-2"), nil }
	path := writeDeliveryInput(t, `{"outcome":"SENT","keep":true}`)
	if err := executeDeliveries(t, f, "resolve-unknown", "d1", "--input", path); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodPost, "/v1/deliveries/d1:resolveUnknown", "", `{"keep":true,"outcome":"SENT","request_id":"`+deliveryReqID+`"}`, "ns-2", true)
	if stdout.String() != deliveryTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestDeliveriesResolveUnknownRequestIDSourcesAndConflicts(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(deliveryFixture))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, nil, &stdout, true)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	path := writeDeliveryInput(t, `{"outcome":"RETRY","request_id":"input-id"}`)
	if err := executeDeliveries(t, f, "resolve-unknown", "deliveries/d1", "--input", path); err != nil {
		t.Fatalf("input id: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodPost, "/v1/deliveries/d1:resolveUnknown", "", `{"outcome":"RETRY","request_id":"input-id"}`, "", true)
	if stdout.String() != deliveryFixture {
		t.Fatalf("json=%q", stdout.String())
	}
	*recs = nil
	flagPath := writeDeliveryInput(t, `{"outcome":"SENT"}`)
	if err := executeDeliveries(t, f, "resolve-unknown", "d1", "--input", flagPath, "--request-id", "flag-id"); err != nil {
		t.Fatalf("flag id: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodPost, "/v1/deliveries/d1:resolveUnknown", "", `{"outcome":"SENT","request_id":"flag-id"}`, "", true)
	conflict := deliveriesFactory(t, nil, io.Discard, false)
	conflict.Client = func() (*apiclient.Client, error) {
		t.Fatal("client")
		return nil, nil
	}
	assertUsageErr(t, executeDeliveries(t, conflict, "resolve-unknown", "d1", "--input", path, "--request-id", "flag-id"), "--request-id")
}

func TestDeliveriesResolveUnknownStdinAndUsage(t *testing.T) {
	srv, recs := captureCampaigns(t, writeOK(deliveryFixture))
	var stdout bytes.Buffer
	f := deliveriesFactory(t, strings.NewReader(`{"outcome":"SENT"}`), &stdout, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeDeliveries(t, f, "resolve-unknown", "d1", "--input", "-"); err != nil {
		t.Fatalf("stdin: %v", err)
	}
	assertCampaignRequest(t, (*recs)[0], http.MethodPost, "/v1/deliveries/d1:resolveUnknown", "", `{"outcome":"SENT","request_id":"`+deliveryReqID+`"}`, "", true)
	missing := deliveriesFactory(t, nil, io.Discard, false)
	missing.Client = func() (*apiclient.Client, error) {
		t.Fatal("client")
		return nil, nil
	}
	assertUsageErr(t, executeDeliveries(t, missing, "resolve-unknown", "d1"), "--input")
	assertUsageErr(t, executeDeliveries(t, missing, "resolve-unknown", "d1", "--input", writeDeliveryInput(t, `[]`)), "JSON object")
	if err := executeDeliveries(t, missing, "resolve-unknown"); err == nil {
		t.Fatal("expected args error")
	}
	pass, recs2 := captureCampaigns(t, writeOK(deliveryFixture))
	passthrough := deliveriesFactory(t, nil, io.Discard, false)
	passthrough.Client = func() (*apiclient.Client, error) { return testClient(t, pass, ""), nil }
	if err := executeDeliveries(t, passthrough, "resolve-unknown", "d1", "--input", writeDeliveryInput(t, `{"keep":true}`)); err != nil {
		t.Fatalf("passthrough: %v", err)
	}
	assertCampaignRequest(t, (*recs2)[0], http.MethodPost, "/v1/deliveries/d1:resolveUnknown", "", `{"keep":true,"request_id":"`+deliveryReqID+`"}`, "", true)
}

func TestDeliveriesResolveUnknown502(t *testing.T) {
	srv, _ := captureCampaigns(t, writeStatus(http.StatusBadGateway, analytics502))
	f := deliveriesFactory(t, nil, io.Discard, false)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	err := executeDeliveries(t, f, "resolve-unknown", "d1", "--input", writeDeliveryInput(t, `{"outcome":"SENT"}`))
	assertUnavailable(t, err, http.MethodPost, "/v1/deliveries/d1:resolveUnknown")
}

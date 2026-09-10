package campaigns

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
)

func operationsFactory(t *testing.T, srv *httptest.Server, namespace string, stdout io.Writer, jsonOut bool) *cli.Factory {
	t.Helper()
	f := testFactory(nil, stdout)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, namespace), nil }
	f.Printer = func() *output.Printer { return output.New(stdout, output.Options{JSON: jsonOut}) }
	return f
}

func TestOperationsHelpIsImperativeAndLazy(t *testing.T) {
	var stdout bytes.Buffer
	called := false
	f := testFactory(nil, &stdout)
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	cmd := newOperationsCommand(f)
	if cmd.Use != "operations" || cmd.Short != "Inspect campaign operations" || strings.Contains(cmd.Short, "\n") {
		t.Fatalf("use=%q short=%q", cmd.Use, cmd.Short)
	}
	if len(cmd.Commands()) != 1 || cmd.Commands()[0].Name() != "get" || cmd.Commands()[0].Short != "Get an operation" || strings.Contains(cmd.Commands()[0].Short, "\n") {
		t.Fatalf("children=%v", cmd.Commands())
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
	if !strings.Contains(help, "Inspect campaign operations") || !strings.Contains(help, "get") {
		t.Fatalf("help=%q", help)
	}
}

func TestOperationsGetExactRouteHeadersAndTable(t *testing.T) {
	raw := `{"name":"operations/op-1","state":"RUNNING","campaign":"campaigns/c1","command_type":"PAUSE","cancelled_delivery_count":4,"already_attempting_delivery_count":1,"campaign_result":{"name":"campaigns/c1","state":"PAUSING"},"create_time":"2026-01-01T00:00:00Z","update_time":"2026-01-02T03:04:05Z"}`
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(raw))
	})
	var stdout bytes.Buffer
	f := operationsFactory(t, srv, "ns-3", &stdout, false)
	if err := executeNoun(t, newOperationsCommand(f), "get", "op-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	rec := (*got)[0]
	if rec.method != http.MethodGet || rec.host != "campaigns.example.test" || rec.path != "/v1/operations/op-1" || rec.rawQuery != "" || rec.body != "" {
		t.Fatalf("request %s %s %s?%s body=%q", rec.method, rec.host, rec.path, rec.rawQuery, rec.body)
	}
	assertApplicationHeaders(t, rec.headers, wantHeaders("ns-3", false))
	if rec.headers.Get("sl-organization-id") != "" || rec.headers.Get("Authorization") != "" {
		t.Fatalf("forbidden headers %#v", applicationHeaders(rec.headers))
	}
	if stdout.String() != "op-1\tc1\tRUNNING\tPAUSE\t4\t1\t2026-01-02T03:04:05Z\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestOperationsGetFailedStateRemainsSuccess(t *testing.T) {
	raw := `{"name":"operations/op-fail","state":"FAILED","campaign":"campaigns/c9","command_type":"UNSUBSCRIBE","cancelled_delivery_count":2,"already_attempting_delivery_count":null,"error":{"code":"INTERNAL","message":"settle failed"},"create_time":"2026-03-01T00:00:00Z","update_time":"2026-03-02T00:00:00Z"}`
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(raw))
	})
	var stdout bytes.Buffer
	f := operationsFactory(t, srv, "", &stdout, false)
	if err := executeNoun(t, newOperationsCommand(f), "get", "op-fail"); err != nil {
		t.Fatalf("FAILED state must remain success: %v", err)
	}
	if len(*got) != 1 || (*got)[0].method != http.MethodGet || (*got)[0].path != "/v1/operations/op-fail" {
		t.Fatalf("request %#v", *got)
	}
	assertApplicationHeaders(t, (*got)[0].headers, wantHeaders("", false))
	if stdout.String() != "op-fail\tc9\tFAILED\tUNSUBSCRIBE\t2\t\t2026-03-02T00:00:00Z\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestOperationsGetSucceededJSONPreservesCampaignResult(t *testing.T) {
	raw := `{"name":"operations/op-2","state":"SUCCEEDED","campaign":"campaigns/c2","command_type":"COMPLETE","cancelled_delivery_count":null,"already_attempting_delivery_count":0,"campaign_result":{"name":"campaigns/c2","state":"COMPLETED"},"create_time":"t0","update_time":"t1"}`
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(raw))
	})
	var stdout bytes.Buffer
	f := operationsFactory(t, srv, "", &stdout, true)
	if err := executeNoun(t, newOperationsCommand(f), "get", "op-2"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 || (*got)[0].path != "/v1/operations/op-2" || (*got)[0].rawQuery != "" {
		t.Fatalf("request %#v", *got)
	}
	if stdout.String() != raw {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestOperationsGetEscapedPathAndTTYHeaders(t *testing.T) {
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"operations/op/1","state":"SUCCEEDED","campaign":"campaigns/c/1","command_type":"PAUSE","cancelled_delivery_count":0,"already_attempting_delivery_count":0,"update_time":"t1"}`))
	})
	var stdout bytes.Buffer
	f := operationsFactory(t, srv, "", &stdout, false)
	f.Printer = func() *output.Printer { return output.New(&stdout, output.Options{TTY: true}) }
	if err := executeNoun(t, newOperationsCommand(f), "get", "op/1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 || (*got)[0].path != "/v1/operations/op%2F1" {
		t.Fatalf("path=%q", (*got)[0].path)
	}
	gotOut := stdout.String()
	if !strings.Contains(gotOut, "ID  CAMPAIGN ID  STATE      COMMAND  CANCELLED  ALREADY ATTEMPTING  UPDATED") || !strings.Contains(gotOut, "1   1            SUCCEEDED  PAUSE    0          0                   t1") {
		t.Fatalf("stdout=%q", gotOut)
	}
}

func TestOperationsGetRequiresID(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	f := operationsFactory(t, srv, "", io.Discard, false)
	err := executeNoun(t, newOperationsCommand(f), "get")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "operation id") || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
}

func TestOperationsGetNotFound(t *testing.T) {
	srv, got := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NOT_FOUND","message":"unknown operation"}`))
	})
	f := operationsFactory(t, srv, "", io.Discard, false)
	err := executeNoun(t, newOperationsCommand(f), "get", "missing")
	var apiErr *apiclient.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound || apiErr.Code != "NOT_FOUND" || apiErr.Method != http.MethodGet || apiErr.Path != "/v1/operations/missing" {
		t.Fatalf("err=%v", err)
	}
	if len(*got) != 1 || (*got)[0].path != "/v1/operations/missing" {
		t.Fatalf("request %#v", *got)
	}
}

func TestOperationsGetTransportFailureIsError(t *testing.T) {
	srv, _ := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"code":"INTERNAL","message":"boom"}`))
	})
	f := operationsFactory(t, srv, "", io.Discard, false)
	err := executeNoun(t, newOperationsCommand(f), "get", "op-1")
	var apiErr *apiclient.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusInternalServerError || apiErr.Code != "INTERNAL" {
		t.Fatalf("err=%v", err)
	}
}

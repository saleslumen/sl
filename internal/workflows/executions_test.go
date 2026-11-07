package workflows

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
)

func TestExecutionsListDefaultPageSizeAndTable(t *testing.T) {
	t.Setenv("SL_ORGANIZATION_ID", "org-configured")
	raw := `{"executions":[{"id":"exec-1","status":1,"sourceType":1,"version":3,"startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:01:00Z"}],"nextPageToken":"","totalCount":1}`
	var requests []capturedRequest
	f, stdout, stderr := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
	if err := execute(t, f, "executions", "list", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := requests[0]
	if got.method != http.MethodGet || got.path != "/v1/workflows/wf-1/executions" || got.query != "pageSize=50" || got.body != "" {
		t.Fatalf("request %s %s?%s body=%q", got.method, got.path, got.query, got.body)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if stdout.String() != "exec-1\tACTIVE\tMANUAL\t3\t2026-01-01T00:00:00Z\t2026-01-01T00:01:00Z\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestExecutionsListTTYUnknownEnumAndJSON(t *testing.T) {
	raw := `{"executions":[{"id":"exec-1","status":1,"sourceType":1,"version":3,"startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:01:00Z"},{"id":"exec-2","status":99,"sourceType":7,"version":0,"startedAt":"2026-02-01T00:00:00Z","completedAt":""}],"nextPageToken":""}`
	var requests []capturedRequest
	f, stdout, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{stdoutTTY: true})
	if err := execute(t, f, "executions", "list", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	wantTable := "ID      STATUS  SOURCE  VERSION  STARTED               COMPLETED\nexec-1  ACTIVE  MANUAL  3        2026-01-01T00:00:00Z  2026-01-01T00:01:00Z\nexec-2  99      7       0        2026-02-01T00:00:00Z  \n"
	if stdout.String() != wantTable {
		t.Fatalf("tty stdout=%q want=%q", stdout.String(), wantTable)
	}
	requests = nil
	f, stdout, _ = testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "executions", "list", "--workflow", "wf-1"); err != nil {
		t.Fatalf("json execute: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte(raw)) {
		t.Fatalf("json stdout=%q", stdout.Bytes())
	}
}

func TestExecutionsListPagination(t *testing.T) {
	var got []capturedRequest
	page1 := `{"executions":[{"id":"exec-1","status":1,"sourceType":1,"version":1,"startedAt":"2026-01-01T00:00:00Z","completedAt":""},{"id":"exec-2","status":2,"sourceType":2,"version":1,"startedAt":"2026-01-02T00:00:00Z","completedAt":""}],"nextPageToken":"2"}`
	page2 := `{"executions":[{"id":"exec-3","status":3,"sourceType":3,"version":1,"startedAt":"2026-01-03T00:00:00Z","completedAt":"2026-01-03T00:01:00Z"},{"id":"exec-4","status":4,"sourceType":4,"version":1,"startedAt":"2026-01-04T00:00:00Z","completedAt":""}],"nextPageToken":""}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		got = append(got, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.RawQuery {
		case "pageSize=3":
			_, _ = w.Write([]byte(page1))
		case "pageSize=3&pageToken=2":
			_, _ = w.Write([]byte(page2))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
			http.Error(w, "bad query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	f, stdout, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	if err := execute(t, f, "executions", "list", "--workflow", "wf-1", "--limit", "3"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("requests=%d", len(got))
	}
	if got[0].method != http.MethodGet || got[0].path != "/v1/workflows/wf-1/executions" || got[0].query != "pageSize=3" || got[0].body != "" {
		t.Fatalf("page1 %s %s?%s body=%q", got[0].method, got[0].path, got[0].query, got[0].body)
	}
	if got[1].method != http.MethodGet || got[1].path != "/v1/workflows/wf-1/executions" || got[1].query != "pageSize=3&pageToken=2" || got[1].body != "" {
		t.Fatalf("page2 %s %s?%s body=%q", got[1].method, got[1].path, got[1].query, got[1].body)
	}
	assertApplicationHeaders(t, got[0].headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	assertApplicationHeaders(t, got[1].headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	want := "exec-1\tACTIVE\tMANUAL\t1\t2026-01-01T00:00:00Z\t\nexec-2\tPAUSED\tWEBHOOK\t1\t2026-01-02T00:00:00Z\t\nexec-3\tCOMPLETED\tSCHEDULE\t1\t2026-01-03T00:00:00Z\t2026-01-03T00:01:00Z\n"
	if stdout.String() != want {
		t.Fatalf("stdout=%q want=%q", stdout.String(), want)
	}
}

func TestExecutionsGetRequestAndTable(t *testing.T) {
	raw := `{"execution":{"id":"exec-9","status":5,"sourceType":0,"version":4,"startedAt":"2026-03-01T00:00:00Z","completedAt":"2026-03-01T00:05:00Z"}}`
	var requests []capturedRequest
	f, stdout, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
	if err := execute(t, f, "executions", "get", "exec-9", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := requests[0]
	if got.method != http.MethodGet || got.path != "/v1/workflows/wf-1/executions/exec-9" || got.query != "" || got.body != "" {
		t.Fatalf("request %s %s?%s body=%q", got.method, got.path, got.query, got.body)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if stdout.String() != "exec-9\tCANCELLED\tUNSPECIFIED\t4\t2026-03-01T00:00:00Z\t2026-03-01T00:05:00Z\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestExecutionsCancelSendsEmptyObject(t *testing.T) {
	raw := `{"execution":{"id":"exec-9","status":5,"sourceType":1,"version":4,"startedAt":"2026-03-01T00:00:00Z","completedAt":"2026-03-01T00:05:00Z"}}`
	var requests []capturedRequest
	f, stdout, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "executions", "cancel", "exec-9", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := requests[0]
	if got.method != http.MethodPost || got.path != "/v1/workflows/wf-1/executions/exec-9:cancel" || got.query != "" || got.body != "{}" {
		t.Fatalf("request %s %s?%s body=%q", got.method, got.path, got.query, got.body)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if !bytes.Equal(stdout.Bytes(), []byte(raw)) {
		t.Fatalf("stdout=%q", stdout.Bytes())
	}
}

func TestExecutionsRequiresWorkflow(t *testing.T) {
	f, stdout, _ := testFactory(t, nil, factoryOptions{})
	called := false
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, errors.New("client should not be created")
	}
	err := execute(t, f, "executions", "list")
	if usageError(t, err) != "--workflow is required" {
		t.Fatalf("error=%v", err)
	}
	err = execute(t, f, "executions", "get", "exec-9")
	if usageError(t, err) != "--workflow is required" {
		t.Fatalf("error=%v", err)
	}
	err = execute(t, f, "executions", "cancel", "exec-9")
	if usageError(t, err) != "--workflow is required" {
		t.Fatalf("error=%v", err)
	}
	err = execute(t, f, "executions", "start")
	if usageError(t, err) != "--workflow is required" {
		t.Fatalf("error=%v", err)
	}
	err = execute(t, f, "executions", "resume", "exec-9")
	if usageError(t, err) != "--workflow is required" {
		t.Fatalf("error=%v", err)
	}
	if called || stdout.Len() != 0 {
		t.Fatalf("called=%v stdout=%q", called, stdout.String())
	}
}

func TestExecutionsListRejectsNegativeLimit(t *testing.T) {
	f, _, _ := testFactory(t, nil, factoryOptions{})
	err := execute(t, f, "executions", "list", "--workflow", "wf-1", "--limit", "-1")
	if usageError(t, err) != "--limit must be >= 0" {
		t.Fatalf("error=%v", err)
	}
}

func TestExecutionsGetSendsNamespace(t *testing.T) {
	raw := `{"execution":{"id":"exec-9","status":1,"sourceType":1,"version":1}}`
	var requests []capturedRequest
	srv := captureServer(t, http.StatusOK, raw, &requests)
	f, _, _ := testFactory(t, testClient(t, srv, "ns-1"), factoryOptions{})
	if err := execute(t, f, "executions", "get", "exec-9", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertApplicationHeaders(t, requests[0].headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json"})
}

func TestExecutionsCancelOmitsOrganizationHeader(t *testing.T) {
	t.Setenv("SL_ORGANIZATION_ID", "org-configured")
	raw := `{"execution":{"id":"exec-9","status":5,"sourceType":1,"version":1}}`
	var requests []capturedRequest
	f, _, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
	if err := execute(t, f, "executions", "cancel", "exec-9", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if requests[0].headers.Get("sl-organization-id") != "" {
		t.Fatalf("sl-organization-id=%q", requests[0].headers.Get("sl-organization-id"))
	}
}

func TestExecutionsRejectsEmptyID(t *testing.T) {
	f, _, _ := testFactory(t, nil, factoryOptions{})
	err := execute(t, f, "executions", "get", "   ", "--workflow", "wf-1")
	if usageError(t, err) != "execution ID is required" {
		t.Fatalf("error=%v", err)
	}
}

func TestExecutionEnumUnknownDecimal(t *testing.T) {
	if got := enumName(executionStatusNames, 1); got != "ACTIVE" {
		t.Fatalf("status=%q", got)
	}
	if got := enumName(executionStatusNames, 99); got != "99" {
		t.Fatalf("unknown status=%q", got)
	}
	if got := enumName(executionSourceTypeNames, 4); got != "EVENT" {
		t.Fatalf("source=%q", got)
	}
	if got := enumName(executionSourceTypeNames, 8); got != "8" {
		t.Fatalf("unknown source=%q", got)
	}
}

func TestExecutionsStartSendsBodyAndNamespace(t *testing.T) {
	raw := `{"execution":{"id":"exec-1","status":1,"sourceType":1,"version":3,"startedAt":"2026-01-01T00:00:00Z","completedAt":""}}`
	inputBody := `{"input":{"foo":"bar"}}`
	var requests []capturedRequest
	f, stdout, stderr := testFactory(t, testOAuthClient(t, captureServer(t, http.StatusOK, raw, &requests), "ns-1"), factoryOptions{})
	if err := execute(t, f, "executions", "start", "--workflow", "wf-1", "--input", writeInput(t, inputBody)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	got := requests[0]
	if got.method != http.MethodPost || got.path != "/v1/workflows/wf-1/executions:start" || got.query != "" {
		t.Fatalf("request %s %s?%s", got.method, got.path, got.query)
	}
	assertJSONEqual(t, got.body, inputBody)
	assertApplicationHeaders(t, got.headers, map[string]string{"Authorization": "Bearer " + testAccess, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if stdout.String() != "exec-1\tACTIVE\tMANUAL\t3\t2026-01-01T00:00:00Z\t\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	assertNoTokens(t, stdout.String(), stderr.String())
}

func TestExecutionsStartRequiresInput(t *testing.T) {
	var requests []capturedRequest
	f, stdout, stderr := testFactory(t, testClient(t, captureServer(t, http.StatusOK, `{}`, &requests), ""), factoryOptions{})
	err := execute(t, f, "executions", "start", "--workflow", "wf-1")
	if usageError(t, err) != "--input is required" {
		t.Fatalf("error=%v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("requests=%d", len(requests))
	}
	assertNoTokens(t, stdout.String(), stderr.String())
}

func TestExecutionsStartRequiresUserOAuth(t *testing.T) {
	var requests []capturedRequest
	f, stdout, stderr := testFactory(t, testClient(t, captureServer(t, http.StatusOK, `{}`, &requests), ""), factoryOptions{})
	f.RequireUserOAuth = func() error { return cli.ErrUserOAuthRequired }
	err := execute(t, f, "executions", "start", "--workflow", "wf-1", "--input", writeInput(t, `{"input":{}}`))
	if !errors.Is(err, cli.ErrUserOAuthRequired) {
		t.Fatalf("error=%v", err)
	}
	if err.Error() != "user OAuth login required" {
		t.Fatalf("error=%v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("requests=%d", len(requests))
	}
	assertNoTokens(t, stdout.String(), stderr.String())
}

func TestExecutionsResumeSendsBodyAndNamespace(t *testing.T) {
	raw := `{"execution":{"id":"exec-9","status":1,"sourceType":1,"version":4,"startedAt":"2026-03-01T00:00:00Z","completedAt":""}}`
	inputBody := `{"input":{"choice":"yes"}}`
	var requests []capturedRequest
	f, stdout, stderr := testFactory(t, testOAuthClient(t, captureServer(t, http.StatusOK, raw, &requests), "ns-1"), factoryOptions{})
	if err := execute(t, f, "executions", "resume", "exec-9", "--workflow", "wf-1", "--input", writeInput(t, inputBody)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	got := requests[0]
	if got.method != http.MethodPost || got.path != "/v1/workflows/wf-1/executions/exec-9:resume" || got.query != "" {
		t.Fatalf("request %s %s?%s", got.method, got.path, got.query)
	}
	assertJSONEqual(t, got.body, inputBody)
	assertApplicationHeaders(t, got.headers, map[string]string{"Authorization": "Bearer " + testAccess, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if stdout.String() != "exec-9\tACTIVE\tMANUAL\t4\t2026-03-01T00:00:00Z\t\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	assertNoTokens(t, stdout.String(), stderr.String())
}

func TestExecutionsResumeRequiresInput(t *testing.T) {
	var requests []capturedRequest
	f, stdout, stderr := testFactory(t, testClient(t, captureServer(t, http.StatusOK, `{}`, &requests), ""), factoryOptions{})
	err := execute(t, f, "executions", "resume", "exec-9", "--workflow", "wf-1")
	if usageError(t, err) != "--input is required" {
		t.Fatalf("error=%v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("requests=%d", len(requests))
	}
	assertNoTokens(t, stdout.String(), stderr.String())
}

func TestExecutionsResumeRequiresUserOAuth(t *testing.T) {
	var requests []capturedRequest
	f, stdout, stderr := testFactory(t, testClient(t, captureServer(t, http.StatusOK, `{}`, &requests), ""), factoryOptions{})
	f.RequireUserOAuth = func() error { return cli.ErrUserOAuthRequired }
	err := execute(t, f, "executions", "resume", "exec-9", "--workflow", "wf-1", "--input", writeInput(t, `{"input":{}}`))
	if !errors.Is(err, cli.ErrUserOAuthRequired) {
		t.Fatalf("error=%v", err)
	}
	if err.Error() != "user OAuth login required" {
		t.Fatalf("error=%v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("requests=%d", len(requests))
	}
	assertNoTokens(t, stdout.String(), stderr.String())
}

func TestExecutionsListHelpDoesNotResolveClient(t *testing.T) {
	f, stdout, _ := testFactory(t, nil, factoryOptions{})
	called := false
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	if err := execute(t, f, "executions", "--help"); err != nil {
		t.Fatalf("help: %v", err)
	}
	if called {
		t.Fatal("help resolved client")
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Manage workflow executions")) {
		t.Fatalf("help=%q", stdout.String())
	}
}

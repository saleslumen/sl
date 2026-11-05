package workflows

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
)

func TestVersionsGetRequestAndTable(t *testing.T) {
	t.Setenv("SL_ORGANIZATION_ID", "org-configured")
	raw := `{"version":{"id":"ver-2","version":2,"createdAt":"2026-01-02T00:00:00Z","isLive":true}}`
	var requests []capturedRequest
	f, stdout, stderr := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
	if err := execute(t, f, "versions", "get", "2", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := requests[0]
	if got.method != http.MethodGet || got.path != "/v1/workflows/wf-1/versions/2" || got.query != "" || got.body != "" {
		t.Fatalf("request %s %s?%s body=%q", got.method, got.path, got.query, got.body)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if stdout.String() != "2\tver-2\ttrue\tfalse\t2026-01-02T00:00:00Z\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestVersionsGetTTYAndJSON(t *testing.T) {
	raw := `{"version":{"id":"ver-2","version":2,"createdAt":"2026-01-02T00:00:00Z","isLive":true}}`
	var requests []capturedRequest
	f, stdout, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{stdoutTTY: true})
	if err := execute(t, f, "versions", "get", "2", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	wantTable := "VERSION  ID     LIVE  DRAFT  CREATED\n2        ver-2  true  false  2026-01-02T00:00:00Z\n"
	if stdout.String() != wantTable {
		t.Fatalf("tty stdout=%q want=%q", stdout.String(), wantTable)
	}
	requests = nil
	f, stdout, _ = testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "versions", "get", "2", "--workflow", "wf-1"); err != nil {
		t.Fatalf("json execute: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte(raw)) {
		t.Fatalf("json stdout=%q", stdout.Bytes())
	}
}

func TestVersionsRevertSendsEmptyObject(t *testing.T) {
	raw := `{"workflow":{"id":"wf-1","name":"Lead","version":3,"isActive":false,"updatedAt":"2026-01-03T00:00:00Z"}}`
	var requests []capturedRequest
	f, stdout, stderr := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
	if err := execute(t, f, "versions", "revert", "2", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := requests[0]
	if got.method != http.MethodPost || got.path != "/v1/workflows/wf-1/versions/2:revert" || got.query != "" || got.body != "{}" {
		t.Fatalf("request %s %s?%s body=%q", got.method, got.path, got.query, got.body)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if stdout.String() != "wf-1\tLead\t3\t\tfalse\t2026-01-03T00:00:00Z\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr=%q", stderr.String())
	}
	requests = nil
	f, stdout, _ = testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "versions", "revert", "2", "--workflow", "wf-1"); err != nil {
		t.Fatalf("json execute: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), []byte(raw)) {
		t.Fatalf("json stdout=%q", stdout.Bytes())
	}
}

func TestVersionsRequiresWorkflow(t *testing.T) {
	f, stdout, _ := testFactory(t, nil, factoryOptions{})
	called := false
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, errors.New("client should not be created")
	}
	err := execute(t, f, "versions", "get", "2")
	if usageError(t, err) != "--workflow is required" {
		t.Fatalf("error=%v", err)
	}
	if called || stdout.Len() != 0 {
		t.Fatalf("called=%v stdout=%q", called, stdout.String())
	}
}

func TestVersionsRejectsInvalidVersion(t *testing.T) {
	f, _, _ := testFactory(t, nil, factoryOptions{})
	err := execute(t, f, "versions", "revert", "v2", "--workflow", "wf-1")
	if usageError(t, err) != "version must be a non-negative integer" {
		t.Fatalf("error=%v", err)
	}
}

func TestVersionsGetSendsNamespace(t *testing.T) {
	raw := `{"version":{"id":"ver-2","version":2}}`
	var requests []capturedRequest
	srv := captureServer(t, http.StatusOK, raw, &requests)
	f, _, _ := testFactory(t, testClient(t, srv, "ns-1"), factoryOptions{})
	if err := execute(t, f, "versions", "get", "2", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertApplicationHeaders(t, requests[0].headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json"})
}

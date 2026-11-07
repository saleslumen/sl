package workflows

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const workflowResponseJSON = `{"name":"workflows/wf-1","uid":"uid-1","displayName":"Alpha","etag":"e1","createTime":"2026-01-01T00:00:00Z","updateTime":"2026-01-02T00:00:00Z","servingRevision":"workflows/wf-1/revisions/r1"}`

func TestListQueryUsesCurrentSearchContract(t *testing.T) {
	var requests []capturedRequest
	body := `{"workflows":[{"name":"workflows/wf-1","uid":"u1","displayName":"Alpha","etag":"e1","createTime":"t1","updateTime":"t2","servingRevision":"workflows/wf-1/revisions/r1"}],"nextPageToken":"unexpected"}`
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{})
	if err := execute(t, f, "list", "--query", "alpha", "--limit", "100"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 || requests[0].method != http.MethodGet || requests[0].path != "/v1/workflows" || requests[0].query != "pageSize=20&query=alpha" {
		t.Fatalf("requests=%+v", requests)
	}
	if out.String() != "workflows/wf-1\tAlpha\tu1\te1\tt1\tt2\tworkflows/wf-1/revisions/r1\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestListQueryJSONPreservesRawCurrentResponseAndLimit(t *testing.T) {
	var requests []capturedRequest
	raw := `{"workflows":[{"name":"workflows/wf-1","uid":"u1","displayName":"Alpha","definition":{"unknown":{"kept":true}}}],"nextPageToken":"ignored"}`
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "list", "--query", "alpha", "--limit", "5"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 || requests[0].query != "pageSize=5&query=alpha" {
		t.Fatalf("requests=%+v", requests)
	}
	if out.String() != raw {
		t.Fatalf("stdout=%q want=%q", out.String(), raw)
	}
}

func TestWorkflowMutationWireContracts(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone(), body: readBody(t, r)})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(workflowResponseJSON))
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{confirm: func(prompt string) error {
		if prompt != "Delete workflow wf-1?" {
			t.Fatalf("prompt=%q", prompt)
		}
		return nil
	}})

	if err := execute(t, f, "create", "--display-name", "Alpha", "--workflow-id", "wf-1"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := execute(t, f, "update", "wf-1", "--display-name", "Beta", "--update-mask", "displayName"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := execute(t, f, "delete", "wf-1", "--etag", "e1", "--force"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := execute(t, f, "deploy", "wf-1", "--revision", "workflows/wf-1/revisions/r1", "--etag", "e2"); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if err := execute(t, f, "undeploy", "wf-1"); err != nil {
		t.Fatalf("undeploy: %v", err)
	}
	want := []capturedRequest{
		{method: http.MethodPost, path: "/v1/workflows", query: "workflowId=wf-1", body: `{"displayName":"Alpha"}`},
		{method: http.MethodPatch, path: "/v1/workflows/wf-1", query: "updateMask=displayName", body: `{"displayName":"Beta"}`},
		{method: http.MethodDelete, path: "/v1/workflows/wf-1", query: "etag=e1&force=true"},
		{method: http.MethodPost, path: "/v1/workflows/wf-1:deploy", body: `{"etag":"e2","revision":"workflows/wf-1/revisions/r1"}`},
		{method: http.MethodPost, path: "/v1/workflows/wf-1:undeploy", body: `{}`},
	}
	if len(requests) != len(want) {
		t.Fatalf("requests=%+v", requests)
	}
	for i := range want {
		if requests[i].method != want[i].method || requests[i].path != want[i].path || requests[i].query != want[i].query {
			t.Fatalf("request[%d]=%+v want=%+v", i, requests[i], want[i])
		}
		if want[i].body != "" {
			assertJSONEqual(t, requests[i].body, want[i].body)
		}
		wantHeaders := map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"}
		if want[i].body != "" {
			wantHeaders["Content-Type"] = "application/json"
		}
		assertApplicationHeaders(t, requests[i].headers, wantHeaders)
	}
	if !strings.Contains(out.String(), "workflows/wf-1") {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestWorkflowUpdateRequiresInputOrDisplayName(t *testing.T) {
	f, _, _, calls := newLazyFactory()
	if err := execute(t, f, "update", "wf-1"); err == nil {
		t.Fatal("update without body succeeded")
	}
	if *calls != 0 {
		t.Fatalf("calls=%d", *calls)
	}
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	var data strings.Builder
	if _, err := io.Copy(&data, r.Body); err != nil {
		t.Fatal(err)
	}
	return data.String()
}

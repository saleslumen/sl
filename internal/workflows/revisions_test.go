package workflows

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRevisionCreateAndGetUseCurrentResourceRoutes(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone(), body: readBody(t, r)})
		_, _ = w.Write([]byte(`{"name":"workflows/wf-1/revisions/r1","changelog":"first","createTime":"t1","definition":{"root":{"steps":[]}}}`))
	}))
	t.Cleanup(srv.Close)
	f, _, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{stdin: strings.NewReader(`{"definition":{"root":{"steps":[]}},"changelog":"first"}`)})
	if err := execute(t, f, "revisions", "create", "--workflow", "wf-1", "--revision-id", "r1", "--input", "-"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := execute(t, f, "revisions", "get", "r1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests=%+v", requests)
	}
	if requests[0].method != http.MethodPost || requests[0].path != "/v1/workflows/wf-1/revisions" || requests[0].query != "revisionId=r1" {
		t.Fatalf("create request=%+v", requests[0])
	}
	assertJSONEqual(t, requests[0].body, `{"definition":{"root":{"steps":[]}},"changelog":"first"}`)
	if requests[1].method != http.MethodGet || requests[1].path != "/v1/workflows/wf-1/revisions/r1" || requests[1].query != "" {
		t.Fatalf("get request=%+v", requests[1])
	}
}

func TestRevisionPaginationPreservesDefinitionJSON(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone()})
		switch r.URL.Query().Get("pageToken") {
		case "":
			_, _ = w.Write([]byte(`{"revisions":[{"name":"workflows/wf-1/revisions/r1","definition":{"unknown":{"keep":1}},"createTime":"t1"}],"nextPageToken":"p2"}`))
		case "p2":
			_, _ = w.Write([]byte(`{"revisions":[{"name":"workflows/wf-1/revisions/r2","definition":{"unknown":{"keep":2}},"createTime":"t2"}]}`))
		default:
			t.Errorf("unexpected query %s", r.URL.RawQuery)
		}
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "revisions", "list", "--workflow", "wf-1", "--limit", "0"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(requests) != 2 || requests[0].query != "pageSize=50" || requests[1].query != "pageSize=50&pageToken=p2" {
		t.Fatalf("requests=%+v", requests)
	}
	assertJSONEqual(t, out.String(), `{"revisions":[{"name":"workflows/wf-1/revisions/r1","definition":{"unknown":{"keep":1}},"createTime":"t1"},{"name":"workflows/wf-1/revisions/r2","definition":{"unknown":{"keep":2}},"createTime":"t2"}]}`)
}

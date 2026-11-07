package workflows

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saleslumen/sl/internal/cli"
)

const executionResponseJSON = `{"name":"workflows/wf-1/executions/e1","uid":"eu1","revision":"workflows/wf-1/revisions/r1","state":"STATE_QUEUED","etag":"ee1","createTime":"t1","updateTime":"t2","input":{"x":1}}`

func TestExecutionCommandWireContracts(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone(), body: readBody(t, r)})
		_, _ = w.Write([]byte(executionResponseJSON))
	}))
	t.Cleanup(srv.Close)
	f, _, _ := testFactory(t, testOAuthClient(t, srv, ""), factoryOptions{})
	requestID := "00000000-0000-4000-8000-000000000001"
	createInput := writeInput(t, `{"revision":"workflows/wf-1/revisions/r1","input":{"x":1}}`)
	signalInput := writeInput(t, `{"signal":"approve","payload":null}`)
	if err := execute(t, f, "executions", "create", "--workflow", "wf-1", "--execution-id", "e1", "--request-id", requestID, "--input", createInput); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := execute(t, f, "executions", "get", "e1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if err := execute(t, f, "executions", "cancel", "e1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := execute(t, f, "executions", "signal", "e1", "--workflow", "wf-1", "--request-id", requestID, "--input", signalInput); err != nil {
		t.Fatalf("signal: %v", err)
	}
	want := []capturedRequest{
		{method: http.MethodPost, path: "/v1/workflows/wf-1/executions", query: "executionId=e1&requestId=" + requestID, body: `{"revision":"workflows/wf-1/revisions/r1","input":{"x":1}}`},
		{method: http.MethodGet, path: "/v1/workflows/wf-1/executions/e1"},
		{method: http.MethodPost, path: "/v1/workflows/wf-1/executions/e1:cancel", body: `{}`},
		{method: http.MethodPost, path: "/v1/workflows/wf-1/executions/e1:signal", body: `{"signal":"approve","payload":null,"requestId":"` + requestID + `"}`},
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
		wantHeaders := map[string]string{"Authorization": "Bearer " + testAccess, "User-Agent": "sl/test", "Accept": "application/json"}
		if want[i].body != "" {
			wantHeaders["Content-Type"] = "application/json"
		}
		assertApplicationHeaders(t, requests[i].headers, wantHeaders)
	}
}

func TestExecutionListFilterPaginationPreservesRawJSON(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery})
		switch r.URL.Query().Get("pageToken") {
		case "":
			_, _ = w.Write([]byte(`{"executions":[{"name":"workflows/wf-1/executions/e1","state":"STATE_ACTIVE","input":{"future":{"x":1}},"result":null}],"nextPageToken":"p2"}`))
		case "p2":
			_, _ = w.Write([]byte(`{"executions":[{"name":"workflows/wf-1/executions/e2","state":"STATE_SUCCEEDED","result":{"future":{"kept":true}}}]}`))
		default:
			t.Errorf("unexpected query %s", r.URL.RawQuery)
		}
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "executions", "list", "--workflow", "wf-1", "--limit", "0", "--filter", `state="STATE_ACTIVE"`); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(requests) != 2 || requests[0].query != `filter=state%3D%22STATE_ACTIVE%22&pageSize=50` || requests[1].query != `filter=state%3D%22STATE_ACTIVE%22&pageSize=50&pageToken=p2` {
		t.Fatalf("requests=%+v", requests)
	}
	assertJSONEqual(t, out.String(), `{"executions":[{"name":"workflows/wf-1/executions/e1","state":"STATE_ACTIVE","input":{"future":{"x":1}},"result":null},{"name":"workflows/wf-1/executions/e2","state":"STATE_SUCCEEDED","result":{"future":{"kept":true}}}]}`)
}

func TestExecutionCreateAndSignalRejectAPIKeyBeforeHTTP(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path})
	}))
	t.Cleanup(srv.Close)
	f, _, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	f.RequireUserOAuth = func() error { return cli.ErrUserOAuthRequired }
	createInput := writeInput(t, `{"input":{}}`)
	signalInput := writeInput(t, `{"signal":"approve","payload":null}`)
	for _, args := range [][]string{
		{"executions", "create", "--workflow", "wf-1", "--input", createInput},
		{"executions", "signal", "e1", "--workflow", "wf-1", "--input", signalInput},
	} {
		if err := execute(t, f, args...); err != cli.ErrUserOAuthRequired {
			t.Fatalf("%v error=%v", args, err)
		}
	}
	if len(requests) != 0 {
		t.Fatalf("HTTP requests=%+v", requests)
	}
}

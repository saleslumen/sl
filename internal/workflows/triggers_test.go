package workflows

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saleslumen/sl/internal/cli"
)

const triggerResponseJSON = `{"name":"workflows/wf-1/triggers/tr-1","uid":"tu1","etag":"te1","kind":"KIND_WEBHOOK","enabled":true,"disabledReason":"DISABLED_REASON_UNSPECIFIED","createTime":"t1","updateTime":"t2","webhook_url":"https://example.test/hook"}`

func TestTriggerCommandWireContracts(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone(), body: readBody(t, r)})
		_, _ = w.Write([]byte(triggerResponseJSON))
	}))
	t.Cleanup(srv.Close)
	f, _, _ := testFactory(t, testOAuthClient(t, srv, ""), factoryOptions{confirm: func(prompt string) error {
		if prompt != "Delete trigger tr-1?" {
			t.Fatalf("prompt=%q", prompt)
		}
		return nil
	}})
	createInput := writeInput(t, `{"kind":"KIND_WEBHOOK","enabled":true}`)
	updateInput := writeInput(t, `{"etag":"te1","enabled":false}`)
	if err := execute(t, f, "triggers", "create", "--workflow", "wf-1", "--trigger-id", "tr-1", "--input", createInput); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := execute(t, f, "triggers", "list", "--workflow", "wf-1", "--show-deleted"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := execute(t, f, "triggers", "get", "tr-1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if err := execute(t, f, "triggers", "update", "tr-1", "--workflow", "wf-1", "--input", updateInput); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := execute(t, f, "triggers", "delete", "tr-1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := execute(t, f, "triggers", "rotate-webhook-token", "tr-1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	want := []capturedRequest{
		{method: http.MethodPost, path: "/v1/workflows/wf-1/triggers", query: "triggerId=tr-1", body: `{"kind":"KIND_WEBHOOK","enabled":true}`},
		{method: http.MethodGet, path: "/v1/workflows/wf-1/triggers", query: "pageSize=50&showDeleted=true"},
		{method: http.MethodGet, path: "/v1/workflows/wf-1/triggers/tr-1"},
		{method: http.MethodPatch, path: "/v1/workflows/wf-1/triggers/tr-1", body: `{"etag":"te1","enabled":false}`},
		{method: http.MethodDelete, path: "/v1/workflows/wf-1/triggers/tr-1"},
		{method: http.MethodPost, path: "/v1/workflows/wf-1/triggers/tr-1:rotateWebhookToken", body: `{}`},
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

func TestTriggerCreateRejectsAPIKeyBeforeHTTP(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method})
	}))
	t.Cleanup(srv.Close)
	f, _, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	f.RequireUserOAuth = func() error { return cli.ErrUserOAuthRequired }
	if err := execute(t, f, "triggers", "create", "--workflow", "wf-1", "--input", writeInput(t, `{"kind":"KIND_WEBHOOK"}`)); err != cli.ErrUserOAuthRequired {
		t.Fatalf("error=%v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("HTTP requests=%+v", requests)
	}
}

func TestEventTypesUsesCurrentCollectionKeyAndPagination(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery})
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"triggerEventTypes":[{"eventType":"TRIGGER_EVENT_MESSAGE_REPLIED"}],"nextPageToken":"p2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"triggerEventTypes":[{"eventType":"TRIGGER_EVENT_MESSAGE_BOUNCED"}]}`))
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "event-types", "list", "--limit", "0"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(requests) != 2 || requests[0].path != "/v1/triggerEventTypes" || requests[0].query != "pageSize=50" || requests[1].query != "pageSize=50&pageToken=p2" {
		t.Fatalf("requests=%+v", requests)
	}
	assertJSONEqual(t, out.String(), `{"triggerEventTypes":[{"eventType":"TRIGGER_EVENT_MESSAGE_REPLIED"},{"eventType":"TRIGGER_EVENT_MESSAGE_BOUNCED"}]}`)
}

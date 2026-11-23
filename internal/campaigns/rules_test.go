package campaigns

import (
	"io"
	"net/http"
	"testing"
)

func TestRulesCommandsMatchCurrentAPI(t *testing.T) {
	srv, requests := captureRootCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/v1/campaigns/c1/rules" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"rules":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"name":"campaigns/c1/rules/r1","etag":"9"}`))
	})
	defer srv.Close()
	f := campaignsFactory(t, nil, io.Discard, true, nil)
	bindCampaignClient(t, f, srv, "")

	if err := executeCampaigns(t, f, "rules", "list", "--campaign", "c1", "--limit", "5"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := executeCampaigns(t, f, "rules", "get", "r1", "--campaign", "c1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	create := writeCampaignInput(t, `{"display_name":"Reply rule","event_type":"MESSAGE_REPLIED","srl_expression":"true","priority":1,"delay_seconds":0,"action":{"type":"STOP"}}`)
	if err := executeCampaigns(t, f, "rules", "create", "--campaign", "c1", "--input", create); err != nil {
		t.Fatalf("create: %v", err)
	}
	update := writeCampaignInput(t, `{"rule":{"display_name":"Updated"},"update_mask":"display_name"}`)
	if err := executeCampaigns(t, f, "rules", "update", "r1", "--campaign", "c1", "--input", update, "--etag", "9", "--request-id", "rid"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := executeCampaigns(t, f, "rules", "delete", "r1", "--campaign", "c1", "--etag", "9", "--request-id", "rid"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(*requests) != 5 {
		t.Fatalf("requests=%d", len(*requests))
	}
	want := []struct{ method, path, query string }{
		{http.MethodGet, "/v1/campaigns/c1/rules", "page_size=5"},
		{http.MethodGet, "/v1/campaigns/c1/rules/r1", ""},
		{http.MethodPost, "/v1/campaigns/c1/rules", ""},
		{http.MethodPatch, "/v1/campaigns/c1/rules/r1", ""},
		{http.MethodDelete, "/v1/campaigns/c1/rules/r1", "etag=9&request_id=rid"},
	}
	for i, expected := range want {
		got := (*requests)[i]
		if got.Method != expected.method || got.Path != expected.path || got.Query != expected.query {
			t.Fatalf("request[%d]=%s %s?%s", i, got.Method, got.Path, got.Query)
		}
	}
}

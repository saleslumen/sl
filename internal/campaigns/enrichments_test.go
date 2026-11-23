package campaigns

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestEnrichmentCommandRoutesAndPagination(t *testing.T) {
	server, requests := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"enrichments":[],"next_page_token":"next"}`))
	})
	var out bytes.Buffer
	f := operationsFactory(t, server, "", &out, true)
	if err := executeNoun(t, newEnrichmentsCommand(f), "list", "--campaign", "c1", "--page-size", "5", "--page-token", "cursor"); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 1 || (*requests)[0].path != "/v1/campaigns/c1/enrichments" || (*requests)[0].rawQuery != "page_size=5&page_token=cursor" {
		t.Fatalf("requests=%+v", *requests)
	}
}

func TestEnrichmentMutationBodies(t *testing.T) {
	server, requests := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"campaigns/c1/operations/o1","done":false}`))
	})
	f := operationsFactory(t, server, "", io.Discard, true)
	cases := []struct {
		args               []string
		method, path, body string
	}{
		{[]string{"run", "--campaign", "c1", "--input", writePeopleInput(t, `{"strategy":"RUN_ALL","column_scope":{"all":true},"row_scope":{"all":true},"connections":{"crm":"installations/inst-1/connections/conn-1"}}`)}, http.MethodPost, "/v1/campaigns/c1/enrichments:run", `{"strategy":"RUN_ALL","column_scope":{"all":true},"row_scope":{"all":true},"connections":{"crm":"installations/inst-1/connections/conn-1"},"request_id":"` + peopleRequestID + `"}`},
		{[]string{"create", "--campaign", "c1", "--input", writePeopleInput(t, `{"custom_script":{"script_id":"s","function_name":"f"},"output_binding":{"column":"title"},"connections":{"crm":"installations/inst-1/connections/conn-1"}}`)}, http.MethodPost, "/v1/campaigns/c1/enrichments", `{"custom_script":{"script_id":"s","function_name":"f"},"output_binding":{"column":"title"},"connections":{"crm":"installations/inst-1/connections/conn-1"},"request_id":"` + peopleRequestID + `"}`},
		{[]string{"update", "e1", "--campaign", "c1", "--input", writePeopleInput(t, `{"enrichment":{"name":"campaigns/c1/enrichments/e1","etag":"v1"},"update_mask":"output_acceptance"}`)}, http.MethodPatch, "/v1/campaigns/c1/enrichments/e1", `{"enrichment":{"name":"campaigns/c1/enrichments/e1","etag":"v1"},"update_mask":"output_acceptance","request_id":"` + peopleRequestID + `"}`},
	}
	for _, tc := range cases {
		if err := executeNoun(t, newEnrichmentsCommand(f), tc.args...); err != nil {
			t.Fatal(err)
		}
		got := (*requests)[len(*requests)-1]
		if got.method != tc.method || got.path != tc.path {
			t.Fatalf("request=%+v", got)
		}
		assertPeopleJSON(t, got.body, tc.body)
	}
}

func TestEnrichmentGetAndDelete(t *testing.T) {
	server, requests := recordCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = w.Write([]byte(`{"name":"campaigns/c1/enrichments/e1","etag":"v1"}`))
	})
	f := operationsFactory(t, server, "", io.Discard, true)
	f.Confirm = func(string) error { return nil }
	if err := executeNoun(t, newEnrichmentsCommand(f), "get", "e1", "--campaign", "c1"); err != nil {
		t.Fatal(err)
	}
	if err := executeNoun(t, newEnrichmentsCommand(f), "delete", "e1", "--campaign", "c1", "--etag", "v1", "--request-id", peopleRequestID); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 2 || (*requests)[0].method != http.MethodGet || (*requests)[0].path != "/v1/campaigns/c1/enrichments/e1" || (*requests)[1].method != http.MethodDelete || (*requests)[1].path != "/v1/campaigns/c1/enrichments/e1" || (*requests)[1].rawQuery != "etag=v1&request_id="+peopleRequestID {
		t.Fatalf("requests=%+v", *requests)
	}
}

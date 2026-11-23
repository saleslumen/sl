package marketplace

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/cli"
)

const testConnection = "installations/inst-1/connections/conn-1"

func TestConnectionCreatePostsKeyAndDisplayName(t *testing.T) {
	var requests []capturedRequest
	body := `{"name":"` + testConnection + `","connectionKey":"crm","displayName":"CRM","status":"DISCONNECTED","kind":"API_KEY","usable":false}`
	f, out := testFactory(t, captureServer(t, http.StatusOK, body, &requests), nil, nil)
	if err := execute(t, f, "connections", "create", "--installation", "inst-1", "--connection-key", "crm", "--display-name", "CRM"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].method != http.MethodPost || requests[0].path != "/v1/installations/inst-1/connections" {
		t.Fatalf("request %+v", requests)
	}
	assertJSON(t, requests[0].body, `{"connectionKey":"crm","displayName":"CRM"}`)
	if out.String() != testConnection+"\tcrm\tCRM\tDISCONNECTED\tAPI_KEY\tfalse\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestConnectionListFiltersAndFollowsPages(t *testing.T) {
	var requests []capturedRequest
	srv := captureServer(t, http.StatusOK, `{}`, &requests)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := `{"connections":[{"name":"` + testConnection + `","connectionKey":"crm","displayName":"CRM","status":"READY","kind":"OAUTH","usable":true}]}`
		if r.URL.Query().Get("pageToken") == "" {
			raw = `{"connections":[{"name":"` + testConnection + `","connectionKey":"crm","displayName":"CRM","status":"READY","kind":"OAUTH","usable":true}],"nextPageToken":"p2"}`
		}
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery})
		_, _ = w.Write([]byte(raw))
	})
	f, out := testFactory(t, srv, nil, nil)
	if err := execute(t, f, "connections", "list", "--installation", "inst-1", "--connection-key", "crm"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0].query != `filter=connection_key+%3D+%22crm%22&pageSize=100` || requests[1].query != `filter=connection_key+%3D+%22crm%22&pageSize=100&pageToken=p2` {
		t.Fatalf("requests %+v", requests)
	}
	if !strings.Contains(out.String(), testConnection+"\tcrm\tCRM\tREADY\tOAUTH\ttrue\n") {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestConnectionUpdateSendsFetchedEtag(t *testing.T) {
	var requests []capturedRequest
	srv := captureServer(t, http.StatusOK, `{}`, &requests)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := readBody(r)
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: string(raw)})
		_, _ = w.Write([]byte(`{"name":"` + testConnection + `","connectionKey":"crm","displayName":"Renamed","status":"DISCONNECTED","kind":"API_KEY","usable":false,"etag":"e2"}`))
	})
	f, _ := testFactory(t, srv, nil, nil)
	if err := execute(t, f, "connections", "update", testConnection, "--display-name", "Renamed"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0].method != http.MethodGet || requests[1].method != http.MethodPatch || requests[1].path != "/v1/installations/inst-1/connections/conn-1" || requests[1].query != "updateMask=display_name" {
		t.Fatalf("requests %+v", requests)
	}
	assertJSON(t, requests[1].body, `{"displayName":"Renamed","etag":"e2"}`)
}

func TestConnectionDeleteConfirms(t *testing.T) {
	var requests []capturedRequest
	var prompt string
	f, _ := testFactory(t, captureServer(t, http.StatusOK, `{}`, &requests), nil, nil)
	f.Confirm = func(got string) error {
		prompt = got
		return nil
	}
	if err := execute(t, f, "connections", "delete", testConnection); err != nil {
		t.Fatal(err)
	}
	if prompt != "Delete connection "+testConnection+"?" || len(requests) != 1 || requests[0].method != http.MethodDelete || requests[0].path != "/v1/installations/inst-1/connections/conn-1" {
		t.Fatalf("prompt=%q requests=%+v", prompt, requests)
	}
	f.Confirm = func(string) error { return cli.ErrCancelled }
	if err := execute(t, f, "connections", "delete", testConnection); !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("err=%v", err)
	}
}

func TestConnectionAuthorizeAndUnbindUseResourceName(t *testing.T) {
	var requests []capturedRequest
	srv := captureServer(t, http.StatusOK, `{}`, &requests)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path})
		if strings.HasSuffix(r.URL.Path, ":initiateAuthorization") {
			_, _ = w.Write([]byte(`{"connection":{"name":"` + testConnection + `","status":"AUTHORIZATION_PENDING"},"authorizationStartUri":"https://broker.example/start"}`))
			return
		}
		_, _ = w.Write([]byte(`{"name":"` + testConnection + `","connectionKey":"crm","displayName":"CRM","status":"DISCONNECTED","kind":"OAUTH","usable":false}`))
	})
	f, out := testFactory(t, srv, nil, nil)
	if err := execute(t, f, "connections", "authorize", testConnection); err != nil {
		t.Fatal(err)
	}
	if err := execute(t, f, "connections", "unbind", testConnection); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0].path != "/v1/installations/inst-1/connections/conn-1:initiateAuthorization" || requests[1].path != "/v1/installations/inst-1/connections/conn-1:unbind" {
		t.Fatalf("requests %+v", requests)
	}
	if !strings.Contains(out.String(), "https://broker.example/start") {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestConnectionNameRejectsKey(t *testing.T) {
	var requests []capturedRequest
	f, _ := testFactory(t, captureServer(t, http.StatusOK, `{}`, &requests), nil, nil)
	err := execute(t, f, "connections", "get", "crm")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || usage.Error() != "connection must be installations/{installation_id}/connections/{connection_id}" || len(requests) != 0 {
		t.Fatalf("err=%v requests=%d", err, len(requests))
	}
}

func assertJSON(t *testing.T, got, want string) {
	t.Helper()
	var left, right any
	if err := json.Unmarshal([]byte(got), &left); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &right); err != nil {
		t.Fatal(err)
	}
	lb, _ := json.Marshal(left)
	rb, _ := json.Marshal(right)
	if string(lb) != string(rb) {
		t.Fatalf("json=%s want=%s", lb, rb)
	}
}

func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

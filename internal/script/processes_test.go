package script

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
)

func TestProcessesListMapsProjectFilter(t *testing.T) {
	raw := `{"processes":[{"projectName":"Acme","functionName":"greet","processType":"PROCESS_EXECUTION_API","processStatus":"COMPLETED","userAccessLevel":"OWNER","startTime":"2026-01-01T00:00:00Z","duration":"1s","runtimeVersion":"NODEJS22"}]}`
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(raw))
	})
	res := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "processes", "list", "--project", "proj-1")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := (*recs)[0]
	if got.Method != http.MethodGet || got.Path != "/v1/processes" {
		t.Fatalf("request %s %s", got.Method, got.Path)
	}
	if got.Query.Get("userProcessFilterScriptId") != "proj-1" || got.Query.Get("scriptId") != "" || got.Query.Get("pageSize") != "50" {
		t.Fatalf("query %v", got.Query)
	}
	assertScriptHeaders(t, got.Header, "")
	want := "Acme\tgreet\tPROCESS_EXECUTION_API\tCOMPLETED\tOWNER\t2026-01-01T00:00:00Z\t1s\tNODEJS22\n"
	if out != want {
		t.Fatalf("stdout=%q want=%q", out, want)
	}
}

func TestProcessesListScriptProcessesMapsScriptId(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(`{"processes":[]}`))
	})
	if err := execScript(t, testClient(t, srv, "ns-2"), nil, false, nil, "script", "processes", "list-script-processes", "--project", "proj-9").err; err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := (*recs)[0]
	if got.Path != "/v1/processes:listScriptProcesses" {
		t.Fatalf("path %s", got.Path)
	}
	if got.Query.Get("scriptId") != "proj-9" || got.Query.Get("userProcessFilterScriptId") != "" {
		t.Fatalf("query %v", got.Query)
	}
	assertScriptHeaders(t, got.Header, "ns-2")
}

func TestProcessesListPaginates(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"processes":[{"functionName":"a"},{"functionName":"b"}],"nextPageToken":"n2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"processes":[{"functionName":"c"}]}`))
	})
	res := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "processes", "list", "--project", "proj-1", "--limit", "3")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*recs) != 2 || (*recs)[0].Query.Get("pageSize") != "3" || (*recs)[1].Query.Get("pageSize") != "1" || (*recs)[1].Query.Get("pageToken") != "n2" {
		t.Fatalf("recs %#v", *recs)
	}
	if (*recs)[0].Query.Get("userProcessFilterScriptId") != "proj-1" || (*recs)[1].Query.Get("userProcessFilterScriptId") != "proj-1" {
		t.Fatalf("filter %#v", *recs)
	}
	if !strings.Contains(out, "\ta\t") || !strings.Contains(out, "\tb\t") || !strings.Contains(out, "\tc\t") {
		t.Fatalf("stdout=%q", out)
	}
}

func TestProcessesListJSON(t *testing.T) {
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(`{"processes":[{"functionName":"greet"}]}`))
	})
	res := execScript(t, testClient(t, srv, ""), nil, false, nil, "--json", "script", "processes", "list-script-processes", "--project", "proj-1")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	var payload struct {
		Processes []process `json:"processes"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(payload.Processes) != 1 || payload.Processes[0].FunctionName != "greet" {
		t.Fatalf("payload %#v", payload)
	}
}

func TestProcessesListRequiresProject(t *testing.T) {
	err := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "processes", "list").err
	requireUsage(t, err, "--project")
	err = execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "processes", "list-script-processes").err
	requireUsage(t, err, "--project")
}

func TestProcessesListRejectsNegativeLimit(t *testing.T) {
	err := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "processes", "list", "--project", "proj-1", "--limit", "-1").err
	requireUsage(t, err, "--limit")
}

func TestProcessesListAPIError(t *testing.T) {
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":3,"message":"script_id is required","details":[]}`))
	})
	err := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "processes", "list", "--project", "proj-1").err
	var api *apiclient.Error
	if !errors.As(err, &api) || api.Code != "INVALID_ARGUMENT" {
		t.Fatalf("err: %v", err)
	}
}

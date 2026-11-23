package script

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
)

func TestDeploymentsListTableAndHeaders(t *testing.T) {
	body := `{"deployments":[{"deploymentId":"dep-1","deploymentConfig":{"versionNumber":1,"description":"prod"},"deploymentType":"DEPLOYMENT_TYPE_EXECUTION_API","lifecycleState":"DEPLOYMENT_LIFECYCLE_ACTIVE","updateTime":"2026-01-01T00:00:00Z"}],"nextPageToken":""}`
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(body))
	})
	res := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "deployments", "list", "--project", "proj-1")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests %d", len(*recs))
	}
	got := (*recs)[0]
	if got.Method != http.MethodGet || got.Host != "script.example.test" || got.Path != "/v1/projects/proj-1/deployments" {
		t.Fatalf("request %s %s %s", got.Method, got.Host, got.Path)
	}
	if got.Query.Get("pageSize") != "50" || got.Query.Get("pageToken") != "" {
		t.Fatalf("query %v", got.Query)
	}
	assertScriptHeaders(t, got.Header, "")
	want := "dep-1\t1\tDEPLOYMENT_TYPE_EXECUTION_API\tDEPLOYMENT_LIFECYCLE_ACTIVE\t2026-01-01T00:00:00Z\t\tprod\n"
	if out != want {
		t.Fatalf("stdout=%q want=%q", out, want)
	}
}

func TestDeploymentsListPaginatesAndTruncates(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"deployments":[{"deploymentId":"a","deploymentConfig":{"versionNumber":1}},{"deploymentId":"b","deploymentConfig":{"versionNumber":2}}],"nextPageToken":"p2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"deployments":[{"deploymentId":"c","deploymentConfig":{"versionNumber":3}},{"deploymentId":"d","deploymentConfig":{"versionNumber":4}}]}`))
	})
	res := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "deployments", "list", "--project", "proj-1", "--limit", "3")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*recs) != 2 || (*recs)[0].Query.Get("pageSize") != "3" || (*recs)[1].Query.Get("pageSize") != "1" || (*recs)[1].Query.Get("pageToken") != "p2" {
		t.Fatalf("recs %#v", *recs)
	}
	if !strings.Contains(out, "a\t") || !strings.Contains(out, "b\t") || !strings.Contains(out, "c\t") || strings.Contains(out, "d\t") {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDeploymentsListPageSizeCapsAt100(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(`{"deployments":[]}`))
	})
	if err := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "deployments", "list", "--project", "proj-1", "--limit", "250").err; err != nil {
		t.Fatalf("execute: %v", err)
	}
	if (*recs)[0].Query.Get("pageSize") != "100" {
		t.Fatalf("pageSize=%q", (*recs)[0].Query.Get("pageSize"))
	}
}

func TestDeploymentsGetEscapesPathAndJSON(t *testing.T) {
	raw := `{"deploymentId":"dep/1","deploymentConfig":{"versionNumber":2,"description":"canary"},"deploymentType":"DEPLOYMENT_TYPE_WEB_APP","lifecycleState":"DEPLOYMENT_LIFECYCLE_ACTIVE","updateTime":"2026-02-01T00:00:00Z"}`
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(raw))
	})
	res := execScript(t, testClient(t, srv, "ns-1"), nil, false, nil, "--json", "script", "deployments", "get", "dep/1", "--project", "proj/1")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if (*recs)[0].Path != "/v1/projects/proj%2F1/deployments/dep%2F1" {
		t.Fatalf("path %s", (*recs)[0].Path)
	}
	assertScriptHeaders(t, (*recs)[0].Header, "ns-1")
	if out != raw {
		t.Fatalf("stdout=%q", out)
	}
}

func TestDeploymentsCreateFlagsAndInputMerge(t *testing.T) {
	raw := `{"deploymentId":"dep-9","deploymentConfig":{"versionNumber":1,"description":"prod","deploymentType":"DEPLOYMENT_TYPE_EXECUTION_API","manifestFileName":"appsscript.json"},"deploymentType":"DEPLOYMENT_TYPE_EXECUTION_API","lifecycleState":"DEPLOYMENT_LIFECYCLE_ACTIVE"}`
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(raw))
	})
	input := writeInputFile(t, `{"manifestFileName":"appsscript.json","extra":true}`)
	if err := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "deployments", "create", "--project", "proj-1", "--version-number", "1", "--description", "prod", "--deployment-type", "DEPLOYMENT_TYPE_EXECUTION_API", "--input", input).err; err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := (*recs)[0]
	if got.Method != http.MethodPost || got.Path != "/v1/projects/proj-1/deployments" {
		t.Fatalf("request %s %s", got.Method, got.Path)
	}
	assertScriptHeaders(t, got.Header, "")
	if got.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("content-type %q", got.Header.Get("Content-Type"))
	}
	body := decodeJSON(t, got.Body)
	if body["versionNumber"] != float64(1) || body["description"] != "prod" || body["deploymentType"] != "DEPLOYMENT_TYPE_EXECUTION_API" || body["manifestFileName"] != "appsscript.json" || body["extra"] != true {
		t.Fatalf("body %#v", body)
	}
	if _, wrapped := body["deploymentConfig"]; wrapped {
		t.Fatal("wrapped deploymentConfig")
	}
}

func TestDeploymentsCreateStdinAndConflict(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(`{"deploymentId":"dep-2","deploymentConfig":{"versionNumber":4}}`))
	})
	if err := execScript(t, testClient(t, srv, ""), strings.NewReader(`{"versionNumber":4,"keep":1}`), false, nil, "script", "deployments", "create", "--project", "proj-1", "--input", "-").err; err != nil {
		t.Fatalf("stdin: %v", err)
	}
	body := decodeJSON(t, (*recs)[0].Body)
	if body["keep"] != float64(1) || body["versionNumber"] != float64(4) {
		t.Fatalf("stdin body %#v", body)
	}
	err := execScript(t, testClient(t, srv, ""), strings.NewReader(`{"versionNumber":1}`), false, nil, "script", "deployments", "create", "--project", "proj-1", "--version-number", "2", "--input", "-").err
	requireUsage(t, err, "versionNumber")
}

func TestDeploymentsCreateRequiresVersionNumber(t *testing.T) {
	err := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "deployments", "create", "--project", "proj-1", "--description", "prod").err
	requireUsage(t, err, "versionNumber")
}

func TestDeploymentsCreateRejectsInvalidInput(t *testing.T) {
	err := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), strings.NewReader("not-json"), false, nil, "script", "deployments", "create", "--project", "proj-1", "--input", "-").err
	requireUsage(t, err, "JSON")
}

func TestDeploymentsUpdateRequiresUnwrappedInput(t *testing.T) {
	raw := `{"deploymentId":"dep-3","deploymentConfig":{"versionNumber":2}}`
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(raw))
	})
	input := writeInputFile(t, `{"versionNumber":2,"description":"next"}`)
	if err := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "deployments", "update", "dep-3", "--project", "proj-1", "--input", input).err; err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := (*recs)[0]
	if got.Method != http.MethodPut || got.Path != "/v1/projects/proj-1/deployments/dep-3" {
		t.Fatalf("request %s %s", got.Method, got.Path)
	}
	if string(got.Body) != `{"versionNumber":2,"description":"next"}` {
		t.Fatalf("body %s", got.Body)
	}
	err := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "deployments", "update", "dep-3", "--project", "proj-1").err
	requireUsage(t, err, "--input")
}

func TestDeploymentsDeleteConfirmAndCancel(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		w.WriteHeader(http.StatusOK)
	})
	var prompt string
	res := execScript(t, testClient(t, srv, ""), nil, false, func(p string) error {
		prompt = p
		return nil
	}, "script", "deployments", "delete", "dep-4", "--project", "proj-1")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete deployment dep-4?" || (*recs)[0].Method != http.MethodDelete || (*recs)[0].Path != "/v1/projects/proj-1/deployments/dep-4" {
		t.Fatalf("prompt=%q request %s %s", prompt, (*recs)[0].Method, (*recs)[0].Path)
	}
	if out != "" {
		t.Fatalf("stdout=%q", out)
	}
	hits := 0
	cancelSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	t.Cleanup(cancelSrv.Close)
	err = execScript(t, testClient(t, cancelSrv, ""), nil, false, func(string) error {
		return cli.ErrCancelled
	}, "script", "deployments", "delete", "dep-4", "--project", "proj-1").err
	if !errors.Is(err, cli.ErrCancelled) || hits != 0 {
		t.Fatalf("cancel err=%v hits=%d", err, hits)
	}
	err = execScript(t, testClient(t, cancelSrv, ""), nil, false, func(string) error {
		return &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	}, "script", "deployments", "delete", "dep-4", "--project", "proj-1").err
	requireUsage(t, err, "--yes")
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestDeploymentsListRequiresProject(t *testing.T) {
	err := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "deployments", "list").err
	requireUsage(t, err, "--project")
}

func TestDeploymentsAPIError(t *testing.T) {
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"deployment not found","details":[]}`))
	})
	err := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "deployments", "get", "missing", "--project", "proj-1").err
	var api *apiclient.Error
	if !errors.As(err, &api) || api.Code != "NOT_FOUND" {
		t.Fatalf("err: %v", err)
	}
}

func TestDeploymentsHelpMentionsLibraryRestriction(t *testing.T) {
	res := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "deployments", "create", "--help")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(out, "LIBRARY") {
		t.Fatalf("help=%q", out)
	}
}

package script

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
)

func TestMetricsGetTableAndGranularity(t *testing.T) {
	raw := `{"activeUsers":[{"value":"3","startTime":"2026-01-01T00:00:00Z","endTime":"2026-01-02T00:00:00Z"}],"totalExecutions":[{"value":"10","startTime":"2026-01-01T00:00:00Z","endTime":"2026-01-02T00:00:00Z"}],"failedExecutions":[{"value":"1","startTime":"2026-01-01T00:00:00Z","endTime":"2026-01-02T00:00:00Z"}]}`
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(raw))
	})
	res := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "metrics", "get", "--project", "proj-1", "--granularity", "HOURLY")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := (*recs)[0]
	if got.Method != http.MethodGet || got.Path != "/v1/projects/proj-1/metrics" {
		t.Fatalf("request %s %s", got.Method, got.Path)
	}
	if got.Query.Get("metricsGranularity") != "HOURLY" || got.Query.Get("metricsFilterDeploymentId") != "" {
		t.Fatalf("query %v", got.Query)
	}
	assertScriptHeaders(t, got.Header, "")
	want := strings.Join([]string{
		"activeUsers\t3\t2026-01-01T00:00:00Z\t2026-01-02T00:00:00Z",
		"totalExecutions\t10\t2026-01-01T00:00:00Z\t2026-01-02T00:00:00Z",
		"failedExecutions\t1\t2026-01-01T00:00:00Z\t2026-01-02T00:00:00Z",
	}, "\n") + "\n"
	if out != want {
		t.Fatalf("stdout=%q want=%q", out, want)
	}
}

func TestMetricsGetOmitsDefaultGranularity(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(`{"activeUsers":[],"totalExecutions":[],"failedExecutions":[]}`))
	})
	if err := execScript(t, testClient(t, srv, "ns-9"), nil, false, nil, "script", "metrics", "get", "--project", "proj-1").err; err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, ok := (*recs)[0].Query["metricsGranularity"]; ok {
		t.Fatalf("query %v", (*recs)[0].Query)
	}
	assertScriptHeaders(t, (*recs)[0].Header, "ns-9")
}

func TestMetricsGetJSONUsesRawBody(t *testing.T) {
	raw := `{"activeUsers":[{"value":"1","startTime":"s","endTime":"e"}],"totalExecutions":[],"failedExecutions":[]}`
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(raw))
	})
	res := execScript(t, testClient(t, srv, ""), nil, false, nil, "--json", "script", "metrics", "get", "--project", "proj-1", "--granularity", "DAILY")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != raw {
		t.Fatalf("stdout=%q", out)
	}
}

func TestMetricsGetRejectsUnknownGranularity(t *testing.T) {
	err := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "metrics", "get", "--project", "proj-1", "--granularity", "WEEKLY").err
	requireUsage(t, err, "granularity")
}

func TestMetricsGetRequiresProject(t *testing.T) {
	err := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "metrics", "get").err
	requireUsage(t, err, "--project")
}

func TestMetricsGetAPIError(t *testing.T) {
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":3,"message":"unsupported granularity","details":[]}`))
	})
	err := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "metrics", "get", "--project", "proj-1", "--granularity", "DAILY").err
	var api *apiclient.Error
	if !errors.As(err, &api) || api.Code != "INVALID_ARGUMENT" {
		t.Fatalf("err: %v", err)
	}
}

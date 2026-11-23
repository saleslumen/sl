package workflows

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStepRunListAndGetUseSequenceResource(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone()})
		_, _ = w.Write([]byte(`{"name":"workflows/wf-1/executions/e1/stepRuns/2","stepPath":"root/approve","kind":"STEP_KIND_AWAIT_SIGNAL","state":"STATE_WAITING","attempt":1,"startTime":"t1"}`))
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	if err := execute(t, f, "step-runs", "get", "02", "--workflow", "wf-1", "--execution", "e1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(requests) != 1 || requests[0].method != http.MethodGet || requests[0].path != "/v1/workflows/wf-1/executions/e1/stepRuns/2" || requests[0].query != "" {
		t.Fatalf("requests=%+v", requests)
	}
	if out.String() != "workflows/wf-1/executions/e1/stepRuns/2\troot/approve\tSTEP_KIND_AWAIT_SIGNAL\tSTATE_WAITING\t0\t\t\t\t1\tt1\t\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestStepRunPrintsCycleScriptVersionAndRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"workflows/wf-1/executions/e1/stepRuns/2","stepPath":"root/approve","kind":"STEP_KIND_AWAIT_SIGNAL","state":"STATE_WAITING","cycle":2,"scriptVersionNumber":0,"releaseId":"rel-1","attempt":1,"startTime":"t1"}`))
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	if err := execute(t, f, "step-runs", "get", "2", "--workflow", "wf-1", "--execution", "e1"); err != nil {
		t.Fatal(err)
	}
	want := "workflows/wf-1/executions/e1/stepRuns/2\troot/approve\tSTEP_KIND_AWAIT_SIGNAL\tSTATE_WAITING\t2\t0\trel-1\t\t1\tt1\t\n"
	if out.String() != want {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestStepRunListFilter(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery})
		_, _ = w.Write([]byte(`{"stepRuns":[]}`))
	}))
	t.Cleanup(srv.Close)
	f, _, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	if err := execute(t, f, "step-runs", "list", "--workflow", "wf-1", "--execution", "e1", "--filter", "cycle = 2"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].path != "/v1/workflows/wf-1/executions/e1/stepRuns" || !strings.Contains(requests[0].query, "filter=cycle+%3D+2") || !strings.Contains(requests[0].query, "pageSize=") {
		t.Fatalf("requests=%+v", requests)
	}
}

func TestStepRunShowsKnownAgentUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"workflows/wf-1/executions/e1/stepRuns/3","stepPath":"root/agent","kind":"STEP_KIND_AGENT","state":"STATE_FAILED","agentUsage":{"modelRequests":"2","toolCalls":"1","inputTokens":"42","usageComplete":false},"attempt":1}`))
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	if err := execute(t, f, "step-runs", "get", "3", "--workflow", "wf-1", "--execution", "e1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := out.String(); !strings.Contains(got, `{"modelRequests":"2","toolCalls":"1","inputTokens":"42","usageComplete":false}`) {
		t.Fatalf("stdout=%q", got)
	}
}

func TestStepRunSequenceMustBePositiveIntegerBeforeHTTP(t *testing.T) {
	for _, sequence := range []string{"0", "-1", "abc"} {
		f, _, _, calls := newLazyFactory()
		err := execute(t, f, "step-runs", "get", sequence, "--workflow", "wf-1", "--execution", "e1")
		if err == nil {
			t.Fatalf("sequence %q succeeded", sequence)
		}
		if *calls != 0 {
			t.Fatalf("sequence %q resolved factory", sequence)
		}
	}
}

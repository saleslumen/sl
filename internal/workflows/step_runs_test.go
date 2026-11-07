package workflows

import (
	"net/http"
	"net/http/httptest"
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
	if out.String() != "workflows/wf-1/executions/e1/stepRuns/2\troot/approve\tSTEP_KIND_AWAIT_SIGNAL\tSTATE_WAITING\t1\tt1\t\n" {
		t.Fatalf("stdout=%q", out.String())
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

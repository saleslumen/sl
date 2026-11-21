package script

import (
	"errors"
	"net/http"
	"testing"

	"github.com/saleslumen/sl/internal/cli"
)

func TestScriptRunSendsBodyAndNamespace(t *testing.T) {
	rawIn := `{"executionRequest":{"function":"main","parameters":["one"]}}`
	rawOut := `{"done":true,"response":{"result":1}}`
	var gotBody string
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodPost || rec.Path != "/v1/scripts/script-1:run" || rec.Host != "script.example.test" {
			t.Errorf("request %s %s host=%s", rec.Method, rec.Path, rec.Host)
		}
		assertScriptOAuthHeaders(t, rec.Header, "ns-1")
		if rec.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type=%q", rec.Header.Get("Content-Type"))
		}
		gotBody = string(rec.Body)
		_, _ = w.Write([]byte(rawOut))
	})
	got := execScript(t, testOAuthClient(t, srv, "ns-1"), nil, false, nil, "script", "run", "script-1", "--input", writeInputFile(t, rawIn))
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if decodeJSON(t, []byte(gotBody))["executionRequest"] == nil {
		t.Fatalf("body=%s", gotBody)
	}
	req := decodeJSON(t, []byte(gotBody))["executionRequest"].(map[string]any)
	if req["function"] != "main" {
		t.Fatalf("body=%s", gotBody)
	}
	if got.stdout != rawOut {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
	assertNoTokens(t, got.stdout, got.stderr)
}

func TestScriptRunRequiresInput(t *testing.T) {
	hits := 0
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		hits++
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "run", "script-1")
	requireUsage(t, got.err, "--input")
	if hits != 0 || len(*recs) != 0 {
		t.Fatalf("hits=%d recs=%d", hits, len(*recs))
	}
	assertNoTokens(t, got.stdout, got.stderr)
}

func TestScriptRunRequiresUserOAuth(t *testing.T) {
	hits := 0
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		hits++
	})
	got := execScriptOAuth(t, testClient(t, srv, ""), nil, false, nil, func() error {
		return cli.ErrUserOAuthRequired
	}, "script", "run", "script-1", "--input", writeInputFile(t, `{"executionRequest":{"function":"main"}}`))
	if !errors.Is(got.err, cli.ErrUserOAuthRequired) {
		t.Fatalf("err=%v", got.err)
	}
	if got.err.Error() != "user OAuth login required" {
		t.Fatalf("err=%v", got.err)
	}
	if hits != 0 || len(*recs) != 0 {
		t.Fatalf("hits=%d recs=%d", hits, len(*recs))
	}
	assertNoTokens(t, got.stdout, got.stderr)
}

package script

import (
	"net/http"
	"testing"

	"github.com/saleslumen/sl/internal/cli"
)

func TestScriptRunSendsBodyAndNamespace(t *testing.T) {
	rawIn := `{"executionRequest":{"function":"main","parameters":["one"]}}`
	rawOut := `{"done":true,"response":{"result":1}}`
	jsonOut := `{"done":true,"versionNumber":3,"releaseId":"rel-9","response":{"result":1}}`
	respond := rawOut
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
		_, _ = w.Write([]byte(respond))
	})
	inputPath := writeInputFile(t, rawIn)
	got := execScript(t, testOAuthClient(t, srv, "ns-1"), nil, false, nil, "script", "run", "script-1", "--input", inputPath)
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
	if got.stdout != "true\t\t\t\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
	respond = jsonOut
	got = execScript(t, testOAuthClient(t, srv, "ns-1"), nil, false, nil, "--json", "script", "run", "script-1", "--input", inputPath)
	if got.err != nil {
		t.Fatalf("json execute: %v", got.err)
	}
	if got.stdout != jsonOut {
		t.Fatalf("json stdout=%q", got.stdout)
	}
	if len(*recs) != 2 {
		t.Fatalf("requests=%d", len(*recs))
	}
	assertNoTokens(t, got.stdout, got.stderr)
}

func TestScriptRunPrintsDraftVersionReleaseAndError(t *testing.T) {
	rawOut := `{"done":true,"versionNumber":0,"releaseId":"rel-1","error":{"code":9,"message":"archived"}}`
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(rawOut))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "run", "script-1", "--input", writeInputFile(t, `{"executionRequest":{"function":"main"}}`))
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	want := "true\t0\trel-1\tarchived\n"
	if got.stdout != want {
		t.Fatalf("stdout=%q want=%q", got.stdout, want)
	}
}

func TestScriptRunOmitsAbsentVersionNumber(t *testing.T) {
	absent, err := decodeObject[scriptOperation]([]byte(`{"done":false,"releaseId":"rel-2"}`))
	if err != nil {
		t.Fatal(err)
	}
	if absent.VersionNumber != nil {
		t.Fatalf("absent version=%v", *absent.VersionNumber)
	}
	draft, err := decodeObject[scriptOperation]([]byte(`{"done":true,"versionNumber":0}`))
	if err != nil {
		t.Fatal(err)
	}
	if draft.VersionNumber == nil || *draft.VersionNumber != 0 {
		t.Fatalf("draft version=%v", draft.VersionNumber)
	}
	rawOut := `{"done":false,"releaseId":"rel-2"}`
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(rawOut))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "run", "script-1", "--input", writeInputFile(t, `{"executionRequest":{"function":"main"}}`))
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	want := "false\t\trel-2\t\n"
	if got.stdout != want {
		t.Fatalf("stdout=%q want=%q", got.stdout, want)
	}
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

func TestScriptRunAPIKeyPostsWithoutUserOAuth(t *testing.T) {
	rawOut := `{"done":true}`
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodPost || rec.Path != "/v1/scripts/script-1:run" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		assertScriptHeaders(t, rec.Header, "")
		_, _ = w.Write([]byte(rawOut))
	})
	got := execScriptOAuth(t, testClient(t, srv, ""), nil, false, nil, func() error {
		return cli.ErrUserOAuthRequired
	}, "script", "run", "script-1", "--input", writeInputFile(t, `{"executionRequest":{"function":"main"}}`))
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
	if got.stdout != "true\t\t\t\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	assertNoTokens(t, got.stdout, got.stderr)
}

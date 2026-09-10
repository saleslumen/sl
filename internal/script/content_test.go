package script

import (
	"net/http"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
)

const contentFixture = `{"scriptId":"p1","files":[{"name":"Code.js","type":"SERVER_JS","updateTime":"2026-01-02T00:00:00Z","functionSet":{"values":[{"name":"myFunction","parameters":[]},{"name":"other","parameters":["x"]}]}},{"name":"appsscript.json","type":"JSON","updateTime":"2026-01-02T00:00:00Z"}],"etag":"W/\"1\""}`

func TestContentGetRequiresProject(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "content", "get")
	requireUsage(t, got.err, "project")
}

func TestContentGetDraftOmitsVersionQuery(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/projects/p1/content" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		if rec.Query.Get("versionNumber") != "" {
			t.Errorf("versionNumber=%q", rec.Query.Get("versionNumber"))
		}
		assertScriptHeaders(t, rec.Header, "")
		_, _ = w.Write([]byte(contentFixture))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "content", "get", "--project", "p1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	want := "Code.js\tSERVER_JS\t2026-01-02T00:00:00Z\tmyFunction,other\nappsscript.json\tJSON\t2026-01-02T00:00:00Z\t\n"
	if got.stdout != want {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestContentGetSendsVersionNumber(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Query.Get("versionNumber") != "3" {
			t.Errorf("versionNumber=%q", rec.Query.Get("versionNumber"))
		}
		_, _ = w.Write([]byte(contentFixture))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "content", "get", "--project", "p1", "--version", "3")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestContentGetJSONReturnsRawBody(t *testing.T) {
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(contentFixture))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "--json", "script", "content", "get", "--project", "p1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != contentFixture {
		t.Fatalf("stdout=%q", got.stdout)
	}
}

func TestContentUpdateRequiresInput(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "content", "update", "--project", "p1")
	requireUsage(t, got.err, "input")
}

func TestContentUpdatePassesFullHTTPBody(t *testing.T) {
	rawIn := `{"content":{"files":[{"name":"Code.js","type":"SERVER_JS","source":"function x(){}"}],"extra":true}}`
	var gotBody string
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodPut || rec.Path != "/v1/projects/p1/content" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		assertScriptHeaders(t, rec.Header, "ns-9")
		if rec.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type=%q", rec.Header.Get("Content-Type"))
		}
		gotBody = string(rec.Body)
		_, _ = w.Write([]byte(contentFixture))
	})
	got := execScript(t, testClient(t, srv, "ns-9"), nil, false, nil, "script", "content", "update", "--project", "p1", "--input", writeInputFile(t, rawIn))
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if gotBody != rawIn {
		t.Fatalf("body=%s", gotBody)
	}
	payload := decodeJSON(t, []byte(gotBody))
	content, ok := payload["content"].(map[string]any)
	if !ok || content["extra"] != true {
		t.Fatalf("body=%s", gotBody)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestContentUpdateReadsStdin(t *testing.T) {
	rawIn := `{"content":{"files":[]}}`
	var gotBody string
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		gotBody = string(rec.Body)
		_, _ = w.Write([]byte(`{"scriptId":"p1","files":[],"etag":"W/\"2\""}`))
	})
	got := execScript(t, testClient(t, srv, ""), strings.NewReader(rawIn), false, nil, "script", "content", "update", "--project", "p1", "--input", "-")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if gotBody != rawIn {
		t.Fatalf("body=%s", gotBody)
	}
}

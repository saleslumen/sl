package script

import (
	"net/http"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
)

func TestLibrariesLookupTableAndHost(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/libraries/lib-1:lookup" || rec.Host != "script.example.test" {
			t.Errorf("request %s %s host=%s", rec.Method, rec.Path, rec.Host)
		}
		assertScriptHeaders(t, rec.Header, "")
		_, _ = w.Write([]byte(`{"libraryId":"lib-1","title":"Lib","description":"Shared helpers","recommendedVersionNumber":2,"versions":[],"canUseDevelopmentMode":true}`))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "libraries", "lookup", "lib-1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != "lib-1\tLib\t2\ttrue\tShared helpers\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestLibrariesListVersionsPagesAndTable(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/libraries/lib-1/versions" || rec.Host != "script.example.test" {
			t.Errorf("request %s %s host=%s", rec.Method, rec.Path, rec.Host)
		}
		assertScriptHeaders(t, rec.Header, "")
		switch rec.Query.Get("pageToken") {
		case "":
			if rec.Query.Get("pageSize") != "2" {
				t.Errorf("pageSize=%q", rec.Query.Get("pageSize"))
			}
			_, _ = w.Write([]byte(`{"versions":[{"versionNumber":1,"description":"v1","createTime":"t1","recommended":true}],"nextPageToken":"n2"}`))
		case "n2":
			if rec.Query.Get("pageSize") != "1" {
				t.Errorf("pageSize=%q", rec.Query.Get("pageSize"))
			}
			_, _ = w.Write([]byte(`{"versions":[{"versionNumber":2,"description":"v2","createTime":"t2","recommended":false},{"versionNumber":3,"description":"v3","createTime":"t3","recommended":false}]}`))
		default:
			t.Errorf("token %q", rec.Query.Get("pageToken"))
		}
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "libraries", "list-versions", "lib-1", "--limit", "2")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if len(*recs) != 2 {
		t.Fatalf("requests=%d", len(*recs))
	}
	if got.stdout != "1\ttrue\tt1\tv1\n2\tfalse\tt2\tv2\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
}

func TestLibrariesValidateDependencyGraphRequiresInputAndPassesBody(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "libraries", "validate-dependency-graph")
	requireUsage(t, got.err, "--input")
	var body map[string]any
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodPost || rec.Path != "/v1/libraries:validateDependencyGraph" || rec.Host != "script.example.test" {
			t.Errorf("request %s %s host=%s", rec.Method, rec.Path, rec.Host)
		}
		assertScriptHeaders(t, rec.Header, "")
		if rec.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type=%q", rec.Header.Get("Content-Type"))
		}
		body = decodeJSON(t, rec.Body)
		_, _ = w.Write([]byte(`{"valid":false,"dependencyLock":"lock-1","errors":["cycle","missing"]}`))
	})
	got = execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "libraries", "validate-dependency-graph", "--input", writeInputFile(t, `{"keep":true,"libraries":[{"libraryId":"lib-1"}]}`))
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if body["keep"] != true {
		t.Fatalf("body=%#v", body)
	}
	libs, ok := body["libraries"].([]any)
	if !ok || len(libs) != 1 {
		t.Fatalf("body=%#v", body)
	}
	if got.stdout != "false\tlock-1\tcycle,missing\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestLibrariesGetReferenceRequiresVersionAndPrintsSymbols(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "libraries", "get-reference", "lib-1")
	requireUsage(t, got.err, "--version")
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/libraries/lib-1/versions/3/reference" || rec.Host != "script.example.test" {
			t.Errorf("request %s %s host=%s", rec.Method, rec.Path, rec.Host)
		}
		assertScriptHeaders(t, rec.Header, "")
		_, _ = w.Write([]byte(`{"libraryId":"lib-1","versionNumber":3,"symbols":[{"name":"foo","description":"Add","parameters":["a","b"]},{"name":"bar","description":"None","parameters":[]}]}`))
	})
	got = execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "libraries", "get-reference", "lib-1", "--version", "3")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != "foo\ta,b\tAdd\nbar\t\tNone\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestLibrariesRejectsEmptyIDVersionAndEscapesPath(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "libraries", "lookup", "   ")
	requireUsage(t, got.err, "library ID is required")
	got = execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "libraries", "get-reference", "lib-1", "--version", "0")
	requireUsage(t, got.err, "--version must be at least 1")
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/libraries/lib%2F1:lookup" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		_, _ = w.Write([]byte(`{"libraryId":"lib/1","title":"Lib","description":"","recommendedVersionNumber":1,"versions":[],"canUseDevelopmentMode":false}`))
	})
	got = execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "libraries", "lookup", "lib/1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestLibrariesCommandTree(t *testing.T) {
	root := NewCommand(&cli.Factory{})
	cases := []struct{ path, use string }{
		{"libraries lookup", "lookup ID"},
		{"libraries list-versions", "list-versions ID"},
		{"libraries validate-dependency-graph", "validate-dependency-graph --input FILE"},
		{"libraries get-reference", "get-reference ID --version N"},
	}
	for _, tc := range cases {
		cmd, _, err := root.Find(strings.Fields(tc.path))
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if cmd.Use != tc.use {
			t.Errorf("%s Use=%q want=%q", tc.path, cmd.Use, tc.use)
		}
	}
}

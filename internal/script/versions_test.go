package script

import (
	"net/http"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
)

func TestVersionsListRequiresProject(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "versions", "list")
	requireUsage(t, got.err, "project")
}

func TestVersionsListPagesAndTable(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/projects/p1/versions" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		switch rec.Query.Get("pageToken") {
		case "":
			if rec.Query.Get("pageSize") != "2" {
				t.Errorf("pageSize=%q", rec.Query.Get("pageSize"))
			}
			_, _ = w.Write([]byte(`{"versions":[{"scriptId":"p1","versionNumber":1,"description":"v1","createTime":"t1","contentDigest":"c1","manifestDigest":"m1","contractDigest":"k1"}],"nextPageToken":"n2"}`))
		case "n2":
			if rec.Query.Get("pageSize") != "1" {
				t.Errorf("pageSize=%q", rec.Query.Get("pageSize"))
			}
			_, _ = w.Write([]byte(`{"versions":[{"scriptId":"p1","versionNumber":2,"description":"v2","createTime":"t2","contentDigest":"c2","manifestDigest":"m2","contractDigest":"k2"},{"scriptId":"p1","versionNumber":3,"description":"v3","createTime":"t3","contentDigest":"c3","manifestDigest":"m3","contractDigest":"k3"}]}`))
		default:
			t.Errorf("token %q", rec.Query.Get("pageToken"))
		}
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "versions", "list", "--project", "p1", "--limit", "2")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if len(*recs) != 2 {
		t.Fatalf("requests=%d", len(*recs))
	}
	if got.stdout != "1\tv1\tt1\tc1\tm1\tk1\n2\tv2\tt2\tc2\tm2\tk2\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
}

func TestVersionsGet(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/projects/p1/versions/2" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		assertScriptHeaders(t, rec.Header, "")
		_, _ = w.Write([]byte(`{"scriptId":"p1","versionNumber":2,"description":"v2","createTime":"t2","contentDigest":"c2","manifestDigest":"m2","contractDigest":"k2"}`))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "versions", "get", "2", "--project", "p1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != "2\tv2\tt2\tc2\tm2\tk2\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestVersionsContent(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/projects/p1/versions/2/content" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		_, _ = w.Write([]byte(contentFixture))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "versions", "content", "2", "--project", "p1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if !strings.Contains(got.stdout, "Code.js\tSERVER_JS\t") {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestVersionsCompareQuery(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/projects/p1/versions:compare" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		if rec.Query.Get("fromVersionNumber") != "0" || rec.Query.Get("toVersionNumber") != "1" {
			t.Errorf("query=%v", rec.Query)
		}
		_, _ = w.Write([]byte(`{"scriptId":"p1","fromVersionNumber":0,"toVersionNumber":1,"fileDiffs":[{"name":"Code.js","status":"FILE_MODIFIED"},{"name":"appsscript.json","status":"FILE_UNCHANGED"}]}`))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "versions", "compare", "--project", "p1", "--from", "0", "--to", "1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != "Code.js\tFILE_MODIFIED\nappsscript.json\tFILE_UNCHANGED\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestVersionsCompareRequiresFromTo(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "versions", "compare", "--project", "p1")
	requireUsage(t, got.err, "--from")
}

func TestVersionsRestoreUsesFlagEtagWithoutGet(t *testing.T) {
	var body map[string]any
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodPost || rec.Path != "/v1/projects/p1/versions/2:restore" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		body = decodeJSON(t, rec.Body)
		_, _ = w.Write([]byte(contentFixture))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "versions", "restore", "2", "--project", "p1", "--draft-etag", `W/"flag"`)
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if body["draftEtag"] != `W/"flag"` || len(body) != 1 {
		t.Fatalf("body=%#v", body)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestVersionsRestoreUsesInputEtagWithoutGet(t *testing.T) {
	var body map[string]any
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Path != "/v1/projects/p1/versions/2:restore" {
			t.Errorf("path=%s", rec.Path)
		}
		body = decodeJSON(t, rec.Body)
		_, _ = w.Write([]byte(contentFixture))
	})
	input := writeInputFile(t, `{"draftEtag":"W/\"in\"","keep":true}`)
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "versions", "restore", "2", "--project", "p1", "--input", input)
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if body["draftEtag"] != `W/"in"` || body["keep"] != true {
		t.Fatalf("body=%#v", body)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestVersionsRestoreFetchesDraftEtagOnce(t *testing.T) {
	var methods []string
	var restoreBody map[string]any
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		methods = append(methods, rec.Method+" "+rec.Path)
		assertScriptHeaders(t, rec.Header, "ns-1")
		switch {
		case rec.Method == http.MethodGet && rec.Path == "/v1/projects/p1/content":
			if rec.Query.Get("versionNumber") != "" {
				t.Errorf("content query=%v", rec.Query)
			}
			_, _ = w.Write([]byte(`{"scriptId":"p1","files":[],"etag":"W/\"fetched\""}`))
		case rec.Method == http.MethodPost && rec.Path == "/v1/projects/p1/versions/2:restore":
			restoreBody = decodeJSON(t, rec.Body)
			_, _ = w.Write([]byte(contentFixture))
		default:
			t.Errorf("unexpected %s %s", rec.Method, rec.Path)
		}
	})
	input := writeInputFile(t, `{"keep":"yes"}`)
	got := execScript(t, testClient(t, srv, "ns-1"), nil, false, nil, "script", "versions", "restore", "2", "--project", "p1", "--input", input)
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if strings.Join(methods, ",") != "GET /v1/projects/p1/content,POST /v1/projects/p1/versions/2:restore" {
		t.Fatalf("methods=%v", methods)
	}
	if restoreBody["draftEtag"] != `W/"fetched"` || restoreBody["keep"] != "yes" {
		t.Fatalf("body=%#v", restoreBody)
	}
	if len(*recs) != 2 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestVersionsRestoreRejectsDuplicateDraftEtag(t *testing.T) {
	hits := 0
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		hits++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	input := writeInputFile(t, `{"draftEtag":"W/\"in\""}`)
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "versions", "restore", "2", "--project", "p1", "--draft-etag", `W/"flag"`, "--input", input)
	requireUsage(t, got.err, "draftEtag")
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestVersionsRestoreDoesNotPostWhenGetFails(t *testing.T) {
	var methods []string
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		methods = append(methods, rec.Method+" "+rec.Path)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"project not found","details":[]}`))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "versions", "restore", "2", "--project", "p1")
	if got.err == nil || !strings.Contains(got.err.Error(), "project not found") {
		t.Fatalf("err=%v", got.err)
	}
	if strings.Join(methods, ",") != "GET /v1/projects/p1/content" {
		t.Fatalf("methods=%v", methods)
	}
}

func TestVersionsHelpMentionsUserCredential(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "versions", "--help")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if !strings.Contains(got.stdout, "user credential") {
		t.Fatalf("help=%q", got.stdout)
	}
}

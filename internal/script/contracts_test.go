package script

import (
	"errors"
	"net/http"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
)

func TestContractsGetTable(t *testing.T) {
	raw := `{"contracts":[{"name":"greet","description":"Greet a caller","inputs":{"name":{"type":"string","description":"Person","required":true},"title":{"type":"string"}},"outputs":{"greeting":{"type":"string"}},"errors":[{"code":"Error","description":"bad"}]}],"contractDigest":"abc"}`
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(raw))
	})
	res := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "contracts", "get", "proj-1")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := (*recs)[0]
	if got.Method != http.MethodGet || got.Path != "/v1/scripts/proj-1/contracts" {
		t.Fatalf("request %s %s", got.Method, got.Path)
	}
	if _, ok := got.Query["versionNumber"]; ok {
		t.Fatalf("query %v", got.Query)
	}
	assertScriptHeaders(t, got.Header, "")
	want := "greet\tGreet a caller\tname,title\tgreeting\tError\n"
	if out != want {
		t.Fatalf("stdout=%q want=%q", out, want)
	}
}

func TestContractsGetVersionAndNamespace(t *testing.T) {
	raw := `{"contracts":[],"contractDigest":""}`
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(raw))
	})
	res := execScript(t, testClient(t, srv, "ns-3"), nil, false, nil, "--json", "script", "contracts", "get", "proj/1", "--version", "2")
	out, err := res.stdout, res.err
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := (*recs)[0]
	if got.Path != "/v1/scripts/proj%2F1/contracts" {
		t.Fatalf("path %s", got.Path)
	}
	if got.Query.Get("versionNumber") != "2" {
		t.Fatalf("query %v", got.Query)
	}
	assertScriptHeaders(t, got.Header, "ns-3")
	if out != raw {
		t.Fatalf("stdout=%q", out)
	}
}

func TestContractsGetRequiresProject(t *testing.T) {
	err := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "contracts", "get").err
	requireUsage(t, err, "")
}

func TestContractsGetAPIError(t *testing.T) {
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"project not found","details":[]}`))
	})
	err := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "contracts", "get", "missing").err
	var api *apiclient.Error
	if !errors.As(err, &api) || api.Code != "NOT_FOUND" {
		t.Fatalf("err: %v", err)
	}
}

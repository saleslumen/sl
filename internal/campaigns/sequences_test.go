package campaigns

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

const (
	seqCampaign = "camp-1"
	seqID       = "seq-1"
	seqFixture  = `{"name":"campaigns/camp-1/sequences/seq-1","campaign":"campaigns/camp-1","kind":"TRIGGERED","display_name":"Replies","steps":[{},{}],"etag":"3","create_time":"2026-01-01T00:00:00Z","update_time":"2026-01-02T03:04:05Z"}`
	seqTable    = "seq-1\tReplies\tTRIGGERED\t2\t3\t2026-01-02T03:04:05Z\n"
	seqPreview  = `{"base_render_identity":"rid-1","subject":"Hello","html":"<p>Hi</p>","text":"Hi","content_mode":"MULTIPART","track_opens":true,"track_clicks":false,"unsubscribe_policy":"HEADER_AND_FOOTER","warnings":["missing_var"],"operational_urls_differ":true}`
	seqPrevRow  = "Hello\tMULTIPART\ttrue\tfalse\tHEADER_AND_FOOTER\tmissing_var\ttrue\n"
	seqReqID    = "00000000-0000-4000-8000-000000000001"
)

type seqRequest struct {
	Method  string
	Host    string
	Path    string
	Query   string
	Headers http.Header
	Body    string
}

func sequencesFactory(t *testing.T, stdin io.Reader, stdout io.Writer, jsonMode bool, confirm func(string) error) *cli.Factory {
	t.Helper()
	f := testFactory(stdin, stdout)
	f.Organization = func() (string, error) {
		t.Fatal("Organization must not be called")
		return "", nil
	}
	f.Printer = func() *output.Printer {
		return output.New(stdout, output.Options{JSON: jsonMode})
	}
	if confirm != nil {
		f.Confirm = confirm
	} else {
		f.Confirm = func(string) error { return nil }
	}
	return f
}

func executeSequences(t *testing.T, f *cli.Factory, args ...string) error {
	t.Helper()
	cmd := newSequencesCommand(f)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func writeSeqInput(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func captureSequences(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]seqRequest) {
	t.Helper()
	var got []seqRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		got = append(got, seqRequest{Method: r.Method, Host: r.Host, Path: r.URL.Path, Query: r.URL.RawQuery, Headers: r.Header.Clone(), Body: string(raw)})
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func seqOK(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}
}

func assertSeqRequest(t *testing.T, got seqRequest, method, path, query, body, namespace string, contentType bool) {
	t.Helper()
	if got.Method != method || got.Host != "campaigns.example.test" || got.Path != path || got.Query != query || got.Body != body {
		t.Fatalf("request %s %s %s?%s body=%q", got.Method, got.Host, got.Path, got.Query, got.Body)
	}
	want := map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"}
	if namespace != "" {
		want["Sl-Namespace-Id"] = namespace
	}
	if contentType {
		want["Content-Type"] = "application/json"
	}
	assertApplicationHeaders(t, got.Headers, want)
	if got.Headers.Get("sl-organization-id") != "" || got.Headers.Get("Authorization") != "" {
		t.Fatalf("forbidden headers %#v", applicationHeaders(got.Headers))
	}
}

func assertUsage(t *testing.T, err error, substr string) {
	t.Helper()
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, substr) {
		t.Fatalf("usage=%v want %q", err, substr)
	}
}

func TestSequencesCommandWiring(t *testing.T) {
	var stdout bytes.Buffer
	called := false
	f := sequencesFactory(t, nil, &stdout, false, nil)
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	cmd := newSequencesCommand(f)
	if cmd.Use != "sequences" || cmd.Short != "Manage campaign sequences" || strings.Contains(cmd.Short, "\n") {
		t.Fatalf("use=%q short=%q", cmd.Use, cmd.Short)
	}
	if cmd.PersistentFlags().Lookup("campaign") == nil {
		t.Fatal("missing persistent --campaign")
	}
	want := map[string]string{"list": "List campaign sequences", "get": "Get a campaign sequence", "create": "Create a triggered sequence", "update": "Replace a campaign sequence", "delete": "Delete a triggered sequence", "preview": "Preview a sequence variant"}
	wantUse := map[string]string{"list": "list --campaign ID", "get": "get ID --campaign ID", "create": "create --campaign ID --input FILE", "update": "update ID --campaign ID --input FILE", "delete": "delete ID --campaign ID", "preview": "preview ID --campaign ID --input FILE"}
	got := map[string]*cobra.Command{}
	for _, child := range cmd.Commands() {
		got[child.Name()] = child
		if want[child.Name()] != child.Short || strings.Contains(child.Short, "\n") {
			t.Fatalf("%s short=%q", child.Name(), child.Short)
		}
		if child.Use != wantUse[child.Name()] {
			t.Fatalf("%s use=%q want=%q", child.Name(), child.Use, wantUse[child.Name()])
		}
		if child.InheritedFlags().Lookup("campaign") == nil {
			t.Fatalf("%s missing --campaign", child.Name())
		}
	}
	for name := range want {
		if got[name] == nil {
			t.Fatalf("missing %s in %#v", name, got)
		}
	}
	if got["list"].Flags().Lookup("limit") == nil || got["list"].Flags().Lookup("limit").DefValue != "50" {
		t.Fatalf("list --limit default=%v", got["list"].Flags().Lookup("limit"))
	}
	for _, name := range []string{"create", "update", "preview"} {
		if got[name].Flags().Lookup("input") == nil {
			t.Fatalf("%s missing --input", name)
		}
	}
	for _, name := range []string{"create", "update", "delete"} {
		if got[name].Flags().Lookup("request-id") == nil {
			t.Fatalf("%s missing --request-id", name)
		}
	}
	for _, name := range []string{"update", "delete"} {
		if flag := got[name].Flags().Lookup("etag"); flag == nil || !strings.Contains(flag.Usage, "fetched") {
			t.Fatalf("%s --etag=%v", name, got[name].Flags().Lookup("etag"))
		}
	}
	if got["preview"].Flags().Lookup("request-id") != nil || got["preview"].Flags().Lookup("etag") != nil {
		t.Fatal("preview must not expose control flags")
	}
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	if called {
		t.Fatal("help resolved client")
	}
	help := stdout.String()
	for name := range want {
		if !strings.Contains(help, name) {
			t.Fatalf("help missing %s: %s", name, help)
		}
	}
}

func TestSequencesListSendsQueryHeadersAndTable(t *testing.T) {
	srv, got := captureSequences(t, seqOK(`{"sequences":[`+seqFixture+`],"next_page_token":""}`))
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeSequences(t, f, "list", "--campaign", seqCampaign); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertSeqRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns/camp-1/sequences", "page_size=50", "", "", false)
	if out.String() != seqTable {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestSequencesListPaginationAndJSON(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/campaigns/camp-1/sequences" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		assertApplicationHeaders(t, r.Header, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json"})
		queries = append(queries, r.URL.RawQuery)
		switch r.URL.Query().Get("page_token") {
		case "":
			_, _ = w.Write([]byte(`{"sequences":[{"name":"campaigns/camp-1/sequences/a","display_name":"A","kind":"MAIN","steps":[],"etag":"1","update_time":"t1"},{"name":"campaigns/camp-1/sequences/b","display_name":"B","kind":"TRIGGERED","steps":[{}],"etag":"2","update_time":"t2"}],"next_page_token":"p2"}`))
		case "p2":
			_, _ = w.Write([]byte(`{"sequences":[{"name":"campaigns/camp-1/sequences/c","display_name":"C","kind":"TRIGGERED","steps":[{},{},{}],"etag":"3","update_time":"t3"},{"name":"campaigns/camp-1/sequences/d","display_name":"D","kind":"TRIGGERED","steps":null,"etag":"4","update_time":"t4"}]}`))
		default:
			t.Errorf("token %q", r.URL.Query().Get("page_token"))
		}
	}))
	t.Cleanup(srv.Close)
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-1"), nil }
	if err := executeSequences(t, f, "list", "--campaign", seqCampaign, "--limit", "3"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(queries) != 2 || queries[0] != "page_size=3" || queries[1] != "page_size=1&page_token=p2" {
		t.Fatalf("queries %#v", queries)
	}
	want := "a\tA\tMAIN\t0\t1\tt1\nb\tB\tTRIGGERED\t1\t2\tt2\nc\tC\tTRIGGERED\t3\t3\tt3\n"
	if out.String() != want {
		t.Fatalf("stdout=%q", out.String())
	}
	out.Reset()
	queries = nil
	f.Printer = func() *output.Printer { return output.New(&out, output.Options{JSON: true}) }
	if err := executeSequences(t, f, "list", "--campaign", seqCampaign, "--limit", "3"); err != nil {
		t.Fatalf("json: %v", err)
	}
	var payload struct {
		Sequences []map[string]json.RawMessage `json:"sequences"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("decode json: %v %s", err, out.String())
	}
	if len(payload.Sequences) != 3 {
		t.Fatalf("json items=%d body=%s", len(payload.Sequences), out.String())
	}
	if string(payload.Sequences[2]["name"]) != `"campaigns/camp-1/sequences/c"` {
		t.Fatalf("json body=%s", out.String())
	}
}

func TestSequencesListRequiresCampaignAndValidLimit(t *testing.T) {
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { t.Fatal("client"); return nil, nil }
	assertUsage(t, executeSequences(t, f, "list"), "--campaign")
	assertUsage(t, executeSequences(t, f, "list", "--campaign", seqCampaign, "--limit", "0"), "--limit")
	assertUsage(t, executeSequences(t, f, "list", "--campaign", seqCampaign, "--limit", "-1"), "--limit")
}

func TestSequencesListEmptyAndCappedPageSize(t *testing.T) {
	srv, got := captureSequences(t, seqOK(`{"sequences":[]}`))
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeSequences(t, f, "list", "--campaign", seqCampaign, "--limit", "500"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 || (*got)[0].Query != "page_size=200" {
		t.Fatalf("request %#v", *got)
	}
	if out.String() != "" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestSequencesGetSendsPathHeadersAndTable(t *testing.T) {
	srv, got := captureSequences(t, seqOK(seqFixture))
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-9"), nil }
	if err := executeSequences(t, f, "get", seqID, "--campaign", seqCampaign); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertSeqRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns/camp-1/sequences/seq-1", "", "", "ns-9", false)
	if out.String() != seqTable {
		t.Fatalf("stdout=%q", out.String())
	}
	out.Reset()
	f.Printer = func() *output.Printer { return output.New(&out, output.Options{JSON: true}) }
	if err := executeSequences(t, f, "get", seqID, "--campaign", seqCampaign); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.String() != seqFixture {
		t.Fatalf("json=%q", out.String())
	}
}

func TestSequencesGetRequiresCampaignAndID(t *testing.T) {
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { t.Fatal("client"); return nil, nil }
	assertUsage(t, executeSequences(t, f, "get", seqID), "--campaign")
	if err := executeSequences(t, f, "get", "--campaign", seqCampaign); err == nil {
		t.Fatal("expected missing id")
	}
}

func TestSequencesCreateInjectsRequestIDAndPreservesBody(t *testing.T) {
	srv, got := captureSequences(t, seqOK(seqFixture))
	var out bytes.Buffer
	input := writeSeqInput(t, `{"display_name":"Replies","trigger":{"event":"EMAIL.REPLIED"},"steps":[{"kind":"EMAIL"}]}`)
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeSequences(t, f, "create", "--campaign", seqCampaign, "--input", input); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	wantBody := `{"display_name":"Replies","request_id":"` + seqReqID + `","steps":[{"kind":"EMAIL"}],"trigger":{"event":"EMAIL.REPLIED"}}`
	assertSeqRequest(t, (*got)[0], http.MethodPost, "/v1/campaigns/camp-1/sequences", "", wantBody, "", true)
	if out.String() != seqTable {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestSequencesCreateRequestIDSourcesAndConflicts(t *testing.T) {
	srv, got := captureSequences(t, seqOK(seqFixture))
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-2"), nil }
	input := writeSeqInput(t, `{"display_name":"Replies","request_id":"input-id","trigger":{"event":"EMAIL.OPENED"}}`)
	if err := executeSequences(t, f, "create", "--campaign", seqCampaign, "--input", input); err != nil {
		t.Fatalf("input id: %v", err)
	}
	assertSeqRequest(t, (*got)[0], http.MethodPost, "/v1/campaigns/camp-1/sequences", "", `{"display_name":"Replies","request_id":"input-id","trigger":{"event":"EMAIL.OPENED"}}`, "ns-2", true)
	*got = nil
	flagInput := writeSeqInput(t, `{"display_name":"Replies","trigger":{"event":"EMAIL.OPENED"}}`)
	if err := executeSequences(t, f, "create", "--campaign", seqCampaign, "--input", flagInput, "--request-id", "flag-id"); err != nil {
		t.Fatalf("flag id: %v", err)
	}
	assertSeqRequest(t, (*got)[0], http.MethodPost, "/v1/campaigns/camp-1/sequences", "", `{"display_name":"Replies","request_id":"flag-id","trigger":{"event":"EMAIL.OPENED"}}`, "ns-2", true)
	conflict := writeSeqInput(t, `{"request_id":"input-id","display_name":"Replies"}`)
	assertUsage(t, executeSequences(t, f, "create", "--campaign", seqCampaign, "--input", conflict, "--request-id", "flag-id"), "--request-id")
}

func TestSequencesCreateRequiresInputCampaignAndValidJSON(t *testing.T) {
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { t.Fatal("client"); return nil, nil }
	assertUsage(t, executeSequences(t, f, "create", "--input", writeSeqInput(t, `{}`)), "--campaign")
	assertUsage(t, executeSequences(t, f, "create", "--campaign", seqCampaign), "--input")
	assertUsage(t, executeSequences(t, f, "create", "--campaign", seqCampaign, "--input", writeSeqInput(t, `[]`)), "JSON object")
	stdin := sequencesFactory(t, strings.NewReader(`{"display_name":"X","trigger":{"event":"EMAIL.CLICKED"}}`), &out, false, nil)
	srv, got := captureSequences(t, seqOK(seqFixture))
	stdin.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeSequences(t, stdin, "create", "--campaign", seqCampaign, "--input", "-"); err != nil {
		t.Fatalf("stdin: %v", err)
	}
	if !strings.Contains((*got)[0].Body, `"display_name":"X"`) || !strings.Contains((*got)[0].Body, `"request_id":"`+seqReqID+`"`) {
		t.Fatalf("stdin body=%s", (*got)[0].Body)
	}
}

func TestSequencesUpdatePrefetchesEtagAndPatches(t *testing.T) {
	srv, got := captureSequences(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/campaigns/camp-1/sequences/seq-1":
			_, _ = w.Write([]byte(`{"name":"campaigns/camp-1/sequences/seq-1","etag":"77","display_name":"Kept"}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/campaigns/camp-1/sequences/seq-1":
			_, _ = w.Write([]byte(seqFixture))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	input := writeSeqInput(t, `{"sequence":{"display_name":"Replies","kind":"TRIGGERED","steps":[]},"update_mask":"*"}`)
	if err := executeSequences(t, f, "update", seqID, "--campaign", seqCampaign, "--input", input); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 2 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertSeqRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns/camp-1/sequences/seq-1", "", "", "", false)
	wantBody := `{"etag":"77","request_id":"` + seqReqID + `","sequence":{"display_name":"Replies","kind":"TRIGGERED","steps":[]},"update_mask":"*"}`
	assertSeqRequest(t, (*got)[1], http.MethodPatch, "/v1/campaigns/camp-1/sequences/seq-1", "", wantBody, "", true)
	if out.String() != seqTable {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestSequencesUpdateControlFlagsAndConflicts(t *testing.T) {
	srv, got := captureSequences(t, seqOK(seqFixture))
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-3"), nil }
	input := writeSeqInput(t, `{"sequence":{"display_name":"X"},"update_mask":"steps"}`)
	if err := executeSequences(t, f, "update", seqID, "--campaign", seqCampaign, "--input", input, "--etag", "flag-etag", "--request-id", "flag-id"); err != nil {
		t.Fatalf("flags: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("prefetch leaked hits=%d", len(*got))
	}
	assertSeqRequest(t, (*got)[0], http.MethodPatch, "/v1/campaigns/camp-1/sequences/seq-1", "", `{"etag":"flag-etag","request_id":"flag-id","sequence":{"display_name":"X"},"update_mask":"steps"}`, "ns-3", true)
	*got = nil
	kept := writeSeqInput(t, `{"etag":"input-etag","request_id":"input-id","sequence":{"display_name":"X"},"update_mask":"*"}`)
	if err := executeSequences(t, f, "update", seqID, "--campaign", seqCampaign, "--input", kept); err != nil {
		t.Fatalf("input controls: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("input prefetch hits=%d", len(*got))
	}
	if !strings.Contains((*got)[0].Body, `"etag":"input-etag"`) || !strings.Contains((*got)[0].Body, `"request_id":"input-id"`) {
		t.Fatalf("body=%s", (*got)[0].Body)
	}
	conflictEtag := writeSeqInput(t, `{"etag":"input-etag","sequence":{}}`)
	assertUsage(t, executeSequences(t, f, "update", seqID, "--campaign", seqCampaign, "--input", conflictEtag, "--etag", "flag-etag"), "--etag")
	conflictRID := writeSeqInput(t, `{"request_id":"input-id","sequence":{}}`)
	assertUsage(t, executeSequences(t, f, "update", seqID, "--campaign", seqCampaign, "--input", conflictRID, "--request-id", "flag-id"), "--request-id")
	assertUsage(t, executeSequences(t, f, "update", seqID, "--campaign", seqCampaign), "--input")
	assertUsage(t, executeSequences(t, f, "update", seqID, "--input", input), "--campaign")
}

func TestSequencesDeletePrefetchesQueryControlsAndConfirms(t *testing.T) {
	var prompt string
	srv, got := captureSequences(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"name":"campaigns/camp-1/sequences/seq-1","etag":"9"}`))
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, func(p string) error {
		prompt = p
		return nil
	})
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeSequences(t, f, "delete", seqID, "--campaign", seqCampaign); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete sequence seq-1?" {
		t.Fatalf("prompt=%q", prompt)
	}
	if len(*got) != 2 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertSeqRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns/camp-1/sequences/seq-1", "", "", "", false)
	assertSeqRequest(t, (*got)[1], http.MethodDelete, "/v1/campaigns/camp-1/sequences/seq-1", "etag=9&request_id="+seqReqID, "", "", false)
	if out.String() != "" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestSequencesDeleteFlagsDeclineAndConflict(t *testing.T) {
	srv, got := captureSequences(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, "ns-4"), nil }
	if err := executeSequences(t, f, "delete", seqID, "--campaign", seqCampaign, "--etag", "flag-etag", "--request-id", "flag-id"); err != nil {
		t.Fatalf("flags: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("prefetch leaked hits=%d", len(*got))
	}
	assertSeqRequest(t, (*got)[0], http.MethodDelete, "/v1/campaigns/camp-1/sequences/seq-1", "etag=flag-etag&request_id=flag-id", "", "ns-4", false)
	declined := sequencesFactory(t, nil, &out, false, func(string) error { return cli.ErrCancelled })
	declined.Client = f.Client
	if err := executeSequences(t, declined, "delete", seqID, "--campaign", seqCampaign, "--etag", "1"); !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("declined: %v", err)
	}
	missingYes := sequencesFactory(t, nil, &out, false, func(string) error {
		return &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	})
	missingYes.Client = f.Client
	assertUsage(t, executeSequences(t, missingYes, "delete", seqID, "--campaign", seqCampaign, "--etag", "1"), "--yes")
	assertUsage(t, executeSequences(t, f, "delete", seqID), "--campaign")
}

func TestSequencesDeleteIsSilentAndReportsPrefetchError(t *testing.T) {
	srv, _ := captureSequences(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			_, _ = w.Write([]byte(`{"deleted":true}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NOT_FOUND","message":"missing"}`))
	})
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	if err := executeSequences(t, f, "delete", seqID, "--campaign", seqCampaign); err == nil || !strings.Contains(err.Error(), "fetch etag") {
		t.Fatalf("prefetch: %v", err)
	}
	out.Reset()
	ok, got := captureSequences(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"deleted":true}`))
	})
	f.Client = func() (*apiclient.Client, error) { return testClient(t, ok, ""), nil }
	if err := executeSequences(t, f, "delete", seqID, "--campaign", seqCampaign, "--etag", "7"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if (*got)[0].Method != http.MethodDelete || out.String() != "" {
		t.Fatalf("delete stdout=%q req=%#v", out.String(), (*got)[0])
	}
}

func TestSequencesPreviewSendsBodyTableAndRawJSON(t *testing.T) {
	srv, got := captureSequences(t, seqOK(seqPreview))
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	input := writeSeqInput(t, `{"variant_id":"var-1","person_id":"p-1"}`)
	if err := executeSequences(t, f, "preview", seqID, "--campaign", seqCampaign, "--input", input); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("hits=%d", len(*got))
	}
	assertSeqRequest(t, (*got)[0], http.MethodPost, "/v1/campaigns/camp-1/sequences/seq-1:preview", "", `{"person_id":"p-1","variant_id":"var-1"}`, "", true)
	if out.String() != seqPrevRow {
		t.Fatalf("stdout=%q", out.String())
	}
	out.Reset()
	f.Printer = func() *output.Printer { return output.New(&out, output.Options{JSON: true}) }
	sample := writeSeqInput(t, `{"variant_id":"var-1","sample_variables":{"given_name":"Ada"}}`)
	if err := executeSequences(t, f, "preview", seqID, "--campaign", seqCampaign, "--input", sample); err != nil {
		t.Fatalf("json: %v", err)
	}
	if out.String() != seqPreview {
		t.Fatalf("json=%q", out.String())
	}
	if strings.Contains((*got)[1].Body, "request_id") || strings.Contains((*got)[1].Body, "etag") {
		t.Fatalf("preview injected controls %s", (*got)[1].Body)
	}
	assertUsage(t, executeSequences(t, f, "preview", seqID, "--campaign", seqCampaign), "--input")
	assertUsage(t, executeSequences(t, f, "preview", seqID, "--input", input), "--campaign")
}

func TestSequencesPreviewObjectFallback(t *testing.T) {
	srv, _ := captureSequences(t, seqOK(`["not-an-object"]`))
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	err := executeSequences(t, f, "preview", seqID, "--campaign", seqCampaign, "--input", writeSeqInput(t, `{"variant_id":"var-1","person_id":"p-1"}`))
	if err == nil || !strings.Contains(err.Error(), "decode sequence preview") {
		t.Fatalf("err: %v", err)
	}
	if out.String() != "" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestSequencesAPIConflict(t *testing.T) {
	srv, _ := captureSequences(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"ABORTED","message":"etag mismatch"}`))
	})
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	err := executeSequences(t, f, "create", "--campaign", seqCampaign, "--input", writeSeqInput(t, `{"display_name":"X","trigger":{"event":"EMAIL.OPENED"}}`))
	var apiErr *apiclient.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
		t.Fatalf("conflict: %v", err)
	}
}

func TestSequencesStepCountFormatting(t *testing.T) {
	srv, _ := captureSequences(t, seqOK(`{"name":"campaigns/camp-1/sequences/seq-1","kind":"MAIN","display_name":"Main","steps":{"n":1},"etag":"1","update_time":"t"}`))
	var out bytes.Buffer
	f := sequencesFactory(t, nil, &out, false, nil)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, ""), nil }
	err := executeSequences(t, f, "get", seqID, "--campaign", seqCampaign)
	if err == nil || !strings.Contains(err.Error(), "decode sequence") {
		t.Fatalf("err: %v", err)
	}
}

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
)

const (
	peopleRequestID = "00000000-0000-4000-8000-000000000001"
	personJSON      = `{"name":"campaigns/c1/people/p1","campaign":"campaigns/c1","email_address":"alex@example.test","variables":{"given_name":"Alex"},"enrollment_state":"ELIGIBLE","active_sequence":"campaigns/c1/sequences/s1","eligible_time":"2026-01-01T00:00:00Z","etag":"3","create_time":"2026-01-01T00:00:00Z","update_time":"2026-01-02T00:00:00Z"}`
	personTable     = "p1\talex@example.test\tELIGIBLE\ts1\t2026-01-01T00:00:00Z\t2026-01-02T00:00:00Z\n"
	campaignJSON    = `{"name":"campaigns/c1","etag":"77","display_name":"Kept"}`
	operationJSON   = `{"name":"operations/op1","state":"RUNNING","campaign":"campaigns/c1","command_type":"PAUSE_PERSON","cancelled_delivery_count":2,"already_attempting_delivery_count":1,"create_time":"2026-01-01T00:00:00Z","update_time":"2026-01-04T00:00:00Z"}`
	operationTable  = "op1\tc1\tRUNNING\tPAUSE_PERSON\t2\t1\t2026-01-04T00:00:00Z\n"
	taskJSON        = `{"name":"campaigns/c1/tasks/t1","campaign":"campaigns/c1","type":"IMPORT_PEOPLE","state":"PENDING","message":"queued","create_time":"2026-01-01T00:00:00Z","update_time":"2026-01-05T00:00:00Z"}`
	taskTable       = "t1\tc1\tIMPORT_PEOPLE\tPENDING\tqueued\t2026-01-05T00:00:00Z\n"
	activityJSON    = `{"event_id":"e1","occurred_at":"2026-01-03T00:00:00Z","type":"ENROLLED","reason_code":"USER_REQUEST","summary":"enrolled"}`
	activityTable   = "e1\tENROLLED\tUSER_REQUEST\tenrolled\t2026-01-03T00:00:00Z\n"
)

type peopleReq struct {
	method  string
	host    string
	path    string
	query   string
	headers http.Header
	body    string
}

func executePeople(t *testing.T, f *cli.Factory, args ...string) error {
	t.Helper()
	cmd := newPeopleCommand(f)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func peopleFactory(t *testing.T, srv *httptest.Server, namespace string, stdin io.Reader, stdout io.Writer, jsonMode bool) *cli.Factory {
	t.Helper()
	f := testFactory(stdin, stdout)
	f.Client = func() (*apiclient.Client, error) {
		return testClient(t, srv, namespace), nil
	}
	f.Printer = func() *output.Printer {
		return output.New(stdout, output.Options{JSON: jsonMode})
	}
	f.Confirm = func(string) error { return nil }
	return f
}

func writePeopleInput(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func peopleUsage(t *testing.T, err error) string {
	t.Helper()
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("error=%v", err)
	}
	return usage.Error()
}

func assertPeopleJSON(t *testing.T, got, want string) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("got json: %v body=%s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("want json: %v body=%s", err, want)
	}
	gotRaw, err := json.Marshal(gotValue)
	if err != nil {
		t.Fatal(err)
	}
	wantRaw, err := json.Marshal(wantValue)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("json=%s want=%s", got, want)
	}
}

func newPeopleServer(t *testing.T, fn func(http.ResponseWriter, *http.Request)) (*[]peopleReq, *httptest.Server) {
	t.Helper()
	reqs := []peopleReq{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		reqs = append(reqs, peopleReq{method: r.Method, host: r.Host, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone(), body: string(raw)})
		fn(w, r)
	}))
	t.Cleanup(srv.Close)
	return &reqs, srv
}

func wantPeopleGETHeaders() map[string]string {
	return map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"}
}

func wantPeopleBodyHeaders() map[string]string {
	return map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"}
}

func TestPeopleHelpAndTree(t *testing.T) {
	var stdout bytes.Buffer
	called := false
	f := testFactory(nil, &stdout)
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	cmd := newPeopleCommand(f)
	if cmd.Use != "people" || cmd.Short != "Manage campaign people" {
		t.Fatalf("use=%q short=%q", cmd.Use, cmd.Short)
	}
	wantUse := map[string]string{
		"list": "list --campaign ID", "get": "get ID --campaign ID", "create": "create --campaign ID --input FILE",
		"update": "update ID --campaign ID --input FILE", "delete": "delete ID --campaign ID", "import": "import --campaign ID --input FILE",
		"batch-delete": "batch-delete --campaign ID --input FILE", "run-script": "run-script --campaign ID --input FILE",
		"pause": "pause ID --campaign ID", "resume": "resume ID --campaign ID", "unsubscribe": "unsubscribe ID --campaign ID",
		"activity": "activity ID --campaign ID",
	}
	names := map[string]bool{}
	for _, child := range cmd.Commands() {
		names[child.Name()] = true
		if child.Use != wantUse[child.Name()] {
			t.Fatalf("%s use=%q want=%q", child.Name(), child.Use, wantUse[child.Name()])
		}
	}
	for name := range wantUse {
		if !names[name] {
			t.Fatalf("missing %s in %#v", name, names)
		}
	}
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	if called {
		t.Fatal("help resolved a lazy dependency")
	}
	help := stdout.String()
	if !strings.Contains(help, "Manage campaign people") || !strings.Contains(help, "installed-app needs a user credential") {
		t.Fatalf("help=%s", help)
	}
}

func TestPeopleRequiresCampaign(t *testing.T) {
	f := testFactory(nil, io.Discard)
	cases := [][]string{{"list"}, {"get", "p1"}, {"create", "--input", writePeopleInput(t, `{"variables":{}}`)}, {"update", "p1", "--input", writePeopleInput(t, `{}`)}, {"delete", "p1"}, {"import", "--input", writePeopleInput(t, `{"rows":[{}]}`)}, {"batch-delete", "--input", writePeopleInput(t, `{"person_ids":["p1"]}`)}, {"run-script", "--input", writePeopleInput(t, `{"custom_script":{"script_id":"s","function_name":"f"},"output_binding":{"column":"email_address"}}`)}, {"pause", "p1"}, {"resume", "p1"}, {"unsubscribe", "p1"}, {"activity", "p1"}}
	for _, args := range cases {
		err := executePeople(t, f, args...)
		if msg := peopleUsage(t, err); !strings.Contains(msg, "--campaign") {
			t.Fatalf("args=%v msg=%q", args, msg)
		}
	}
}

func TestPeopleListRouteHeadersPaginationTableAndJSON(t *testing.T) {
	page1 := `{"people":[{"name":"campaigns/c1/people/p1","email_address":"alex@example.test","enrollment_state":"ELIGIBLE","active_sequence":"campaigns/c1/sequences/s1","eligible_time":"2026-01-01T00:00:00Z","update_time":"2026-01-02T00:00:00Z"},{"name":"campaigns/c1/people/p2","email_address":"bea@example.test","enrollment_state":"WAITING","active_sequence":"campaigns/c1/sequences/s1","eligible_time":null,"update_time":"2026-01-03T00:00:00Z"}],"next_page_token":"p2"}`
	page2 := `{"people":[{"name":"campaigns/c1/people/p3","email_address":"cam@example.test","enrollment_state":"PAUSED","active_sequence":"campaigns/c1/sequences/s2","eligible_time":"2026-01-04T00:00:00Z","update_time":"2026-01-05T00:00:00Z"},{"name":"campaigns/c1/people/p4","email_address":"dan@example.test","enrollment_state":"ELIGIBLE","active_sequence":"s2","update_time":"2026-01-06T00:00:00Z"}]}`
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/campaigns/c1/people" || r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("page_token") == "p2" {
			_, _ = w.Write([]byte(page2))
			return
		}
		_, _ = w.Write([]byte(page1))
	})
	var out bytes.Buffer
	f := peopleFactory(t, srv, "", nil, &out, false)
	if err := executePeople(t, f, "list", "--campaign", "campaigns/c1", "--limit", "3"); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(*reqs) != 2 {
		t.Fatalf("hits=%d", len(*reqs))
	}
	if (*reqs)[0].host != "campaigns.example.test" || (*reqs)[0].query != "page_size=3" || (*reqs)[1].query != "page_size=1&page_token=p2" {
		t.Fatalf("requests %#v", *reqs)
	}
	assertApplicationHeaders(t, (*reqs)[0].headers, wantPeopleGETHeaders())
	if (*reqs)[0].headers.Get("sl-organization-id") != "" || (*reqs)[0].headers.Get("Authorization") != "" {
		t.Fatalf("forbidden headers %#v", applicationHeaders((*reqs)[0].headers))
	}
	wantTable := "p1\talex@example.test\tELIGIBLE\ts1\t2026-01-01T00:00:00Z\t2026-01-02T00:00:00Z\np2\tbea@example.test\tWAITING\ts1\t\t2026-01-03T00:00:00Z\np3\tcam@example.test\tPAUSED\ts2\t2026-01-04T00:00:00Z\t2026-01-05T00:00:00Z\n"
	if out.String() != wantTable {
		t.Fatalf("table=%q want=%q", out.String(), wantTable)
	}
	out.Reset()
	f = peopleFactory(t, srv, "ns-1", nil, &out, true)
	if err := executePeople(t, f, "list", "--campaign", "c1", "--limit", "1"); err != nil {
		t.Fatalf("json: %v", err)
	}
	assertApplicationHeaders(t, (*reqs)[len(*reqs)-1].headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json"})
	assertPeopleJSON(t, out.String(), `{"people":[{"name":"campaigns/c1/people/p1","email_address":"alex@example.test","enrollment_state":"ELIGIBLE","active_sequence":"campaigns/c1/sequences/s1","eligible_time":"2026-01-01T00:00:00Z","update_time":"2026-01-02T00:00:00Z"}]}`)
}

func TestPeopleListDefaultAndCappedPageSize(t *testing.T) {
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"people":[]}`))
	})
	f := peopleFactory(t, srv, "", nil, io.Discard, false)
	if err := executePeople(t, f, "list", "--campaign", "c1"); err != nil {
		t.Fatalf("default: %v", err)
	}
	if (*reqs)[0].query != "page_size=50" {
		t.Fatalf("default query=%q", (*reqs)[0].query)
	}
	if err := executePeople(t, f, "list", "--campaign", "c1", "--limit", "500"); err != nil {
		t.Fatalf("capped: %v", err)
	}
	if (*reqs)[1].query != "page_size=200" {
		t.Fatalf("capped query=%q", (*reqs)[1].query)
	}
	if err := executePeople(t, f, "list", "--campaign", "c1", "--limit", "0"); err == nil || peopleUsage(t, err) != "--limit must be at least 1" {
		t.Fatalf("limit: %v", err)
	}
}

func TestPeopleGetRouteTableAndJSON(t *testing.T) {
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/campaigns/c1/people/p1" || r.URL.RawQuery != "" {
			t.Errorf("request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(personJSON))
	})
	var out bytes.Buffer
	f := peopleFactory(t, srv, "", nil, &out, false)
	if err := executePeople(t, f, "get", "campaigns/c1/people/p1", "--campaign", "c1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	assertApplicationHeaders(t, (*reqs)[0].headers, wantPeopleGETHeaders())
	if out.String() != personTable {
		t.Fatalf("table=%q", out.String())
	}
	out.Reset()
	f = peopleFactory(t, srv, "", nil, &out, true)
	if err := executePeople(t, f, "get", "p1", "--campaign", "c1"); err != nil {
		t.Fatalf("json: %v", err)
	}
	assertPeopleJSON(t, out.String(), personJSON)
}

func TestPeopleCreateEmailFlagVariablesAndRequestID(t *testing.T) {
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/campaigns/c1/people" || r.URL.RawQuery != "" {
			t.Errorf("request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(personJSON))
	})
	var out bytes.Buffer
	input := writePeopleInput(t, `{"variables":{"given_name":"Alex","family_name":"Rivera"},"keep":true}`)
	f := peopleFactory(t, srv, "", nil, &out, false)
	if err := executePeople(t, f, "create", "--campaign", "c1", "--email-address", "alex@example.test", "--input", input); err != nil {
		t.Fatalf("create: %v", err)
	}
	assertApplicationHeaders(t, (*reqs)[0].headers, wantPeopleBodyHeaders())
	assertPeopleJSON(t, (*reqs)[0].body, `{"email_address":"alex@example.test","variables":{"given_name":"Alex","family_name":"Rivera"},"keep":true,"request_id":"`+peopleRequestID+`"}`)
	if out.String() != personTable {
		t.Fatalf("table=%q", out.String())
	}
	if err := executePeople(t, f, "create", "--campaign", "c1", "--input", writePeopleInput(t, `{"email_address":"bea@example.test","variables":{"given_name":"Bea"},"request_id":"input-id"}`)); err != nil {
		t.Fatalf("input wins: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[1].body, `{"email_address":"bea@example.test","variables":{"given_name":"Bea"},"request_id":"input-id"}`)
	if err := executePeople(t, f, "create", "--campaign", "c1", "--request-id", "flag-id", "--input", writePeopleInput(t, `{"variables":{}}`)); err != nil {
		t.Fatalf("flag request: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[2].body, `{"variables":{},"request_id":"flag-id"}`)
}

func TestPeopleCreateConflictsAndRequiredInput(t *testing.T) {
	f := testFactory(nil, io.Discard)
	err := executePeople(t, f, "create", "--campaign", "c1", "--email-address", "a@b.test", "--input", writePeopleInput(t, `{"email_address":"c@d.test","variables":{}}`))
	if msg := peopleUsage(t, err); !strings.Contains(msg, "--email-address") || !strings.Contains(msg, "email_address") {
		t.Fatalf("email conflict: %v", err)
	}
	err = executePeople(t, f, "create", "--campaign", "c1", "--request-id", "flag-id", "--input", writePeopleInput(t, `{"variables":{},"request_id":"input-id"}`))
	if msg := peopleUsage(t, err); !strings.Contains(msg, "--request-id") || !strings.Contains(msg, "request_id") {
		t.Fatalf("request conflict: %v", err)
	}
	err = executePeople(t, f, "create", "--campaign", "c1", "--email-address", "a@b.test")
	if peopleUsage(t, err) != "--input is required" {
		t.Fatalf("input required: %v", err)
	}
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(personJSON))
	})
	if err := executePeople(t, peopleFactory(t, srv, "", strings.NewReader(`{"variables":{"given_name":"A"}}`), io.Discard, false), "create", "--campaign", "c1", "--input", "-"); err != nil {
		t.Fatalf("stdin: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[0].body, `{"variables":{"given_name":"A"},"request_id":"`+peopleRequestID+`"}`)
	if err := executePeople(t, peopleFactory(t, srv, "", nil, io.Discard, false), "create", "--campaign", "c1", "--email-address", "a@b.test", "--input", writePeopleInput(t, `{}`)); err != nil {
		t.Fatalf("passthrough: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[1].body, `{"email_address":"a@b.test","request_id":"`+peopleRequestID+`"}`)
}

func TestPeopleUpdateFetchesPersonEtagAndPassesInput(t *testing.T) {
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/campaigns/c1/people/p1" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(personJSON))
			return
		}
		if r.Method != http.MethodPatch {
			t.Errorf("method %s", r.Method)
		}
		_, _ = w.Write([]byte(personJSON))
	})
	var out bytes.Buffer
	input := writePeopleInput(t, `{"person":{"variables":{"given_name":"Ada"}},"update_mask":"person.variables","keep":1}`)
	f := peopleFactory(t, srv, "", nil, &out, false)
	if err := executePeople(t, f, "update", "p1", "--campaign", "c1", "--input", input); err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(*reqs) != 2 || (*reqs)[0].method != http.MethodGet || (*reqs)[1].method != http.MethodPatch {
		t.Fatalf("reqs %#v", *reqs)
	}
	assertApplicationHeaders(t, (*reqs)[0].headers, wantPeopleGETHeaders())
	assertApplicationHeaders(t, (*reqs)[1].headers, wantPeopleBodyHeaders())
	assertPeopleJSON(t, (*reqs)[1].body, `{"person":{"variables":{"given_name":"Ada"}},"update_mask":"person.variables","keep":1,"request_id":"`+peopleRequestID+`","etag":"3"}`)
	if out.String() != personTable {
		t.Fatalf("table=%q", out.String())
	}
	*reqs = (*reqs)[:0]
	if err := executePeople(t, f, "update", "p1", "--campaign", "c1", "--etag", "flag-etag", "--request-id", "flag-id", "--input", writePeopleInput(t, `{"person":{},"update_mask":"person.variables","etag":"should-conflict"}`)); err == nil {
		t.Fatal("expected etag conflict")
	} else if msg := peopleUsage(t, err); !strings.Contains(msg, "--etag") {
		t.Fatalf("etag conflict: %v", err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("conflict fetched %#v", *reqs)
	}
	if err := executePeople(t, f, "update", "p1", "--campaign", "c1", "--etag", "flag-etag", "--input", writePeopleInput(t, `{"person":{},"update_mask":"person.variables"}`)); err != nil {
		t.Fatalf("flag etag: %v", err)
	}
	if len(*reqs) != 1 || (*reqs)[0].method != http.MethodPatch {
		t.Fatalf("flag etag reqs %#v", *reqs)
	}
	assertPeopleJSON(t, (*reqs)[0].body, `{"person":{},"update_mask":"person.variables","request_id":"`+peopleRequestID+`","etag":"flag-etag"}`)
	if err := executePeople(t, f, "update", "p1", "--campaign", "c1"); peopleUsage(t, err) != "--input is required" {
		t.Fatalf("input required: %v", err)
	}
}

func TestPeopleDeleteUsesCampaignEtagQueryAndConfirm(t *testing.T) {
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/campaigns/c1":
			_, _ = w.Write([]byte(campaignJSON))
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/campaigns/c1/people/p1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	var out bytes.Buffer
	prompt := ""
	f := peopleFactory(t, srv, "", nil, &out, false)
	f.Confirm = func(p string) error {
		prompt = p
		return nil
	}
	if err := executePeople(t, f, "delete", "p1", "--campaign", "c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if prompt != "Delete person p1?" {
		t.Fatalf("prompt=%q", prompt)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout=%q", out.String())
	}
	if len(*reqs) != 2 || (*reqs)[0].method != http.MethodGet || (*reqs)[0].path != "/v1/campaigns/c1" || (*reqs)[1].method != http.MethodDelete || (*reqs)[1].path != "/v1/campaigns/c1/people/p1" {
		t.Fatalf("reqs %#v", *reqs)
	}
	if (*reqs)[1].query != "etag=77&request_id="+peopleRequestID || (*reqs)[1].body != "" {
		t.Fatalf("delete query=%q body=%q", (*reqs)[1].query, (*reqs)[1].body)
	}
	assertApplicationHeaders(t, (*reqs)[0].headers, wantPeopleGETHeaders())
	assertApplicationHeaders(t, (*reqs)[1].headers, wantPeopleGETHeaders())
	*reqs = (*reqs)[:0]
	if err := executePeople(t, f, "delete", "p1", "--campaign", "c1", "--etag", "flag-etag", "--request-id", "flag-id"); err != nil {
		t.Fatalf("flag: %v", err)
	}
	if len(*reqs) != 1 || (*reqs)[0].method != http.MethodDelete || (*reqs)[0].query != "etag=flag-etag&request_id=flag-id" {
		t.Fatalf("flag reqs %#v", *reqs)
	}
	declined := peopleFactory(t, srv, "", nil, io.Discard, false)
	declined.Confirm = func(string) error { return cli.ErrCancelled }
	*reqs = (*reqs)[:0]
	if err := executePeople(t, declined, "delete", "p1", "--campaign", "c1"); !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("declined: %v", err)
	}
	if len(*reqs) != 0 {
		t.Fatalf("declined fetched %#v", *reqs)
	}
	missingYes := peopleFactory(t, srv, "", nil, io.Discard, false)
	usage := &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	missingYes.Confirm = func(string) error { return usage }
	err := executePeople(t, missingYes, "delete", "p1", "--campaign", "c1")
	var got *cli.UsageError
	if !errors.As(err, &got) || got.Msg != usage.Msg {
		t.Fatalf("yes: %v", err)
	}
}

func TestPeopleImportExactRowsOrCSV(t *testing.T) {
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/campaigns/c1/people:import" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(taskJSON))
	})
	var out bytes.Buffer
	f := peopleFactory(t, srv, "", nil, &out, false)
	if err := executePeople(t, f, "import", "--campaign", "c1", "--input", writePeopleInput(t, `{"rows":[{"email_address":"alex@example.test","variables":{"given_name":"Alex"}}]}`)); err != nil {
		t.Fatalf("rows: %v", err)
	}
	assertApplicationHeaders(t, (*reqs)[0].headers, wantPeopleBodyHeaders())
	assertPeopleJSON(t, (*reqs)[0].body, `{"rows":[{"email_address":"alex@example.test","variables":{"given_name":"Alex"}}],"request_id":"`+peopleRequestID+`"}`)
	if out.String() != taskTable {
		t.Fatalf("table=%q", out.String())
	}
	out.Reset()
	f = peopleFactory(t, srv, "", nil, &out, true)
	if err := executePeople(t, f, "import", "--campaign", "c1", "--request-id", "flag-id", "--input", writePeopleInput(t, `{"csv":"email_address\nalex@example.test"}`)); err != nil {
		t.Fatalf("csv: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[1].body, `{"csv":"email_address\nalex@example.test","request_id":"flag-id"}`)
	assertPeopleJSON(t, out.String(), taskJSON)
	if err := executePeople(t, f, "import", "--campaign", "c1", "--input", writePeopleInput(t, `{"rows":[{}],"csv":"x"}`)); err != nil {
		t.Fatalf("both: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[2].body, `{"rows":[{}],"csv":"x","request_id":"`+peopleRequestID+`"}`)
	if err := executePeople(t, f, "import", "--campaign", "c1", "--input", writePeopleInput(t, `{}`)); err != nil {
		t.Fatalf("neither: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[3].body, `{"request_id":"`+peopleRequestID+`"}`)
	if peopleUsage(t, executePeople(t, testFactory(nil, io.Discard), "import", "--campaign", "c1")) != "--input is required" {
		t.Fatal("input required")
	}
	if msg := peopleUsage(t, executePeople(t, testFactory(nil, io.Discard), "import", "--campaign", "c1", "--request-id", "flag-id", "--input", writePeopleInput(t, `{"rows":[{}],"request_id":"input-id"}`))); !strings.Contains(msg, "--request-id") {
		t.Fatalf("conflict: %s", msg)
	}
}

func TestPeopleBatchDeleteUsesCampaignEtagAndConfirm(t *testing.T) {
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/campaigns/c1":
			_, _ = w.Write([]byte(campaignJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/campaigns/c1/people:batchDelete":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	var out bytes.Buffer
	prompt := ""
	f := peopleFactory(t, srv, "", nil, &out, false)
	f.Confirm = func(p string) error {
		prompt = p
		return nil
	}
	if err := executePeople(t, f, "batch-delete", "--campaign", "c1", "--input", writePeopleInput(t, `{"person_ids":["p1","p2"]}`)); err != nil {
		t.Fatalf("batch: %v", err)
	}
	if prompt != "Delete people?" {
		t.Fatalf("prompt=%q", prompt)
	}
	if out.Len() != 0 {
		t.Fatalf("table=%q", out.String())
	}
	if len(*reqs) != 2 || (*reqs)[0].path != "/v1/campaigns/c1" || (*reqs)[1].path != "/v1/campaigns/c1/people:batchDelete" {
		t.Fatalf("reqs %#v", *reqs)
	}
	assertPeopleJSON(t, (*reqs)[1].body, `{"person_ids":["p1","p2"],"request_id":"`+peopleRequestID+`","etag":"77"}`)
	assertApplicationHeaders(t, (*reqs)[1].headers, wantPeopleBodyHeaders())
	out.Reset()
	f = peopleFactory(t, srv, "", nil, &out, true)
	f.Confirm = func(string) error { return nil }
	before := len(*reqs)
	if err := executePeople(t, f, "batch-delete", "--campaign", "c1", "--etag", "flag-etag", "--input", writePeopleInput(t, `{"person_ids":["p1"]}`)); err != nil {
		t.Fatalf("flag: %v", err)
	}
	if len(*reqs) != before+1 || (*reqs)[before].method != http.MethodPost || (*reqs)[before].path != "/v1/campaigns/c1/people:batchDelete" {
		t.Fatalf("flag etag fetched campaign %#v", (*reqs)[before:])
	}
	assertPeopleJSON(t, (*reqs)[before].body, `{"person_ids":["p1"],"request_id":"`+peopleRequestID+`","etag":"flag-etag"}`)
	if out.Len() != 0 {
		t.Fatalf("json stdout=%q", out.String())
	}
	declined := peopleFactory(t, srv, "", nil, io.Discard, false)
	generated := false
	declined.NewRequestID = func() string {
		generated = true
		return peopleRequestID
	}
	declined.Confirm = func(string) error { return cli.ErrCancelled }
	before = len(*reqs)
	if err := executePeople(t, declined, "batch-delete", "--campaign", "c1", "--input", writePeopleInput(t, `{"person_ids":["p1"]}`)); !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("declined: %v", err)
	}
	if generated || len(*reqs) != before {
		t.Fatalf("declined generated=%v fetched=%d", generated, len(*reqs)-before)
	}
	if err := executePeople(t, f, "batch-delete", "--campaign", "c1", "--etag", "flag-etag", "--input", writePeopleInput(t, `{}`)); err != nil {
		t.Fatalf("passthrough: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[len(*reqs)-1].body, `{"request_id":"`+peopleRequestID+`","etag":"flag-etag"}`)
}

func TestPeopleRunScriptExactTargetAndBindings(t *testing.T) {
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/campaigns/c1/people:runScript" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(strings.Replace(taskJSON, "IMPORT_PEOPLE", "RUN_SCRIPT", 1)))
	})
	var out bytes.Buffer
	f := peopleFactory(t, srv, "", nil, &out, false)
	custom := `{"custom_script":{"script_id":"scr","function_name":"run","parameter_names":["person"]},"input_bindings":{"given_name":{"template":"{{person.given_name}}"}},"output_binding":{"column":"email_address","path":"email"}}`
	if err := executePeople(t, f, "run-script", "--campaign", "c1", "--input", writePeopleInput(t, custom)); err != nil {
		t.Fatalf("custom: %v", err)
	}
	assertApplicationHeaders(t, (*reqs)[0].headers, wantPeopleBodyHeaders())
	assertPeopleJSON(t, (*reqs)[0].body, `{"custom_script":{"script_id":"scr","function_name":"run","parameter_names":["person"]},"input_bindings":{"given_name":{"template":"{{person.given_name}}"}},"output_binding":{"column":"email_address","path":"email"},"request_id":"`+peopleRequestID+`"}`)
	if out.String() != strings.Replace(taskTable, "IMPORT_PEOPLE", "RUN_SCRIPT", 1) {
		t.Fatalf("table=%q", out.String())
	}
	installed := `{"installed_app":{"installation_id":"inst","operation":"enrich"},"output_binding":{"column":"given_name"}}`
	if err := executePeople(t, f, "run-script", "--campaign", "c1", "--request-id", "flag-id", "--input", writePeopleInput(t, installed)); err != nil {
		t.Fatalf("installed: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[1].body, `{"installed_app":{"installation_id":"inst","operation":"enrich"},"output_binding":{"column":"given_name"},"request_id":"flag-id"}`)
	if err := executePeople(t, f, "run-script", "--campaign", "c1", "--input", writePeopleInput(t, `{"custom_script":{"script_id":"s","function_name":"f"},"installed_app":{"installation_id":"i","operation":"o"},"output_binding":{"column":"email_address"}}`)); err != nil {
		t.Fatalf("both: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[2].body, `{"custom_script":{"script_id":"s","function_name":"f"},"installed_app":{"installation_id":"i","operation":"o"},"output_binding":{"column":"email_address"},"request_id":"`+peopleRequestID+`"}`)
	if err := executePeople(t, f, "run-script", "--campaign", "c1", "--input", writePeopleInput(t, `{"output_binding":{"column":"email_address"}}`)); err != nil {
		t.Fatalf("neither: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[3].body, `{"output_binding":{"column":"email_address"},"request_id":"`+peopleRequestID+`"}`)
	if peopleUsage(t, executePeople(t, testFactory(nil, io.Discard), "run-script", "--campaign", "c1")) != "--input is required" {
		t.Fatal("input required")
	}
}

func TestPeopleLifecycleFetchesPersonEtagAndTables(t *testing.T) {
	failedOp := `{"name":"operations/op1","state":"FAILED","campaign":"campaigns/c1","command_type":"UNSUBSCRIBE_PERSON","cancelled_delivery_count":null,"already_attempting_delivery_count":null,"update_time":"2026-01-04T00:00:00Z"}`
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/campaigns/c1/people/p1":
			_, _ = w.Write([]byte(personJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/campaigns/c1/people/p1:pause":
			_, _ = w.Write([]byte(operationJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/campaigns/c1/people/p1:resume":
			_, _ = w.Write([]byte(personJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/campaigns/c1/people/p1:unsubscribe":
			_, _ = w.Write([]byte(failedOp))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	var out bytes.Buffer
	f := peopleFactory(t, srv, "", nil, &out, false)
	if err := executePeople(t, f, "pause", "p1", "--campaign", "c1"); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if len(*reqs) != 2 || (*reqs)[0].method != http.MethodGet || (*reqs)[1].path != "/v1/campaigns/c1/people/p1:pause" {
		t.Fatalf("pause reqs %#v", *reqs)
	}
	assertPeopleJSON(t, (*reqs)[1].body, `{"request_id":"`+peopleRequestID+`","etag":"3"}`)
	assertApplicationHeaders(t, (*reqs)[1].headers, wantPeopleBodyHeaders())
	if out.String() != operationTable {
		t.Fatalf("pause table=%q", out.String())
	}
	out.Reset()
	if err := executePeople(t, f, "resume", "p1", "--campaign", "c1", "--etag", "flag-etag"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	last := (*reqs)[len(*reqs)-1]
	if last.method != http.MethodPost || last.path != "/v1/campaigns/c1/people/p1:resume" {
		t.Fatalf("resume %#v", last)
	}
	assertPeopleJSON(t, last.body, `{"request_id":"`+peopleRequestID+`","etag":"flag-etag"}`)
	if out.String() != personTable {
		t.Fatalf("resume table=%q", out.String())
	}
	out.Reset()
	if err := executePeople(t, f, "unsubscribe", "p1", "--campaign", "c1"); err != nil {
		t.Fatalf("failed operation must succeed: %v", err)
	}
	if (*reqs)[len(*reqs)-1].path != "/v1/campaigns/c1/people/p1:unsubscribe" {
		t.Fatalf("unsubscribe path %s", (*reqs)[len(*reqs)-1].path)
	}
	if out.String() != "op1\tc1\tFAILED\tUNSUBSCRIBE_PERSON\t\t\t2026-01-04T00:00:00Z\n" {
		t.Fatalf("failed table=%q", out.String())
	}
	out.Reset()
	f = peopleFactory(t, srv, "", nil, &out, true)
	if err := executePeople(t, f, "pause", "p1", "--campaign", "c1", "--etag", "9", "--request-id", "rid"); err != nil {
		t.Fatalf("json pause: %v", err)
	}
	assertPeopleJSON(t, (*reqs)[len(*reqs)-1].body, `{"request_id":"rid","etag":"9"}`)
	assertPeopleJSON(t, out.String(), operationJSON)
}

func TestPeopleActivitySnakePagination(t *testing.T) {
	page1 := `{"activities":[{"event_id":"e1","type":"ENROLLED","reason_code":"USER_REQUEST","summary":"enrolled","occurred_at":"2026-01-03T00:00:00Z"}],"next_page_token":"tok"}`
	page2 := `{"activities":[{"event_id":"e2","type":"PAUSED","reason_code":"USER_REQUEST","summary":"paused","occurred_at":"2026-01-04T00:00:00Z"},{"event_id":"e3","type":"RESUMED","reason_code":"USER_REQUEST","summary":"resumed","occurred_at":"2026-01-05T00:00:00Z"}]}`
	reqs, srv := newPeopleServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/campaigns/c1/people/p1:activity" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("page_token") == "tok" {
			_, _ = w.Write([]byte(page2))
			return
		}
		_, _ = w.Write([]byte(page1))
	})
	var out bytes.Buffer
	f := peopleFactory(t, srv, "", nil, &out, false)
	if err := executePeople(t, f, "activity", "p1", "--campaign", "c1", "--limit", "2"); err != nil {
		t.Fatalf("activity: %v", err)
	}
	if len(*reqs) != 2 || (*reqs)[0].query != "page_size=2" || (*reqs)[1].query != "page_size=1&page_token=tok" {
		t.Fatalf("reqs %#v", *reqs)
	}
	assertApplicationHeaders(t, (*reqs)[0].headers, wantPeopleGETHeaders())
	want := activityTable + "e2\tPAUSED\tUSER_REQUEST\tpaused\t2026-01-04T00:00:00Z\n"
	if out.String() != want {
		t.Fatalf("table=%q want=%q", out.String(), want)
	}
	out.Reset()
	f = peopleFactory(t, srv, "", nil, &out, true)
	if err := executePeople(t, f, "activity", "p1", "--campaign", "c1", "--limit", "1"); err != nil {
		t.Fatalf("json: %v", err)
	}
	assertPeopleJSON(t, out.String(), `{"activities":[{"event_id":"e1","type":"ENROLLED","reason_code":"USER_REQUEST","summary":"enrolled","occurred_at":"2026-01-03T00:00:00Z"}]}`)
	if err := executePeople(t, f, "activity", "p1", "--campaign", "c1", "--limit", "0"); err == nil || peopleUsage(t, err) != "--limit must be at least 1" {
		t.Fatalf("limit: %v", err)
	}
}

func TestPeopleMissingPersonID(t *testing.T) {
	f := testFactory(nil, io.Discard)
	if msg := peopleUsage(t, executePeople(t, f, "get", "  ", "--campaign", "c1")); !strings.Contains(msg, "person id") {
		t.Fatalf("blank: %s", msg)
	}
}

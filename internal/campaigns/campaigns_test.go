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
	campaignID         = "c1"
	campaignReqID      = "00000000-0000-4000-8000-000000000001"
	rootCampaignJSON   = `{"name":"campaigns/c1","display_name":"Acme","state":"DRAFT","schedule":"campaignSchedules/s1","etag":"7","update_time":"2026-01-02T03:04:05Z"}`
	campaignTable      = "c1\tAcme\tDRAFT\tcampaignSchedules/s1\t7\t2026-01-02T03:04:05Z\n"
	campaignCreate     = `{"campaign":{"name":"campaigns/c1","display_name":"Acme","state":"DRAFT","schedule":"campaignSchedules/s1","etag":"7","update_time":"2026-01-02T03:04:05Z"},"main_sequence":{"name":"campaigns/c1/sequences/main"}}`
	rootOperationJSON  = `{"name":"operations/op1","campaign":"campaigns/c1","state":"RUNNING","command_type":"PAUSE","cancelled_delivery_count":2,"already_attempting_delivery_count":1,"update_time":"t2"}`
	rootOperationTable = "op1\tc1\tRUNNING\tPAUSE\t2\t1\tt2\n"
	senderJSON         = `{"account_ids":["a1","a2"],"etag":"8"}`
)

type campaignRequest struct {
	Method  string
	Host    string
	Path    string
	Query   string
	Headers http.Header
	Body    string
}

func campaignsFactory(t *testing.T, stdin io.Reader, stdout io.Writer, jsonMode bool, confirm func(string) error) *cli.Factory {
	t.Helper()
	f := testFactory(stdin, stdout)
	f.Printer = func() *output.Printer {
		return output.New(stdout, output.Options{JSON: jsonMode})
	}
	if confirm == nil {
		confirm = func(string) error { return nil }
	}
	f.Confirm = confirm
	return f
}

func executeCampaigns(t *testing.T, f *cli.Factory, args ...string) error {
	t.Helper()
	cmd := NewCommand(f)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func writeCampaignInput(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func captureRootCampaigns(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *[]campaignRequest) {
	t.Helper()
	var got []campaignRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		got = append(got, campaignRequest{Method: r.Method, Host: r.Host, Path: r.URL.Path, Query: r.URL.RawQuery, Headers: r.Header.Clone(), Body: string(raw)})
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func campaignOK(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}
}

func campaignPrefetchThen(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && !strings.Contains(r.URL.Path, ":") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"campaigns/c1","etag":"77","display_name":"Kept"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}
}

func assertRootCampaignRequest(t *testing.T, got campaignRequest, method, path, query, namespace string, contentType bool) {
	t.Helper()
	if got.Method != method || got.Host != "campaigns.example.test" || got.Path != path || got.Query != query {
		t.Fatalf("request %s %s %s?%s", got.Method, got.Host, got.Path, got.Query)
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

func assertCampaignJSON(t *testing.T, got, want string) {
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
		t.Fatalf("body=%s want=%s", got, want)
	}
}

func assertCampaignUsage(t *testing.T, err error, substr string) {
	t.Helper()
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, substr) {
		t.Fatalf("usage=%v want %q", err, substr)
	}
}

func bindCampaignClient(t *testing.T, f *cli.Factory, srv *httptest.Server, namespace string) {
	t.Helper()
	f.Client = func() (*apiclient.Client, error) {
		return testClient(t, srv, namespace), nil
	}
}

func TestNewCampaignCommandsOrderAndHelp(t *testing.T) {
	f := campaignsFactory(t, nil, io.Discard, false, nil)
	cmds := newCampaignCommands(f)
	want := []string{"list", "get", "create", "update", "delete", "activate", "pause", "resume", "complete", "archive", "unarchive", "set-variables", "set-sender-accounts"}
	uses := []string{"list", "get ID", "create [--display-name NAME | --input FILE]", "update ID --input FILE", "delete ID", "activate ID", "pause ID", "resume ID", "complete ID", "archive ID", "unarchive ID", "set-variables ID --input FILE", "set-sender-accounts ID --input FILE"}
	shorts := []string{"List campaigns", "Get a campaign", "Create a campaign", "Update a campaign", "Delete a campaign", "Activate a campaign", "Pause a campaign", "Resume a campaign", "Complete a campaign", "Archive a campaign", "Unarchive a campaign", "Set campaign variables", "Set campaign sender accounts"}
	if len(cmds) != len(want) {
		t.Fatalf("commands=%d", len(cmds))
	}
	for i, name := range want {
		if cmds[i].Name() != name || cmds[i].Use != uses[i] || cmds[i].Short != shorts[i] || strings.Contains(cmds[i].Short, "\n") {
			t.Fatalf("command[%d]=%q use=%q short=%q", i, cmds[i].Name(), cmds[i].Use, cmds[i].Short)
		}
	}
	root := NewCommand(f)
	names := map[string]bool{}
	for _, child := range root.Commands() {
		names[child.Name()] = true
	}
	for _, name := range want {
		if !names[name] {
			t.Fatalf("NewCommand missing %s in %#v", name, names)
		}
	}
	if names["campaigns"] {
		t.Fatal("root resource must collapse onto campaigns")
	}
}

func TestCampaignsListDefaultPageSizeAndTable(t *testing.T) {
	t.Setenv("SL_ORGANIZATION_ID", "org-configured")
	var stdout bytes.Buffer
	srv, got := captureRootCampaigns(t, campaignOK(`{"campaigns":[`+rootCampaignJSON+`],"next_page_token":""}`))
	f := campaignsFactory(t, nil, &stdout, false, nil)
	f.Organization = func() (string, error) {
		t.Fatal("Organization must not be called")
		return "", nil
	}
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("requests=%d", len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns", "page_size=50", "", false)
	if (*got)[0].Body != "" {
		t.Fatalf("body=%q", (*got)[0].Body)
	}
	if stdout.String() != campaignTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCampaignsListVisibilityAndLimitPages(t *testing.T) {
	var stdout bytes.Buffer
	srv, got := captureRootCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.RawQuery {
		case "page_size=3&visibility=ARCHIVED":
			_, _ = w.Write([]byte(`{"campaigns":[{"name":"campaigns/a","display_name":"A","state":"DRAFT","schedule":"","etag":"1","update_time":"t1"},{"name":"campaigns/b","display_name":"B","state":"ACTIVE","schedule":"","etag":"2","update_time":"t2"}],"next_page_token":"p2"}`))
		case "page_size=1&page_token=p2&visibility=ARCHIVED":
			_, _ = w.Write([]byte(`{"campaigns":[{"name":"campaigns/c","display_name":"C","state":"PAUSED","schedule":"","etag":"3","update_time":"t3"},{"name":"campaigns/d","display_name":"D","state":"DRAFT","schedule":"","etag":"4","update_time":"t4"}],"next_page_token":"p3"}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	f := campaignsFactory(t, nil, &stdout, false, nil)
	bindCampaignClient(t, f, srv, "ns-1")
	if err := executeCampaigns(t, f, "list", "--limit", "3", "--visibility", "ARCHIVED"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 2 {
		t.Fatalf("requests=%d", len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns", "page_size=3&visibility=ARCHIVED", "ns-1", false)
	assertRootCampaignRequest(t, (*got)[1], http.MethodGet, "/v1/campaigns", "page_size=1&page_token=p2&visibility=ARCHIVED", "ns-1", false)
	if stdout.String() != "a\tA\tDRAFT\t\t1\tt1\nb\tB\tACTIVE\t\t2\tt2\nc\tC\tPAUSED\t\t3\tt3\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCampaignsListJSONEmitsCollectedItems(t *testing.T) {
	var stdout bytes.Buffer
	srv, _ := captureRootCampaigns(t, campaignOK(`{"campaigns":[`+rootCampaignJSON+`],"next_page_token":"","total_size":1}`))
	f := campaignsFactory(t, nil, &stdout, true, nil)
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := `{"campaigns":[` + rootCampaignJSON + `]}`
	if stdout.String() != want {
		t.Fatalf("stdout=%q want=%q", stdout.String(), want)
	}
}

func TestCampaignsListRejectsInvalidLimit(t *testing.T) {
	f := campaignsFactory(t, nil, io.Discard, false, nil)
	assertCampaignUsage(t, executeCampaigns(t, f, "list", "--limit", "0"), "--limit must be at least 1")
	assertCampaignUsage(t, executeCampaigns(t, f, "list", "--limit", "-1"), "--limit must be at least 1")
}

func TestCampaignsGetSendsPathAndTable(t *testing.T) {
	var stdout bytes.Buffer
	srv, got := captureRootCampaigns(t, campaignOK(rootCampaignJSON))
	f := campaignsFactory(t, nil, &stdout, false, nil)
	f.Organization = func() (string, error) {
		t.Fatal("Organization must not be called")
		return "", nil
	}
	bindCampaignClient(t, f, srv, "ns-1")
	if err := executeCampaigns(t, f, "get", campaignID); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("requests=%d", len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns/c1", "", "ns-1", false)
	if (*got)[0].Body != "" {
		t.Fatalf("body=%q", (*got)[0].Body)
	}
	if stdout.String() != campaignTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCampaignsCreateDisplayNameInjectsOrganizationAndRequestID(t *testing.T) {
	var stdout bytes.Buffer
	orgCalls := 0
	srv, got := captureRootCampaigns(t, campaignOK(campaignCreate))
	f := campaignsFactory(t, nil, &stdout, false, nil)
	f.Organization = func() (string, error) {
		orgCalls++
		return "org-test", nil
	}
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "create", "--display-name", "Acme"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if orgCalls != 1 || len(*got) != 1 {
		t.Fatalf("orgCalls=%d requests=%d", orgCalls, len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodPost, "/v1/campaigns", "", "", true)
	assertCampaignJSON(t, (*got)[0].Body, `{"display_name":"Acme","organization":"organizations/org-test","request_id":"`+campaignReqID+`"}`)
	if stdout.String() != campaignTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCampaignsCreateInputPreservesOrganizationAndJSON(t *testing.T) {
	var stdout bytes.Buffer
	orgCalls := 0
	input := `{"display_name":"FromFile","organization":"organizations/from-input","namespace":null}`
	srv, got := captureRootCampaigns(t, campaignOK(campaignCreate))
	f := campaignsFactory(t, nil, &stdout, true, nil)
	f.Organization = func() (string, error) {
		orgCalls++
		return "org-test", nil
	}
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "create", "--input", writeCampaignInput(t, input)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if orgCalls != 0 {
		t.Fatalf("orgCalls=%d", orgCalls)
	}
	assertCampaignJSON(t, (*got)[0].Body, `{"display_name":"FromFile","namespace":null,"organization":"organizations/from-input","request_id":"`+campaignReqID+`"}`)
	if stdout.String() != campaignCreate {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCampaignsCreateReadsStdinAndMergesDisplayName(t *testing.T) {
	var stdout bytes.Buffer
	srv, got := captureRootCampaigns(t, campaignOK(campaignCreate))
	f := campaignsFactory(t, strings.NewReader(`{"namespace":"namespaces/n1"}`), &stdout, false, nil)
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "create", "--input", "-", "--display-name", "Stdin"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertCampaignJSON(t, (*got)[0].Body, `{"display_name":"Stdin","namespace":"namespaces/n1","organization":"organizations/org-test","request_id":"`+campaignReqID+`"}`)
}

func TestCampaignsCreateConflictsAndUsage(t *testing.T) {
	f := campaignsFactory(t, nil, io.Discard, false, nil)
	assertCampaignUsage(t, executeCampaigns(t, f, "create"), "--input or --display-name")
	assertCampaignUsage(t, executeCampaigns(t, f, "create", "--input", writeCampaignInput(t, `{"display_name":"In"}`), "--display-name", "Flag"), "--display-name conflicts with display_name")
	assertCampaignUsage(t, executeCampaigns(t, f, "create", "--input", writeCampaignInput(t, `{"request_id":"input-id","display_name":"In"}`), "--request-id", "flag-id"), "--request-id conflicts with request_id")
	assertCampaignUsage(t, executeCampaigns(t, f, "create", "--input", writeCampaignInput(t, `[]`)), "--input must be a JSON object")
}

func TestCampaignsUpdateInjectsNestedOrganizationAndPrefetchesEtag(t *testing.T) {
	var stdout bytes.Buffer
	orgCalls := 0
	input := `{"campaign":{"display_name":"Renamed"},"update_mask":"display_name"}`
	srv, got := captureRootCampaigns(t, campaignPrefetchThen(rootCampaignJSON))
	f := campaignsFactory(t, nil, &stdout, false, nil)
	f.Organization = func() (string, error) {
		orgCalls++
		return "org-1", nil
	}
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "update", campaignID, "--input", writeCampaignInput(t, input)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if orgCalls != 1 || len(*got) != 2 {
		t.Fatalf("orgCalls=%d requests=%d", orgCalls, len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns/c1", "", "", false)
	assertRootCampaignRequest(t, (*got)[1], http.MethodPatch, "/v1/campaigns/c1", "", "", true)
	assertCampaignJSON(t, (*got)[1].Body, `{"campaign":{"display_name":"Renamed","organization":"organizations/org-1"},"etag":"77","request_id":"`+campaignReqID+`","update_mask":"display_name"}`)
	if stdout.String() != campaignTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCampaignsUpdatePreservesOrganizationAndFlagEtag(t *testing.T) {
	orgCalls := 0
	input := `{"campaign":{"display_name":"Renamed","organization":"organizations/from-input"},"update_mask":"display_name","etag":"9"}`
	srv, got := captureRootCampaigns(t, campaignOK(rootCampaignJSON))
	f := campaignsFactory(t, nil, io.Discard, false, nil)
	f.Organization = func() (string, error) {
		orgCalls++
		return "org-1", nil
	}
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "update", campaignID, "--input", writeCampaignInput(t, input), "--request-id", "flag-id"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if orgCalls != 0 || len(*got) != 1 {
		t.Fatalf("orgCalls=%d requests=%d", orgCalls, len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodPatch, "/v1/campaigns/c1", "", "", true)
	assertCampaignJSON(t, (*got)[0].Body, `{"campaign":{"display_name":"Renamed","organization":"organizations/from-input"},"etag":"9","request_id":"flag-id","update_mask":"display_name"}`)
}

func TestCampaignsUpdateDoesNotInventMissingCampaign(t *testing.T) {
	orgCalls := 0
	srv, got := captureRootCampaigns(t, campaignOK(rootCampaignJSON))
	f := campaignsFactory(t, nil, io.Discard, false, nil)
	f.Organization = func() (string, error) {
		orgCalls++
		return "org-1", nil
	}
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "update", campaignID, "--input", writeCampaignInput(t, `{"update_mask":"display_name"}`), "--etag", "9"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if orgCalls != 0 || len(*got) != 1 {
		t.Fatalf("orgCalls=%d requests=%d", orgCalls, len(*got))
	}
	assertCampaignJSON(t, (*got)[0].Body, `{"etag":"9","request_id":"`+campaignReqID+`","update_mask":"display_name"}`)
}

func TestCampaignsUpdateConflictsAndRequiredInput(t *testing.T) {
	f := campaignsFactory(t, nil, io.Discard, false, nil)
	assertCampaignUsage(t, executeCampaigns(t, f, "update", campaignID), "--input is required")
	assertCampaignUsage(t, executeCampaigns(t, f, "update", campaignID, "--input", writeCampaignInput(t, `{"etag":"1"}`), "--etag", "2"), "--etag conflicts with etag")
	assertCampaignUsage(t, executeCampaigns(t, f, "get"), "campaign id")
}

func TestCampaignsDeleteConfirmsQueryControlsAndPrefetch(t *testing.T) {
	var stdout bytes.Buffer
	var prompt string
	srv, got := captureRootCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"campaigns/c1","etag":"77"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	f := campaignsFactory(t, nil, &stdout, false, func(got string) error {
		prompt = got
		return nil
	})
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "delete", campaignID); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete campaign c1?" {
		t.Fatalf("prompt=%q", prompt)
	}
	if len(*got) != 2 {
		t.Fatalf("requests=%d", len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns/c1", "", "", false)
	assertRootCampaignRequest(t, (*got)[1], http.MethodDelete, "/v1/campaigns/c1", "etag=77&request_id="+campaignReqID, "", false)
	if (*got)[1].Body != "" || stdout.String() != "" {
		t.Fatalf("body=%q stdout=%q", (*got)[1].Body, stdout.String())
	}
}

func TestCampaignsDeleteDeclinedSkipsRequest(t *testing.T) {
	srv, got := captureRootCampaigns(t, campaignOK(`{}`))
	f := campaignsFactory(t, nil, io.Discard, false, func(string) error {
		return cli.ErrCancelled
	})
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "delete", campaignID); !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("err=%v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("requests=%d", len(*got))
	}
}

func TestCampaignsDeleteMissingYesIsUsageError(t *testing.T) {
	usage := &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	f := campaignsFactory(t, nil, io.Discard, false, func(string) error {
		return usage
	})
	assertCampaignUsage(t, executeCampaigns(t, f, "delete", campaignID), usage.Msg)
}

func TestCampaignsLifecycleControlBodies(t *testing.T) {
	cases := []struct {
		args  []string
		path  string
		body  string
		table string
	}{
		{[]string{"activate", campaignID}, "/v1/campaigns/c1:activate", rootCampaignJSON, campaignTable},
		{[]string{"resume", campaignID}, "/v1/campaigns/c1:resume", rootCampaignJSON, campaignTable},
		{[]string{"complete", campaignID}, "/v1/campaigns/c1:complete", rootCampaignJSON, campaignTable},
		{[]string{"unarchive", campaignID}, "/v1/campaigns/c1:unarchive", rootCampaignJSON, campaignTable},
		{[]string{"pause", campaignID}, "/v1/campaigns/c1:pause", rootOperationJSON, rootOperationTable},
	}
	for _, tc := range cases {
		t.Run(tc.args[0], func(t *testing.T) {
			var stdout bytes.Buffer
			orgCalls := 0
			srv, got := captureRootCampaigns(t, campaignPrefetchThen(tc.body))
			f := campaignsFactory(t, nil, &stdout, false, nil)
			f.Organization = func() (string, error) {
				orgCalls++
				return "", nil
			}
			bindCampaignClient(t, f, srv, "")
			if err := executeCampaigns(t, f, tc.args...); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if orgCalls != 0 || len(*got) != 2 {
				t.Fatalf("orgCalls=%d requests=%d", orgCalls, len(*got))
			}
			assertRootCampaignRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns/c1", "", "", false)
			assertRootCampaignRequest(t, (*got)[1], http.MethodPost, tc.path, "", "", true)
			assertCampaignJSON(t, (*got)[1].Body, `{"etag":"77","request_id":"`+campaignReqID+`"}`)
			if stdout.String() != tc.table {
				t.Fatalf("stdout=%q", stdout.String())
			}
		})
	}
}

func TestCampaignsArchiveConfirmsAndControlBody(t *testing.T) {
	var stdout bytes.Buffer
	var prompt string
	srv, got := captureRootCampaigns(t, campaignPrefetchThen(rootCampaignJSON))
	f := campaignsFactory(t, nil, &stdout, false, func(got string) error {
		prompt = got
		return nil
	})
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "archive", campaignID); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Archive campaign c1?" {
		t.Fatalf("prompt=%q", prompt)
	}
	if len(*got) != 2 {
		t.Fatalf("requests=%d", len(*got))
	}
	assertRootCampaignRequest(t, (*got)[1], http.MethodPost, "/v1/campaigns/c1:archive", "", "", true)
	assertCampaignJSON(t, (*got)[1].Body, `{"etag":"77","request_id":"`+campaignReqID+`"}`)
	if stdout.String() != campaignTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCampaignsArchiveDeclinedSkipsRequest(t *testing.T) {
	srv, got := captureRootCampaigns(t, campaignOK(rootCampaignJSON))
	f := campaignsFactory(t, nil, io.Discard, false, func(string) error {
		return cli.ErrCancelled
	})
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "archive", campaignID); !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("err=%v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("requests=%d", len(*got))
	}
}

func TestCampaignsCustomFlagEtagSkipsPrefetch(t *testing.T) {
	srv, got := captureRootCampaigns(t, campaignOK(rootCampaignJSON))
	f := campaignsFactory(t, nil, io.Discard, false, nil)
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "activate", campaignID, "--etag", "flag-etag", "--request-id", "flag-id"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("requests=%d", len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodPost, "/v1/campaigns/c1:activate", "", "", true)
	assertCampaignJSON(t, (*got)[0].Body, `{"etag":"flag-etag","request_id":"flag-id"}`)
}

func TestCampaignsSetVariablesInputAndControls(t *testing.T) {
	var stdout bytes.Buffer
	srv, got := captureRootCampaigns(t, campaignPrefetchThen(rootCampaignJSON))
	f := campaignsFactory(t, nil, &stdout, false, nil)
	f.Organization = func() (string, error) {
		t.Fatal("Organization must not be called")
		return "", nil
	}
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "set-variables", campaignID, "--input", writeCampaignInput(t, `{"variables":["given_name","company"]}`)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 2 {
		t.Fatalf("requests=%d", len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodGet, "/v1/campaigns/c1", "", "", false)
	assertRootCampaignRequest(t, (*got)[1], http.MethodPost, "/v1/campaigns/c1:setVariables", "", "", true)
	assertCampaignJSON(t, (*got)[1].Body, `{"etag":"77","request_id":"`+campaignReqID+`","variables":["given_name","company"]}`)
	if stdout.String() != campaignTable {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestCampaignsSetSenderAccountsJSONAndConflict(t *testing.T) {
	var stdout bytes.Buffer
	srv, got := captureRootCampaigns(t, campaignOK(senderJSON))
	f := campaignsFactory(t, nil, &stdout, true, nil)
	bindCampaignClient(t, f, srv, "ns-1")
	if err := executeCampaigns(t, f, "set-sender-accounts", campaignID, "--input", writeCampaignInput(t, `{"account_ids":["a1","a2"]}`), "--etag", "8"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(*got) != 1 {
		t.Fatalf("requests=%d", len(*got))
	}
	assertRootCampaignRequest(t, (*got)[0], http.MethodPost, "/v1/campaigns/c1:setSenderAccounts", "", "ns-1", true)
	assertCampaignJSON(t, (*got)[0].Body, `{"account_ids":["a1","a2"],"etag":"8","request_id":"`+campaignReqID+`"}`)
	if stdout.String() != senderJSON {
		t.Fatalf("stdout=%q", stdout.String())
	}
	f = campaignsFactory(t, nil, io.Discard, false, nil)
	assertCampaignUsage(t, executeCampaigns(t, f, "set-sender-accounts", campaignID), "--input is required")
	assertCampaignUsage(t, executeCampaigns(t, f, "set-variables", campaignID, "--input", writeCampaignInput(t, `{"etag":"1","variables":[]}`), "--etag", "2"), "--etag conflicts with etag")
}

func TestCampaignsRootVerbRoutes(t *testing.T) {
	cases := []struct {
		args   []string
		method string
		path   string
		query  string
	}{
		{[]string{"list"}, http.MethodGet, "/v1/campaigns", "page_size=50"},
		{[]string{"get", campaignID}, http.MethodGet, "/v1/campaigns/c1", ""},
		{[]string{"create", "--display-name", "Acme"}, http.MethodPost, "/v1/campaigns", ""},
		{[]string{"update", campaignID, "--input", `{"update_mask":"display_name","campaign":{}}`, "--etag", "1"}, http.MethodPatch, "/v1/campaigns/c1", ""},
		{[]string{"delete", campaignID, "--etag", "1"}, http.MethodDelete, "/v1/campaigns/c1", "etag=1&request_id=" + campaignReqID},
		{[]string{"activate", campaignID, "--etag", "1"}, http.MethodPost, "/v1/campaigns/c1:activate", ""},
		{[]string{"pause", campaignID, "--etag", "1"}, http.MethodPost, "/v1/campaigns/c1:pause", ""},
		{[]string{"resume", campaignID, "--etag", "1"}, http.MethodPost, "/v1/campaigns/c1:resume", ""},
		{[]string{"complete", campaignID, "--etag", "1"}, http.MethodPost, "/v1/campaigns/c1:complete", ""},
		{[]string{"archive", campaignID, "--etag", "1"}, http.MethodPost, "/v1/campaigns/c1:archive", ""},
		{[]string{"unarchive", campaignID, "--etag", "1"}, http.MethodPost, "/v1/campaigns/c1:unarchive", ""},
		{[]string{"set-variables", campaignID, "--input", `{"variables":["given_name"]}`, "--etag", "1"}, http.MethodPost, "/v1/campaigns/c1:setVariables", ""},
		{[]string{"set-sender-accounts", campaignID, "--input", `{"account_ids":["a1"]}`, "--etag", "1"}, http.MethodPost, "/v1/campaigns/c1:setSenderAccounts", ""},
	}
	for _, tc := range cases {
		t.Run(tc.args[0], func(t *testing.T) {
			args := append([]string{}, tc.args...)
			for i, arg := range args {
				if strings.HasPrefix(arg, "{") {
					args[i] = writeCampaignInput(t, arg)
				}
			}
			body := rootCampaignJSON
			if tc.args[0] == "pause" {
				body = rootOperationJSON
			}
			if tc.args[0] == "set-sender-accounts" {
				body = senderJSON
			}
			if tc.args[0] == "create" {
				body = campaignCreate
			}
			srv, got := captureRootCampaigns(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(body))
			})
			f := campaignsFactory(t, nil, io.Discard, false, nil)
			bindCampaignClient(t, f, srv, "")
			if err := executeCampaigns(t, f, args...); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if len(*got) != 1 {
				t.Fatalf("requests=%d", len(*got))
			}
			if (*got)[0].Method != tc.method || (*got)[0].Path != tc.path || (*got)[0].Query != tc.query {
				t.Fatalf("request %s %s?%s", (*got)[0].Method, (*got)[0].Path, (*got)[0].Query)
			}
		})
	}
}

func TestCampaignsResourceIDPathAndWrappedCreate(t *testing.T) {
	srv, got := captureRootCampaigns(t, campaignOK(rootCampaignJSON))
	f := campaignsFactory(t, nil, io.Discard, false, nil)
	bindCampaignClient(t, f, srv, "")
	if err := executeCampaigns(t, f, "get", "campaigns/c1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if (*got)[0].Path != "/v1/campaigns/c1" {
		t.Fatalf("path=%q", (*got)[0].Path)
	}
	row, err := decodeCampaignRecord([]byte(campaignCreate))
	if err != nil || row.Name != "campaigns/c1" || row.DisplayName != "Acme" {
		t.Fatalf("unwrap %#v err=%v", row, err)
	}
}

func TestCampaignsCobraTreeUsesNewCommand(t *testing.T) {
	var stdout bytes.Buffer
	f := campaignsFactory(t, nil, &stdout, false, nil)
	cmd := NewCommand(f)
	if cmd.Use != "campaigns" {
		t.Fatalf("use=%q", cmd.Use)
	}
	parent := &cobra.Command{Use: "sl", SilenceUsage: true, SilenceErrors: true}
	parent.AddCommand(cmd)
	parent.SetArgs([]string{"campaigns", "--help"})
	parent.SetOut(&stdout)
	parent.SetErr(io.Discard)
	if err := parent.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	help := stdout.String()
	for _, name := range []string{"list", "activate", "set-variables", "set-sender-accounts"} {
		if !strings.Contains(help, name) {
			t.Fatalf("help missing %s: %s", name, help)
		}
	}
}

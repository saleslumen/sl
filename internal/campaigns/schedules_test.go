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
	scheduleFixtureID   = "11111111-1111-4111-8111-111111111111"
	scheduleFixtureName = "campaignSchedules/11111111-1111-4111-8111-111111111111"
	scheduleFixtureJSON = `{"name":"campaignSchedules/11111111-1111-4111-8111-111111111111","display_name":"Day mornings","timezone":"America/New_York","days":["MONDAY","TUESDAY","WEDNESDAY","THURSDAY","FRIDAY"],"start_time":"09:00:00","end_time":"12:00:00","minimum_spacing_seconds":300,"organization":"organizations/org-test","etag":"3","create_time":"2026-01-01T00:00:00Z","update_time":"2026-02-01T00:00:00Z"}`
	scheduleTableRow    = "11111111-1111-4111-8111-111111111111\tDay mornings\tAmerica/New_York\tMONDAY,TUESDAY,WEDNESDAY,THURSDAY,FRIDAY\t09:00:00-12:00:00\t300\t2026-02-01T00:00:00Z\n"
	scheduleTestRequest = "00000000-0000-4000-8000-000000000001"
)

type scheduleCall struct {
	method   string
	host     string
	path     string
	rawQuery string
	body     string
	headers  http.Header
}

func scheduleJSONMode(stdout io.Writer) func() *output.Printer {
	return func() *output.Printer {
		return output.New(stdout, output.Options{JSON: true})
	}
}

func writeScheduleInput(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func executeSchedules(t *testing.T, f *cli.Factory, args ...string) (string, error) {
	t.Helper()
	var stdout *bytes.Buffer
	if buf, ok := f.IO.Out.(*bytes.Buffer); ok {
		stdout = buf
		stdout.Reset()
	}
	cmd := newSchedulesCommand(f)
	cmd.SetArgs(args)
	if f.IO != nil && f.IO.In != nil {
		cmd.SetIn(io.NopCloser(f.IO.In))
	}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if stdout == nil {
		return "", err
	}
	return stdout.String(), err
}

func scheduleFactory(t *testing.T, srv *httptest.Server, stdin io.Reader, namespace string, stdout *bytes.Buffer) *cli.Factory {
	t.Helper()
	f := testFactory(stdin, stdout)
	f.Client = func() (*apiclient.Client, error) { return testClient(t, srv, namespace), nil }
	return f
}

func decodeScheduleObject(t *testing.T, raw string) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return obj
}

func assertScheduleJSON(t *testing.T, got, want string) {
	t.Helper()
	if strings.TrimSpace(got) == strings.TrimSpace(want) {
		return
	}
	var gotObj, wantObj any
	if err := json.Unmarshal([]byte(got), &gotObj); err != nil {
		t.Fatalf("got json %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &wantObj); err != nil {
		t.Fatalf("want json %q: %v", want, err)
	}
	gotRaw, err := json.Marshal(gotObj)
	if err != nil {
		t.Fatal(err)
	}
	wantRaw, err := json.Marshal(wantObj)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotRaw) != string(wantRaw) {
		t.Fatalf("json=%s want=%s", got, want)
	}
}

func TestNewSchedulesCommandVerbsAndHelp(t *testing.T) {
	var stdout bytes.Buffer
	called := false
	f := testFactory(nil, &stdout)
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	f.Organization = func() (string, error) {
		called = true
		return "", nil
	}
	cmd := newSchedulesCommand(f)
	if cmd.Use != "schedules" || cmd.Short != "Manage campaign schedules" {
		t.Fatalf("use=%q short=%q", cmd.Use, cmd.Short)
	}
	names := map[string]string{}
	for _, child := range cmd.Commands() {
		names[child.Name()] = child.Short
	}
	want := map[string]string{"list": "List schedules", "get": "Get a schedule", "create": "Create a schedule", "update": "Update a schedule", "delete": "Delete a schedule"}
	for name, short := range want {
		if names[name] != short {
			t.Fatalf("verb %s short=%q want %q in %#v", name, names[name], short, names)
		}
	}
	wantUse := map[string]string{"list": "list", "get": "get SCHEDULE_ID", "create": "create", "update": "update SCHEDULE_ID --input FILE", "delete": "delete SCHEDULE_ID"}
	for _, child := range cmd.Commands() {
		if child.Use != wantUse[child.Name()] {
			t.Fatalf("verb %s use=%q want %q", child.Name(), child.Use, wantUse[child.Name()])
		}
	}
	var createCmd *cobra.Command
	for _, child := range cmd.Commands() {
		if child.Name() == "create" {
			createCmd = child
		}
		if child.Flags().Lookup("days") != nil {
			t.Fatalf("%s registered days", child.Name())
		}
	}
	if createCmd == nil {
		t.Fatal("missing create")
	}
	wantUsage := map[string]map[string]string{
		"create": {"input": "JSON request body file, or - for stdin", "display-name": "Schedule display name", "timezone": "IANA timezone", "start-time": "Local start time (HH:MM)", "end-time": "Local end time (HH:MM)", "minimum-spacing-seconds": "Minimum spacing between sends in seconds", "request-id": "Idempotency request ID"},
		"update": {"input": "JSON request body file, or - for stdin", "request-id": "Idempotency request ID", "etag": "Schedule etag; fetched when omitted"},
		"delete": {"request-id": "Idempotency request ID", "etag": "Schedule etag; fetched when omitted"},
	}
	for _, child := range cmd.Commands() {
		for name, usage := range wantUsage[child.Name()] {
			flag := child.Flags().Lookup(name)
			if flag == nil || flag.Usage != usage {
				got := ""
				if flag != nil {
					got = flag.Usage
				}
				t.Fatalf("%s --%s usage=%q want %q", child.Name(), name, got, usage)
			}
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
	for _, name := range []string{"list", "get", "create", "update", "delete"} {
		if !strings.Contains(help, name) {
			t.Fatalf("help missing %s: %s", name, help)
		}
	}
}

func TestSchedulesListDefaultLimitHeadersTableAndJSON(t *testing.T) {
	var got scheduleCall
	hits := 0
	page := `{"campaign_schedules":[` + scheduleFixtureJSON + `],"next_page_token":""}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		got = scheduleCall{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()}
		_, _ = w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	out, err := executeSchedules(t, f, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if hits != 1 || got.method != http.MethodGet || got.host != "campaigns.example.test" || got.path != "/v1/campaignSchedules" || got.rawQuery != "page_size=50" || got.body != "" {
		t.Fatalf("request hits=%d %#v", hits, got)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if got.headers.Get("sl-organization-id") != "" || got.headers.Get("Authorization") != "" {
		t.Fatalf("forbidden headers %#v", applicationHeaders(got.headers))
	}
	if out != scheduleTableRow {
		t.Fatalf("table=%q want=%q", out, scheduleTableRow)
	}
	f.Printer = scheduleJSONMode(&stdout)
	jsonOut, err := executeSchedules(t, f, "list")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	assertScheduleJSON(t, jsonOut, `{"campaign_schedules":[`+scheduleFixtureJSON+`]}`)
}

func TestSchedulesListSendsNamespaceAndOmitsOrganization(t *testing.T) {
	var headers http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		_, _ = w.Write([]byte(`{"campaign_schedules":[]}`))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "ns-1", &stdout)
	out, err := executeSchedules(t, f, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	assertApplicationHeaders(t, headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json"})
	if headers.Get("sl-organization-id") != "" || out != "" {
		t.Fatalf("headers=%#v out=%q", applicationHeaders(headers), out)
	}
}

func TestSchedulesListPaginationLimitAndReconstructedJSON(t *testing.T) {
	var calls []scheduleCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, scheduleCall{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		switch r.URL.Query().Get("page_token") {
		case "":
			_, _ = w.Write([]byte(`{"campaign_schedules":[{"name":"campaignSchedules/a","display_name":"A","timezone":"UTC","days":["MONDAY"],"start_time":"09:00:00","end_time":"10:00:00","minimum_spacing_seconds":60,"update_time":"2026-01-01T00:00:00Z"},{"name":"campaignSchedules/b","display_name":"B","timezone":"UTC","days":["TUESDAY"],"start_time":"11:00:00","end_time":"12:00:00","minimum_spacing_seconds":90,"update_time":"2026-01-02T00:00:00Z"}],"next_page_token":"p2"}`))
		case "p2":
			_, _ = w.Write([]byte(`{"campaign_schedules":[{"name":"campaignSchedules/c","display_name":"C","timezone":"UTC","days":["WEDNESDAY"],"start_time":"13:00:00","end_time":"14:00:00","minimum_spacing_seconds":120,"update_time":"2026-01-03T00:00:00Z"},{"name":"campaignSchedules/d","display_name":"D","timezone":"UTC","days":["THURSDAY"],"start_time":"15:00:00","end_time":"16:00:00","minimum_spacing_seconds":150,"update_time":"2026-01-04T00:00:00Z"}]}`))
		default:
			t.Errorf("unexpected token %q", r.URL.RawQuery)
		}
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	out, err := executeSchedules(t, f, "list", "--limit", "3")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(calls) != 2 || calls[0].rawQuery != "page_size=3" || calls[1].rawQuery != "page_size=1&page_token=p2" {
		t.Fatalf("calls %#v", calls)
	}
	if calls[0].method != http.MethodGet || calls[0].path != "/v1/campaignSchedules" || calls[0].host != "campaigns.example.test" {
		t.Fatalf("first %#v", calls[0])
	}
	wantTable := "a\tA\tUTC\tMONDAY\t09:00:00-10:00:00\t60\t2026-01-01T00:00:00Z\nb\tB\tUTC\tTUESDAY\t11:00:00-12:00:00\t90\t2026-01-02T00:00:00Z\nc\tC\tUTC\tWEDNESDAY\t13:00:00-14:00:00\t120\t2026-01-03T00:00:00Z\n"
	if out != wantTable {
		t.Fatalf("table=%q want=%q", out, wantTable)
	}
	calls = nil
	f.Printer = scheduleJSONMode(&stdout)
	jsonOut, err := executeSchedules(t, f, "list", "--limit", "3")
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	assertScheduleJSON(t, jsonOut, `{"campaign_schedules":[{"name":"campaignSchedules/a","display_name":"A","timezone":"UTC","days":["MONDAY"],"start_time":"09:00:00","end_time":"10:00:00","minimum_spacing_seconds":60,"update_time":"2026-01-01T00:00:00Z"},{"name":"campaignSchedules/b","display_name":"B","timezone":"UTC","days":["TUESDAY"],"start_time":"11:00:00","end_time":"12:00:00","minimum_spacing_seconds":90,"update_time":"2026-01-02T00:00:00Z"},{"name":"campaignSchedules/c","display_name":"C","timezone":"UTC","days":["WEDNESDAY"],"start_time":"13:00:00","end_time":"14:00:00","minimum_spacing_seconds":120,"update_time":"2026-01-03T00:00:00Z"}]}`)
}

func TestSchedulesListRejectsInvalidLimit(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	for _, limit := range []string{"0", "-1"} {
		_, err := executeSchedules(t, f, "list", "--limit", limit)
		var usage *cli.UsageError
		if !errors.As(err, &usage) || usage.Msg != "--limit must be at least 1" || hits != 0 {
			t.Fatalf("limit %s err=%v hits=%d", limit, err, hits)
		}
	}
}

func TestSchedulesGetTableJSONHeadersAndResourceName(t *testing.T) {
	var got scheduleCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = scheduleCall{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()}
		_, _ = w.Write([]byte(scheduleFixtureJSON))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	out, err := executeSchedules(t, f, "get", scheduleFixtureName)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.method != http.MethodGet || got.host != "campaigns.example.test" || got.path != "/v1/campaignSchedules/"+scheduleFixtureID || got.rawQuery != "" || got.body != "" {
		t.Fatalf("request %#v", got)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if out != scheduleTableRow {
		t.Fatalf("table=%q", out)
	}
	f.Printer = scheduleJSONMode(&stdout)
	jsonOut, err := executeSchedules(t, f, "get", scheduleFixtureID)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	assertScheduleJSON(t, jsonOut, scheduleFixtureJSON)
}

func TestSchedulesGetRequiresID(t *testing.T) {
	var stdout bytes.Buffer
	f := testFactory(nil, &stdout)
	_, err := executeSchedules(t, f, "get")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "schedule id") {
		t.Fatalf("err=%v", err)
	}
}

func TestSchedulesCreateFlagsInjectOrganizationAndRequestID(t *testing.T) {
	var got scheduleCall
	orgCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = scheduleCall{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()}
		_, _ = w.Write([]byte(scheduleFixtureJSON))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "ns-9", &stdout)
	f.Organization = func() (string, error) {
		orgCalls++
		return "org-test", nil
	}
	input := writeScheduleInput(t, `{"days":["MONDAY","TUESDAY","WEDNESDAY","THURSDAY","FRIDAY"],"keep":true}`)
	out, err := executeSchedules(t, f, "create", "--display-name", "Day mornings", "--timezone", "America/New_York", "--start-time", "09:00:00", "--end-time", "12:00:00", "--minimum-spacing-seconds", "300", "--input", input)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if orgCalls != 1 || got.method != http.MethodPost || got.host != "campaigns.example.test" || got.path != "/v1/campaignSchedules" || got.rawQuery != "" {
		t.Fatalf("request org=%d %#v", orgCalls, got)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-9", "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if got.headers.Get("sl-organization-id") != "" {
		t.Fatalf("organization header %q", got.headers.Get("sl-organization-id"))
	}
	body := decodeScheduleObject(t, got.body)
	if body["display_name"] != "Day mornings" || body["timezone"] != "America/New_York" || body["start_time"] != "09:00:00" || body["end_time"] != "12:00:00" || body["minimum_spacing_seconds"] != float64(300) || body["organization"] != "organizations/org-test" || body["request_id"] != scheduleTestRequest || body["keep"] != true {
		t.Fatalf("body %#v", body)
	}
	days, _ := body["days"].([]any)
	if len(days) != 5 || days[0] != "MONDAY" || days[4] != "FRIDAY" {
		t.Fatalf("days %#v", body["days"])
	}
	if _, ok := body["etag"]; ok {
		t.Fatalf("etag leaked %#v", body)
	}
	if out != scheduleTableRow {
		t.Fatalf("table=%q", out)
	}
}

func TestSchedulesCreateInputPreservesOrganizationRequestIDAndJSON(t *testing.T) {
	var got scheduleCall
	orgCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = scheduleCall{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()}
		_, _ = w.Write([]byte(scheduleFixtureJSON))
	}))
	t.Cleanup(srv.Close)
	input := `{"display_name":"Day mornings","timezone":"America/New_York","days":["MONDAY"],"start_time":"09:00:00","end_time":"12:00:00","minimum_spacing_seconds":300,"organization":"organizations/from-input","request_id":"input-id","namespace":null}`
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, strings.NewReader(input), "", &stdout)
	f.Organization = func() (string, error) {
		orgCalls++
		return "org-test", nil
	}
	f.Printer = scheduleJSONMode(&stdout)
	out, err := executeSchedules(t, f, "create", "--input", "-")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if orgCalls != 0 || got.method != http.MethodPost || got.path != "/v1/campaignSchedules" {
		t.Fatalf("org=%d %#v", orgCalls, got)
	}
	body := decodeScheduleObject(t, got.body)
	if body["organization"] != "organizations/from-input" || body["request_id"] != "input-id" || body["namespace"] != nil || body["display_name"] != "Day mornings" {
		t.Fatalf("body %#v", body)
	}
	assertScheduleJSON(t, out, scheduleFixtureJSON)
}

func TestSchedulesCreateRequestIDFlagAndFileInput(t *testing.T) {
	var got scheduleCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = scheduleCall{method: r.Method, path: r.URL.Path, body: string(raw), headers: r.Header.Clone()}
		_, _ = w.Write([]byte(scheduleFixtureJSON))
	}))
	t.Cleanup(srv.Close)
	input := writeScheduleInput(t, `{"days":["FRIDAY"],"display_name":"Friday"}`)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	if _, err := executeSchedules(t, f, "create", "--request-id", "flag-id", "--input", input); err != nil {
		t.Fatalf("create: %v", err)
	}
	body := decodeScheduleObject(t, got.body)
	if body["request_id"] != "flag-id" || body["display_name"] != "Friday" || body["organization"] != "organizations/org-test" {
		t.Fatalf("body %#v", body)
	}
}

func TestSchedulesCreateFlagInputConflicts(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	cases := []struct {
		name  string
		args  []string
		input string
		want  string
	}{
		{"display-name", []string{"--display-name", "Other"}, `{"display_name":"Day mornings"}`, "--display-name conflicts with display_name in --input"},
		{"timezone", []string{"--timezone", "UTC"}, `{"timezone":"America/New_York"}`, "--timezone conflicts with timezone in --input"},
		{"start-time", []string{"--start-time", "08:00:00"}, `{"start_time":"09:00:00"}`, "--start-time conflicts with start_time in --input"},
		{"end-time", []string{"--end-time", "13:00:00"}, `{"end_time":"12:00:00"}`, "--end-time conflicts with end_time in --input"},
		{"minimum-spacing-seconds", []string{"--minimum-spacing-seconds", "60"}, `{"minimum_spacing_seconds":300}`, "--minimum-spacing-seconds conflicts with minimum_spacing_seconds in --input"},
		{"request-id", []string{"--request-id", "flag-id"}, `{"request_id":"input-id"}`, "--request-id conflicts with request_id in --input"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			f := scheduleFactory(t, srv, nil, "", &stdout)
			args := append([]string{"create", "--input", writeScheduleInput(t, tc.input)}, tc.args...)
			_, err := executeSchedules(t, f, args...)
			var usage *cli.UsageError
			if !errors.As(err, &usage) || usage.Msg != tc.want || hits != 0 {
				t.Fatalf("err=%v hits=%d", err, hits)
			}
		})
	}
}

func TestSchedulesCreateConflict409(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"ABORTED","message":"etag/idempotency conflict"}`))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	_, err := executeSchedules(t, f, "create", "--input", writeScheduleInput(t, `{"display_name":"Day mornings"}`))
	var apiErr *apiclient.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Code != "ABORTED" || apiErr.Method != http.MethodPost || apiErr.Path != "/v1/campaignSchedules" || apiErr.Product != "campaigns" {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(apiErr.Message, "etag/idempotency conflict") {
		t.Fatalf("message=%q", apiErr.Message)
	}
}

func TestSchedulesUpdateFetchesEtagAndInjectsControls(t *testing.T) {
	var calls []scheduleCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, scheduleCall{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"name":"` + scheduleFixtureName + `","etag":"77","display_name":"Kept"}`))
			return
		}
		_, _ = w.Write([]byte(scheduleFixtureJSON))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	orgCalls := 0
	f.Organization = func() (string, error) {
		orgCalls++
		return "org-test", nil
	}
	input := writeScheduleInput(t, `{"schedule":{"display_name":"Evenings","timezone":"UTC","days":["MONDAY"],"start_time":"13:00:00","end_time":"17:00:00","minimum_spacing_seconds":120},"update_mask":"display_name,timezone,days,start_time,end_time,minimum_spacing_seconds","keep":true}`)
	out, err := executeSchedules(t, f, "update", scheduleFixtureID, "--input", input)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if orgCalls != 0 || len(calls) != 2 {
		t.Fatalf("org=%d calls=%d", orgCalls, len(calls))
	}
	if calls[0].method != http.MethodGet || calls[0].path != "/v1/campaignSchedules/"+scheduleFixtureID || calls[0].rawQuery != "" || calls[0].body != "" {
		t.Fatalf("get %#v", calls[0])
	}
	assertApplicationHeaders(t, calls[0].headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if calls[1].method != http.MethodPatch || calls[1].host != "campaigns.example.test" || calls[1].path != "/v1/campaignSchedules/"+scheduleFixtureID || calls[1].rawQuery != "" {
		t.Fatalf("patch %#v", calls[1])
	}
	assertApplicationHeaders(t, calls[1].headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	body := decodeScheduleObject(t, calls[1].body)
	if body["request_id"] != scheduleTestRequest || body["etag"] != "77" || body["update_mask"] != "display_name,timezone,days,start_time,end_time,minimum_spacing_seconds" || body["keep"] != true {
		t.Fatalf("body %#v", body)
	}
	schedule, _ := body["schedule"].(map[string]any)
	if schedule["display_name"] != "Evenings" || schedule["minimum_spacing_seconds"] != float64(120) {
		t.Fatalf("schedule %#v", schedule)
	}
	if _, ok := body["organization"]; ok {
		t.Fatalf("organization injected %#v", body)
	}
	if out != scheduleTableRow {
		t.Fatalf("table=%q", out)
	}
}

func TestSchedulesUpdatePreservesInputControlsAndJSON(t *testing.T) {
	var calls []scheduleCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, scheduleCall{method: r.Method, path: r.URL.Path, body: string(raw), headers: r.Header.Clone()})
		_, _ = w.Write([]byte(scheduleFixtureJSON))
	}))
	t.Cleanup(srv.Close)
	input := `{"schedule":{"display_name":"Nights"},"update_mask":"display_name","etag":"input-etag","request_id":"input-id"}`
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, strings.NewReader(input), "ns-2", &stdout)
	f.Printer = scheduleJSONMode(&stdout)
	out, err := executeSchedules(t, f, "update", scheduleFixtureName, "--input", "-")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(calls) != 1 || calls[0].method != http.MethodPatch || calls[0].path != "/v1/campaignSchedules/"+scheduleFixtureID {
		t.Fatalf("calls %#v", calls)
	}
	assertApplicationHeaders(t, calls[0].headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-2", "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	body := decodeScheduleObject(t, calls[0].body)
	if body["etag"] != "input-etag" || body["request_id"] != "input-id" {
		t.Fatalf("body %#v", body)
	}
	assertScheduleJSON(t, out, scheduleFixtureJSON)
}

func TestSchedulesUpdateFlagControlsSkipGet(t *testing.T) {
	var calls []scheduleCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, scheduleCall{method: r.Method, path: r.URL.Path, body: string(raw)})
		_, _ = w.Write([]byte(scheduleFixtureJSON))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	_, err := executeSchedules(t, f, "update", scheduleFixtureID, "--request-id", "flag-id", "--etag", "flag-etag", "--input", writeScheduleInput(t, `{"schedule":{"display_name":"Nights"},"update_mask":"display_name"}`))
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(calls) != 1 || calls[0].method != http.MethodPatch {
		t.Fatalf("calls %#v", calls)
	}
	body := decodeScheduleObject(t, calls[0].body)
	if body["request_id"] != "flag-id" || body["etag"] != "flag-etag" {
		t.Fatalf("body %#v", body)
	}
}

func TestSchedulesUpdateConflictsAndMissingInput(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	_, err := executeSchedules(t, f, "update", scheduleFixtureID)
	var usage *cli.UsageError
	if !errors.As(err, &usage) || usage.Msg != "--input is required" || hits != 0 {
		t.Fatalf("missing input: %v hits=%d", err, hits)
	}
	_, err = executeSchedules(t, f, "update", scheduleFixtureID, "--request-id", "flag-id", "--input", writeScheduleInput(t, `{"request_id":"input-id"}`))
	if !errors.As(err, &usage) || usage.Msg != "--request-id conflicts with request_id in --input" || hits != 0 {
		t.Fatalf("request_id: %v hits=%d", err, hits)
	}
	_, err = executeSchedules(t, f, "update", scheduleFixtureID, "--etag", "flag-etag", "--input", writeScheduleInput(t, `{"etag":"input-etag"}`))
	if !errors.As(err, &usage) || usage.Msg != "--etag conflicts with etag in --input" || hits != 0 {
		t.Fatalf("etag: %v hits=%d", err, hits)
	}
}

func TestSchedulesUpdateConflict409(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"etag":"1"}`))
			return
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"ABORTED","message":"etag mismatch"}`))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	_, err := executeSchedules(t, f, "update", scheduleFixtureID, "--input", writeScheduleInput(t, `{"schedule":{"display_name":"X"},"update_mask":"display_name"}`))
	var apiErr *apiclient.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Code != "ABORTED" || apiErr.Path != "/v1/campaignSchedules/"+scheduleFixtureID {
		t.Fatalf("err=%v", err)
	}
}

func TestSchedulesDeleteFetchesEtagQueryControlsAndConfirmation(t *testing.T) {
	var calls []scheduleCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, scheduleCall{method: r.Method, host: r.Host, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"name":"` + scheduleFixtureName + `","etag":"9"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	var prompt string
	f := scheduleFactory(t, srv, nil, "", &stdout)
	f.Confirm = func(got string) error {
		prompt = got
		return nil
	}
	out, err := executeSchedules(t, f, "delete", scheduleFixtureID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if prompt != "Delete schedule "+scheduleFixtureID+"?" || out != "" || len(calls) != 2 {
		t.Fatalf("prompt=%q out=%q calls=%d", prompt, out, len(calls))
	}
	if calls[0].method != http.MethodGet || calls[0].path != "/v1/campaignSchedules/"+scheduleFixtureID || calls[0].body != "" {
		t.Fatalf("get %#v", calls[0])
	}
	assertApplicationHeaders(t, calls[0].headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if calls[1].method != http.MethodDelete || calls[1].host != "campaigns.example.test" || calls[1].path != "/v1/campaignSchedules/"+scheduleFixtureID || calls[1].rawQuery != "etag=9&request_id="+scheduleTestRequest || calls[1].body != "" {
		t.Fatalf("delete %#v", calls[1])
	}
	assertApplicationHeaders(t, calls[1].headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if calls[1].headers.Get("Content-Type") != "" {
		t.Fatalf("delete content-type %q", calls[1].headers.Get("Content-Type"))
	}
}

func TestSchedulesDeleteUsesProvidedControlsAndNamespace(t *testing.T) {
	var calls []scheduleCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, scheduleCall{method: r.Method, path: r.URL.Path, rawQuery: r.URL.RawQuery, body: string(raw), headers: r.Header.Clone()})
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "ns-3", &stdout)
	out, err := executeSchedules(t, f, "delete", scheduleFixtureName, "--request-id", "rid-1", "--etag", "4")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if out != "" || len(calls) != 1 || calls[0].method != http.MethodDelete || calls[0].path != "/v1/campaignSchedules/"+scheduleFixtureID || calls[0].rawQuery != "etag=4&request_id=rid-1" || calls[0].body != "" {
		t.Fatalf("out=%q calls %#v", out, calls)
	}
	assertApplicationHeaders(t, calls[0].headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-3", "User-Agent": "sl/test", "Accept": "application/json"})
}

func TestSchedulesDeleteConfirmDeclinedAndMissingYes(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	f.Confirm = func(string) error { return cli.ErrCancelled }
	_, err := executeSchedules(t, f, "delete", scheduleFixtureID)
	if !errors.Is(err, cli.ErrCancelled) || hits != 0 {
		t.Fatalf("declined err=%v hits=%d", err, hits)
	}
	f.Confirm = func(string) error {
		return &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	}
	_, err = executeSchedules(t, f, "delete", scheduleFixtureID)
	var usage *cli.UsageError
	if !errors.As(err, &usage) || usage.Msg != "--yes required when stdin is not a TTY" || hits != 0 {
		t.Fatalf("yes err=%v hits=%d", err, hits)
	}
}

func TestSchedulesDeleteConflict409(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"etag":"2"}`))
			return
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"ABORTED","message":"etag mismatch"}`))
	}))
	t.Cleanup(srv.Close)
	var stdout bytes.Buffer
	f := scheduleFactory(t, srv, nil, "", &stdout)
	_, err := executeSchedules(t, f, "delete", scheduleFixtureID)
	var apiErr *apiclient.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict || apiErr.Method != http.MethodDelete || apiErr.Path != "/v1/campaignSchedules/"+scheduleFixtureID {
		t.Fatalf("err=%v", err)
	}
}

func TestSchedulesInvalidInputJSON(t *testing.T) {
	var stdout bytes.Buffer
	f := testFactory(strings.NewReader("[]"), &stdout)
	_, err := executeSchedules(t, f, "create", "--input", "-")
	var usage *cli.UsageError
	if !errors.As(err, &usage) || !strings.Contains(usage.Msg, "JSON object") {
		t.Fatalf("err=%v", err)
	}
}

func TestScheduleWindowAndPathHelpers(t *testing.T) {
	if got := scheduleWindow("", ""); got != "" {
		t.Fatalf("empty=%q", got)
	}
	if got := scheduleWindow("09:00:00", "12:00:00"); got != "09:00:00-12:00:00" {
		t.Fatalf("window=%q", got)
	}
	if got := schedulePath(scheduleFixtureName); got != "/v1/campaignSchedules/"+scheduleFixtureID {
		t.Fatalf("name path=%q", got)
	}
	if got := schedulePath(scheduleFixtureID); got != "/v1/campaignSchedules/"+scheduleFixtureID {
		t.Fatalf("id path=%q", got)
	}
}

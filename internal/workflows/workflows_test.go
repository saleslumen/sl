package workflows

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
)

func TestListSendsPageSizeAndOmitsOrganizationHeader(t *testing.T) {
	t.Setenv("SL_ORGANIZATION_ID", "org-configured")
	var requests []capturedRequest
	body := `{"workflows":[{"id":"wf-1","name":"hello","version":1,"isActive":true,"updatedAt":"2026-01-02T03:04:05Z"}],"nextPageToken":""}`
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{})
	if err := execute(t, f, "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	got := requests[0]
	if got.method != http.MethodGet || got.host != "workflows.example.test" || got.path != "/v1/workflows" || got.query != "pageSize=50" {
		t.Fatalf("request %s %s %s?%s", got.method, got.host, got.path, got.query)
	}
	if got.body != "" {
		t.Fatalf("body=%q", got.body)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if out.String() != "wf-1\thello\t1\ttrue\t2026-01-02T03:04:05Z\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestListFollowsPageTokensAndLimit(t *testing.T) {
	var requests []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		requests = append(requests, capturedRequest{method: r.Method, host: r.Host, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone(), body: string(raw)})
		switch r.URL.RawQuery {
		case "pageSize=2":
			_, _ = w.Write([]byte(`{"workflows":[{"id":"a","name":"A","version":1,"isActive":true,"updatedAt":"t1"}],"nextPageToken":"1"}`))
		case "pageSize=2&pageToken=1":
			_, _ = w.Write([]byte(`{"workflows":[{"id":"b","name":"B","version":2,"isActive":false,"updatedAt":"t2"},{"id":"c","name":"C","version":3,"isActive":true,"updatedAt":"t3"}],"nextPageToken":"2"}`))
		default:
			t.Errorf("unexpected query %q", r.URL.RawQuery)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	if err := execute(t, f, "list", "--limit", "2"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests=%d", len(requests))
	}
	if requests[0].method != http.MethodGet || requests[0].path != "/v1/workflows" || requests[0].query != "pageSize=2" {
		t.Fatalf("page1 %s %s?%s", requests[0].method, requests[0].path, requests[0].query)
	}
	if requests[1].method != http.MethodGet || requests[1].path != "/v1/workflows" || requests[1].query != "pageSize=2&pageToken=1" {
		t.Fatalf("page2 %s %s?%s", requests[1].method, requests[1].path, requests[1].query)
	}
	if out.String() != "a\tA\t1\ttrue\tt1\nb\tB\t2\tfalse\tt2\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestListJSONPreservesAPIResponse(t *testing.T) {
	var requests []capturedRequest
	raw := `{"workflows":[{"id":"wf-1","name":"hello","version":1,"isActive":true,"updatedAt":"t1","nodes":[{"id":"start"}]}],"nextPageToken":"","totalCount":1}`
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.String() != raw {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestGetSendsPathWithoutBody(t *testing.T) {
	var requests []capturedRequest
	raw := `{"workflow":{"id":"wf-1","name":"hello","version":2,"isActive":false,"updatedAt":"t2"}}`
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), "ns-1"), factoryOptions{})
	if err := execute(t, f, "get", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	got := requests[0]
	if got.method != http.MethodGet || got.path != "/v1/workflows/wf-1" || got.query != "" || got.body != "" {
		t.Fatalf("request %s %s?%s body=%q", got.method, got.path, got.query, got.body)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json"})
	if out.String() != "wf-1\thello\t2\tfalse\tt2\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestCreateSendsInputObject(t *testing.T) {
	var requests []capturedRequest
	input := `{"name":"hello","nodes":[{"id":"start","type":1,"start":{}}],"connections":[]}`
	raw := `{"workflow":{"id":"wf-1","name":"hello","version":1,"isActive":true,"updatedAt":"t1"}}`
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
	if err := execute(t, f, "create", "--input", writeInput(t, input)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	got := requests[0]
	if got.method != http.MethodPost || got.path != "/v1/workflows" || got.query != "" {
		t.Fatalf("request %s %s?%s", got.method, got.path, got.query)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	assertJSONEqual(t, got.body, input)
	if out.String() != "wf-1\thello\t1\ttrue\tt1\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestCreateMergesNameIntoInput(t *testing.T) {
	var requests []capturedRequest
	input := `{"nodes":[],"connections":[]}`
	raw := `{"workflow":{"id":"wf-1","name":"from-flag","version":1,"isActive":true,"updatedAt":"t1"}}`
	f, _, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
	if err := execute(t, f, "create", "--input", writeInput(t, input), "--name", "from-flag"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	assertJSONEqual(t, requests[0].body, `{"connections":[],"name":"from-flag","nodes":[]}`)
}

func TestCreateNameOnlySendsObject(t *testing.T) {
	var requests []capturedRequest
	raw := `{"workflow":{"id":"wf-1","name":"solo","version":1,"isActive":true,"updatedAt":"t1"}}`
	f, _, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
	if err := execute(t, f, "create", "--name", "solo"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	assertJSONEqual(t, requests[0].body, `{"name":"solo"}`)
}

func TestCreateNameConflictIsUsageError(t *testing.T) {
	f, _, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), factoryOptions{})
	err := execute(t, f, "create", "--input", writeInput(t, `{"name":"in-file","nodes":[],"connections":[]}`), "--name", "flag")
	if usageError(t, err) != "--name conflicts with name in --input" {
		t.Fatalf("error=%v", err)
	}
}

func TestCreateRejectsNonObjectInput(t *testing.T) {
	f, _, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), factoryOptions{})
	cases := []struct {
		raw  string
		want string
	}{
		{raw: `[]`, want: "--input must be a JSON object"},
		{raw: `"hello"`, want: "--input must be a JSON object"},
		{raw: `null`, want: "--input must be a JSON object"},
		{raw: `{`, want: "--input must be JSON"},
		{raw: ``, want: "--input must be JSON"},
	}
	for _, tc := range cases {
		err := execute(t, f, "create", "--input", writeInput(t, tc.raw))
		if usageError(t, err) != tc.want {
			t.Fatalf("raw=%q error=%v", tc.raw, err)
		}
	}
}

func TestCreateRequiresInputOrName(t *testing.T) {
	f, _, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), factoryOptions{})
	if usageError(t, execute(t, f, "create")) != "--input or --name is required" {
		t.Fatal("expected usage error")
	}
}

func TestCreateReadsStdin(t *testing.T) {
	var requests []capturedRequest
	input := `{"name":"stdin","nodes":[],"connections":[]}`
	raw := `{"workflow":{"id":"wf-1","name":"stdin","version":1,"isActive":true,"updatedAt":"t1"}}`
	f, _, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{stdin: strings.NewReader(input)})
	if err := execute(t, f, "create", "--input", "-"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertJSONEqual(t, requests[0].body, input)
}

func TestUpdateRequiresInput(t *testing.T) {
	f, _, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), factoryOptions{})
	if usageError(t, execute(t, f, "update", "wf-1")) != "--input is required" {
		t.Fatal("expected usage error")
	}
}

func TestUpdateSendsInputObject(t *testing.T) {
	var requests []capturedRequest
	input := `{"name":"renamed","nodes":[],"connections":[]}`
	raw := `{"workflow":{"id":"wf-1","name":"renamed","version":2,"isActive":true,"updatedAt":"t2"}}`
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "update", "wf-1", "--input", writeInput(t, input)); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := requests[0]
	if got.method != http.MethodPut || got.path != "/v1/workflows/wf-1" || got.query != "" {
		t.Fatalf("request %s %s?%s", got.method, got.path, got.query)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	assertJSONEqual(t, got.body, input)
	if out.String() != raw {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestDeleteConfirmsAndSendsNoBody(t *testing.T) {
	var requests []capturedRequest
	var prompt string
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, `{}`, &requests), ""), factoryOptions{confirm: func(got string) error {
		prompt = got
		return nil
	}})
	if err := execute(t, f, "delete", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete workflow wf-1?" {
		t.Fatalf("prompt=%q", prompt)
	}
	got := requests[0]
	if got.method != http.MethodDelete || got.path != "/v1/workflows/wf-1" || got.query != "" || got.body != "" {
		t.Fatalf("request %s %s?%s body=%q", got.method, got.path, got.query, got.body)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if out.String() != "{}" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestDeleteDeclinedReturnsCancelled(t *testing.T) {
	var requests []capturedRequest
	f, _, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, `{}`, &requests), ""), factoryOptions{confirm: func(string) error {
		return cli.ErrCancelled
	}})
	if err := execute(t, f, "delete", "wf-1"); !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("error=%v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("requests=%d", len(requests))
	}
}

func TestCustomMethodsSendEmptyObject(t *testing.T) {
	raw := `{"workflow":{"id":"wf-1","name":"hello","version":2,"isActive":true,"updatedAt":"t2"}}`
	cases := []struct {
		args []string
		path string
	}{
		{[]string{"publish", "wf-1"}, "/v1/workflows/wf-1:publish"},
		{[]string{"activate", "wf-1"}, "/v1/workflows/wf-1:activate"},
		{[]string{"deactivate", "wf-1"}, "/v1/workflows/wf-1:deactivate"},
	}
	for _, tc := range cases {
		t.Run(tc.args[0], func(t *testing.T) {
			var requests []capturedRequest
			f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
			if err := execute(t, f, tc.args...); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if len(requests) != 1 {
				t.Fatalf("requests=%d", len(requests))
			}
			got := requests[0]
			if got.method != http.MethodPost || got.path != tc.path || got.query != "" {
				t.Fatalf("request %s %s?%s", got.method, got.path, got.query)
			}
			if got.body != "{}" {
				t.Fatalf("body=%q", got.body)
			}
			assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
			if out.String() != "wf-1\thello\t2\ttrue\tt2\n" {
				t.Fatalf("stdout=%q", out.String())
			}
		})
	}
}

func TestHistorySendsPathAndRendersVersions(t *testing.T) {
	var requests []capturedRequest
	raw := `{"versions":[{"version":2,"id":"ver-2","isLive":true,"createdAt":"t2"},{"version":1,"id":"ver-1","isLive":false,"createdAt":"t1"}]}`
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{})
	if err := execute(t, f, "history", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := requests[0]
	if got.method != http.MethodGet || got.path != "/v1/workflows/wf-1/history" || got.query != "" || got.body != "" {
		t.Fatalf("request %s %s?%s body=%q", got.method, got.path, got.query, got.body)
	}
	assertApplicationHeaders(t, got.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if out.String() != "2\tver-2\ttrue\tt2\n1\tver-1\tfalse\tt1\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestHistoryJSONPreservesAPIResponse(t *testing.T) {
	var requests []capturedRequest
	raw := `{"versions":[{"version":1,"id":"ver-1","isLive":true,"createdAt":"t1","workflow":{"id":"wf-1"}}]}`
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, raw, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "history", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.String() != raw {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestEnumNameRendersUnknownAsDecimal(t *testing.T) {
	if got := enumName(executionStatusNames, 1); got != "ACTIVE" {
		t.Fatalf("known=%q", got)
	}
	if got := enumName(executionStatusNames, 99); got != "99" {
		t.Fatalf("unknown=%q", got)
	}
}

func TestPageHelpers(t *testing.T) {
	if pageSizeForLimit(0, 0) != defaultPageSize {
		t.Fatalf("default page size=%d", pageSizeForLimit(0, 0))
	}
	if pageSizeForLimit(200, maxTriggerPageSize) != maxTriggerPageSize {
		t.Fatalf("capped page size=%d", pageSizeForLimit(200, maxTriggerPageSize))
	}
	query := pageQuery(50, "1")
	if query.Encode() != "pageSize=50&pageToken=1" {
		t.Fatalf("query=%q", query.Encode())
	}
}

func TestRequireFlag(t *testing.T) {
	got, err := requireFlag("workflow", " wf-1 ")
	if err != nil || got != "wf-1" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	_, err = requireFlag("workflow", " ")
	if usageError(t, err) != "--workflow is required" {
		t.Fatalf("error=%v", err)
	}
}

func TestSharedColumns(t *testing.T) {
	if len(triggerColumns()) != 5 || len(executionColumns()) != 6 || len(eventTypeColumns()) != 3 {
		t.Fatal("column contract")
	}
	if enumName(triggerKindNames, 1) != "WEBHOOK" || enumName(executionSourceTypeNames, 3) != "SCHEDULE" || enumName(triggerDisabledReasonNames, 1) != "RUN_AS_UNAUTHORIZED" {
		t.Fatal("enum maps")
	}
}

func TestNewWorkflowCommandsOrder(t *testing.T) {
	f, _, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), factoryOptions{})
	cmds := newWorkflowCommands(f)
	want := []string{"list", "get", "create", "update", "delete", "publish", "activate", "deactivate", "history"}
	if len(cmds) != len(want) {
		t.Fatalf("commands=%d", len(cmds))
	}
	for i, name := range want {
		if cmds[i].Name() != name {
			t.Fatalf("command[%d]=%q want=%q", i, cmds[i].Name(), name)
		}
	}
}

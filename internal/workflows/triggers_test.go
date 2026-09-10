package workflows

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
)

func TestTriggersListSendsMethodPathQueryAndHeaders(t *testing.T) {
	body := `{"triggers":[{"id":"trg-1","kind":1,"enabled":true,"disabledReason":0,"updatedAt":"2026-01-02T03:04:05Z"}],"nextPageToken":"","totalCount":1}`
	var requests []capturedRequest
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{})
	if err := execute(t, f, "triggers", "list", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("requests=%d", len(requests))
	}
	req := requests[0]
	if req.method != http.MethodGet || req.host != "workflows.example.test" || req.path != "/v1/workflows/wf-1/triggers" {
		t.Fatalf("request %s %s %s", req.method, req.host, req.path)
	}
	query, _ := url.ParseQuery(req.query)
	if query.Get("pageSize") != "50" || query.Get("pageToken") != "" || query.Get("filter") != "" {
		t.Fatalf("query=%v", query)
	}
	if req.body != "" {
		t.Fatalf("body=%q", req.body)
	}
	assertApplicationHeaders(t, req.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if out.String() != "trg-1\tWEBHOOK\ttrue\tUNSPECIFIED\t2026-01-02T03:04:05Z\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersListPaginationFollowsPageTokens(t *testing.T) {
	var got []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		got = append(got, capturedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, headers: r.Header.Clone(), body: string(raw)})
		switch r.URL.Query().Get("pageToken") {
		case "":
			_, _ = w.Write([]byte(`{"triggers":[{"id":"trg-1","kind":2,"enabled":false,"disabledReason":1,"updatedAt":"t1"},{"id":"trg-2","kind":3,"enabled":true,"disabledReason":0,"updatedAt":"t2"}],"nextPageToken":"2"}`))
		case "2":
			_, _ = w.Write([]byte(`{"triggers":[{"id":"trg-3","kind":1,"enabled":true,"disabledReason":0,"updatedAt":"t3"}],"nextPageToken":""}`))
		default:
			t.Errorf("unexpected pageToken %q", r.URL.Query().Get("pageToken"))
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, "ns-1"), factoryOptions{})
	if err := execute(t, f, "triggers", "list", "--workflow", "wf-1", "--limit", "80"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("requests=%d", len(got))
	}
	page1, _ := url.ParseQuery(got[0].query)
	if page1.Get("pageSize") != "80" || page1.Get("pageToken") != "" {
		t.Fatalf("page1 query=%v", page1)
	}
	page2, _ := url.ParseQuery(got[1].query)
	if page2.Get("pageSize") != "80" || page2.Get("pageToken") != "2" {
		t.Fatalf("page2 query=%v", page2)
	}
	assertApplicationHeaders(t, got[0].headers, map[string]string{"Sl-Api-Key": testKey, "Sl-Namespace-Id": "ns-1", "User-Agent": "sl/test", "Accept": "application/json"})
	want := "trg-1\tSCHEDULE\tfalse\tRUN_AS_UNAUTHORIZED\tt1\ntrg-2\tEVENT\ttrue\tUNSPECIFIED\tt2\ntrg-3\tWEBHOOK\ttrue\tUNSPECIFIED\tt3\n"
	if out.String() != want {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersListLimitTruncatesFirstPage(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("pageSize") != "2" || r.URL.Query().Get("pageToken") != "" {
			t.Errorf("query=%v", r.URL.Query())
		}
		_, _ = w.Write([]byte(`{"triggers":[{"id":"trg-1","kind":1,"enabled":true,"disabledReason":0,"updatedAt":"t1"},{"id":"trg-2","kind":2,"enabled":true,"disabledReason":0,"updatedAt":"t2"}],"nextPageToken":"2"}`))
	}))
	t.Cleanup(srv.Close)
	f, out, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	if err := execute(t, f, "triggers", "list", "--workflow", "wf-1", "--limit", "2"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
	if out.String() != "trg-1\tWEBHOOK\ttrue\tUNSPECIFIED\tt1\ntrg-2\tSCHEDULE\ttrue\tUNSPECIFIED\tt2\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersListJSONUsesRawBody(t *testing.T) {
	body := `{"triggers":[{"id":"trg-1","kind":99,"enabled":true,"disabledReason":7,"updatedAt":"t1"}],"nextPageToken":"","totalCount":1}`
	var requests []capturedRequest
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "triggers", "list", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.String() != body {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersListUnknownKindIsDecimal(t *testing.T) {
	body := `{"triggers":[{"id":"trg-1","kind":99,"enabled":false,"disabledReason":8,"updatedAt":"t1"}]}`
	var requests []capturedRequest
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{})
	if err := execute(t, f, "triggers", "list", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.String() != "trg-1\t99\tfalse\t8\tt1\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersListRequiresWorkflow(t *testing.T) {
	f, out, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey}), factoryOptions{})
	err := execute(t, f, "triggers", "list")
	if usageError(t, err) != "--workflow is required" {
		t.Fatalf("err=%v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersListRejectsNegativeLimit(t *testing.T) {
	f, _, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey}), factoryOptions{})
	err := execute(t, f, "triggers", "list", "--workflow", "wf-1", "--limit", "-1")
	if usageError(t, err) != "--limit must be >= 0" {
		t.Fatalf("err=%v", err)
	}
}

func TestTriggersWorkflowFlagPrecedesSubcommand(t *testing.T) {
	body := `{"trigger":{"id":"trg-1","kind":1,"enabled":true,"disabledReason":0,"updatedAt":"t1"}}`
	var requests []capturedRequest
	f, _, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{})
	if err := execute(t, f, "triggers", "--workflow", "wf-1", "get", "trg-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if req := requests[0]; req.path != "/v1/workflows/wf-1/triggers/trg-1" {
		t.Fatalf("path=%q", req.path)
	}
}

func TestTriggersGetSendsMethodPathAndHeaders(t *testing.T) {
	body := `{"trigger":{"id":"trg-1","kind":2,"enabled":true,"disabledReason":0,"updatedAt":"2026-01-02T03:04:05Z"}}`
	var requests []capturedRequest
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{})
	if err := execute(t, f, "triggers", "get", "trg-1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	req := requests[0]
	if req.method != http.MethodGet || req.path != "/v1/workflows/wf-1/triggers/trg-1" || req.body != "" {
		t.Fatalf("request %#v", req)
	}
	assertApplicationHeaders(t, req.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if out.String() != "trg-1\tSCHEDULE\ttrue\tUNSPECIFIED\t2026-01-02T03:04:05Z\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersGetJSONUsesRawBody(t *testing.T) {
	body := `{"trigger":{"id":"trg-1","kind":1,"enabled":true},"webhookToken":"must-not-table"}`
	var requests []capturedRequest
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "triggers", "get", "trg-1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.String() != body {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersUpdateSendsRawBodyAndCanonicalMask(t *testing.T) {
	body := `{"trigger":{"id":"trg-1","kind":2,"enabled":false,"disabledReason":0,"updatedAt":"t1"}}`
	var requests []capturedRequest
	input := filepath.Join(t.TempDir(), "trigger.json")
	if err := os.WriteFile(input, []byte("{\n  \"enabled\": false,\n  \"schedule\": {\"cron\": \"30 8 * * 1-5\"}\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{})
	if err := execute(t, f, "triggers", "update", "trg-1", "--workflow", "wf-1", "--input", input, "--update-mask", " enabled , schedule.cron "); err != nil {
		t.Fatalf("execute: %v", err)
	}
	req := requests[0]
	if req.method != http.MethodPatch || req.path != "/v1/workflows/wf-1/triggers/trg-1" {
		t.Fatalf("request %s %s", req.method, req.path)
	}
	query, _ := url.ParseQuery(req.query)
	if query.Get("updateMask") != "enabled,schedule.cron" {
		t.Fatalf("updateMask=%q query=%v", query.Get("updateMask"), query)
	}
	if strings.Contains(req.body, `"trigger"`) {
		t.Fatalf("wrapped body=%s", req.body)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(req.body), &payload); err != nil {
		t.Fatalf("body json: %v", err)
	}
	if payload["enabled"] != false {
		t.Fatalf("body=%s", req.body)
	}
	assertApplicationHeaders(t, req.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if out.String() != "trg-1\tSCHEDULE\tfalse\tUNSPECIFIED\tt1\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersUpdateReadsStdinObject(t *testing.T) {
	body := `{"trigger":{"id":"trg-1","kind":1,"enabled":true,"disabledReason":0,"updatedAt":"t1"}}`
	var requests []capturedRequest
	f, _, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{stdin: strings.NewReader(`{"enabled":true}`)})
	if err := execute(t, f, "triggers", "update", "trg-1", "--workflow", "wf-1", "--input", "-", "--update-mask", "enabled"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if requests[0].body != `{"enabled":true}` {
		t.Fatalf("body=%s", requests[0].body)
	}
	query, _ := url.ParseQuery(requests[0].query)
	if query.Get("updateMask") != "enabled" {
		t.Fatalf("query=%v", query)
	}
}

func TestTriggersUpdateRejectsInvalidMasks(t *testing.T) {
	f, _, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey}), factoryOptions{stdin: strings.NewReader(`{"enabled":true}`)})
	cases := []struct {
		mask string
		want string
	}{
		{mask: "", want: "--update-mask is required"},
		{mask: "   ", want: "--update-mask is required"},
		{mask: "enabled,,schedule.cron", want: "--update-mask entries must be nonempty"},
		{mask: "enabled,enabled", want: "--update-mask has duplicate enabled"},
	}
	for _, tc := range cases {
		args := []string{"triggers", "update", "trg-1", "--workflow", "wf-1", "--input", "-"}
		if tc.mask != "" {
			args = append(args, "--update-mask", tc.mask)
		}
		err := execute(t, f, args...)
		if usageError(t, err) != tc.want {
			t.Fatalf("mask %q err=%v", tc.mask, err)
		}
	}
}

func TestTriggersUpdateRejectsNonObjectInput(t *testing.T) {
	f, _, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey}), factoryOptions{stdin: strings.NewReader(`["enabled"]`)})
	err := execute(t, f, "triggers", "update", "trg-1", "--workflow", "wf-1", "--input", "-", "--update-mask", "enabled")
	if usageError(t, err) != "--input must be a JSON object" {
		t.Fatalf("err=%v", err)
	}
}

func TestTriggersUpdateRequiresInput(t *testing.T) {
	f, _, _ := testFactory(t, apiclient.New(apiclient.Options{APIKey: testKey}), factoryOptions{})
	err := execute(t, f, "triggers", "update", "trg-1", "--workflow", "wf-1", "--update-mask", "enabled")
	if usageError(t, err) != "--input is required" {
		t.Fatalf("err=%v", err)
	}
}

func TestTriggersDeleteConfirmsThenDeletes(t *testing.T) {
	var requests []capturedRequest
	var prompt string
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, "", &requests), ""), factoryOptions{confirm: func(p string) error {
		prompt = p
		return nil
	}})
	if err := execute(t, f, "triggers", "delete", "trg-1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if prompt != "Delete trigger trg-1?" {
		t.Fatalf("prompt=%q", prompt)
	}
	req := requests[0]
	if req.method != http.MethodDelete || req.path != "/v1/workflows/wf-1/triggers/trg-1" || req.body != "" {
		t.Fatalf("request %#v", req)
	}
	assertApplicationHeaders(t, req.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if out.Len() != 0 {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersDeleteCancelledReturnsError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	f, _, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{confirm: func(string) error {
		return cli.ErrCancelled
	}})
	if err := execute(t, f, "triggers", "delete", "trg-1", "--workflow", "wf-1"); !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("error=%v", err)
	}
	if calls != 0 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestTriggersDeleteConfirmErrorSkipsRequest(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	f, _, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{confirm: func(string) error {
		return &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	}})
	err := execute(t, f, "triggers", "delete", "trg-1", "--workflow", "wf-1")
	if usageError(t, err) != "--yes required when stdin is not a TTY" {
		t.Fatalf("err=%v", err)
	}
	if calls != 0 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestTriggersRotateSendsEmptyObject(t *testing.T) {
	body := `{"trigger":{"id":"trg-1","kind":1,"enabled":true,"disabledReason":0,"updatedAt":"t1"},"webhookUrl":"https://workflows.example.test/v1/hooks/secret-token","webhookToken":"secret-token"}`
	var requests []capturedRequest
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{})
	if err := execute(t, f, "triggers", "rotate-webhook-token", "trg-1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	req := requests[0]
	if req.method != http.MethodPost || req.path != "/v1/workflows/wf-1/triggers/trg-1:rotateWebhookToken" {
		t.Fatalf("request %s %s", req.method, req.path)
	}
	if req.body != "{}" {
		t.Fatalf("body=%s", req.body)
	}
	assertApplicationHeaders(t, req.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json", "Content-Type": "application/json"})
	if out.String() != "trg-1\tWEBHOOK\ttrue\tUNSPECIFIED\tt1\n" {
		t.Fatalf("stdout=%q", out.String())
	}
	if strings.Contains(out.String(), "secret-token") {
		t.Fatalf("table leaked webhook token: %q", out.String())
	}
}

func TestTriggersRotateJSONIncludesWebhookToken(t *testing.T) {
	body := `{"trigger":{"id":"trg-1","kind":1,"enabled":true},"webhookToken":"secret-token"}`
	var requests []capturedRequest
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "triggers", "rotate-webhook-token", "trg-1", "--workflow", "wf-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.String() != body {
		t.Fatalf("stdout=%q", out.String())
	}
	if !strings.Contains(out.String(), "secret-token") {
		t.Fatalf("json missing token: %q", out.String())
	}
}

func TestEventTypesListSendsExactPath(t *testing.T) {
	body := `{"eventTypes":[{"eventType":"CAMPAIGN_DELIVERY_SENT","displayName":"Campaign delivery sent","description":"Fired when Campaigns applies MESSAGE_SENT."}]}`
	var requests []capturedRequest
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{})
	if err := execute(t, f, "event-types", "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	req := requests[0]
	if req.method != http.MethodGet || req.host != "workflows.example.test" || req.path != "/v1/triggerEventTypes" || req.body != "" || req.query != "" {
		t.Fatalf("request %#v", req)
	}
	assertApplicationHeaders(t, req.headers, map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"})
	if out.String() != "CAMPAIGN_DELIVERY_SENT\tCampaign delivery sent\tFired when Campaigns applies MESSAGE_SENT.\n" {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestEventTypesListJSONUsesRawBody(t *testing.T) {
	body := `{"eventTypes":[{"eventType":"MESSAGE_REPLIED","displayName":"Replied","description":"Inbound reply"}]}`
	var requests []capturedRequest
	f, out, _ := testFactory(t, testClient(t, captureServer(t, http.StatusOK, body, &requests), ""), factoryOptions{jsonMode: true})
	if err := execute(t, f, "event-types", "list"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out.String() != body {
		t.Fatalf("stdout=%q", out.String())
	}
}

func TestTriggersListCapsPageSizeAt100(t *testing.T) {
	var pageSize string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageSize = r.URL.Query().Get("pageSize")
		_, _ = w.Write([]byte(`{"triggers":[]}`))
	}))
	t.Cleanup(srv.Close)
	f, _, _ := testFactory(t, testClient(t, srv, ""), factoryOptions{})
	if err := execute(t, f, "triggers", "list", "--workflow", "wf-1", "--limit", "200"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if pageSize != "100" {
		t.Fatalf("pageSize=%q", pageSize)
	}
}

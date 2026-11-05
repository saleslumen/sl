package campaigns

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/output"
)

type tasksCapture struct {
	method, host, path, rawQuery string
	headers                      http.Header
}

func executeTasks(t *testing.T, f *cli.Factory, args ...string) error {
	t.Helper()
	cmd := newTasksCommand(f)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	return cmd.Execute()
}

func tasksFactory(t *testing.T, srv *httptest.Server, stdout io.Writer, jsonMode bool, namespace string) *cli.Factory {
	t.Helper()
	f := testFactory(nil, stdout)
	f.Confirm = func(string) error {
		t.Fatal("confirm used")
		return nil
	}
	if srv != nil {
		f.Client = func() (*apiclient.Client, error) {
			return testClient(t, srv, namespace), nil
		}
	}
	f.Printer = func() *output.Printer {
		return output.New(f.IO.Out, output.Options{JSON: jsonMode})
	}
	return f
}

func serveTasks(t *testing.T, handle func(http.ResponseWriter, *http.Request, *tasksCapture)) (*httptest.Server, *tasksCapture) {
	t.Helper()
	got := &tasksCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.host = r.Host
		got.path = r.URL.Path
		got.rawQuery = r.URL.RawQuery
		got.headers = r.Header.Clone()
		handle(w, r, got)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func writeTaskSSE(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func assertTasksGET(t *testing.T, got *tasksCapture, path, namespace string) {
	t.Helper()
	if got.method != http.MethodGet || got.host != "campaigns.example.test" || got.path != path || got.rawQuery != "" {
		t.Fatalf("request %s %s %s?%s", got.method, got.host, got.path, got.rawQuery)
	}
	want := map[string]string{"Sl-Api-Key": testKey, "User-Agent": "sl/test", "Accept": "application/json"}
	if namespace != "" {
		want["Sl-Namespace-Id"] = namespace
	}
	assertApplicationHeaders(t, got.headers, want)
	if got.headers.Get("Content-Type") != "" {
		t.Fatalf("Content-Type=%q", got.headers.Get("Content-Type"))
	}
	if got.headers.Get("sl-organization-id") != "" {
		t.Fatalf("sl-organization-id=%q", got.headers.Get("sl-organization-id"))
	}
}

func TestTasksHelpAndFlags(t *testing.T) {
	called := false
	f := tasksFactory(t, nil, io.Discard, false, "")
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	cmd := newTasksCommand(f)
	if cmd.Use != "tasks" || cmd.Short != "Manage campaign tasks" {
		t.Fatalf("use=%q short=%q", cmd.Use, cmd.Short)
	}
	if cmd.Flags().Lookup("wait") != nil {
		t.Fatal("parent has --wait")
	}
	shorts := map[string]string{}
	uses := map[string]string{}
	for _, child := range cmd.Commands() {
		shorts[child.Name()] = child.Short
		uses[child.Name()] = child.Use
		if child.Flags().Lookup("wait") != nil {
			t.Fatalf("%s has --wait", child.Name())
		}
		if child.Flags().Lookup("campaign") == nil {
			t.Fatalf("%s missing --campaign", child.Name())
		}
	}
	if uses["get"] != "get TASK_ID --campaign ID" || uses["stream"] != "stream TASK_ID --campaign ID" {
		t.Fatalf("use %#v", uses)
	}
	if shorts["get"] != "Get a campaign task" || shorts["stream"] != "Stream a campaign task" {
		t.Fatalf("shorts %#v", shorts)
	}
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}
	if called {
		t.Fatal("help resolved a lazy dependency")
	}
}

func TestTasksRequireCampaignAndID(t *testing.T) {
	f := tasksFactory(t, nil, io.Discard, false, "")
	var usage *cli.UsageError
	if err := executeTasks(t, f, "get", "t1"); !errors.As(err, &usage) || usage.Msg != "--campaign is required" {
		t.Fatalf("get campaign: %v", err)
	}
	if err := executeTasks(t, f, "stream", "t1"); !errors.As(err, &usage) || usage.Msg != "--campaign is required" {
		t.Fatalf("stream campaign: %v", err)
	}
	if err := executeTasks(t, f, "get", "--campaign", "c1"); !errors.As(err, &usage) || usage.Msg != "task id is required" {
		t.Fatalf("get id: %v", err)
	}
	if err := executeTasks(t, f, "stream", "--campaign", "c1"); !errors.As(err, &usage) || usage.Msg != "task id is required" {
		t.Fatalf("stream id: %v", err)
	}
}

func TestTasksGetOutputAndJSON(t *testing.T) {
	raw := []byte(`{"name":"campaigns/c1/tasks/t1","campaign":"campaigns/c1","type":"IMPORT_PEOPLE","state":"RUNNING","message":"working","result":{"ok":true},"create_time":"2026-01-01T00:00:00Z","update_time":"2026-01-02T03:04:05Z"}`)
	srv, got := serveTasks(t, func(w http.ResponseWriter, r *http.Request, _ *tasksCapture) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})
	var stdout bytes.Buffer
	f := tasksFactory(t, srv, &stdout, false, "ns-1")
	if err := executeTasks(t, f, "get", "t1", "--campaign", "c1"); err != nil {
		t.Fatalf("get: %v", err)
	}
	assertTasksGET(t, got, "/v1/campaigns/c1/tasks/t1", "ns-1")
	if stdout.String() != "t1\tc1\tIMPORT_PEOPLE\tRUNNING\tworking\t2026-01-02T03:04:05Z\n" {
		t.Fatalf("table=%q", stdout.String())
	}
	stdout.Reset()
	f = tasksFactory(t, srv, &stdout, true, "ns-1")
	if err := executeTasks(t, f, "get", "t1", "--campaign", "c1"); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), raw) {
		t.Fatalf("json=%q", stdout.Bytes())
	}
}

func TestTasksStreamConnectedStatusFinished(t *testing.T) {
	srv, got := serveTasks(t, func(w http.ResponseWriter, r *http.Request, _ *tasksCapture) {
		writeTaskSSE(w, ": heartbeat\n\nevent: connected\ndata: {\"task\":\"t1\"}\n\nevent: status\ndata: {\"state\":\"RUNNING\"}\n\nevent: finished\ndata: {\"state\":\"COMPLETED\"}\n\nevent: status\ndata: {\"late\":true}\n\n")
	})
	var stdout bytes.Buffer
	f := tasksFactory(t, srv, &stdout, false, "")
	if err := executeTasks(t, f, "stream", "t1", "--campaign", "c1"); err != nil {
		t.Fatalf("stream: %v", err)
	}
	assertTasksGET(t, got, "/v1/campaigns/c1/tasks/t1:stream", "")
	want := "{\"event\":\"connected\",\"data\":{\"task\":\"t1\"}}\n{\"event\":\"status\",\"data\":{\"state\":\"RUNNING\"}}\n{\"event\":\"finished\",\"data\":{\"state\":\"COMPLETED\"}}\n"
	if stdout.String() != want {
		t.Fatalf("stdout=%q want=%q", stdout.String(), want)
	}
}

func TestTasksStreamErrorTerminal(t *testing.T) {
	srv, got := serveTasks(t, func(w http.ResponseWriter, r *http.Request, _ *tasksCapture) {
		writeTaskSSE(w, "event: connected\ndata: {}\n\nevent: error\ndata: {\"message\":\"boom\"}\n\nevent: status\ndata: {\"late\":true}\n\n")
	})
	var stdout bytes.Buffer
	f := tasksFactory(t, srv, &stdout, false, "")
	if err := executeTasks(t, f, "stream", "t1", "--campaign", "c1"); err != nil {
		t.Fatalf("stream: %v", err)
	}
	assertTasksGET(t, got, "/v1/campaigns/c1/tasks/t1:stream", "")
	want := "{\"event\":\"connected\",\"data\":{}}\n{\"event\":\"error\",\"data\":{\"message\":\"boom\"}}\n"
	if stdout.String() != want {
		t.Fatalf("stdout=%q want=%q", stdout.String(), want)
	}
}

func TestTasksStreamCommentsIgnored(t *testing.T) {
	srv, _ := serveTasks(t, func(w http.ResponseWriter, r *http.Request, _ *tasksCapture) {
		writeTaskSSE(w, ": keep-alive\n\n: heartbeat\n\nevent: connected\ndata: {}\n\n: still-ignored\n\nevent: finished\ndata: {\"ok\":true}\n\n")
	})
	var stdout bytes.Buffer
	f := tasksFactory(t, srv, &stdout, false, "")
	if err := executeTasks(t, f, "stream", "t1", "--campaign", "c1"); err != nil {
		t.Fatalf("stream: %v", err)
	}
	got := stdout.String()
	if strings.Contains(got, "keep-alive") || strings.Contains(got, "heartbeat") || strings.Contains(got, "still-ignored") {
		t.Fatalf("comment leaked: %q", got)
	}
	want := "{\"event\":\"connected\",\"data\":{}}\n{\"event\":\"finished\",\"data\":{\"ok\":true}}\n"
	if got != want {
		t.Fatalf("stdout=%q want=%q", got, want)
	}
}

func TestTasksStreamMalformedPayload(t *testing.T) {
	srv, _ := serveTasks(t, func(w http.ResponseWriter, r *http.Request, _ *tasksCapture) {
		writeTaskSSE(w, "event: status\ndata: not-json {\n\nevent: finished\ndata: {\"ok\":true}\n\n")
	})
	var stdout bytes.Buffer
	f := tasksFactory(t, srv, &stdout, false, "")
	if err := executeTasks(t, f, "stream", "t1", "--campaign", "c1"); err != nil {
		t.Fatalf("stream: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines=%q", stdout.String())
	}
	var first, second taskStreamLine
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first json: %v line=%q", err, lines[0])
	}
	if first.Event != "status" || first.Data != "not-json {" {
		t.Fatalf("first %#v", first)
	}
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("second json: %v line=%q", err, lines[1])
	}
	if second.Event != "finished" {
		t.Fatalf("second %#v", second)
	}
	if stdout.String() != "{\"event\":\"status\",\"data\":\"not-json {\"}\n{\"event\":\"finished\",\"data\":{\"ok\":true}}\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestTasksStreamEOFBeforeTerminal(t *testing.T) {
	srv, _ := serveTasks(t, func(w http.ResponseWriter, r *http.Request, _ *tasksCapture) {
		writeTaskSSE(w, "event: connected\ndata: {}\n\nevent: status\ndata: {\"state\":\"RUNNING\"}\n\n")
	})
	var stdout bytes.Buffer
	f := tasksFactory(t, srv, &stdout, false, "")
	err := executeTasks(t, f, "stream", "t1", "--campaign", "c1")
	if err == nil || err.Error() != "campaigns: task stream ended before a terminal event" {
		t.Fatalf("err: %v", err)
	}
	want := "{\"event\":\"connected\",\"data\":{}}\n{\"event\":\"status\",\"data\":{\"state\":\"RUNNING\"}}\n"
	if stdout.String() != want {
		t.Fatalf("stdout=%q want=%q", stdout.String(), want)
	}
}

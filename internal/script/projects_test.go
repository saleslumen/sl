package script

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/cli"
)

func TestProjectsListDefaultPageSizeAndTable(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/projects" || rec.Query.Get("pageSize") != "50" || rec.Query.Get("pageToken") != "" {
			t.Errorf("request %s %s %v", rec.Method, rec.Path, rec.Query)
		}
		assertScriptHeaders(t, rec.Header, "")
		_, _ = w.Write([]byte(`{"projects":[{"scriptId":"p1","title":"Alpha","lifecycleState":"ACTIVE","updateTime":"2026-01-01T00:00:00Z","archiveTime":""}]}`))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "projects", "list")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != "p1\tAlpha\tACTIVE\t2026-01-01T00:00:00Z\t\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func projectListPage(start, n int, next string) string {
	items := make([]string, n)
	for i := range n {
		id := start + i
		items[i] = `{"scriptId":"p` + strconv.Itoa(id) + `","title":"T` + strconv.Itoa(id) + `","lifecycleState":"ACTIVE","updateTime":"t","archiveTime":""}`
	}
	body := `{"projects":[` + strings.Join(items, ",") + `]`
	if next != "" {
		body += `,"nextPageToken":"` + next + `"`
	}
	return body + `}`
}

func TestProjectsListFollowsPagesAndCapsPageSize(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		switch rec.Query.Get("pageToken") {
		case "":
			if rec.Query.Get("pageSize") != "100" {
				t.Errorf("first pageSize=%q", rec.Query.Get("pageSize"))
			}
			_, _ = w.Write([]byte(projectListPage(1, 100, "n2")))
		case "n2":
			if rec.Query.Get("pageSize") != "50" {
				t.Errorf("second pageSize=%q", rec.Query.Get("pageSize"))
			}
			_, _ = w.Write([]byte(projectListPage(101, 50, "")))
		default:
			t.Errorf("token %q", rec.Query.Get("pageToken"))
		}
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "projects", "list", "--limit", "150")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if len(*recs) != 2 {
		t.Fatalf("requests=%d", len(*recs))
	}
	lines := strings.Split(strings.TrimSuffix(got.stdout, "\n"), "\n")
	if len(lines) != 150 || !strings.HasPrefix(lines[0], "p1\t") || !strings.HasPrefix(lines[149], "p150\t") {
		t.Fatalf("lines=%d first=%q last=%q", len(lines), lines[0], lines[len(lines)-1])
	}
}

func TestProjectsListJSONUsesCollectedItems(t *testing.T) {
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		_, _ = w.Write([]byte(`{"projects":[{"scriptId":"p1","title":"A","lifecycleState":"ACTIVE","updateTime":"t","archiveTime":""}],"nextPageToken":""}`))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "--json", "script", "projects", "list")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	payload := decodeJSON(t, []byte(got.stdout))
	projects, ok := payload["projects"].([]any)
	if !ok || len(projects) != 1 {
		t.Fatalf("json=%s", got.stdout)
	}
}

func TestProjectsGetSendsPathAndHeaders(t *testing.T) {
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodGet || rec.Path != "/v1/projects/proj-1" || rec.Host != "script.example.test" {
			t.Errorf("request %s %s host=%s", rec.Method, rec.Path, rec.Host)
		}
		assertScriptHeaders(t, rec.Header, "ns-1")
		if rec.Header.Get("Content-Type") != "" {
			t.Errorf("content-type=%q", rec.Header.Get("Content-Type"))
		}
		_, _ = w.Write([]byte(`{"scriptId":"proj-1","title":"Beta","lifecycleState":"ACTIVE","updateTime":"2026-02-01T00:00:00Z","archiveTime":""}`))
	})
	got := execScript(t, testClient(t, srv, "ns-1"), nil, false, nil, "script", "projects", "get", "proj-1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if got.stdout != "proj-1\tBeta\tACTIVE\t2026-02-01T00:00:00Z\t\n" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestProjectsGetAPIError(t *testing.T) {
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"project not found","details":[]}`))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "projects", "get", "missing")
	var api *apiclient.Error
	if !errors.As(got.err, &api) || api.Code != "NOT_FOUND" || api.Message != "project not found" {
		t.Fatalf("err=%v", got.err)
	}
	if api.Product != "script" || api.Method != http.MethodGet || api.Path != "/v1/projects/missing" {
		t.Fatalf("api=%#v", api)
	}
}

func TestProjectsCreateTitleFlag(t *testing.T) {
	var body map[string]any
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodPost || rec.Path != "/v1/projects" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		if rec.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type=%q", rec.Header.Get("Content-Type"))
		}
		body = decodeJSON(t, rec.Body)
		_, _ = w.Write([]byte(`{"scriptId":"p-new","title":"Hello","lifecycleState":"ACTIVE","updateTime":"t","archiveTime":""}`))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "projects", "create", "--title", "Hello")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if body["title"] != "Hello" || len(body) != 1 {
		t.Fatalf("body=%#v", body)
	}
	if !strings.HasPrefix(got.stdout, "p-new\tHello\t") {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestProjectsCreateAddsTitleWhenAbsent(t *testing.T) {
	var body map[string]any
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		body = decodeJSON(t, rec.Body)
		_, _ = w.Write([]byte(`{"scriptId":"p-new","title":"Hello","lifecycleState":"ACTIVE","updateTime":"t","archiveTime":""}`))
	})
	input := writeInputFile(t, `{"parentId":"ns-1","extra":true}`)
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "projects", "create", "--title", "Hello", "--input", input)
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if body["title"] != "Hello" || body["parentId"] != "ns-1" || body["extra"] != true {
		t.Fatalf("body=%#v", body)
	}
}

func TestProjectsCreateRejectsDuplicateTitle(t *testing.T) {
	hits := 0
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		hits++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	input := writeInputFile(t, `{"title":"A"}`)
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "projects", "create", "--title", "B", "--input", input)
	requireUsage(t, got.err, "title")
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestProjectsCreateReadsStdinInput(t *testing.T) {
	var body map[string]any
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		body = decodeJSON(t, rec.Body)
		_, _ = w.Write([]byte(`{"scriptId":"p-new","title":"FromInput","lifecycleState":"ACTIVE","updateTime":"t","archiveTime":""}`))
	})
	got := execScript(t, testClient(t, srv, ""), strings.NewReader(`{"title":"FromInput"}`), false, nil, "script", "projects", "create", "--input", "-")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if body["title"] != "FromInput" {
		t.Fatalf("body=%#v", body)
	}
}

func TestProjectsUpdateRequiresInputAndPassesThrough(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "projects", "update", "p1")
	requireUsage(t, got.err, "input")
	var body map[string]any
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodPut || rec.Path != "/v1/projects/p1" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		body = decodeJSON(t, rec.Body)
		_, _ = w.Write([]byte(`{"scriptId":"p1","title":"Renamed","lifecycleState":"ACTIVE","updateTime":"t","archiveTime":""}`))
	})
	input := writeInputFile(t, `{"title":"Renamed","keep":1}`)
	got = execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "projects", "update", "p1", "--input", input)
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if body["title"] != "Renamed" || body["keep"] != float64(1) {
		t.Fatalf("body=%#v", body)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestProjectsDeleteCallsConfirm(t *testing.T) {
	var prompt string
	srv, recs := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		if rec.Method != http.MethodDelete || rec.Path != "/v1/projects/p1" {
			t.Errorf("request %s %s", rec.Method, rec.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, func(p string) error {
		prompt = p
		return nil
	}, "script", "projects", "delete", "p1")
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if prompt != "Delete project p1?" {
		t.Fatalf("prompt=%q", prompt)
	}
	if got.stdout != "" {
		t.Fatalf("stdout=%q", got.stdout)
	}
	if len(*recs) != 1 {
		t.Fatalf("requests=%d", len(*recs))
	}
}

func TestProjectsDeleteCancelled(t *testing.T) {
	hits := 0
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		hits++
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, func(string) error {
		return cli.ErrCancelled
	}, "script", "projects", "delete", "p1")
	if !errors.Is(got.err, cli.ErrCancelled) {
		t.Fatalf("err=%v", got.err)
	}
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestProjectsDeleteConfirmError(t *testing.T) {
	hits := 0
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		hits++
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, func(string) error {
		return &cli.UsageError{Msg: "--yes required when stdin is not a TTY"}
	}, "script", "projects", "delete", "p1")
	requireUsage(t, got.err, "--yes")
	if hits != 0 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestProjectsMissingIDIsUsageError(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), nil, false, nil, "script", "projects", "get")
	requireUsage(t, got.err, "")
}

func TestProjectsCreateInvalidInputJSON(t *testing.T) {
	got := execScript(t, apiclient.New(apiclient.Options{APIKey: testKey, UserAgent: "sl/test"}), strings.NewReader("nope"), false, nil, "script", "projects", "create", "--input", "-")
	requireUsage(t, got.err, "JSON")
}

func TestProjectsUpdatePreservesRawJSON(t *testing.T) {
	rawIn := `{"title":"Renamed","nested":{"ok":true}}`
	var gotBody json.RawMessage
	srv, _ := startScriptServer(t, func(w http.ResponseWriter, r *http.Request, rec recordedRequest) {
		gotBody = append(json.RawMessage(nil), rec.Body...)
		_, _ = w.Write([]byte(`{"scriptId":"p1","title":"Renamed","lifecycleState":"ACTIVE","updateTime":"t","archiveTime":""}`))
	})
	got := execScript(t, testClient(t, srv, ""), nil, false, nil, "script", "projects", "update", "p1", "--input", writeInputFile(t, rawIn))
	if got.err != nil {
		t.Fatalf("execute: %v", got.err)
	}
	if string(gotBody) != rawIn {
		t.Fatalf("body=%s", gotBody)
	}
}

package apiclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStreamLinesSkipsBlankLinesInOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "{\"step\":1}\r\n\r\n  \n{\"step\":2}\n\n{\"step\":3}\r\n")
	}))
	t.Cleanup(srv.Close)
	opts := testOptions(t, srv)
	capture := &deadlineCapture{next: opts.HTTPClient.Transport}
	opts.HTTPClient.Transport = capture
	var logged string
	opts.Logger = func(line string) { logged = line }
	var lines []string
	err := New(opts).StreamLines(context.Background(), Request{Product: "emails", Method: http.MethodPost, Path: "/v1/tools/verify"}, func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})
	if err != nil {
		t.Fatalf("StreamLines: %v", err)
	}
	want := []string{`{"step":1}`, `{"step":2}`, `{"step":3}`}
	if len(lines) != len(want) {
		t.Fatalf("lines=%v", lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line[%d]=%q want=%q", i, lines[i], want[i])
		}
	}
	if capture.has {
		t.Fatalf("stream deadline %s", capture.at)
	}
	if logged != "POST emails.example.test /v1/tools/verify → 200" {
		t.Fatalf("log=%q", logged)
	}
}

func TestStreamLinesDeliversFinalLineWithoutNewline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"final":true}`)
	}))
	t.Cleanup(srv.Close)
	var lines []string
	err := New(testOptions(t, srv)).StreamLines(context.Background(), Request{Product: "emails", Method: http.MethodPost, Path: "/v1/tools/verify"}, func(line []byte) error {
		lines = append(lines, string(line))
		return nil
	})
	if err != nil {
		t.Fatalf("StreamLines: %v", err)
	}
	if len(lines) != 1 || lines[0] != `{"final":true}` {
		t.Fatalf("lines=%v", lines)
	}
}

func TestStreamLinesReturnsContextError(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("flusher")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "{\"ready\":true}\n")
		flusher.Flush()
		close(started)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- New(testOptions(t, srv)).StreamLines(ctx, Request{Product: "emails", Method: http.MethodPost, Path: "/v1/tools/verify"}, func(line []byte) error {
			return nil
		})
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not stop")
	}
}

func TestStreamLinesNormalizesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":"INVALID_ARGUMENT","message":"invalid mailbox"}}`)
	}))
	t.Cleanup(srv.Close)
	err := New(testOptions(t, srv)).StreamLines(context.Background(), Request{Product: "emails", Method: http.MethodPost, Path: "/v1/tools/verify"}, func(line []byte) error {
		return nil
	})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type=%T value=%v", err, err)
	}
	if apiErr.Code != "INVALID_ARGUMENT" || apiErr.Message != "invalid mailbox" || apiErr.Status != http.StatusBadRequest {
		t.Fatalf("error=%+v", apiErr)
	}
}

func TestStreamLinesStopsOnCallbackError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{\"step\":1}\n{\"step\":2}\n")
	}))
	t.Cleanup(srv.Close)
	callbackErr := errors.New("stop")
	calls := 0
	err := New(testOptions(t, srv)).StreamLines(context.Background(), Request{Product: "emails", Method: http.MethodPost, Path: "/v1/tools/verify"}, func(line []byte) error {
		calls++
		return callbackErr
	})
	if !errors.Is(err, callbackErr) {
		t.Fatalf("err=%v", err)
	}
	if calls != 1 {
		t.Fatalf("callbacks=%d", calls)
	}
}

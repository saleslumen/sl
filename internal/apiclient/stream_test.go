package apiclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamParsesEventsDataMultilineAndComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, ": heartbeat\n\n")
		_, _ = io.WriteString(w, "event: status\ndata: {\"state\":\"RUNNING\"}\n\n")
		_, _ = io.WriteString(w, "data: line-one\ndata: line-two\n\n")
		_, _ = io.WriteString(w, "event: finished\ndata: {\"ok\":true}\n")
		flusher.Flush()
	}))
	t.Cleanup(srv.Close)
	opts := testOptions(t, srv)
	capture := &deadlineCapture{next: opts.HTTPClient.Transport}
	opts.HTTPClient.Transport = capture
	var events []string
	var payloads []string
	err := New(opts).Stream(context.Background(), Request{Product: "campaigns", Method: http.MethodGet, Path: "/v1/campaigns/c1/tasks/t1:stream"}, func(name string, data []byte) error {
		events = append(events, name)
		payloads = append(payloads, string(data))
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(events) != 3 || events[0] != "status" || events[1] != "message" || events[2] != "finished" {
		t.Fatalf("events: %#v", events)
	}
	if payloads[0] != `{"state":"RUNNING"}` || payloads[1] != "line-one\nline-two" || payloads[2] != `{"ok":true}` {
		t.Fatalf("payloads: %#v", payloads)
	}
	if capture.has {
		t.Fatalf("stream deadline %s", capture.at)
	}
}

func TestStreamStopsOnCallbackError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "event: status\ndata: one\n\nevent: status\ndata: two\n\n")
	}))
	t.Cleanup(srv.Close)
	callbackErr := errors.New("stop")
	var n int
	err := New(testOptions(t, srv)).Stream(context.Background(), Request{Product: "campaigns", Method: http.MethodGet, Path: "/v1/tasks/t1:stream"}, func(name string, data []byte) error {
		n++
		return callbackErr
	})
	if !errors.Is(err, callbackErr) {
		t.Fatalf("err: %v", err)
	}
	if n != 1 {
		t.Fatalf("callbacks: %d", n)
	}
}

func TestStreamHonorsContextCancel(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("flusher")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "event: connected\ndata: {}\n\n")
		flusher.Flush()
		close(started)
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- New(testOptions(t, srv)).Stream(ctx, Request{Product: "campaigns", Method: http.MethodGet, Path: "/v1/tasks/t1:stream"}, func(name string, data []byte) error {
			return nil
		})
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not stop")
	}
}

func TestStreamNormalizesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":5,"message":"missing task"}`))
	}))
	t.Cleanup(srv.Close)
	err := New(testOptions(t, srv)).Stream(context.Background(), Request{Product: "campaigns", Method: http.MethodGet, Path: "/v1/tasks/t1:stream"}, func(name string, data []byte) error {
		return nil
	})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type: %v", err)
	}
	if apiErr.Code != "NOT_FOUND" || apiErr.Message != "missing task" {
		t.Fatalf("code=%q message=%q", apiErr.Code, apiErr.Message)
	}
}

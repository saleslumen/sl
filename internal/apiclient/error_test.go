package apiclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoNormalizesFourErrorShapes(t *testing.T) {
	cases := []struct {
		name          string
		status        int
		body          string
		code, message string
		wantDetails   bool
	}{
		{name: "google-rpc-numeric", status: http.StatusNotFound, body: `{"code":5,"message":"missing campaign","details":[{"reason":"NOT_FOUND"}]}`, code: "NOT_FOUND", message: "missing campaign", wantDetails: true},
		{name: "nested-error-object", status: http.StatusBadRequest, body: `{"error":{"code":"INVALID_ARGUMENT","message":"bad field"}}`, code: "INVALID_ARGUMENT", message: "bad field"},
		{name: "error-string", status: http.StatusBadRequest, body: `{"error":"invalid config"}`, code: http.StatusText(http.StatusBadRequest), message: "invalid config"},
		{name: "plain-text", status: http.StatusForbidden, body: `namespace denied`, code: http.StatusText(http.StatusForbidden), message: "namespace denied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			_, err := New(testOptions(t, srv)).Do(context.Background(), Request{Product: "emails", Method: http.MethodGet, Path: "/v1/labels"})
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("error type: %v", err)
			}
			if apiErr.Status != tc.status || apiErr.Code != tc.code || apiErr.Message != tc.message {
				t.Fatalf("got status=%d code=%q message=%q", apiErr.Status, apiErr.Code, apiErr.Message)
			}
			if apiErr.Product != "emails" || apiErr.Method != http.MethodGet || apiErr.Path != "/v1/labels" {
				t.Fatalf("identity %s %s %s", apiErr.Product, apiErr.Method, apiErr.Path)
			}
			if tc.wantDetails && len(apiErr.Details) == 0 {
				t.Fatal("missing details")
			}
			if !tc.wantDetails && len(apiErr.Details) != 0 {
				t.Fatalf("details: %s", apiErr.Details)
			}
			want := "emails GET /v1/labels: " + tc.code + ": " + tc.message
			if apiErr.Error() != want {
				t.Fatalf("Error(): %q", apiErr.Error())
			}
		})
	}
}

func TestDoMapsNumericAndStringGRPCCodes(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
	}{
		{name: "numeric", body: `{"code":7,"message":"no"}`, code: "PERMISSION_DENIED"},
		{name: "string-name", body: `{"code":"UNAUTHENTICATED","message":"no"}`, code: "UNAUTHENTICATED"},
		{name: "numeric-string", body: `{"code":"16","message":"no"}`, code: "UNAUTHENTICATED"},
		{name: "nested-numeric", body: `{"error":{"code":3,"message":"bad"}}`, code: "INVALID_ARGUMENT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)
			_, err := New(testOptions(t, srv)).Do(context.Background(), Request{Product: "campaigns", Method: http.MethodGet, Path: "/v1/campaigns"})
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("error type: %v", err)
			}
			if apiErr.Code != tc.code {
				t.Fatalf("code %q, want %q", apiErr.Code, tc.code)
			}
		})
	}
}

func TestDoEmptyErrorBodyUsesStatusText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	_, err := New(testOptions(t, srv)).Do(context.Background(), Request{Product: "resources", Method: http.MethodGet, Path: "/v1/namespaces"})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type: %v", err)
	}
	if apiErr.Code != http.StatusText(http.StatusUnauthorized) || apiErr.Message != http.StatusText(http.StatusUnauthorized) {
		t.Fatalf("code=%q message=%q", apiErr.Code, apiErr.Message)
	}
}

func TestDoSanitizesErrorAndCapturesRequestID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-123")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"BAD\nCODE","message":"bad\tfield\u0000value"}}`))
	}))
	t.Cleanup(srv.Close)
	_, err := New(testOptions(t, srv)).Do(context.Background(), Request{Product: "campaigns", Method: http.MethodPost, Path: "/v1/campaigns"})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type: %v", err)
	}
	if apiErr.Code != "BAD CODE" || apiErr.Message != "bad field value" || apiErr.RequestID != "req-123" {
		t.Fatalf("code=%q message=%q requestID=%q", apiErr.Code, apiErr.Message, apiErr.RequestID)
	}
}

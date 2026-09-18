package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/credentials"
)

const (
	testKey    = "sl_key_123e4567-e89b-12d3-a456-426614174000_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	testPrefix = "sl_key_123e4567-e89b-12d3-a456-426614174000"
)

func storeOf(store credentials.Store) func() (credentials.Store, error) {
	return func() (credentials.Store, error) { return store, nil }
}

func TestLoginWithTokenWritesProfile(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams:      Streams{In: strings.NewReader(testKey + "\n"), Out: stdout, Err: stderr},
		Store:        storeOf(store),
		Profile:      func() string { return "acme" },
		Organization: func() string { return "org-acme" },
	})
	cmd.SetArgs([]string{"login", "--with-token"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}
	if stdout.String() != "Logged in to profile acme\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if strings.Contains(stdout.String(), testKey) || strings.Contains(stderr.String(), testKey) {
		t.Fatal("command output contained the api key")
	}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fields := file.Fields("acme")
	if fields.AuthMode != credentials.AuthModeAPIKey {
		t.Fatalf("auth_mode=%q", fields.AuthMode)
	}
	if fields.APIKey != testKey {
		t.Fatal("profile api key was not recorded")
	}
	if fields.AccessToken != "" || fields.RefreshToken != "" || fields.OAuthUserID != "" {
		t.Fatal("oauth credentials remained after api-key login")
	}
	if fields.OrganizationID != "org-acme" {
		t.Fatalf("organization_id=%q", fields.OrganizationID)
	}
}

func TestLoginRejectsInvalidKey(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{In: strings.NewReader("not-a-key\n"), Out: stdout, Err: stderr},
		Store:   storeOf(store),
		Profile: func() string { return credentials.DefaultProfile },
	})
	cmd.SetArgs([]string{"login", "--with-token"})
	err := cmd.Execute()
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err=%v", err)
	}
	if usage.Msg != "api key must look like sl_key_<uuid>_<secret>" {
		t.Fatalf("message=%q", usage.Msg)
	}
	if strings.Contains(err.Error(), "not-a-key") {
		t.Fatal("error echoed the submitted value")
	}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file.Has(credentials.DefaultProfile) {
		t.Fatal("invalid login wrote a profile")
	}
}

func TestLoginHiddenPrompt(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	stdout := &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams:    Streams{In: strings.NewReader(""), Out: stdout, Err: io.Discard, IsStdinTTY: true},
		Store:      storeOf(store),
		Profile:    func() string { return credentials.DefaultProfile },
		ReadSecret: func() (string, error) { return testKey, nil },
	})
	cmd.SetArgs([]string{"login", "--api-key"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}
	if strings.Contains(stdout.String(), testKey) {
		t.Fatal("command output contained the api key")
	}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file.Fields(credentials.DefaultProfile).APIKey != testKey {
		t.Fatal("prompted key was not stored")
	}
}

func TestLoginRequiresTokenWithoutTTY(t *testing.T) {
	t.Parallel()
	called := false
	cmd := NewCommand(Deps{
		Streams: Streams{In: strings.NewReader(""), Out: io.Discard, Err: io.Discard},
		Store:   storeOf(credentials.Store{Dir: t.TempDir()}),
		Profile: func() string { return credentials.DefaultProfile },
		ReadSecret: func() (string, error) {
			called = true
			return testKey, nil
		},
	})
	cmd.SetArgs([]string{"login", "--api-key"})
	err := cmd.Execute()
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err=%v", err)
	}
	if called {
		t.Fatal("ReadSecret ran before TTY check")
	}
}

func TestLoginPreservesUnknownKeys(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	file := mustFile(t)
	file.Set(credentials.DefaultProfile, credentials.KeyAPIKey, "sl_key_oldvaluexxxx")
	file.Set(credentials.DefaultProfile, "region", "us")
	if err := store.Save(file); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cmd := NewCommand(Deps{
		Streams: Streams{In: strings.NewReader(testKey + "\n"), Out: io.Discard, Err: io.Discard},
		Store:   storeOf(store),
		Profile: func() string { return credentials.DefaultProfile },
	})
	cmd.SetArgs([]string{"login", "--with-token"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, ok := reloaded.Extra(credentials.DefaultProfile, "region"); !ok || got != "us" {
		t.Fatal("unknown key lost on login")
	}
}

func TestStatusPrintsPrefixNotKey(t *testing.T) {
	t.Parallel()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{Out: stdout, Err: stderr},
		Resolve: func() (credentials.Resolved, error) {
			return credentials.Resolved{Profile: "work", AuthMode: credentials.AuthModeAPIKey, APIKey: testKey, OrganizationID: "org-work", NamespaceID: "ns_1", APIDomain: "example.com"}, nil
		},
	})
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status: %v", err)
	}
	want := "profile: work\nauth_mode: api_key\norganization_id: org-work\nnamespace_id: ns_1\napi_domain: example.com\napi_key: " + testPrefix + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if strings.Contains(stdout.String(), testKey) || strings.Contains(stderr.String(), testKey) {
		t.Fatal("status printed the api key")
	}
}

func TestStatusProductProbes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		product string
		path    string
		query   url.Values
	}{
		{product: "campaigns", path: "/v1/campaigns", query: url.Values{"page_size": []string{"1"}}},
		{product: "emails", path: "/v1/accounts", query: url.Values{}},
		{product: "workflows", path: "/v1/workflows", query: url.Values{"pageSize": []string{"1"}}},
		{product: "script", path: "/v1/projects", query: url.Values{"pageSize": []string{"1"}}},
	}
	for _, tc := range cases {
		t.Run(tc.product, func(t *testing.T) {
			t.Parallel()
			var gotProduct, gotMethod, gotPath string
			var gotQuery url.Values
			stdout := &bytes.Buffer{}
			cmd := NewCommand(Deps{
				Streams: Streams{Out: stdout, Err: io.Discard},
				Resolve: func() (credentials.Resolved, error) {
					return credentials.Resolved{Profile: "default", AuthMode: credentials.AuthModeAPIKey, APIKey: testKey, APIDomain: credentials.DefaultAPIDomain}, nil
				},
				Probe: func(ctx context.Context, product, method, path string, query url.Values) error {
					gotProduct, gotMethod, gotPath, gotQuery = product, method, path, query
					return nil
				},
			})
			cmd.SetArgs([]string{"status", "--product", tc.product})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("status: %v", err)
			}
			if gotProduct != tc.product || gotMethod != "GET" || gotPath != tc.path {
				t.Fatalf("probe %s %s %s", gotProduct, gotMethod, gotPath)
			}
			if gotQuery.Encode() != tc.query.Encode() {
				t.Fatalf("query=%q want=%q", gotQuery.Encode(), tc.query.Encode())
			}
			if !strings.Contains(stdout.String(), "product "+tc.product+": ok\n") {
				t.Fatalf("stdout=%q", stdout.String())
			}
			if strings.Contains(stdout.String(), testKey) {
				t.Fatal("probe status printed the api key")
			}
		})
	}
}

func TestStatusUnknownProduct(t *testing.T) {
	t.Parallel()
	called := false
	cmd := NewCommand(Deps{
		Streams: Streams{Out: io.Discard, Err: io.Discard},
		Resolve: func() (credentials.Resolved, error) {
			return credentials.Resolved{Profile: "default", AuthMode: credentials.AuthModeAPIKey, APIKey: testKey, APIDomain: credentials.DefaultAPIDomain}, nil
		},
		Probe: func(ctx context.Context, product, method, path string, query url.Values) error {
			called = true
			return nil
		},
	})
	cmd.SetArgs([]string{"status", "--product", "admin"})
	err := cmd.Execute()
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("err=%v", err)
	}
	if called {
		t.Fatal("probe called for unknown product")
	}
}

func TestStatusProbeError(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{Out: stdout, Err: io.Discard},
		Resolve: func() (credentials.Resolved, error) {
			return credentials.Resolved{Profile: "default", AuthMode: credentials.AuthModeAPIKey, APIKey: testKey, APIDomain: credentials.DefaultAPIDomain}, nil
		},
		Probe: func(ctx context.Context, product, method, path string, query url.Values) error {
			return errors.New("PERMISSION_DENIED: missing grant")
		},
	})
	cmd.SetArgs([]string{"status", "--product", "campaigns"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected probe error")
	}
	if !strings.Contains(err.Error(), "PERMISSION_DENIED: missing grant") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(stdout.String(), "PERMISSION_DENIED") || strings.Contains(stdout.String(), "product campaigns:") {
		t.Fatalf("probe error leaked to stdout=%q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "profile: default\n") {
		t.Fatalf("stdout missing status metadata: %q", stdout.String())
	}
}

func TestLogoutRemovesProfile(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	file := mustFile(t)
	file.Set("acme", credentials.KeyAPIKey, testKey)
	file.Set("acme", "region", "us")
	file.Set(credentials.DefaultProfile, credentials.KeyAPIKey, testKey)
	if err := store.Save(file); err != nil {
		t.Fatalf("Save: %v", err)
	}
	stdout := &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{Out: stdout, Err: io.Discard},
		Store:   storeOf(store),
		Profile: func() string { return "acme" },
	})
	cmd.SetArgs([]string{"logout"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if stdout.String() != "Logged out of profile acme\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if reloaded.Has("acme") {
		t.Fatal("profile still present")
	}
	if !reloaded.Has(credentials.DefaultProfile) {
		t.Fatal("other profile was removed")
	}
}

func TestLogoutMissingProfile(t *testing.T) {
	t.Parallel()
	cmd := NewCommand(Deps{
		Streams: Streams{Out: io.Discard, Err: io.Discard},
		Store:   storeOf(credentials.Store{Dir: t.TempDir()}),
		Profile: func() string { return credentials.DefaultProfile },
	})
	cmd.SetArgs([]string{"logout"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing profile error")
	}
}

func TestLoginStoreResolverFailure(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	resolverErr := errors.New("store unavailable")
	stdout := &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{In: strings.NewReader(testKey + "\n"), Out: stdout, Err: io.Discard},
		Store:   func() (credentials.Store, error) { return credentials.Store{}, resolverErr },
		Profile: func() string { return credentials.DefaultProfile },
	})
	cmd.SetArgs([]string{"login", "--with-token"})
	err := cmd.Execute()
	if !errors.Is(err, resolverErr) {
		t.Fatalf("err=%v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if _, statErr := os.Stat(store.Path()); !os.IsNotExist(statErr) {
		t.Fatal("resolver failure wrote credentials")
	}
}

func mustFile(t *testing.T) *credentials.File {
	t.Helper()
	file, err := credentials.Load("/nonexistent/credentials-test-missing")
	if err != nil {
		t.Fatalf("empty file: %v", err)
	}
	return file
}

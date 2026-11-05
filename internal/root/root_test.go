package root

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/saleslumen/sl/internal/apiclient"
	"github.com/saleslumen/sl/internal/auth"
	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/credentials"
	"github.com/saleslumen/sl/internal/output"
	"github.com/spf13/cobra"
)

func testIO(in io.Reader, stdinTTY, stdoutTTY bool) (*cli.IOStreams, *bytes.Buffer, *bytes.Buffer) {
	if in == nil {
		in = bytes.NewReader(nil)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return &cli.IOStreams{In: in, Out: stdout, Err: stderr, IsStdinTTY: stdinTTY, IsStdoutTTY: stdoutTTY}, stdout, stderr
}

func testFactory(ios *cli.IOStreams) *cli.Factory {
	return &cli.Factory{
		IO: ios,
		Client: func() (*apiclient.Client, error) {
			return nil, cli.ErrNoCredentials
		},
		Organization: func() (string, error) {
			return "org-test", nil
		},
		Printer: func() *output.Printer {
			return output.New(ios.Out, output.Options{TTY: ios.IsStdoutTTY})
		},
		Confirm: func(prompt string) error {
			return confirm(ios, false, prompt)
		},
		NewRequestID: func() string {
			return "00000000-0000-4000-8000-000000000001"
		},
		RequireUserOAuth: func() error {
			return nil
		},
	}
}

type hostRewrite struct {
	target *url.URL
	next   http.RoundTripper
}

func (h hostRewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = h.target.Scheme
	clone.URL.Host = h.target.Host
	return h.next.RoundTrip(clone)
}

func testJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		panic("test jwt")
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func writeProfile(t *testing.T, dir string, mutate func(*credentials.File)) {
	t.Helper()
	store := credentials.Store{Dir: dir}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	mutate(file)
	if err := store.Save(file); err != nil {
		t.Fatalf("save: %v", err)
	}
}

func TestRootRegistersProductsInOrder(t *testing.T) {
	ios, _, _ := testIO(nil, false, false)
	f := testFactory(ios)
	var constructed []string
	product := func(name string) Product {
		return func(got *cli.Factory) *cobra.Command {
			if got != f {
				t.Fatal("product received another factory")
			}
			if got.Client == nil || got.Organization == nil || got.Printer == nil || got.Confirm == nil || got.NewRequestID == nil || got.RequireUserOAuth == nil {
				t.Fatal("product received incomplete factory")
			}
			constructed = append(constructed, name)
			return &cobra.Command{Use: name}
		}
	}
	command := NewRootCommand(f, product("first"), product("second"))
	if strings.Join(constructed, ",") != "first,second" {
		t.Fatalf("construction order=%v", constructed)
	}
	if command.CommandPath() != "sl" {
		t.Fatalf("command path=%q", command.CommandPath())
	}
}

func TestRootHelpDoesNotResolveCredentials(t *testing.T) {
	ios, stdout, stderr := testIO(nil, false, false)
	f := testFactory(ios)
	called := false
	f.Client = func() (*apiclient.Client, error) {
		called = true
		return nil, cli.ErrNoCredentials
	}
	f.Organization = func() (string, error) {
		called = true
		return "", nil
	}
	code := run([]string{"--help"}, f, NewRootCommand(f))
	if code != 0 || called {
		t.Fatalf("exit=%d client_called=%v stderr=%q", code, called, stderr.String())
	}
	for _, name := range []string{"auth", "config", "api", "version", "completion"} {
		if !strings.Contains(stdout.String(), name) {
			t.Fatalf("help missing %s", name)
		}
	}
}

func TestAuthLoginStoresOrganizationFlag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SL_CONFIG_DIR", dir)
	t.Setenv("SL_PROFILE", "")
	t.Setenv("SL_ORGANIZATION_ID", "")
	ios, _, stderr := testIO(strings.NewReader("sl_key_123e4567-e89b-12d3-a456-426614174000_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n"), false, false)
	f := &cli.Factory{IO: ios}
	var command *cobra.Command
	bindFactory(f, func() *cobra.Command { return command })
	command = NewRootCommand(f)
	code := run([]string{"auth", "login", "--organization", "org-login", "--with-token"}, f, command)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	file, err := credentials.Load(credentials.Path(dir))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	fields := file.Fields(credentials.DefaultProfile)
	if fields.OrganizationID != "org-login" {
		t.Fatalf("organization_id=%q", fields.OrganizationID)
	}
	if fields.AuthMode != credentials.AuthModeAPIKey || fields.AccessToken != "" {
		t.Fatal("api-key login did not store an exclusive api_key profile")
	}
}

func TestGlobalFlags(t *testing.T) {
	ios, _, _ := testIO(nil, false, false)
	command := NewRootCommand(testFactory(ios))
	for _, name := range []string{"profile", "namespace", "organization", "json", "jq", "verbose", "yes"} {
		if command.PersistentFlags().Lookup(name) == nil {
			t.Fatalf("missing --%s", name)
		}
	}
}

func TestOrganizationFactoryResolution(t *testing.T) {
	t.Setenv("SL_CONFIG_DIR", t.TempDir())
	t.Setenv("SL_API_KEY", "")
	t.Setenv("SL_ORGANIZATION_ID", "org-env")
	ios, _, _ := testIO(nil, false, false)
	f := &cli.Factory{IO: ios}
	var command *cobra.Command
	bindFactory(f, func() *cobra.Command { return command })
	command = NewRootCommand(f)
	got, err := f.Organization()
	if err != nil || got != "org-env" {
		t.Fatalf("organization=%q err=%v", got, err)
	}
	if err := command.PersistentFlags().Set("organization", "org-flag"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	got, err = f.Organization()
	if err != nil || got != "org-flag" {
		t.Fatalf("flag organization=%q err=%v", got, err)
	}
}

func TestOrganizationFactoryMissingIsUsageError(t *testing.T) {
	t.Setenv("SL_CONFIG_DIR", t.TempDir())
	t.Setenv("SL_API_KEY", "")
	t.Setenv("SL_ORGANIZATION_ID", "")
	ios, _, _ := testIO(nil, false, false)
	f := &cli.Factory{IO: ios}
	var command *cobra.Command
	bindFactory(f, func() *cobra.Command { return command })
	command = NewRootCommand(f)
	_, err := f.Organization()
	var usage *cli.UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("error=%v", err)
	}
	want := "organization required; pass --organization or run 'sl config set organization_id <id>'"
	if usage.Error() != want {
		t.Fatalf("error=%q", usage.Error())
	}
}

func TestVersionAndCompletionWriteStdout(t *testing.T) {
	ios, stdout, stderr := testIO(nil, false, false)
	f := testFactory(ios)
	if code := run([]string{"version"}, f, NewRootCommand(f)); code != 0 {
		t.Fatalf("version exit=%d stderr=%q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "sl ") {
		t.Fatalf("version stdout=%q", stdout.String())
	}
	ios, stdout, stderr = testIO(nil, false, false)
	f = testFactory(ios)
	if code := run([]string{"completion", "bash"}, f, NewRootCommand(f)); code != 0 {
		t.Fatalf("completion exit=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "sl") {
		t.Fatalf("completion stdout=%q", stdout.String())
	}
}

func TestArgumentValidationIsUsageError(t *testing.T) {
	ios, stdout, stderr := testIO(nil, false, false)
	f := testFactory(ios)
	code := run([]string{"auth", "login", "extra-arg"}, f, NewRootCommand(f))
	if code != 2 || stdout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunErrorTextDoesNotClassifyUsage(t *testing.T) {
	ios, _, stderr := testIO(nil, false, false)
	f := testFactory(ios)
	command := NewRootCommand(f, func(*cli.Factory) *cobra.Command {
		return &cobra.Command{Use: "fail", RunE: func(*cobra.Command, []string) error {
			return errors.New("backend accepts no traffic")
		}}
	})
	code := run([]string{"fail"}, f, command)
	if code != 1 || stderr.String() != "sl: backend accepts no traffic\n" {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
}

func TestRootJQValidationRunsBeforeProductHook(t *testing.T) {
	ios, stdout, stderr := testIO(nil, false, false)
	f := testFactory(ios)
	productHookCalled := false
	command := NewRootCommand(f, func(*cli.Factory) *cobra.Command {
		return &cobra.Command{
			Use: "product",
			PersistentPreRunE: func(*cobra.Command, []string) error {
				productHookCalled = true
				return nil
			},
			RunE: func(*cobra.Command, []string) error {
				return nil
			},
		}
	})
	code := run([]string{"--jq", ".[", "product"}, f, command)
	if code != 2 || stdout.Len() != 0 || productHookCalled {
		t.Fatalf("exit=%d hook=%v stdout=%q stderr=%q", code, productHookCalled, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid --jq expression") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestNoCredentialsUsesCommonErrorPath(t *testing.T) {
	ios, stdout, stderr := testIO(nil, false, false)
	f := testFactory(ios)
	code := run([]string{"api", "campaigns", "/v1/campaigns"}, f, NewRootCommand(f))
	if code != 4 || stdout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q", code, stdout.String())
	}
	if stderr.String() != "sl: no credentials found; run 'sl auth login'\n" {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestFlagErrorIsUsageError(t *testing.T) {
	ios, _, stderr := testIO(nil, false, false)
	f := testFactory(ios)
	code := run([]string{"--unknown"}, f, NewRootCommand(f))
	if code != 2 || !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
}

func TestAPIErrorExitCodesAndFormat(t *testing.T) {
	cases := []struct {
		status int
		exit   int
	}{
		{status: http.StatusNotFound, exit: 1},
		{status: http.StatusUnauthorized, exit: 4},
		{status: http.StatusForbidden, exit: 4},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			ios, _, stderr := testIO(nil, false, false)
			err := &apiclient.Error{Status: tc.status, Code: "CODE", Message: "message", Product: "campaigns", Method: "GET", Path: "/v1/campaigns"}
			command := &cobra.Command{Use: "sl", SilenceErrors: true, RunE: func(*cobra.Command, []string) error { return err }}
			if code := run(nil, testFactory(ios), command); code != tc.exit {
				t.Fatalf("exit=%d", code)
			}
			if stderr.String() != "sl: campaigns GET /v1/campaigns: CODE: message\n" {
				t.Fatalf("stderr=%q", stderr.String())
			}
		})
	}
}

func TestVerboseAPIErrorIncludesStatusAndRequestID(t *testing.T) {
	var stderr bytes.Buffer
	err := &apiclient.Error{
		Status:    http.StatusBadRequest,
		Code:      "INVALID_ARGUMENT",
		Message:   "bad field",
		RequestID: "req-123",
		Product:   "emails",
		Method:    http.MethodPost,
		Path:      "/v1/accounts",
	}
	writeCommandError(&stderr, err, true)
	want := "sl: emails POST /v1/accounts: INVALID_ARGUMENT: bad field\nHTTP 400\nrequest id: req-123\n"
	if stderr.String() != want {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestConfirmBehavior(t *testing.T) {
	ios, _, stderr := testIO(nil, false, false)
	if err := confirm(ios, true, "delete"); err != nil {
		t.Fatalf("yes err=%v", err)
	}
	var usage *cli.UsageError
	if err := confirm(ios, false, "delete"); !errors.As(err, &usage) {
		t.Fatalf("non-tty err=%v", err)
	}
	ios, _, stderr = testIO(strings.NewReader("yes\n"), true, false)
	if err := confirm(ios, false, "delete"); err != nil {
		t.Fatalf("tty err=%v", err)
	}
	if stderr.String() != "delete [y/N] " {
		t.Fatalf("prompt=%q", stderr.String())
	}
	ios, _, stderr = testIO(strings.NewReader("no\n"), true, false)
	err := confirm(ios, false, "delete")
	if !errors.Is(err, cli.ErrCancelled) {
		t.Fatalf("decline err=%v", err)
	}
	writeCommandError(stderr, err, false)
	if exitCode(err) != 1 || !strings.HasSuffix(stderr.String(), "sl: cancelled\n") {
		t.Fatalf("exit=%d stderr=%q", exitCode(err), stderr.String())
	}
}

func TestBoundPrinterUsesStructuredOptions(t *testing.T) {
	ios, stdout, stderr := testIO(nil, false, false)
	f := &cli.Factory{IO: ios}
	var command *cobra.Command
	bindFactory(f, func() *cobra.Command { return command })
	command = NewRootCommand(f, func(*cli.Factory) *cobra.Command {
		return &cobra.Command{Use: "emit", RunE: func(*cobra.Command, []string) error {
			return f.Printer().Object([]byte(`{"name":"Ada"}`))
		}}
	})
	if code := run([]string{"--jq", ".name", "emit"}, f, command); code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, stderr.String())
	}
	if stdout.String() != "Ada\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestClientRefreshesExpiredOAuthAndSendsBearerOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SL_CONFIG_DIR", dir)
	t.Setenv("SL_API_KEY", testAPIKey)
	t.Setenv("SL_PROFILE", "")
	oldAccess := testJWT(map[string]any{"sub": "user-1", "sl_organization_id": "org-oauth"})
	newAccess := testJWT(map[string]any{"sub": "user-1", "sl_organization_id": "org-oauth"})
	const oldRefresh = "refresh-token-old-fixture"
	const newRefresh = "refresh-token-new-fixture"
	writeProfile(t, dir, func(file *credentials.File) {
		file.SetOAuth(credentials.DefaultProfile, oldAccess, oldRefresh, "2026-09-18T11:00:00Z", "user-1", "org-oauth")
	})
	fixed := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	mux := http.NewServeMux()
	oauth := httptest.NewUnstartedServer(mux)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"authorization_endpoint": oauth.URL + "/oauth2/auth",
			"token_endpoint":         oauth.URL + "/oauth2/token",
		})
	})
	mux.HandleFunc("/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Header.Get("Authorization") != "" || r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != oldRefresh {
			http.Error(w, "bad token request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": newAccess, "refresh_token": newRefresh, "expires_in": 3600})
	})
	oauth.Start()
	t.Cleanup(oauth.Close)
	var headers http.Header
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(api.Close)
	apiURL, err := url.Parse(api.URL)
	if err != nil {
		t.Fatal(err)
	}
	ios, stdout, stderr := testIO(nil, false, false)
	f := &cli.Factory{IO: ios}
	var command *cobra.Command
	bindFactoryWith(f, func() *cobra.Command { return command }, auth.TokenSource{
		HTTPClient:   oauth.Client(),
		DiscoveryURL: oauth.URL + "/.well-known/openid-configuration",
		Now:          func() time.Time { return fixed },
	}, &http.Client{Transport: hostRewrite{target: apiURL, next: http.DefaultTransport}})
	command = NewRootCommand(f)
	client, err := f.Client()
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if _, err := client.Do(context.Background(), apiclient.Request{Product: "campaigns", Method: http.MethodGet, Path: "/v1/campaigns"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if headers.Get("Authorization") != "Bearer "+newAccess || headers.Get("sl-api-key") != "" {
		t.Fatal("client did not send the refreshed bearer token exclusively")
	}
	file, err := credentials.Load(credentials.Path(dir))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	fields := file.Fields(credentials.DefaultProfile)
	if fields.AccessToken != newAccess || fields.RefreshToken != newRefresh || fields.APIKey != "" {
		t.Fatal("refresh did not update the exclusive oauth profile")
	}
	if strings.Contains(stdout.String()+stderr.String(), oldAccess) || strings.Contains(stdout.String()+stderr.String(), newAccess) || strings.Contains(stdout.String()+stderr.String(), oldRefresh) || strings.Contains(stdout.String()+stderr.String(), newRefresh) || strings.Contains(stdout.String()+stderr.String(), testAPIKey) {
		t.Fatal("command output contained a secret")
	}
}

func TestClientOAuthIgnoresEnvAPIKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SL_CONFIG_DIR", dir)
	t.Setenv("SL_API_KEY", testAPIKey)
	t.Setenv("SL_PROFILE", "")
	access := testJWT(map[string]any{"sub": "user-1", "sl_organization_id": "org-oauth"})
	writeProfile(t, dir, func(file *credentials.File) {
		file.SetOAuth(credentials.DefaultProfile, access, "refresh-token-fixture", "2026-09-18T13:00:00Z", "user-1", "org-oauth")
	})
	var headers http.Header
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(api.Close)
	apiURL, err := url.Parse(api.URL)
	if err != nil {
		t.Fatal(err)
	}
	ios, stdout, stderr := testIO(nil, false, false)
	f := &cli.Factory{IO: ios}
	var command *cobra.Command
	bindFactoryWith(f, func() *cobra.Command { return command }, auth.TokenSource{
		Now: func() time.Time { return time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC) },
	}, &http.Client{Transport: hostRewrite{target: apiURL, next: http.DefaultTransport}})
	command = NewRootCommand(f)
	client, err := f.Client()
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if _, err := client.Do(context.Background(), apiclient.Request{Product: "emails", Method: http.MethodGet, Path: "/v1/accounts"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if headers.Get("Authorization") != "Bearer "+access || headers.Get("sl-api-key") != "" {
		t.Fatal("SL_API_KEY attached to an oauth request")
	}
	if strings.Contains(stdout.String()+stderr.String(), access) || strings.Contains(stdout.String()+stderr.String(), testAPIKey) {
		t.Fatal("command output contained a secret")
	}
}

func TestClientAPIKeySendsOnlyAPIKeyHeader(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SL_CONFIG_DIR", dir)
	t.Setenv("SL_API_KEY", "")
	t.Setenv("SL_PROFILE", "")
	writeProfile(t, dir, func(file *credentials.File) {
		file.SetAPIKey(credentials.DefaultProfile, testAPIKey, "org-login")
	})
	var headers http.Header
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(api.Close)
	apiURL, err := url.Parse(api.URL)
	if err != nil {
		t.Fatal(err)
	}
	ios, stdout, stderr := testIO(nil, false, false)
	f := &cli.Factory{IO: ios}
	var command *cobra.Command
	bindFactoryWith(f, func() *cobra.Command { return command }, auth.TokenSource{}, &http.Client{Transport: hostRewrite{target: apiURL, next: http.DefaultTransport}})
	command = NewRootCommand(f)
	client, err := f.Client()
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	if _, err := client.Do(context.Background(), apiclient.Request{Product: "campaigns", Method: http.MethodGet, Path: "/v1/campaigns"}); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if headers.Get("sl-api-key") != testAPIKey || headers.Get("Authorization") != "" {
		t.Fatal("api-key client headers were not exclusive")
	}
	if strings.Contains(stdout.String()+stderr.String(), testAPIKey) {
		t.Fatal("command output contained a secret")
	}
}

func TestRequireUserOAuthAPIKeyProfile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SL_CONFIG_DIR", dir)
	t.Setenv("SL_API_KEY", "")
	t.Setenv("SL_PROFILE", "")
	writeProfile(t, dir, func(file *credentials.File) {
		file.SetAPIKey(credentials.DefaultProfile, testAPIKey, "org-login")
	})
	ios, stdout, stderr := testIO(nil, false, false)
	f := &cli.Factory{IO: ios}
	var command *cobra.Command
	bindFactory(f, func() *cobra.Command { return command })
	command = NewRootCommand(f, func(got *cli.Factory) *cobra.Command {
		return &cobra.Command{Use: "need-user", RunE: func(*cobra.Command, []string) error {
			return got.RequireUserOAuth()
		}}
	})
	code := run([]string{"need-user"}, f, command)
	if code != 4 || stdout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stderr.String() != "sl: user OAuth login required\n" {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if !strings.Contains(stderr.String(), cli.ErrUserOAuthRequired.Error()) {
		t.Fatal("error text mismatch")
	}
}

const testAPIKey = "sl_key_123e4567-e89b-12d3-a456-426614174000_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

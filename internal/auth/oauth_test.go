package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/saleslumen/sl/internal/credentials"
)

const (
	testRefreshOld = "refresh-token-old-fixture"
	testRefreshNew = "refresh-token-new-fixture"
)

type scriptedLoopback struct {
	uri, code, state string
	err              error
}

func (s *scriptedLoopback) RedirectURI() string { return s.uri }
func (s *scriptedLoopback) Wait(context.Context) (string, string, error) {
	return s.code, s.state, s.err
}
func (s *scriptedLoopback) Close() error { return nil }

func testJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		panic("test jwt")
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func assertNoSecrets(t *testing.T, text string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if secret != "" && strings.Contains(text, secret) {
			t.Fatal("output contained a secret")
		}
	}
}

func oauthDiscoveryServer(t *testing.T, handleToken http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"authorization_endpoint": srv.URL + "/oauth2/auth",
			"token_endpoint":         srv.URL + "/oauth2/token",
		})
	})
	mux.HandleFunc("/oauth2/token", handleToken)
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func TestOAuthLoginWritesExclusiveProfile(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	seed := mustFile(t)
	seed.SetAPIKey(credentials.DefaultProfile, testKey, "org-old")
	if err := store.Save(seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	access := testJWT(map[string]any{"sub": "user-1", "sl_organization_id": "org-oauth"})
	refresh := testRefreshNew
	fixed := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	var tokenForm url.Values
	var tokenAuth string
	srv := oauthDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		tokenForm = r.Form
		tokenAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  access,
			"refresh_token": refresh,
			"expires_in":    3600,
		})
	})
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	loopback := &scriptedLoopback{uri: RedirectURI, code: "auth-code-1"}
	var opened string
	cmd := NewCommand(Deps{
		Streams:      Streams{Out: stdout, Err: stderr},
		Store:        storeOf(store),
		Profile:      func() string { return credentials.DefaultProfile },
		HTTPClient:   srv.Client(),
		DiscoveryURL: srv.URL + "/.well-known/openid-configuration",
		Now:          func() time.Time { return fixed },
		StartLoopback: func() (Loopback, error) {
			return loopback, nil
		},
		OpenURL: func(raw string) error {
			opened = raw
			parsed, err := url.Parse(raw)
			if err != nil {
				return err
			}
			loopback.state = parsed.Query().Get("state")
			return nil
		},
	})
	cmd.SetArgs([]string{"login"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}
	if stdout.String() != "Logged in to profile default\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	assertNoSecrets(t, stdout.String()+stderr.String(), access, refresh, testKey, loopback.code)
	if opened == "" {
		t.Fatal("browser was not opened")
	}
	authURL, err := url.Parse(opened)
	if err != nil {
		t.Fatalf("auth url: %v", err)
	}
	query := authURL.Query()
	if query.Get("client_id") != ClientID || query.Get("response_type") != "code" || query.Get("redirect_uri") != RedirectURI {
		t.Fatal("authorization url missing required identity fields")
	}
	if query.Get("audience") != Audience || query.Get("code_challenge_method") != "S256" || query.Get("state") == "" || query.Get("code_challenge") == "" {
		t.Fatal("authorization url missing pkce or audience")
	}
	if query.Get("scope") != Scope {
		t.Fatalf("scope=%q", query.Get("scope"))
	}
	assertNoSecrets(t, opened, access, refresh, testKey)
	if tokenAuth != "" || tokenForm.Get("client_secret") != "" {
		t.Fatal("token request used a client secret")
	}
	if tokenForm.Get("grant_type") != "authorization_code" || tokenForm.Get("code") != "auth-code-1" || tokenForm.Get("client_id") != ClientID || tokenForm.Get("redirect_uri") != RedirectURI {
		t.Fatal("token request fields mismatch")
	}
	verifier := tokenForm.Get("code_verifier")
	if verifier == "" {
		t.Fatal("token request missing code_verifier")
	}
	sum := sha256.Sum256([]byte(verifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != query.Get("code_challenge") {
		t.Fatal("pkce challenge did not match verifier")
	}
	assertNoSecrets(t, stdout.String()+stderr.String(), verifier)
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fields := file.Fields(credentials.DefaultProfile)
	if fields.AuthMode != credentials.AuthModeOAuth || fields.APIKey != "" {
		t.Fatal("oauth login did not store an exclusive oauth profile")
	}
	if fields.AccessToken != access || fields.RefreshToken != refresh {
		t.Fatal("oauth tokens were not stored")
	}
	if fields.OAuthUserID != "user-1" || fields.OrganizationID != "org-oauth" {
		t.Fatalf("identity user=%q org=%q", fields.OAuthUserID, fields.OrganizationID)
	}
	if fields.OAuthExpiresAt != "2026-09-18T13:00:00Z" {
		t.Fatalf("expires=%q", fields.OAuthExpiresAt)
	}
}

func TestOAuthLoginStateMismatchWritesNothing(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	access := testJWT(map[string]any{"sub": "user-1"})
	tokenCalled := false
	srv := oauthDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		tokenCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": access, "refresh_token": testRefreshNew, "expires_in": 60})
	})
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams:      Streams{Out: stdout, Err: stderr},
		Store:        storeOf(store),
		Profile:      func() string { return credentials.DefaultProfile },
		HTTPClient:   srv.Client(),
		DiscoveryURL: srv.URL + "/.well-known/openid-configuration",
		StartLoopback: func() (Loopback, error) {
			return &scriptedLoopback{uri: RedirectURI, code: "auth-code-1", state: "wrong-state"}, nil
		},
		OpenURL: func(string) error { return nil },
	})
	cmd.SetArgs([]string{"login"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "state mismatch") {
		t.Fatalf("err=%v", err)
	}
	if tokenCalled {
		t.Fatal("token endpoint called after state mismatch")
	}
	assertNoSecrets(t, stdout.String()+stderr.String()+err.Error(), access, testRefreshNew)
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file.Has(credentials.DefaultProfile) {
		t.Fatal("state mismatch wrote a profile")
	}
}

func TestAPIKeyLoginWipesOAuth(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	access := testJWT(map[string]any{"sub": "user-1"})
	seed := mustFile(t)
	seed.SetOAuth(credentials.DefaultProfile, access, testRefreshOld, "2026-09-18T13:00:00Z", "user-1", "org-oauth")
	if err := store.Save(seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{In: strings.NewReader(testKey + "\n"), Out: stdout, Err: stderr},
		Store:   storeOf(store),
		Profile: func() string { return credentials.DefaultProfile },
	})
	cmd.SetArgs([]string{"login", "--with-token"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login: %v", err)
	}
	assertNoSecrets(t, stdout.String()+stderr.String(), access, testRefreshOld, testKey)
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fields := file.Fields(credentials.DefaultProfile)
	if fields.AuthMode != credentials.AuthModeAPIKey || fields.APIKey != testKey {
		t.Fatal("api-key login did not replace the profile")
	}
	if fields.AccessToken != "" || fields.RefreshToken != "" || fields.OAuthUserID != "" || fields.OAuthExpiresAt != "" {
		t.Fatal("oauth credentials remained after api-key login")
	}
}

func TestStatusOAuthOmitsTokens(t *testing.T) {
	t.Parallel()
	access := testJWT(map[string]any{"sub": "user-1"})
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{Out: stdout, Err: stderr},
		Resolve: func() (credentials.Resolved, error) {
			return credentials.Resolved{
				Profile:        "work",
				AuthMode:       credentials.AuthModeOAuth,
				AccessToken:    access,
				RefreshToken:   testRefreshNew,
				OAuthUserID:    "user-1",
				OrganizationID: "org-oauth",
				NamespaceID:    "ns_1",
				APIDomain:      "example.com",
			}, nil
		},
	})
	cmd.SetArgs([]string{"status"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status: %v", err)
	}
	want := "profile: work\nauth_mode: oauth\norganization_id: org-oauth\nnamespace_id: ns_1\napi_domain: example.com\noauth_user_id: user-1\n"
	if stdout.String() != want {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if strings.Contains(stdout.String(), "api_key:") {
		t.Fatal("oauth status printed api_key")
	}
	assertNoSecrets(t, stdout.String()+stderr.String(), access, testRefreshNew)
}

func TestEnsureAccessTokenRefreshesExpiredSession(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	oldAccess := testJWT(map[string]any{"sub": "user-1", "sl_organization_id": "org-oauth"})
	newAccess := testJWT(map[string]any{"sub": "user-1", "sl_organization_id": "org-oauth"})
	fixed := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	seed := mustFile(t)
	seed.SetOAuth(credentials.DefaultProfile, oldAccess, testRefreshOld, "2026-09-18T11:00:00Z", "user-1", "org-oauth")
	if err := store.Save(seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	var tokenForm url.Values
	var tokenAuth string
	srv := oauthDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		tokenForm = r.Form
		tokenAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  newAccess,
			"refresh_token": testRefreshNew,
			"expires_in":    3600,
		})
	})
	resolved := credentials.Resolved{
		Profile:        credentials.DefaultProfile,
		AuthMode:       credentials.AuthModeOAuth,
		AccessToken:    oldAccess,
		RefreshToken:   testRefreshOld,
		OAuthExpiresAt: "2026-09-18T11:00:00Z",
		OAuthUserID:    "user-1",
		OrganizationID: "org-oauth",
	}
	got, err := EnsureAccessToken(context.Background(), store, resolved, TokenSource{
		HTTPClient:   srv.Client(),
		DiscoveryURL: srv.URL + "/.well-known/openid-configuration",
		Now:          func() time.Time { return fixed },
	})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got.AccessToken != newAccess || got.RefreshToken != testRefreshNew || got.APIKey != "" {
		t.Fatal("refresh did not return exclusive rotated tokens")
	}
	if got.OAuthExpiresAt != "2026-09-18T13:00:00Z" {
		t.Fatalf("expires=%q", got.OAuthExpiresAt)
	}
	if tokenAuth != "" || tokenForm.Get("grant_type") != "refresh_token" || tokenForm.Get("client_id") != ClientID || tokenForm.Get("refresh_token") != testRefreshOld {
		t.Fatal("refresh token request mismatch")
	}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fields := file.Fields(credentials.DefaultProfile)
	if fields.AccessToken != newAccess || fields.RefreshToken != testRefreshNew || fields.APIKey != "" {
		t.Fatal("profile was not updated by refresh")
	}
}

func TestLoginAPIKeyAndWithTokenAreMutuallyExclusive(t *testing.T) {
	t.Parallel()
	cmd := NewCommand(Deps{
		Streams: Streams{In: strings.NewReader(testKey + "\n"), Out: io.Discard, Err: io.Discard},
		Store:   storeOf(credentials.Store{Dir: t.TempDir()}),
		Profile: func() string { return credentials.DefaultProfile },
	})
	cmd.SetArgs([]string{"login", "--api-key", "--with-token"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected mutually exclusive flags")
	}
}

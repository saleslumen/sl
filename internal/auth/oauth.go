package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/saleslumen/sl/internal/credentials"
)

const (
	DefaultDiscoveryURL = "https://oauth.saleslumenapis.com/.well-known/openid-configuration"
	ClientID            = "saleslumen-cli"
	RedirectURI         = "http://127.0.0.1:53682/callback"
	ListenAddr          = "127.0.0.1:53682"
	Scope               = "openid offline_access api:read"
	Audience            = "saleslumen-api"
	refreshSkew         = 30 * time.Second
)

type Loopback interface {
	RedirectURI() string
	Wait(ctx context.Context) (code, state string, err error)
	Close() error
}

type TokenSource struct {
	HTTPClient   *http.Client
	DiscoveryURL string
	Now          func() time.Time
}

type discoveryDocument struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
}

type callbackResult struct {
	code  string
	state string
	err   error
}

type loopbackServer struct {
	uri    string
	result chan callbackResult
	srv    *http.Server
}

func (s *loopbackServer) RedirectURI() string {
	return s.uri
}

func (s *loopbackServer) Wait(ctx context.Context) (string, string, error) {
	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	case got := <-s.result:
		return got.code, got.state, got.err
	}
}

func (s *loopbackServer) Close() error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Close()
}

func ListenLoopback() (Loopback, error) {
	ln, err := net.Listen("tcp", ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("auth: cannot bind %s: %w", ListenAddr, err)
	}
	server := &loopbackServer{uri: RedirectURI, result: make(chan callbackResult, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", server.handleCallback)
	server.srv = &http.Server{Handler: mux}
	go func() {
		_ = server.srv.Serve(ln)
	}()
	return server, nil
}

func (s *loopbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	result := callbackResult{code: query.Get("code"), state: query.Get("state")}
	if errParam := query.Get("error"); errParam != "" {
		result.err = fmt.Errorf("auth: authorization %s", errParam)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "<!DOCTYPE html><html><body>Login failed.</body></html>")
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!DOCTYPE html><html><body>You can close this window.</body></html>")
	}
	select {
	case s.result <- result:
	default:
	}
}

func OpenBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("auth: open browser: %w", err)
	}
	return nil
}

func EnsureAccessToken(ctx context.Context, store credentials.Store, resolved credentials.Resolved, src TokenSource) (credentials.Resolved, error) {
	if resolved.AuthMode != credentials.AuthModeOAuth {
		return resolved, nil
	}
	now := src.now()
	if !needsRefresh(resolved, now) {
		if resolved.AccessToken == "" {
			return credentials.Resolved{}, fmt.Errorf("auth: access token is missing")
		}
		return resolved, nil
	}
	if resolved.RefreshToken == "" {
		return credentials.Resolved{}, fmt.Errorf("auth: access token expired")
	}
	disc, err := fetchDiscovery(ctx, src.httpClient(), src.discoveryURL())
	if err != nil {
		return credentials.Resolved{}, err
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", resolved.RefreshToken)
	form.Set("client_id", ClientID)
	tok, err := requestToken(ctx, src.httpClient(), disc.TokenEndpoint, form)
	if err != nil {
		return credentials.Resolved{}, err
	}
	refresh := tok.RefreshToken
	if refresh == "" {
		refresh = resolved.RefreshToken
	}
	userID, orgID := tokenIdentity(tok.AccessToken, tok.IDToken)
	if userID == "" {
		userID = resolved.OAuthUserID
	}
	expiresAt := now.Add(time.Duration(tok.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	file, err := store.Load()
	if err != nil {
		return credentials.Resolved{}, err
	}
	file.SetOAuth(resolved.Profile, tok.AccessToken, refresh, expiresAt, userID, orgID)
	if err := store.Save(file); err != nil {
		return credentials.Resolved{}, err
	}
	resolved.AccessToken = tok.AccessToken
	resolved.RefreshToken = refresh
	resolved.OAuthExpiresAt = expiresAt
	resolved.OAuthUserID = userID
	if orgID != "" {
		resolved.OrganizationID = orgID
	}
	resolved.APIKey = ""
	return resolved, nil
}

func runOAuthLogin(cmdCtx context.Context, d Deps) error {
	profile, err := profileName(d)
	if err != nil {
		return err
	}
	store, err := openStore(d)
	if err != nil {
		return err
	}
	start := d.StartLoopback
	if start == nil {
		start = ListenLoopback
	}
	loopback, err := start()
	if err != nil {
		return err
	}
	defer loopback.Close()
	state, err := randomToken(16)
	if err != nil {
		return err
	}
	verifier, err := randomToken(32)
	if err != nil {
		return err
	}
	client := httpClient(d)
	disc, err := fetchDiscovery(cmdCtx, client, discoveryURL(d))
	if err != nil {
		return err
	}
	authURL, err := authorizationURL(disc.AuthorizationEndpoint, loopback.RedirectURI(), state, pkceChallenge(verifier))
	if err != nil {
		return err
	}
	open := d.OpenURL
	if open == nil {
		open = OpenBrowser
	}
	if err := open(authURL); err != nil {
		return err
	}
	code, gotState, err := loopback.Wait(cmdCtx)
	if err != nil {
		return err
	}
	if gotState != state {
		return fmt.Errorf("auth: state mismatch")
	}
	if code == "" {
		return fmt.Errorf("auth: authorization code is missing")
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", loopback.RedirectURI())
	form.Set("client_id", ClientID)
	form.Set("code_verifier", verifier)
	tok, err := requestToken(cmdCtx, client, disc.TokenEndpoint, form)
	if err != nil {
		return err
	}
	userID, orgID := tokenIdentity(tok.AccessToken, tok.IDToken)
	expiresAt := nowFn(d)().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
	file, err := store.Load()
	if err != nil {
		return err
	}
	file.SetOAuth(profile, tok.AccessToken, tok.RefreshToken, expiresAt, userID, orgID)
	if err := store.Save(file); err != nil {
		return err
	}
	fmt.Fprintf(d.Streams.Out, "Logged in to profile %s\n", profile)
	return nil
}

func authorizationURL(endpoint, redirectURI, state, challenge string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("auth: invalid authorization endpoint")
	}
	query := parsed.Query()
	query.Set("client_id", ClientID)
	query.Set("response_type", "code")
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", Scope)
	query.Set("audience", Audience)
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func fetchDiscovery(ctx context.Context, client *http.Client, discoveryURL string) (discoveryDocument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("auth: discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("auth: discovery: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return discoveryDocument{}, fmt.Errorf("auth: read discovery: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return discoveryDocument{}, fmt.Errorf("auth: discovery: HTTP %d", resp.StatusCode)
	}
	var doc discoveryDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return discoveryDocument{}, fmt.Errorf("auth: parse discovery: %w", err)
	}
	if doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return discoveryDocument{}, fmt.Errorf("auth: discovery is missing endpoints")
	}
	return doc, nil
}

func requestToken(ctx context.Context, client *http.Client, tokenURL string, form url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("auth: token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("auth: token: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("auth: read token: %w", err)
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		tok.Error = ""
	}
	if resp.StatusCode != http.StatusOK || tok.AccessToken == "" {
		if tok.Error != "" {
			return tokenResponse{}, fmt.Errorf("auth: token endpoint: %s", tok.Error)
		}
		return tokenResponse{}, fmt.Errorf("auth: token endpoint: HTTP %d", resp.StatusCode)
	}
	return tok, nil
}

func needsRefresh(resolved credentials.Resolved, now time.Time) bool {
	if resolved.OAuthExpiresAt == "" {
		return resolved.AccessToken == "" && resolved.RefreshToken != ""
	}
	expires, err := time.Parse(time.RFC3339, resolved.OAuthExpiresAt)
	if err != nil {
		return resolved.RefreshToken != ""
	}
	return !now.Add(refreshSkew).Before(expires)
}

func tokenIdentity(accessToken, idToken string) (string, string) {
	userID := jwtClaim(accessToken, "sub")
	orgID := jwtClaim(accessToken, "sl_organization_id")
	if userID == "" {
		userID = jwtClaim(idToken, "sub")
	}
	if orgID == "" {
		orgID = jwtClaim(idToken, "sl_organization_id")
	}
	return userID, orgID
}

func jwtClaim(token, key string) string {
	payload := jwtPayload(token)
	if payload == nil {
		return ""
	}
	if value := stringClaim(payload, key); value != "" {
		return value
	}
	ext, _ := payload["ext"].(map[string]any)
	return stringClaim(ext, key)
}

func jwtPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		return nil
	}
	return payload
}

func stringClaim(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s TokenSource) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{}
}

func (s TokenSource) discoveryURL() string {
	if s.DiscoveryURL != "" {
		return s.DiscoveryURL
	}
	return DefaultDiscoveryURL
}

func (s TokenSource) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func httpClient(d Deps) *http.Client {
	if d.HTTPClient != nil {
		return d.HTTPClient
	}
	return &http.Client{}
}

func discoveryURL(d Deps) string {
	if d.DiscoveryURL != "" {
		return d.DiscoveryURL
	}
	return DefaultDiscoveryURL
}

func nowFn(d Deps) func() time.Time {
	if d.Now != nil {
		return d.Now
	}
	return time.Now
}

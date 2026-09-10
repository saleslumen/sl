//go:build monorepo

package emails

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/routepolicy"
	"gopkg.in/yaml.v3"
)

type route struct {
	Method string
	Path   string
	Body   string
}

// cliRoutes is the API-key-safe command surface.
var cliRoutes = []route{
	{Method: "GET", Path: "/v1/accounts"},
	{Method: "POST", Path: "/v1/accounts", Body: "*"},
	{Method: "GET", Path: "/v1/accounts/{id}"},
	{Method: "PUT", Path: "/v1/accounts/{id}", Body: "*"},
	{Method: "DELETE", Path: "/v1/accounts/{id}"},
	{Method: "POST", Path: "/v1/accounts:batch", Body: "*"},
	{Method: "GET", Path: "/v1/messages"},
	{Method: "GET", Path: "/v1/messages/{id}"},
	{Method: "POST", Path: "/v1/messages:send", Body: "*"},
	{Method: "POST", Path: "/v1/messages/{id}:modify", Body: "*"},
	{Method: "POST", Path: "/v1/messages/{id}:trash"},
	{Method: "POST", Path: "/v1/messages/{id}:untrash"},
	{Method: "DELETE", Path: "/v1/messages/{id}"},
	{Method: "POST", Path: "/v1/messages:batchModify", Body: "*"},
	{Method: "POST", Path: "/v1/messages:batchDelete", Body: "*"},
	{Method: "POST", Path: "/v1/messages:generateQuotedContent", Body: "*"},
	{Method: "GET", Path: "/v1/drafts"},
	{Method: "GET", Path: "/v1/drafts/{id}"},
	{Method: "POST", Path: "/v1/drafts", Body: "*"},
	{Method: "PUT", Path: "/v1/drafts/{id}", Body: "*"},
	{Method: "POST", Path: "/v1/drafts:send", Body: "*"},
	{Method: "DELETE", Path: "/v1/drafts/{id}"},
	{Method: "GET", Path: "/v1/messages/{messageId}/attachments/{id}"},
	{Method: "GET", Path: "/v1/threads"},
	{Method: "GET", Path: "/v1/threads/{id}"},
	{Method: "POST", Path: "/v1/threads/{id}:modify", Body: "*"},
	{Method: "DELETE", Path: "/v1/threads/{id}"},
	{Method: "POST", Path: "/v1/threads/{id}:trash"},
	{Method: "POST", Path: "/v1/threads/{id}:untrash"},
	{Method: "GET", Path: "/v1/labels"},
	{Method: "GET", Path: "/v1/labels/{id}"},
	{Method: "POST", Path: "/v1/labels", Body: "*"},
	{Method: "PATCH", Path: "/v1/labels/{label.id}", Body: "label"},
	{Method: "DELETE", Path: "/v1/labels/{id}"},
	{Method: "GET", Path: "/v1/tools:discover"},
	{Method: "POST", Path: "/v1/tools:verify", Body: "*"},
	{Method: "POST", Path: "/v1/tools:imap", Body: "*"},
	{Method: "POST", Path: "/v1/tools:smtp", Body: "*"},
}

const (
	reasonUserCredential  = "user credential required"
	reasonBrowserRedirect = "browser redirect"
)

type omittedRoute struct {
	Method string
	Path   string
	Reason string
}

var allowList = []omittedRoute{
	{Method: "POST", Path: "/v1/oauth2/login/google", Reason: reasonUserCredential},
	{Method: "POST", Path: "/v1/oauth2/login/microsoft", Reason: reasonUserCredential},
	{Method: "POST", Path: "/v1/oauth2/refresh", Reason: reasonUserCredential},
	{Method: "GET", Path: "/v1/oauth2/callback/google", Reason: reasonBrowserRedirect},
	{Method: "GET", Path: "/v1/oauth2/callback/microsoft", Reason: reasonBrowserRedirect},
}

var (
	docRouteRE   = regexp.MustCompile("`((?:GET|POST|PUT|PATCH|DELETE)) (/v1/[^`]+)`")
	flaskRouteRE = regexp.MustCompile(`@app\.route\(\s*["']([^"']+)["']\s*,\s*methods\s*=\s*\[([^\]]+)]`)
	httpMethods  = map[string]struct{}{"get": {}, "put": {}, "post": {}, "delete": {}, "patch": {}}
)

func TestNormalizePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/v1/labels/{label.id}", "/v1/labels/{}"},
		{"/v1/labels/{id}?updateMask=name", "/v1/labels/{}"},
		{"/v1/messages/{messageId}/attachments/{id}", "/v1/messages/{}/attachments/{}"},
		{"/v1/drafts/{id}?requestId=...&etag=...", "/v1/drafts/{}"},
		{"/v1/tools:verify", "/v1/tools:verify"},
	}
	for _, c := range cases {
		if got := normalizePath(c.in); got != c.want {
			t.Fatalf("normalizePath(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestParseProtoAdditionalBindings(t *testing.T) {
	src := `
    option (google.api.http) = {
      get: "/v1/alpha/{id}"
      additional_bindings {
        post: "/v1/alpha/{id}:act"
        body: "*"
        additional_bindings {
          patch: "/v1/alpha/{label.id}"
          body: "label"
        }
      }
    };
`
	got, err := parseProtoHTTP(src)
	if err != nil {
		t.Fatalf("parse proto: %v", err)
	}
	want := []route{
		{Method: "GET", Path: "/v1/alpha/{id}"},
		{Method: "POST", Path: "/v1/alpha/{id}:act", Body: "*"},
		{Method: "PATCH", Path: "/v1/alpha/{label.id}", Body: "label"},
	}
	if len(got) != len(want) {
		t.Fatalf("bindings=%d want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if routeKey(got[i].Method, got[i].Path) != routeKey(want[i].Method, want[i].Path) || got[i].Body != want[i].Body {
			t.Fatalf("binding[%d]=%+v want %+v", i, got[i], want[i])
		}
	}
}

func TestContractParity(t *testing.T) {
	root := repoRoot(t)
	docs, err := parseEnglishDocs(filepath.Join(root, "developers/docs/website/docs/emails/reference/apis"))
	if err != nil {
		t.Fatalf("docs: %v", err)
	}
	artifacts, err := loadArtifacts(root)
	if err != nil {
		t.Fatalf("artifacts: %v", err)
	}
	cli := routeSet(cliRoutes)
	docSet := routeSet(docs)
	artSet := routeSet(artifacts)
	t.Logf("cli=%d docs=%d artifacts=%d allow-list=%d", len(cli), len(docSet), len(artSet), len(allowList))
	if len(cli) != 38 {
		t.Fatalf("cli routes: got %d, want 38", len(cli))
	}
	if len(docSet) != 43 {
		t.Fatalf("docs unique routes: got %d, want 43", len(docSet))
	}
	if len(artSet) != 56 {
		t.Fatalf("artifact unique routes: got %d, want 56", len(artSet))
	}
	if len(allowList) != 5 {
		t.Fatalf("allow-list: got %d, want 5", len(allowList))
	}
	t.Run("cli subset docs", func(t *testing.T) {
		assertSubset(t, cli, docSet, "CLI route missing from English docs")
	})
	t.Run("docs subset artifacts", func(t *testing.T) {
		assertSubset(t, docSet, artSet, "documented route missing from artifacts")
	})
	t.Run("docs minus CLI allow-list", func(t *testing.T) {
		got := subtract(keysOf(docSet), keysOf(cli))
		want := omittedKeys(allowList)
		assertEqualSets(t, got, want, "docs-minus-CLI")
	})
	t.Run("producer principals", func(t *testing.T) {
		idx, err := routepolicy.Load(
			filepath.Join(root, "emails/accounts/routepolicy.yaml"),
			filepath.Join(root, "emails/messages/messages/routepolicy.yaml"),
			filepath.Join(root, "emails/tools/discover/routepolicy.yaml"),
			filepath.Join(root, "emails/tools/test/connection/routepolicy.yaml"),
		)
		if err != nil {
			t.Fatalf("producer routes: %v", err)
		}
		keys := make([]string, 0, len(cli))
		for key := range cli {
			keys = append(keys, key)
		}
		allow := make(map[string]string, len(allowList))
		for _, item := range allowList {
			allow[routeKey(item.Method, item.Path)] = item.Reason
		}
		routepolicy.AssertCLIHasAPIKey(t, idx, keys)
		routepolicy.AssertAllowListLacksAPIKey(t, idx, allow)
	})
}

func TestBodySelectors(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "emails/messages/messages/api/messages.proto"))
	if err != nil {
		t.Fatalf("messages.proto: %v", err)
	}
	proto, err := parseProtoHTTP(string(data))
	if err != nil {
		t.Fatalf("parse proto: %v", err)
	}
	bodies := map[string]string{}
	for _, r := range proto {
		bodies[routeKey(r.Method, r.Path)] = r.Body
	}
	empty := []string{
		"DELETE /v1/messages/{}",
		"POST /v1/messages/{}:trash",
		"POST /v1/messages/{}:untrash",
		"DELETE /v1/threads/{}",
		"POST /v1/threads/{}:trash",
		"POST /v1/threads/{}:untrash",
		"DELETE /v1/drafts/{}",
	}
	for _, key := range empty {
		body, ok := bodies[key]
		if !ok {
			t.Fatalf("proto missing %s", key)
		}
		if body != "" {
			t.Fatalf("%s proto body %q; trash/delete transcode as query, not body", key, body)
		}
	}
	if body := bodies["PATCH /v1/labels/{}"]; body != "label" {
		t.Fatalf("PATCH /v1/labels/{} proto body %q want label", body)
	}
	cli := routeSet(cliRoutes)
	for _, r := range proto {
		key := routeKey(r.Method, r.Path)
		cr, ok := cli[key]
		if !ok {
			continue
		}
		if cr.Body != r.Body {
			t.Fatalf("CLI %s body %q proto body %q", key, cr.Body, r.Body)
		}
	}
}

func TestFlaskExcludesOptionsAndHealth(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "emails/tools/test/connection/app.py"))
	if err != nil {
		t.Fatalf("connection app: %v", err)
	}
	got := routeSet(parseFlaskRoutes(string(data)))
	if _, ok := got["GET /health"]; ok {
		t.Fatal("Flask /health must be excluded")
	}
	if _, ok := got["OPTIONS /v1/tools:imap"]; ok {
		t.Fatal("Flask OPTIONS must be excluded")
	}
	if _, ok := got["POST /v1/tools:imap"]; !ok {
		t.Fatal("missing POST /v1/tools:imap")
	}
	if _, ok := got["POST /v1/tools:smtp"]; !ok {
		t.Fatal("missing POST /v1/tools:smtp")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			if st, err := os.Stat(filepath.Join(dir, "sl")); err == nil && st.IsDir() {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found: walk to AGENTS.md and sl/")
		}
		dir = parent
	}
}

func loadArtifacts(root string) ([]route, error) {
	accounts, err := os.ReadFile(filepath.Join(root, "emails/accounts/api.yaml"))
	if err != nil {
		return nil, fmt.Errorf("accounts openapi: %w", err)
	}
	ar, err := parseOpenAPI(accounts)
	if err != nil {
		return nil, fmt.Errorf("accounts openapi: %w", err)
	}
	discover, err := os.ReadFile(filepath.Join(root, "emails/tools/discover/api.yaml"))
	if err != nil {
		return nil, fmt.Errorf("discover openapi: %w", err)
	}
	dr, err := parseOpenAPI(discover)
	if err != nil {
		return nil, fmt.Errorf("discover openapi: %w", err)
	}
	proto, err := os.ReadFile(filepath.Join(root, "emails/messages/messages/api/messages.proto"))
	if err != nil {
		return nil, fmt.Errorf("messages.proto: %w", err)
	}
	pr, err := parseProtoHTTP(string(proto))
	if err != nil {
		return nil, fmt.Errorf("messages.proto: %w", err)
	}
	flask, err := os.ReadFile(filepath.Join(root, "emails/tools/test/connection/app.py"))
	if err != nil {
		return nil, fmt.Errorf("connection app: %w", err)
	}
	out := make([]route, 0, len(ar)+len(dr)+len(pr)+8)
	out = append(out, ar...)
	out = append(out, dr...)
	out = append(out, pr...)
	out = append(out, parseFlaskRoutes(string(flask))...)
	return out, nil
}

func parseEnglishDocs(dir string) ([]route, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read docs: %w", err)
	}
	var out []route
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !isEnglishMarkdown(name) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, m := range docRouteRE.FindAllSubmatch(data, -1) {
			out = append(out, route{Method: string(m[1]), Path: string(m[2])})
		}
	}
	return out, nil
}

func isEnglishMarkdown(name string) bool {
	if !strings.HasSuffix(name, ".md") {
		return false
	}
	base := strings.TrimSuffix(name, ".md")
	return !strings.HasSuffix(base, "-es") && !strings.HasSuffix(base, "-pt")
}

func parseOpenAPI(data []byte) ([]route, error) {
	var doc struct {
		Servers []struct {
			URL string `yaml:"url"`
		} `yaml:"servers"`
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	prefix := ""
	if len(doc.Servers) > 0 {
		u, err := url.Parse(doc.Servers[0].URL)
		if err != nil {
			return nil, fmt.Errorf("server url: %w", err)
		}
		prefix = strings.TrimSuffix(u.Path, "/")
	}
	var out []route
	for p, ops := range doc.Paths {
		full := joinServerPath(prefix, p)
		for method := range ops {
			if _, ok := httpMethods[strings.ToLower(method)]; !ok {
				continue
			}
			out = append(out, route{Method: strings.ToUpper(method), Path: full})
		}
	}
	return out, nil
}

func joinServerPath(prefix, path string) string {
	if strings.HasPrefix(path, "/v1/") || path == "/v1" {
		return path
	}
	if prefix == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return prefix + path
}

func parseFlaskRoutes(src string) []route {
	var out []route
	for _, m := range flaskRouteRE.FindAllStringSubmatch(src, -1) {
		path := m[1]
		if path == "/health" || !strings.HasPrefix(path, "/v1/") {
			continue
		}
		for _, part := range strings.Split(m[2], ",") {
			part = strings.Trim(strings.TrimSpace(part), `"'`)
			switch strings.ToUpper(part) {
			case "GET", "POST", "PUT", "PATCH", "DELETE":
				out = append(out, route{Method: strings.ToUpper(part), Path: path, Body: "*"})
			}
		}
	}
	return out
}

func parseProtoHTTP(src string) ([]route, error) {
	const marker = "option (google.api.http)"
	var out []route
	i := 0
	for i < len(src) {
		idx := strings.Index(src[i:], marker)
		if idx < 0 {
			return out, nil
		}
		i += idx + len(marker)
		i = skipWSAndComments(src, i)
		if i < len(src) && src[i] == '=' {
			i = skipWSAndComments(src, i+1)
		}
		if i >= len(src) || src[i] != '{' {
			return nil, fmt.Errorf("google.api.http: expected '{'")
		}
		block, end, err := extractBraceBlock(src, i)
		if err != nil {
			return nil, fmt.Errorf("google.api.http: %w", err)
		}
		routes, err := parseHTTPBinding(block)
		if err != nil {
			return nil, fmt.Errorf("google.api.http: %w", err)
		}
		out = append(out, routes...)
		i = end
	}
	return out, nil
}

func parseHTTPBinding(block string) ([]route, error) {
	var primary route
	var extra []route
	i := 0
	for i < len(block) {
		i = skipWSAndComments(block, i)
		if i >= len(block) {
			break
		}
		ident, next, err := readIdent(block, i)
		if err != nil {
			return nil, err
		}
		i = skipWSAndComments(block, next)
		if ident == "additional_bindings" {
			if i < len(block) && block[i] == ':' {
				i = skipWSAndComments(block, i+1)
			}
			if i >= len(block) || block[i] != '{' {
				return nil, fmt.Errorf("additional_bindings: expected '{'")
			}
			inner, end, err := extractBraceBlock(block, i)
			if err != nil {
				return nil, fmt.Errorf("additional_bindings: %w", err)
			}
			nested, err := parseHTTPBinding(inner)
			if err != nil {
				return nil, err
			}
			extra = append(extra, nested...)
			i = end
			continue
		}
		if i >= len(block) || block[i] != ':' {
			return nil, fmt.Errorf("%s: expected ':'", ident)
		}
		i = skipWSAndComments(block, i+1)
		if i < len(block) && block[i] == '{' {
			_, end, err := extractBraceBlock(block, i)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", ident, err)
			}
			i = end
			continue
		}
		if i >= len(block) || block[i] != '"' {
			return nil, fmt.Errorf("%s: expected string", ident)
		}
		val, end, err := readQuoted(block, i)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ident, err)
		}
		i = end
		switch ident {
		case "get", "post", "put", "patch", "delete":
			primary.Method = strings.ToUpper(ident)
			primary.Path = val
		case "body":
			primary.Body = val
		}
	}
	if primary.Method == "" {
		return extra, nil
	}
	return append([]route{primary}, extra...), nil
}

func extractBraceBlock(s string, open int) (string, int, error) {
	if open >= len(s) || s[open] != '{' {
		return "", 0, fmt.Errorf("expected '{'")
	}
	depth := 0
	inString := false
	escape := false
	inLine := false
	inBlock := false
	for i := open; i < len(s); i++ {
		c := s[i]
		if inLine {
			if c == '\n' {
				inLine = false
			}
			continue
		}
		if inBlock {
			if c == '*' && i+1 < len(s) && s[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			inLine = true
			i++
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		if c == '"' {
			inString = true
			continue
		}
		if c == '{' {
			depth++
		}
		if c == '}' {
			depth--
			if depth == 0 {
				return s[open+1 : i], i + 1, nil
			}
		}
	}
	return "", 0, fmt.Errorf("unbalanced braces")
}

func skipWSAndComments(s string, i int) int {
	for i < len(s) {
		if s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r' {
			i++
			continue
		}
		if s[i] == '/' && i+1 < len(s) && s[i+1] == '/' {
			i += 2
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if s[i] == '/' && i+1 < len(s) && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < len(s) {
				i += 2
			}
			continue
		}
		return i
	}
	return i
}

func readIdent(s string, i int) (string, int, error) {
	if i >= len(s) || !isIdentStart(s[i]) {
		return "", i, fmt.Errorf("expected identifier")
	}
	j := i + 1
	for j < len(s) && isIdentCont(s[j]) {
		j++
	}
	return s[i:j], j, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isIdentCont(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func readQuoted(s string, i int) (string, int, error) {
	if i >= len(s) || s[i] != '"' {
		return "", i, fmt.Errorf("expected '\"'")
	}
	var b strings.Builder
	escape := false
	for j := i + 1; j < len(s); j++ {
		c := s[j]
		if escape {
			b.WriteByte(c)
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			return b.String(), j + 1, nil
		}
		b.WriteByte(c)
	}
	return "", i, fmt.Errorf("unterminated string")
}

func routeKey(method, path string) string {
	return strings.ToUpper(method) + " " + normalizePath(path)
}

func normalizePath(path string) string {
	path, _, _ = strings.Cut(path, "?")
	var b strings.Builder
	i := 0
	for i < len(path) {
		if path[i] == '{' {
			end := strings.IndexByte(path[i:], '}')
			if end < 0 {
				b.WriteString(path[i:])
				break
			}
			b.WriteString("{}")
			i += end + 1
			continue
		}
		b.WriteByte(path[i])
		i++
	}
	return b.String()
}

func routeSet(routes []route) map[string]route {
	out := make(map[string]route, len(routes))
	for _, r := range routes {
		out[routeKey(r.Method, r.Path)] = r
	}
	return out
}

func keysOf(m map[string]route) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

func omittedKeys(groups ...[]omittedRoute) map[string]struct{} {
	out := map[string]struct{}{}
	for _, group := range groups {
		for _, o := range group {
			out[routeKey(o.Method, o.Path)] = struct{}{}
		}
	}
	return out
}

func subtract(a, b map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for k := range a {
		if _, ok := b[k]; !ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func assertSubset(t *testing.T, have, universe map[string]route, label string) {
	t.Helper()
	missing := subtract(keysOf(have), keysOf(universe))
	if len(missing) == 0 {
		return
	}
	t.Fatalf("%s: %s", label, strings.Join(sortedKeys(missing), ", "))
}

func assertEqualSets(t *testing.T, got, want map[string]struct{}, label string) {
	t.Helper()
	missing := subtract(want, got)
	extra := subtract(got, want)
	if len(missing) == 0 && len(extra) == 0 {
		return
	}
	t.Fatalf("%s mismatch\nmissing: %s\nextra: %s", label, strings.Join(sortedKeys(missing), ", "), strings.Join(sortedKeys(extra), ", "))
}

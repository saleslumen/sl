//go:build monorepo

package script

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/routepolicy"
)

type omittedRoute struct {
	route
	reason string
}

const userCredentialRequired = "user credential required"

// userOnlyRoutes are documented routes omitted because they require a user credential.
var userOnlyRoutes = []omittedRoute{
	{route{http.MethodPost, "/v1/projects/{}/versions"}, userCredentialRequired},
	{route{http.MethodPost, "/v1/installations/{}:run"}, userCredentialRequired},
}

var userOAuthCLIRoutes = []route{
	{http.MethodPost, "/v1/scripts/{script_id}:run"},
}

// artifactOnlyRoutes are annotated proto routes with no English API-reference page.
var artifactOnlyRoutes = []omittedRoute{
	{route{http.MethodGet, "/macros/d/{}/usercallback"}, "browser redirect"},
}

func TestContractParity(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cli, err := uniqueRoutes(cliRoutes[:])
	if err != nil {
		t.Fatalf("cli routes: %v", err)
	}
	if len(cli) != len(cliRoutes) || len(cli) != 26 {
		t.Fatalf("cli routes: got %d unique of %d, want 26", len(cli), len(cliRoutes))
	}
	docs, err := loadDocRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 28 {
		t.Fatalf("docs unique routes: got %d, want 28\n%s", len(docs), formatRoutes(docs))
	}
	proto, err := loadProtoRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(proto) != 29 {
		t.Fatalf("proto HTTP routes: got %d, want 29\n%s", len(proto), formatRoutes(proto))
	}
	assertSubset(t, "CLI", cli, docs)
	assertSubset(t, "docs", docs, proto)
	assertExcludedFromCLI(t, cli)
	docsMinusCLI := difference(docs, cli)
	if len(docsMinusCLI) != 2 {
		t.Fatalf("docs-minus-CLI: got %d, want 2\n%s", len(docsMinusCLI), formatRoutes(docsMinusCLI))
	}
	assertExactAllowList(t, "user-only", docsMinusCLI, omitRoutes(userOnlyRoutes))
	artifactOnly := difference(proto, docs)
	if len(artifactOnly) != 1 {
		t.Fatalf("artifact-only: got %d, want 1\n%s", len(artifactOnly), formatRoutes(artifactOnly))
	}
	assertExactAllowList(t, "artifact-only", artifactOnly, omitRoutes(artifactOnlyRoutes))
}

func TestRoutePolicyParity(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	idx, err := routepolicy.Load(filepath.Join(root, "script", "routepolicy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cli, err := uniqueRoutes(cliRoutes[:])
	if err != nil {
		t.Fatalf("cli routes: %v", err)
	}
	keys := make([]string, 0, len(cli))
	userOAuth := routeSet(userOAuthCLIRoutes)
	var apiKeyKeys, userOAuthKeys []string
	for _, r := range cli {
		key := routeKey(r)
		keys = append(keys, key)
		if _, ok := userOAuth[key]; ok {
			userOAuthKeys = append(userOAuthKeys, key)
			continue
		}
		apiKeyKeys = append(apiKeyKeys, key)
	}
	if len(userOAuthKeys) != len(userOAuthCLIRoutes) {
		t.Fatalf("user-OAuth CLI routes: got %d, want %d", len(userOAuthKeys), len(userOAuthCLIRoutes))
	}
	allow := make(map[string]string, len(userOnlyRoutes)+len(artifactOnlyRoutes))
	for _, item := range userOnlyRoutes {
		allow[routeKey(item.route)] = item.reason
	}
	for _, item := range artifactOnlyRoutes {
		allow[routeKey(item.route)] = item.reason
	}
	routepolicy.AssertCLICovered(t, idx, keys)
	routepolicy.AssertCLIHasAPIKey(t, idx, apiKeyKeys)
	routepolicy.AssertCLIUserOAuth(t, idx, userOAuthKeys)
	routepolicy.AssertAllowListLacksAPIKey(t, idx, allow)
}

func TestNormalizePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/v1/projects/{script_id}", "/v1/projects/{}"},
		{"/v1/labels/{label.id}", "/v1/labels/{}"},
		{"/v1/projects/{script_id}/versions:compare", "/v1/projects/{}/versions:compare"},
		{"/v1/libraries/{library_id}:lookup", "/v1/libraries/{}:lookup"},
		{"/macros/d/{script_id}/usercallback", "/macros/d/{}/usercallback"},
	}
	for _, tc := range cases {
		if got := normalizePath(tc.in); got != tc.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractProtoHTTPBindings(t *testing.T) {
	src := `
service S {
  rpc A(X) returns (Y) {
    option (google.api.http) = {
      get: "/v1/foo/{id}"
      additional_bindings {
        post: "/v1/foo/{name.id}:custom"
        body: "keep {braces} here"
        additional_bindings {
          delete: "/v1/bar"
        }
      }
    };
  }
  // option (google.api.http) = { get: "/v1/commented" }
  rpc B(X) returns (Y);
  rpc C(X) returns (Y) {
    option (google.api.http) = {
      put: "/v1/q/{id}"
    };
  }
}
`
	got, err := extractProtoHTTPRoutes(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err = uniqueRoutes(got)
	if err != nil {
		t.Fatal(err)
	}
	want := []route{
		{http.MethodGet, "/v1/foo/{}"},
		{http.MethodPost, "/v1/foo/{}:custom"},
		{http.MethodDelete, "/v1/bar"},
		{http.MethodPut, "/v1/q/{}"},
	}
	if !routeKeysEqual(got, want) {
		t.Fatalf("bindings: got\n%s\nwant\n%s", formatRoutes(got), formatRoutes(want))
	}
	if _, err := extractProtoHTTPRoutes(`option (google.api.http) = { get: "/v1/x"`); err == nil {
		t.Fatal("unclosed annotation: want error")
	}
}

func loadDocRoutes(root string) ([]route, error) {
	dir := filepath.Join(root, "developers", "docs", "website", "docs", "apps-script", "reference", "apis")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("docs: %w", err)
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	var routes []route
	for _, entry := range entries {
		if entry.IsDir() || !englishAPIDoc(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("docs %s: %w", entry.Name(), err)
		}
		routes = append(routes, extractDocRoutes(string(data))...)
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("docs: no HTTP routes in %s", dir)
	}
	return uniqueRoutes(routes)
}

func loadProtoRoutes(root string) ([]route, error) {
	path := filepath.Join(root, "script", "pkg", "api", "v1", "script.proto")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("proto: %w", err)
	}
	routes, err := extractProtoHTTPRoutes(string(data))
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("proto: no google.api.http bindings in %s", path)
	}
	return uniqueRoutes(routes)
}

func extractDocRoutes(text string) []route {
	var routes []route
	for i := 0; i < len(text); i++ {
		if text[i] != '`' {
			continue
		}
		j := i + 1
		for j < len(text) && text[j] != '`' {
			j++
		}
		if j >= len(text) {
			break
		}
		if r, ok := parseRouteSpan(text[i+1 : j]); ok {
			routes = append(routes, r)
		}
		i = j
	}
	return routes
}

func parseRouteSpan(span string) (route, bool) {
	span = strings.TrimSpace(span)
	method, rest, ok := strings.Cut(span, " ")
	if !ok {
		return route{}, false
	}
	method, ok = httpMethod(method)
	if !ok {
		return route{}, false
	}
	path := strings.TrimSpace(rest)
	path, _, _ = strings.Cut(path, "?")
	path, _, _ = strings.Cut(path, " ")
	if !strings.HasPrefix(path, "/") {
		return route{}, false
	}
	return route{Method: method, Path: path}, true
}

func extractProtoHTTPRoutes(src string) ([]route, error) {
	const marker = "(google.api.http)"
	var routes []route
	i := 0
	for i < len(src) {
		i = skipSpaceAndComments(src, i)
		if i >= len(src) {
			break
		}
		if src[i] == '"' {
			_, next, err := readString(src, i)
			if err != nil {
				return nil, fmt.Errorf("proto: %w", err)
			}
			i = next
			continue
		}
		ident, next, ok := readIdent(src, i)
		if !ok {
			i++
			continue
		}
		if ident != "option" {
			i = next
			continue
		}
		afterOption := next
		i = skipSpaceAndComments(src, next)
		if !strings.HasPrefix(src[i:], marker) {
			i = afterOption
			continue
		}
		i = skipSpaceAndComments(src, i+len(marker))
		if i >= len(src) || src[i] != '=' {
			return nil, fmt.Errorf("proto: option (google.api.http) missing '='")
		}
		i = skipSpaceAndComments(src, i+1)
		if i >= len(src) || src[i] != '{' {
			return nil, fmt.Errorf("proto: option (google.api.http) missing '{'")
		}
		inner, end, err := scanBalancedBlock(src, i)
		if err != nil {
			return nil, fmt.Errorf("proto: option (google.api.http): %w", err)
		}
		parsed, err := parseHTTPBinding(inner)
		if err != nil {
			return nil, err
		}
		if len(parsed) == 0 {
			return nil, fmt.Errorf("proto: option (google.api.http) has no HTTP binding")
		}
		routes = append(routes, parsed...)
		i = end
	}
	return routes, nil
}

func parseHTTPBinding(body string) ([]route, error) {
	var routes []route
	i := 0
	for i < len(body) {
		i = skipSpaceAndComments(body, i)
		if i >= len(body) {
			break
		}
		if body[i] == ';' {
			i++
			continue
		}
		ident, next, ok := readIdent(body, i)
		if !ok {
			return nil, fmt.Errorf("http option: unexpected token %q", snippet(body, i))
		}
		i = skipSpaceAndComments(body, next)
		switch ident {
		case "get", "put", "post", "delete", "patch":
			if i >= len(body) || body[i] != ':' {
				return nil, fmt.Errorf("http option: expected ':' after %s", ident)
			}
			i = skipSpaceAndComments(body, i+1)
			path, next, err := readString(body, i)
			if err != nil {
				return nil, fmt.Errorf("http option %s: %w", ident, err)
			}
			if path == "" {
				return nil, fmt.Errorf("http option: empty %s path", ident)
			}
			routes = append(routes, route{Method: strings.ToUpper(ident), Path: path})
			i = next
		case "custom":
			block, end, err := readNamedBlock(body, i)
			if err != nil {
				return nil, fmt.Errorf("http option custom: %w", err)
			}
			r, err := parseCustomBinding(block)
			if err != nil {
				return nil, err
			}
			routes = append(routes, r)
			i = end
		case "additional_bindings":
			block, end, err := readNamedBlock(body, i)
			if err != nil {
				return nil, fmt.Errorf("http option additional_bindings: %w", err)
			}
			nested, err := parseHTTPBinding(block)
			if err != nil {
				return nil, err
			}
			routes = append(routes, nested...)
			i = end
		default:
			if i < len(body) && body[i] == ':' {
				next, err := skipValue(body, i+1)
				if err != nil {
					return nil, fmt.Errorf("http option %s: %w", ident, err)
				}
				i = next
				continue
			}
			if i < len(body) && body[i] == '{' {
				_, end, err := scanBalancedBlock(body, i)
				if err != nil {
					return nil, fmt.Errorf("http option %s: %w", ident, err)
				}
				i = end
				continue
			}
			return nil, fmt.Errorf("http option: unexpected field %q", ident)
		}
	}
	return routes, nil
}

func parseCustomBinding(body string) (route, error) {
	var kind, path string
	i := 0
	for i < len(body) {
		i = skipSpaceAndComments(body, i)
		if i >= len(body) {
			break
		}
		if body[i] == ';' {
			i++
			continue
		}
		ident, next, ok := readIdent(body, i)
		if !ok {
			return route{}, fmt.Errorf("custom: unexpected token %q", snippet(body, i))
		}
		i = skipSpaceAndComments(body, next)
		if i >= len(body) || body[i] != ':' {
			return route{}, fmt.Errorf("custom: expected ':' after %s", ident)
		}
		i = skipSpaceAndComments(body, i+1)
		val, next, err := readString(body, i)
		if err != nil {
			return route{}, fmt.Errorf("custom %s: %w", ident, err)
		}
		i = next
		switch ident {
		case "kind":
			kind = val
		case "path":
			path = val
		}
	}
	if kind == "" || path == "" {
		return route{}, fmt.Errorf("custom: missing kind or path")
	}
	method, ok := httpMethod(kind)
	if !ok {
		method = strings.ToUpper(kind)
	}
	return route{Method: method, Path: path}, nil
}

func readNamedBlock(src string, i int) (string, int, error) {
	i = skipSpaceAndComments(src, i)
	if i < len(src) && src[i] == ':' {
		i = skipSpaceAndComments(src, i+1)
	}
	if i >= len(src) || src[i] != '{' {
		return "", i, fmt.Errorf("expected '{'")
	}
	return scanBalancedBlock(src, i)
}

func scanBalancedBlock(src string, open int) (string, int, error) {
	if open >= len(src) || src[open] != '{' {
		return "", open, fmt.Errorf("expected '{'")
	}
	depth := 0
	inString := false
	escape := false
	for i := open; i < len(src); i++ {
		c := src[i]
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
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[open+1 : i], i + 1, nil
			}
		}
	}
	return "", open, fmt.Errorf("unclosed '{'")
}

func skipSpaceAndComments(src string, i int) int {
	for i < len(src) {
		switch {
		case src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r':
			i++
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '/':
			i += 2
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			if i+1 < len(src) {
				i += 2
			}
		default:
			return i
		}
	}
	return i
}

func readIdent(src string, i int) (string, int, bool) {
	if i >= len(src) || !identStart(src[i]) {
		return "", i, false
	}
	start := i
	for i < len(src) && identCont(src[i]) {
		i++
	}
	return src[start:i], i, true
}

func readString(src string, i int) (string, int, error) {
	if i >= len(src) || src[i] != '"' {
		return "", i, fmt.Errorf("expected string")
	}
	var b strings.Builder
	escape := false
	for j := i + 1; j < len(src); j++ {
		c := src[j]
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

func skipValue(src string, i int) (int, error) {
	i = skipSpaceAndComments(src, i)
	if i >= len(src) {
		return i, fmt.Errorf("expected value")
	}
	if src[i] == '"' {
		_, next, err := readString(src, i)
		return next, err
	}
	if src[i] == '{' {
		_, end, err := scanBalancedBlock(src, i)
		return end, err
	}
	if identStart(src[i]) {
		_, next, _ := readIdent(src, i)
		return next, nil
	}
	if src[i] == '-' || (src[i] >= '0' && src[i] <= '9') {
		i++
		for i < len(src) && ((src[i] >= '0' && src[i] <= '9') || src[i] == '.') {
			i++
		}
		return i, nil
	}
	return i, fmt.Errorf("unexpected value %q", snippet(src, i))
}

func normalizePath(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); {
		if path[i] != '{' {
			b.WriteByte(path[i])
			i++
			continue
		}
		end := strings.IndexByte(path[i:], '}')
		if end < 0 {
			b.WriteString(path[i:])
			break
		}
		b.WriteString("{}")
		i += end + 1
	}
	return b.String()
}

func normalizeRoute(r route) route {
	return route{Method: strings.ToUpper(strings.TrimSpace(r.Method)), Path: normalizePath(r.Path)}
}

func uniqueRoutes(routes []route) ([]route, error) {
	seen := make(map[string]route, len(routes))
	var out []route
	for _, r := range routes {
		n := normalizeRoute(r)
		if n.Method == "" || n.Path == "" {
			return nil, fmt.Errorf("empty route %q %q", r.Method, r.Path)
		}
		key := routeKey(n)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = n
		out = append(out, n)
	}
	return out, nil
}

func routeKey(r route) string {
	n := normalizeRoute(r)
	return n.Method + " " + n.Path
}

func routeSet(routes []route) map[string]route {
	out := make(map[string]route, len(routes))
	for _, r := range routes {
		n := normalizeRoute(r)
		out[routeKey(n)] = n
	}
	return out
}

func difference(left, right []route) []route {
	have := routeSet(right)
	var out []route
	for _, r := range left {
		n := normalizeRoute(r)
		if _, ok := have[routeKey(n)]; !ok {
			out = append(out, n)
		}
	}
	return out
}

func omitRoutes(list []omittedRoute) []route {
	out := make([]route, len(list))
	for i, item := range list {
		out[i] = normalizeRoute(item.route)
	}
	return out
}

func assertSubset(t *testing.T, name string, inner, outer []route) {
	t.Helper()
	missing := difference(inner, outer)
	if len(missing) > 0 {
		t.Fatalf("%s ⊈ superset:\n%s", name, formatRoutes(missing))
	}
}

func assertExactAllowList(t *testing.T, name string, got, want []route) {
	t.Helper()
	if extra := difference(got, want); len(extra) > 0 {
		t.Fatalf("%s allow-list missing:\n%s", name, formatRoutes(extra))
	}
	if stale := difference(want, got); len(stale) > 0 {
		t.Fatalf("%s allow-list stale:\n%s", name, formatRoutes(stale))
	}
}

func assertExcludedFromCLI(t *testing.T, cli []route) {
	t.Helper()
	have := routeSet(cli)
	var included []route
	for _, r := range append(omitRoutes(userOnlyRoutes), omitRoutes(artifactOnlyRoutes)...) {
		if _, ok := have[routeKey(r)]; ok {
			included = append(included, r)
		}
	}
	if len(included) > 0 {
		t.Fatalf("CLI includes excluded routes:\n%s", formatRoutes(included))
	}
}

func routeKeysEqual(got, want []route) bool {
	return slices.Equal(sortedKeys(got), sortedKeys(want))
}

func sortedKeys(routes []route) []string {
	keys := make([]string, 0, len(routes))
	for _, r := range routes {
		keys = append(keys, routeKey(r))
	}
	slices.Sort(keys)
	return keys
}

func formatRoutes(routes []route) string {
	keys := sortedKeys(routes)
	return strings.Join(keys, "\n")
}

func englishAPIDoc(name string) bool {
	if !strings.HasSuffix(name, ".md") {
		return false
	}
	base := strings.TrimSuffix(name, ".md")
	return !strings.HasSuffix(base, "-es") && !strings.HasSuffix(base, "-pt")
}

func httpMethod(s string) (string, bool) {
	switch strings.ToUpper(s) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return strings.ToUpper(s), true
	default:
		return "", false
	}
}

func identStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func identCont(c byte) bool {
	return identStart(c) || (c >= '0' && c <= '9')
}

func snippet(src string, i int) string {
	if i >= len(src) {
		return ""
	}
	end := i + 16
	if end > len(src) {
		end = len(src)
	}
	return src[i:end]
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("contract test path unavailable")
	}
	dir := filepath.Dir(file)
	for {
		if fileExists(filepath.Join(dir, "AGENTS.md")) && dirExists(filepath.Join(dir, "sl")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repository root with AGENTS.md and sl/ not found from %s", file)
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

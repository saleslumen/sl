//go:build monorepo

package resources

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/routepolicy"
	"gopkg.in/yaml.v3"
)

type omittedRoute struct {
	route
	reason string
}

var allowList = []omittedRoute{}

const resourcesHost = "https://resources.saleslumenapis.com"

func TestContractParity(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cli, err := uniqueRoutes(cliRoutes[:])
	if err != nil {
		t.Fatalf("cli routes: %v", err)
	}
	if len(cli) != len(cliRoutes) || len(cli) != 6 {
		t.Fatalf("cli routes: got %d unique of %d, want 6\n%s", len(cli), len(cliRoutes), formatRoutes(cli))
	}
	docs, err := loadDocRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 6 {
		t.Fatalf("docs unique routes: got %d, want 6\n%s", len(docs), formatRoutes(docs))
	}
	api, err := loadAPIRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(api) != 6 {
		t.Fatalf("api unique routes: got %d, want 6\n%s", len(api), formatRoutes(api))
	}
	if len(allowList) != 0 {
		t.Fatalf("allow-list: got %d, want 0", len(allowList))
	}
	assertSubset(t, "CLI", cli, docs)
	assertSubset(t, "docs", docs, api)
	docsMinusCLI := difference(docs, cli)
	assertExactAllowList(t, "docs-minus-CLI", docsMinusCLI, omitRoutes(allowList))
	assertExcludedFromCLI(t, cli)
}

func TestRoutePolicyParity(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	idx, err := routepolicy.Load(filepath.Join(root, "resources", "routepolicy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cli, err := uniqueRoutes(cliRoutes[:])
	if err != nil {
		t.Fatalf("cli routes: %v", err)
	}
	var userKeys, apiKeyKeys []string
	for _, r := range cli {
		key := routeKey(r)
		if key == "GET /v1/organizations" || key == "POST /v1/organizations" {
			userKeys = append(userKeys, key)
			continue
		}
		apiKeyKeys = append(apiKeyKeys, key)
	}
	if len(userKeys) != 2 || len(apiKeyKeys) != 4 {
		t.Fatalf("policy split user=%d api_key=%d", len(userKeys), len(apiKeyKeys))
	}
	routepolicy.AssertCLICovered(t, idx, append(append([]string{}, userKeys...), apiKeyKeys...))
	routepolicy.AssertCLIUserOAuth(t, idx, userKeys)
	routepolicy.AssertCLIHasAPIKey(t, idx, apiKeyKeys)
}

func TestNormalizePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/v1/organizations/{organization_id}", "/v1/organizations/{}"},
		{"/v1/organizations/{organization_id}/namespaces", "/v1/organizations/{}/namespaces"},
		{"/v1/organizations/{organization_id}/namespaces/{namespace_id}", "/v1/organizations/{}/namespaces/{}"},
	}
	for _, tc := range cases {
		if got := normalizePath(tc.in); got != tc.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func loadDocRoutes(root string) ([]route, error) {
	dir := filepath.Join(root, "developers/docs/website/docs/management/reference/apis")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var routes []route
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasSuffix(name, "-es.md") || strings.HasSuffix(name, "-pt.md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("docs %s: %w", name, err)
		}
		if !strings.Contains(string(data), resourcesHost) {
			continue
		}
		routes = append(routes, extractDocRoutes(string(data))...)
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("docs: no HTTP routes")
	}
	return uniqueRoutes(routes)
}

func loadAPIRoutes(root string) ([]route, error) {
	path := filepath.Join(root, "resources", "api.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("api.yaml: %w", err)
	}
	routes, err := parseOpenAPI(data)
	if err != nil {
		return nil, fmt.Errorf("api.yaml: %w", err)
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("api.yaml: no HTTP routes in %s", path)
	}
	return uniqueRoutes(routes)
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
			m, ok := httpMethod(method)
			if !ok {
				continue
			}
			out = append(out, route{Method: m, Path: full})
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
	method, path, ok := strings.Cut(span, " ")
	if !ok || strings.Contains(path, " ") {
		return route{}, false
	}
	normalizedMethod, ok := httpMethod(method)
	if !ok || method != normalizedMethod || strings.Contains(path, "?") || !strings.HasPrefix(path, "/v1/") {
		return route{}, false
	}
	return route{Method: normalizedMethod, Path: path}, true
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
	seen := make(map[string]struct{}, len(routes))
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
		seen[key] = struct{}{}
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
	extra := difference(got, want)
	stale := difference(want, got)
	if len(extra) == 0 && len(stale) == 0 {
		return
	}
	t.Fatalf("%s mismatch\nextra: %s\nmissing: %s", name, formatRoutes(extra), formatRoutes(stale))
}

func assertExcludedFromCLI(t *testing.T, cli []route) {
	t.Helper()
	have := routeSet(cli)
	var included []route
	for _, r := range omitRoutes(allowList) {
		if _, ok := have[routeKey(r)]; ok {
			included = append(included, r)
		}
	}
	if len(included) > 0 {
		t.Fatalf("CLI includes excluded routes:\n%s", formatRoutes(included))
	}
}

func formatRoutes(routes []route) string {
	keys := make([]string, 0, len(routes))
	for _, r := range routes {
		keys = append(keys, routeKey(r))
	}
	slices.Sort(keys)
	return strings.Join(keys, "\n")
}

func httpMethod(s string) (string, bool) {
	switch strings.ToUpper(s) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return strings.ToUpper(s), true
	default:
		return "", false
	}
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

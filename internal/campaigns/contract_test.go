//go:build monorepo

package campaigns

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

func TestContractParity(t *testing.T) {
	allowList := []route{}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cli, err := uniqueRoutes(cliRoutes[:])
	if err != nil {
		t.Fatalf("cli routes: %v", err)
	}
	if len(cli) != len(cliRoutes) || len(cli) != 46 {
		t.Fatalf("cli routes: got %d unique of %d, want 46\n%s", len(cli), len(cliRoutes), formatRoutes(cli))
	}
	docs, err := loadDocRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 46 {
		t.Fatalf("docs unique routes: got %d, want 46\n%s", len(docs), formatRoutes(docs))
	}
	api, err := loadAPIRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(api) != 46 {
		t.Fatalf("api unique routes: got %d, want 46\n%s", len(api), formatRoutes(api))
	}
	t.Logf("cli=%d docs=%d api=%d allow-list=%d", len(cli), len(docs), len(api), len(allowList))
	assertSubset(t, "CLI", cli, docs)
	assertSubset(t, "docs", docs, api)
	assertExactAllowList(t, "docs-minus-CLI", difference(docs, cli), allowList)
}

func TestRoutePolicyParity(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	idx, err := routepolicy.Load(filepath.Join(root, "campaigns", "campaigns", "routepolicy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cli, err := uniqueRoutes(cliRoutes[:])
	if err != nil {
		t.Fatalf("cli routes: %v", err)
	}
	keys := make([]string, 0, len(cli))
	for _, r := range cli {
		keys = append(keys, routeKey(r))
	}
	routepolicy.AssertCLIHasAPIKey(t, idx, keys)
	routepolicy.AssertAllowListLacksAPIKey(t, idx, map[string]string{})
}

func TestNormalizePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/v1/campaigns/{campaign_id}", "/v1/campaigns/{}"},
		{"/v1/labels/{label.id}", "/v1/labels/{}"},
		{"/v1/campaigns/{campaign_id}:setVariables", "/v1/campaigns/{}:setVariables"},
		{"/v1/campaignSchedules/{schedule_id}", "/v1/campaignSchedules/{}"},
		{"/v1/campaigns/{campaign_id}/tasks/{task_id}:stream", "/v1/campaigns/{}/tasks/{}:stream"},
	}
	for _, tc := range cases {
		if got := normalizePath(tc.in); got != tc.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseRouteSpan(t *testing.T) {
	valid := []string{"GET /v1/campaigns", "POST /v1/campaigns/{campaign_id}:pause"}
	for _, span := range valid {
		if _, ok := parseRouteSpan(span); !ok {
			t.Errorf("parseRouteSpan(%q) rejected", span)
		}
	}
	invalid := []string{
		"POST /v1/x:pause now",
		"GET /v1/x?y=1",
		"GET /campaigns",
		"get /v1/campaigns",
	}
	for _, span := range invalid {
		if _, ok := parseRouteSpan(span); ok {
			t.Errorf("parseRouteSpan(%q) accepted", span)
		}
	}
}

func loadDocRoutes(root string) ([]route, error) {
	dir := filepath.Join(root, "developers", "docs", "website", "docs", "campaigns", "reference", "apis")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("docs: %w", err)
	}
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

func loadAPIRoutes(root string) ([]route, error) {
	path := filepath.Join(root, "campaigns", "campaigns", "api.yaml")
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
		for method := range ops {
			m, ok := httpMethod(method)
			if !ok {
				continue
			}
			out = append(out, route{Method: m, Path: prefix + p})
		}
	}
	return out, nil
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

func formatRoutes(routes []route) string {
	keys := make([]string, 0, len(routes))
	for _, r := range routes {
		keys = append(keys, routeKey(r))
	}
	slices.Sort(keys)
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

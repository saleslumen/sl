//go:build monorepo

package marketplace

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/routepolicy"
	"gopkg.in/yaml.v3"
)

func TestContractParity(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cli, err := uniqueRoutes(cliRoutes[:])
	if err != nil {
		t.Fatalf("cli routes: %v", err)
	}
	if len(cli) != len(cliRoutes) || len(cli) != 50 {
		t.Fatalf("cli routes: got %d unique of %d, want 50\n%s", len(cli), len(cliRoutes), formatRoutes(cli))
	}
	docs, err := loadDocRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 50 {
		t.Fatalf("docs unique routes: got %d, want 50\n%s", len(docs), formatRoutes(docs))
	}
	protoRoutes, err := loadProtoRoutes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(protoRoutes) != 50 {
		t.Fatalf("proto HTTP routes: got %d, want 50\n%s", len(protoRoutes), formatRoutes(protoRoutes))
	}
	assertSubset(t, "CLI", cli, docs)
	assertSubset(t, "docs", docs, protoRoutes)
	docsMinusCLI := difference(docs, cli)
	if len(docsMinusCLI) != 0 {
		t.Fatalf("docs-minus-CLI: got %d, want 0\n%s", len(docsMinusCLI), formatRoutes(docsMinusCLI))
	}
	if err := assertScriptBindDocumented(root); err != nil {
		t.Fatal(err)
	}
}

func TestRoutePolicyParity(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	idx, err := routepolicy.Load(filepath.Join(root, "marketplace", "routepolicy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := loadDefaultPrincipals(filepath.Join(root, "marketplace", "routepolicy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(defaults) != 1 || defaults[0] != routepolicy.KindUser {
		t.Fatalf("default principals = %v, want [user]", defaults)
	}
	routepolicy.AssertCLIHasAPIKey(t, idx, []string{routeKey(route{Method: http.MethodGet, Path: "/v1/apps"})})
	effective := routepolicy.Index{}
	for key, row := range idx {
		effective[key] = row
	}
	var userKeys []string
	for _, item := range cliRoutes {
		key := routeKey(item)
		if key == "GET /v1/apps" {
			continue
		}
		userKeys = append(userKeys, key)
		if _, ok := effective[key]; !ok {
			effective[key] = routepolicy.Route{Method: item.Method, Path: item.Path, Principals: append([]string(nil), defaults...)}
		}
	}
	if len(userKeys) != 49 {
		t.Fatalf("user-OAuth routes: got %d, want 49", len(userKeys))
	}
	routepolicy.AssertCLIUserOAuth(t, effective, userKeys)
	scriptIdx, err := routepolicy.Load(filepath.Join(root, "script", "routepolicy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	routepolicy.AssertCLIUserOAuth(t, scriptIdx, []string{routeKey(scriptBindRoute)})
}

func assertScriptBindDocumented(root string) error {
	doc, err := os.ReadFile(filepath.Join(root, "developers/docs/website/docs/apps-script/reference/apis/execution.md"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(doc), "POST /v1/installedConnectionApiKeyCredentials:bind") {
		return fmt.Errorf("execution.md must document the api key bind route")
	}
	proto, err := os.ReadFile(filepath.Join(root, "script/pkg/api/v1/script.proto"))
	if err != nil {
		return err
	}
	if !strings.Contains(string(proto), `post: "/v1/installedConnectionApiKeyCredentials:bind"`) {
		return fmt.Errorf("script.proto must annotate the api key bind route")
	}
	return nil
}

func loadDocRoutes(root string) ([]route, error) {
	dir := filepath.Join(root, "developers/docs/website/docs/marketplace/reference/apis")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var routes []route
	for _, entry := range entries {
		if entry.IsDir() || !englishAPIDoc(entry.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		routes = append(routes, extractDocRoutes(string(data))...)
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("docs: no HTTP routes")
	}
	return uniqueRoutes(routes)
}

func loadProtoRoutes(root string) ([]route, error) {
	data, err := os.ReadFile(filepath.Join(root, "marketplace/pkg/api/v1/marketplace.proto"))
	if err != nil {
		return nil, err
	}
	routes, err := extractProtoHTTPRoutes(string(data))
	if err != nil {
		return nil, err
	}
	return uniqueRoutes(routes)
}

func extractDocRoutes(text string) []route {
	var routes []route
	for i := 0; i < len(text); i++ {
		if text[i] != '`' {
			continue
		}
		j := strings.IndexByte(text[i+1:], '`')
		if j < 0 {
			break
		}
		j += i + 1
		if parsed, ok := parseRouteSpan(text[i+1 : j]); ok {
			routes = append(routes, parsed)
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
	if !strings.HasPrefix(path, "/v1/") {
		return route{}, false
	}
	return route{Method: method, Path: path}, true
}

var httpVerb = regexp.MustCompile(`\b(get|put|post|patch|delete)\s*:\s*"([^"]+)"`)

func extractProtoHTTPRoutes(src string) ([]route, error) {
	const marker = "option (google.api.http)"
	var routes []route
	for {
		i := strings.Index(src, marker)
		if i < 0 {
			break
		}
		rest := src[i+len(marker):]
		open := strings.Index(rest, "{")
		if open < 0 {
			return nil, fmt.Errorf("proto: option (google.api.http) missing '{'")
		}
		block, end, err := scanBalancedBlock(rest, open)
		if err != nil {
			return nil, err
		}
		for _, match := range httpVerb.FindAllStringSubmatch(block, -1) {
			routes = append(routes, route{Method: strings.ToUpper(match[1]), Path: match[2]})
		}
		src = rest[end:]
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("proto: no google.api.http bindings")
	}
	return routes, nil
}

func scanBalancedBlock(src string, open int) (string, int, error) {
	if open >= len(src) || src[open] != '{' {
		return "", 0, fmt.Errorf("expected '{'")
	}
	depth := 0
	for i := open; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[open+1 : i], i + 1, nil
			}
		}
	}
	return "", 0, fmt.Errorf("unbalanced '{'")
}

func loadDefaultPrincipals(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Spec struct {
			Default struct {
				Principals []string `yaml:"principals"`
			} `yaml:"default"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc.Spec.Default.Principals, nil
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

func normalizeRoute(item route) route {
	return route{Method: strings.ToUpper(strings.TrimSpace(item.Method)), Path: normalizePath(item.Path)}
}

func uniqueRoutes(routes []route) ([]route, error) {
	seen := map[string]route{}
	var out []route
	for _, item := range routes {
		n := normalizeRoute(item)
		if n.Method == "" || n.Path == "" {
			return nil, fmt.Errorf("empty route")
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

func routeKey(item route) string {
	n := normalizeRoute(item)
	return n.Method + " " + n.Path
}

func difference(left, right []route) []route {
	have := map[string]struct{}{}
	for _, item := range right {
		have[routeKey(item)] = struct{}{}
	}
	var out []route
	for _, item := range left {
		if _, ok := have[routeKey(item)]; !ok {
			out = append(out, normalizeRoute(item))
		}
	}
	return out
}

func assertSubset(t *testing.T, name string, left, right []route) {
	t.Helper()
	have := map[string]struct{}{}
	for _, item := range right {
		have[routeKey(item)] = struct{}{}
	}
	for _, item := range left {
		if _, ok := have[routeKey(item)]; !ok {
			t.Errorf("%s route missing from the right set: %s", name, routeKey(item))
		}
	}
}

func formatRoutes(routes []route) string {
	lines := make([]string, 0, len(routes))
	for _, item := range routes {
		lines = append(lines, routeKey(item))
	}
	return strings.Join(lines, "\n")
}

func englishAPIDoc(name string) bool {
	if !strings.HasSuffix(name, ".md") {
		return false
	}
	base := strings.TrimSuffix(name, ".md")
	return !strings.HasSuffix(base, "-es") && !strings.HasSuffix(base, "-pt")
}

func httpMethod(value string) (string, bool) {
	switch strings.ToUpper(value) {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return strings.ToUpper(value), true
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
			return "", fmt.Errorf("repository root not found from %s", file)
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

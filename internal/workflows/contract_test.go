//go:build monorepo

package workflows

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/routepolicy"
	"gopkg.in/yaml.v3"
)

var docRouteSpan = regexp.MustCompile("`([A-Z]+) (/v1/[^`]+)`")
var pathPlaceholder = regexp.MustCompile(`\{[^}]+\}`)

var httpMethods = map[string]struct{}{
	"GET": {}, "PUT": {}, "POST": {}, "DELETE": {}, "PATCH": {}, "HEAD": {}, "OPTIONS": {},
}

var englishAPIDocs = []string{
	"developers/docs/website/docs/workflows/reference/apis/workflows.md",
	"developers/docs/website/docs/workflows/reference/apis/executions.md",
	"developers/docs/website/docs/workflows/reference/apis/triggers.md",
}

const openAPIArtifact = "workflows/api/openapi.yaml"

var uncoveredAllowList = map[string]string{
	"POST /v1/workflows/{}/executions:start":     "user credential required",
	"POST /v1/workflows/{}/executions/{}:resume": "user credential required",
	"POST /v1/workflows/{}/triggers":             "user credential required",
	"POST /v1/hooks/{}":                          "anonymous webhook ingress",
}

func TestContractParity(t *testing.T) {
	root := repoRoot(t)
	cliAll, cliCovered := loadCLIRoutes()
	docs := loadDocRoutes(t, root)
	operations := loadArtifactOperations(t, root)
	artifact := operationKeys(operations)
	uncovered := difference(docs, cliCovered)
	t.Logf("contract parity counts: cli=%d covered=%d docs=%d artifact=%d uncovered=%d allow-list=%d", len(cliAll), len(cliCovered), len(docs), len(artifact), len(uncovered), len(uncoveredAllowList))
	if missing := missingKeys(cliAll, docs); len(missing) > 0 {
		t.Errorf("cli (%d) not subset of docs (%d): %s", len(cliAll), len(docs), strings.Join(missing, ", "))
	}
	if missing := missingKeys(docs, artifact); len(missing) > 0 {
		t.Errorf("docs (%d) not subset of artifact (%d): %s", len(docs), len(artifact), strings.Join(missing, ", "))
	}
	allow := keysOf(uncoveredAllowList)
	if extra, missing := symmetricDiff(uncovered, allow); len(extra) > 0 || len(missing) > 0 {
		t.Errorf("documented uncovered (%d) != allow-list (%d): extra=%s missing=%s", len(uncovered), len(allow), strings.Join(extra, ", "), strings.Join(missing, ", "))
	}
	for key, reason := range uncoveredAllowList {
		if _, ok := docs[key]; !ok {
			t.Errorf("allow-list stale %s (%s): not in docs (%d)", key, reason, len(docs))
		}
		if _, ok := artifact[key]; !ok {
			t.Errorf("allow-list stale %s (%s): not in artifact (%d)", key, reason, len(artifact))
		}
		if _, ok := cliCovered[key]; ok {
			t.Errorf("allow-list stale %s (%s): covered by CLI (covered=%d)", key, reason, len(cliCovered))
		}
	}
	for key := range docs {
		_, covered := cliCovered[key]
		_, allowed := uncoveredAllowList[key]
		if !covered && !allowed {
			t.Errorf("documented %s is neither covered nor allow-listed (docs=%d covered=%d)", key, len(docs), len(cliCovered))
		}
	}
	for _, route := range routes {
		key := routeKey(route.Method, route.Path)
		operation, ok := operations[key]
		if !ok {
			continue
		}
		if operation.rpc != route.RPC {
			t.Errorf("%s RPC=%q want %q", key, route.RPC, operation.rpc)
		}
		if route.Body == "*" {
			if operation.body == "" {
				t.Errorf("%s body missing; want *", key)
			}
		} else if operation.body != route.Body {
			t.Errorf("%s body=%q want %q", key, route.Body, operation.body)
		}
	}
	idx, err := routepolicy.Load(filepath.Join(root, "workflows", "routepolicy.yaml"))
	if err != nil {
		t.Fatalf("routepolicy: %v", err)
	}
	cliKeys := make([]string, 0, len(cliCovered))
	for key := range cliCovered {
		cliKeys = append(cliKeys, key)
	}
	routepolicy.AssertCLIHasAPIKey(t, idx, cliKeys)
	routepolicy.AssertAllowListLacksAPIKey(t, idx, uncoveredAllowList)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("contract test path unavailable")
	}
	dir := filepath.Dir(file)
	for {
		agents := filepath.Join(dir, "AGENTS.md")
		sl := filepath.Join(dir, "sl")
		agentsInfo, agentsErr := os.Stat(agents)
		slInfo, slErr := os.Stat(sl)
		if agentsErr == nil && !agentsInfo.IsDir() && slErr == nil && slInfo.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root with AGENTS.md and sl/ not found")
		}
		dir = parent
	}
}

func loadCLIRoutes() (all map[string]struct{}, covered map[string]struct{}) {
	all = map[string]struct{}{}
	covered = map[string]struct{}{}
	for _, r := range routes {
		key := routeKey(r.Method, r.Path)
		all[key] = struct{}{}
		if r.Covered {
			covered[key] = struct{}{}
		}
	}
	return all, covered
}

func loadDocRoutes(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	for _, rel := range englishAPIDocs {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, m := range docRouteSpan.FindAllSubmatch(raw, -1) {
			method := string(m[1])
			if _, ok := httpMethods[method]; !ok {
				continue
			}
			out[routeKey(method, string(m[2]))] = struct{}{}
		}
	}
	return out
}

type openAPIOperation struct {
	rpc  string
	body string
}

func loadArtifactOperations(t *testing.T, root string) map[string]openAPIOperation {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, openAPIArtifact))
	if err != nil {
		t.Fatalf("read %s: %v", openAPIArtifact, err)
	}
	type parameter struct {
		Name string `yaml:"name"`
		In   string `yaml:"in"`
	}
	type operation struct {
		OperationID string      `yaml:"operationId"`
		Parameters  []parameter `yaml:"parameters"`
	}
	var doc struct {
		Paths map[string]map[string]operation `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", openAPIArtifact, err)
	}
	out := map[string]openAPIOperation{}
	for path, ops := range doc.Paths {
		for method, op := range ops {
			u := strings.ToUpper(method)
			if _, ok := httpMethods[u]; !ok {
				continue
			}
			rpc := op.OperationID
			if separator := strings.LastIndexByte(rpc, '_'); separator >= 0 {
				rpc = rpc[separator+1:]
			}
			body := ""
			for _, parameter := range op.Parameters {
				if parameter.In == "body" {
					body = parameter.Name
					break
				}
			}
			out[routeKey(u, path)] = openAPIOperation{rpc: rpc, body: body}
		}
	}
	return out
}

func operationKeys(operations map[string]openAPIOperation) map[string]struct{} {
	out := make(map[string]struct{}, len(operations))
	for key := range operations {
		out[key] = struct{}{}
	}
	return out
}

func routeKey(method, path string) string {
	path = strings.TrimSpace(path)
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	path = pathPlaceholder.ReplaceAllString(path, "{}")
	return strings.ToUpper(strings.TrimSpace(method)) + " " + path
}

func keysOf(m map[string]string) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

func difference(have, minus map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(have))
	for k := range have {
		if _, ok := minus[k]; !ok {
			out[k] = struct{}{}
		}
	}
	return out
}

func missingKeys(need, have map[string]struct{}) []string {
	var missing []string
	for k := range need {
		if _, ok := have[k]; !ok {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	return missing
}

func symmetricDiff(got, want map[string]struct{}) (extra, missing []string) {
	extra = missingKeys(got, want)
	missing = missingKeys(want, got)
	return extra, missing
}

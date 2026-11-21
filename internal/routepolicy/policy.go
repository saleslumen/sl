package routepolicy

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	KindAnonymous = "anonymous"
	KindUser      = "user"
	KindAPIKey    = "api_key"
	KindService   = "service"
)

var publicOrder = []string{KindAnonymous, KindUser, KindAPIKey}

type document struct {
	Spec struct {
		Default *struct {
			Principals []string `yaml:"principals"`
		} `yaml:"default"`
		Routes []struct {
			Method     string   `yaml:"method"`
			Path       string   `yaml:"path"`
			Principals []string `yaml:"principals"`
		} `yaml:"routes"`
	} `yaml:"spec"`
}

type Route struct {
	Method     string
	Path       string
	Principals []string
}

type Index map[string]Route

func Load(paths ...string) (Index, error) {
	out := Index{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("routepolicy: read %s: %w", path, err)
		}
		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		for {
			var doc document
			if err := decoder.Decode(&doc); err != nil {
				if err == io.EOF {
					break
				}
				return nil, fmt.Errorf("routepolicy: parse %s: %w", path, err)
			}
			if err := mergeDocument(out, path, doc); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func mergeDocument(out Index, path string, doc document) error {
	for _, route := range doc.Spec.Routes {
		key := Key(route.Method, route.Path)
		next := Route{Method: strings.ToUpper(route.Method), Path: route.Path, Principals: append([]string(nil), route.Principals...)}
		if prev, ok := out[key]; ok && !principalsEqual(prev.Principals, next.Principals) {
			return fmt.Errorf("routepolicy: %s conflicts on %s", path, key)
		}
		out[key] = next
	}
	return nil
}

func Key(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + NormalizePath(path)
}

func NormalizePath(path string) string {
	path, _, _ = strings.Cut(strings.TrimSpace(path), "?")
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

func Public(principals []string) []string {
	have := map[string]struct{}{}
	for _, p := range principals {
		if p == KindService {
			continue
		}
		have[p] = struct{}{}
	}
	out := make([]string, 0, len(publicOrder))
	for _, p := range publicOrder {
		if _, ok := have[p]; ok {
			out = append(out, p)
		}
	}
	return out
}

func Has(principals []string, kind string) bool {
	for _, p := range principals {
		if p == kind {
			return true
		}
	}
	return false
}

func principalsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left := map[string]struct{}{}
	for _, p := range a {
		left[p] = struct{}{}
	}
	for _, p := range b {
		if _, ok := left[p]; !ok {
			return false
		}
	}
	return true
}

func AssertCLICovered(t *testing.T, idx Index, methodPathKeys []string) {
	t.Helper()
	for _, key := range methodPathKeys {
		route, ok := idx[key]
		if !ok {
			t.Errorf("CLI %s missing from producer routepolicy", key)
			continue
		}
		if !Has(route.Principals, KindAPIKey) && !Has(route.Principals, KindUser) {
			t.Errorf("CLI %s producer principals %v admit neither api_key nor user", key, route.Principals)
		}
	}
}

func AssertCLIHasAPIKey(t *testing.T, idx Index, methodPathKeys []string) {
	t.Helper()
	for _, key := range methodPathKeys {
		route, ok := idx[key]
		if !ok {
			t.Errorf("CLI %s missing from producer routepolicy", key)
			continue
		}
		if !Has(route.Principals, KindAPIKey) {
			t.Errorf("CLI %s producer principals %v do not contain api_key", key, route.Principals)
		}
	}
}

func AssertCLIUserOAuth(t *testing.T, idx Index, methodPathKeys []string) {
	t.Helper()
	for _, key := range methodPathKeys {
		route, ok := idx[key]
		if !ok {
			t.Errorf("CLI user-OAuth %s missing from producer routepolicy", key)
			continue
		}
		if !Has(route.Principals, KindUser) {
			t.Errorf("CLI user-OAuth %s producer principals %v do not contain user", key, route.Principals)
		}
	}
}

func AssertAllowListLacksAPIKey(t *testing.T, idx Index, allow map[string]string) {
	t.Helper()
	for _, key := range sortedKeys(allow) {
		reason := allow[key]
		if reason == "browser redirect" || reason == "browser oauth initiation" || reason == "programmatic oauth" {
			continue
		}
		route, ok := idx[key]
		if !ok {
			t.Errorf("allow-list %s (%s) missing from producer routepolicy", key, reason)
			continue
		}
		if Has(route.Principals, KindAnonymous) {
			continue
		}
		if Has(route.Principals, KindAPIKey) {
			t.Errorf("allow-list %s (%s) producer principals %v contain api_key", key, reason, route.Principals)
		}
		if reason != "user credential required" {
			t.Errorf("allow-list %s user-only reason %q want %q", key, reason, "user credential required")
		}
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

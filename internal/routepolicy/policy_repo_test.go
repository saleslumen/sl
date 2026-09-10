//go:build monorepo

package routepolicy

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadCampaignsHasAPIKey(t *testing.T) {
	root := repoRoot(t)
	idx, err := Load(filepath.Join(root, "campaigns/campaigns/routepolicy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	route, ok := idx[Key("GET", "/v1/campaigns")]
	if !ok || !Has(route.Principals, KindAPIKey) {
		t.Fatalf("GET /v1/campaigns=%v ok=%v", route, ok)
	}
}

func TestLoadEmailsMergeAndOAuth(t *testing.T) {
	root := repoRoot(t)
	idx, err := Load(
		filepath.Join(root, "emails/accounts/routepolicy.yaml"),
		filepath.Join(root, "emails/messages/messages/routepolicy.yaml"),
		filepath.Join(root, "emails/tools/discover/routepolicy.yaml"),
		filepath.Join(root, "emails/tools/test/connection/routepolicy.yaml"),
		filepath.Join(root, "emails/messages/sender/routepolicy.yaml"),
	)
	if err != nil {
		t.Fatal(err)
	}
	send := idx[Key("POST", "/v1/messages:send")]
	if !Has(send.Principals, KindAPIKey) {
		t.Fatalf("send principals=%v", send.Principals)
	}
	login := idx[Key("POST", "/v1/oauth2/login/google")]
	if Has(login.Principals, KindAPIKey) {
		t.Fatalf("login principals=%v", login.Principals)
	}
	callback := idx[Key("GET", "/v1/oauth2/callback/google")]
	if !Has(callback.Principals, KindAPIKey) {
		t.Fatalf("callback principals=%v", callback.Principals)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller path unavailable")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			if st, err := os.Stat(filepath.Join(dir, "sl")); err == nil && st.IsDir() {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

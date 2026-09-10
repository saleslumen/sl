package routepolicy

import (
	"testing"
)

func TestPublicOmitsServiceAndOrdersKinds(t *testing.T) {
	got := Public([]string{KindService, KindAPIKey, KindUser, KindAnonymous, KindAPIKey})
	want := []string{KindAnonymous, KindUser, KindAPIKey}
	if len(got) != len(want) {
		t.Fatalf("Public=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Public=%v want %v", got, want)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	if got := NormalizePath("/v1/organizations/{organization_id}/namespaces/{namespace_id}?x=1"); got != "/v1/organizations/{}/namespaces/{}" {
		t.Fatalf("NormalizePath=%q", got)
	}
}

func TestAssertAllowListBrowserException(t *testing.T) {
	idx := Index{
		Key("GET", "/v1/oauth2/callback/google"): {Principals: []string{KindUser, KindAPIKey}},
		Key("POST", "/v1/hooks/{token}"):         {Principals: []string{KindAnonymous}},
		Key("POST", "/v1/oauth2/login/google"):   {Principals: []string{KindUser}},
	}
	AssertAllowListLacksAPIKey(t, idx, map[string]string{
		Key("GET", "/v1/oauth2/callback/google"): "browser redirect",
		Key("POST", "/v1/hooks/{token}"):         "anonymous webhook ingress",
		Key("POST", "/v1/oauth2/login/google"):   "user credential required",
	})
	AssertCLIHasAPIKey(t, idx, []string{Key("GET", "/v1/oauth2/callback/google")})
}

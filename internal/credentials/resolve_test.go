package credentials

import (
	"errors"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	t.Parallel()
	const (
		flagNS     = "ns-flag"
		envNS      = "ns-env"
		fileNS     = "ns-file"
		flagOrg    = "org-flag"
		envOrg     = "org-env"
		fileOrg    = "org-file"
		envDomain  = "env.example.com"
		fileDomain = "file.example.com"
		envKey     = "sl_key_fromenvvalue"
		fileKey    = "sl_key_fromprofilev"
	)
	file := newFile()
	file.Set("work", KeyAPIKey, fileKey)
	file.Set("work", KeyOrganizationID, fileOrg)
	file.Set("work", KeyNamespaceID, fileNS)
	file.Set("work", KeyAPIDomain, fileDomain)
	t.Run("flag over env over profile over default", func(t *testing.T) {
		t.Parallel()
		got, err := Resolve(Flags{Profile: "work", OrganizationID: flagOrg, NamespaceID: flagNS}, Env{APIKey: envKey, OrganizationID: envOrg, NamespaceID: envNS, APIDomain: envDomain}, file)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got.Profile != "work" || got.OrganizationID != flagOrg || got.NamespaceID != flagNS || got.APIDomain != envDomain {
			t.Fatalf("resolved fields profile=%q organization=%q ns=%q domain=%q", got.Profile, got.OrganizationID, got.NamespaceID, got.APIDomain)
		}
		if got.APIKey != envKey {
			t.Fatal("env api key lost to a lower source")
		}
	})
	t.Run("env over profile", func(t *testing.T) {
		t.Parallel()
		got, err := Resolve(Flags{Profile: "work"}, Env{APIKey: envKey, OrganizationID: envOrg, NamespaceID: envNS, APIDomain: envDomain}, file)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got.OrganizationID != envOrg || got.NamespaceID != envNS || got.APIDomain != envDomain {
			t.Fatalf("env fields organization=%q ns=%q domain=%q", got.OrganizationID, got.NamespaceID, got.APIDomain)
		}
		if got.APIKey != envKey {
			t.Fatal("env api key did not beat profile")
		}
	})
	t.Run("profile over default", func(t *testing.T) {
		t.Parallel()
		got, err := Resolve(Flags{Profile: "work"}, Env{}, file)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got.OrganizationID != fileOrg || got.NamespaceID != fileNS || got.APIDomain != fileDomain {
			t.Fatalf("profile fields organization=%q ns=%q domain=%q", got.OrganizationID, got.NamespaceID, got.APIDomain)
		}
		if got.APIKey != fileKey {
			t.Fatal("profile api key was not used")
		}
	})
	t.Run("default profile and api domain", func(t *testing.T) {
		t.Parallel()
		only := newFile()
		only.Set(DefaultProfile, KeyAPIKey, fileKey)
		got, err := Resolve(Flags{}, Env{}, only)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got.Profile != DefaultProfile || got.APIDomain != DefaultAPIDomain {
			t.Fatalf("defaults profile=%q domain=%q", got.Profile, got.APIDomain)
		}
	})
}

func TestResolveMissingKey(t *testing.T) {
	t.Parallel()
	_, err := Resolve(Flags{}, Env{}, newFile())
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveProfileDoesNotRequireKey(t *testing.T) {
	t.Parallel()
	got := ResolveProfile(Flags{OrganizationID: "org-flag"}, Env{}, newFile())
	if got.OrganizationID != "org-flag" || got.APIKey != "" {
		t.Fatalf("resolved=%+v", got)
	}
}

func TestResolveIgnoresProcessEnvironment(t *testing.T) {
	t.Setenv(EnvAPIKey, "sl_key_processenvxx")
	t.Setenv(EnvProfile, "process")
	t.Setenv(EnvOrganizationID, "org-process")
	t.Setenv(EnvAPIDomain, "process.example.com")
	file := newFile()
	file.Set(DefaultProfile, KeyAPIKey, "sl_key_fromprofilev")
	got, err := Resolve(Flags{}, Env{}, file)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.Profile != DefaultProfile || got.APIDomain != DefaultAPIDomain {
		t.Fatalf("process env leaked profile=%q domain=%q", got.Profile, got.APIDomain)
	}
	if got.APIKey != "sl_key_fromprofilev" {
		t.Fatal("process env api key leaked into Resolve")
	}
}

func TestResolveIgnoresUnknownProfileKeys(t *testing.T) {
	t.Parallel()
	file := newFile()
	file.Set(DefaultProfile, KeyAPIKey, testKey)
	file.Set(DefaultProfile, "region", "us")
	got, err := Resolve(Flags{}, Env{}, file)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if extra, ok := file.Extra(DefaultProfile, "region"); !ok || extra != "us" {
		t.Fatal("unknown key was not kept")
	}
	if got.NamespaceID != "" || got.APIDomain != DefaultAPIDomain {
		t.Fatalf("unexpected resolved ns=%q domain=%q", got.NamespaceID, got.APIDomain)
	}
}

func TestEnvFrom(t *testing.T) {
	t.Parallel()
	got := EnvFrom(func(key string) string {
		switch key {
		case EnvConfigDir:
			return "/tmp/cfg"
		case EnvProfile:
			return "work"
		case EnvAPIKey:
			return testKey
		case EnvOrganizationID:
			return "org"
		case EnvNamespaceID:
			return "ns"
		case EnvAPIDomain:
			return "example.com"
		default:
			return "unused"
		}
	})
	if got.ConfigDir != "/tmp/cfg" || got.Profile != "work" || got.OrganizationID != "org" || got.NamespaceID != "ns" || got.APIDomain != "example.com" {
		t.Fatalf("env fields=%+v", Env{ConfigDir: got.ConfigDir, Profile: got.Profile, OrganizationID: got.OrganizationID, NamespaceID: got.NamespaceID, APIDomain: got.APIDomain})
	}
	if got.APIKey != testKey {
		t.Fatal("EnvFrom api key mismatch")
	}
}

func TestResolveOAuthWithoutAPIKey(t *testing.T) {
	t.Parallel()
	file := newFile()
	file.SetOAuth(DefaultProfile, "access-token-fixture", "refresh-token-fixture", "2026-09-18T13:00:00Z", "user-1", "org-oauth")
	got, err := Resolve(Flags{}, Env{APIKey: testKey}, file)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.AuthMode != AuthModeOAuth || got.APIKey != "" || got.AccessToken != "access-token-fixture" {
		t.Fatal("oauth resolve mixed in an api key")
	}
	if got.OrganizationID != "org-oauth" || got.OAuthUserID != "user-1" {
		t.Fatalf("identity org=%q user=%q", got.OrganizationID, got.OAuthUserID)
	}
}

func TestResolveLegacyAPIKeyMode(t *testing.T) {
	t.Parallel()
	file := newFile()
	file.Set(DefaultProfile, KeyAPIKey, testKey)
	got, err := Resolve(Flags{}, Env{}, file)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.AuthMode != AuthModeAPIKey || got.APIKey != testKey || got.AccessToken != "" {
		t.Fatal("legacy api key profile was not exclusive")
	}
}

func TestResolveOAuthRefreshableWithoutAccessToken(t *testing.T) {
	t.Parallel()
	file := newFile()
	file.Set(DefaultProfile, KeyAuthMode, AuthModeOAuth)
	file.Set(DefaultProfile, KeyOAuthRefreshToken, "refresh-token-fixture")
	got, err := Resolve(Flags{}, Env{}, file)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.AuthMode != AuthModeOAuth || got.RefreshToken != "refresh-token-fixture" || got.APIKey != "" {
		t.Fatal("refreshable oauth session was rejected")
	}
}

func TestStoreResolve(t *testing.T) {
	t.Parallel()
	store := Store{Dir: t.TempDir()}
	file := newFile()
	file.Set(DefaultProfile, KeyAPIKey, testKey)
	if err := store.Save(file); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Resolve(Flags{}, Env{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.APIKey != testKey || got.APIDomain != DefaultAPIDomain {
		t.Fatal("store resolve mismatch")
	}
}

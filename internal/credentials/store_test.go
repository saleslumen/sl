package credentials

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testKey    = "sl_key_123e4567-e89b-12d3-a456-426614174000_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	testPrefix = "sl_key_123e4567-e89b-12d3-a456-426614174000"
)

func TestDirUsesConfigDirBeforeHome(t *testing.T) {
	t.Parallel()
	dir, err := Dir(Env{ConfigDir: "/tmp/sl-config-test"}, func() (string, error) {
		t.Fatal("home must not be used when SL_CONFIG_DIR is set")
		return "", nil
	})
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if dir != "/tmp/sl-config-test" {
		t.Fatalf("Dir=%q", dir)
	}
}

func TestDirJoinsHomeWhenConfigDirEmpty(t *testing.T) {
	t.Parallel()
	dir, err := Dir(Env{}, func() (string, error) { return "/home/test-user", nil })
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if dir != filepath.Join("/home/test-user", DefaultDirName) {
		t.Fatalf("Dir=%q", dir)
	}
}

func TestDirDoesNotReadProcessHome(t *testing.T) {
	t.Parallel()
	_, err := Dir(Env{}, func() (string, error) { return "", nil })
	if err == nil {
		t.Fatal("expected empty home error")
	}
}

func TestSavePermissionsAndMissingLoad(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "nested")
	store := Store{Dir: dir}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	file.Set(DefaultProfile, KeyAPIKey, testKey)
	if err := store.Save(file); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("dir perm=%o", info.Mode().Perm())
	}
	info, err = os.Stat(store.Path())
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file perm=%o", info.Mode().Perm())
	}
}

func TestSavePreservesUnknownKeysAndIsAtomic(t *testing.T) {
	t.Parallel()
	store := Store{Dir: t.TempDir()}
	original := []byte("schema_version = 2\n\n[default]\napi_key = \"sl_key_oldvaluexxxx\"\nregion = \"us\"\n")
	if err := os.WriteFile(store.Path(), original, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, ok := file.ExtraRoot("schema_version"); !ok || got == nil {
		t.Fatal("missing root extra")
	}
	if got, ok := file.Extra(DefaultProfile, "region"); !ok || got != "us" {
		t.Fatalf("region extra=%v ok=%v", got, ok)
	}
	file.Set(DefaultProfile, KeyAPIKey, testKey)
	file.Set(DefaultProfile, KeyOrganizationID, "org-acme")
	file.Set(DefaultProfile, KeyNamespaceID, "ns_1")
	if err := store.Save(file); err != nil {
		t.Fatalf("Save: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(store.Dir, ".credentials-*.tmp"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp leftovers=%d", len(matches))
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	fields := reloaded.Fields(DefaultProfile)
	if fields.APIKey != testKey {
		t.Fatal("api_key was not updated")
	}
	if fields.OrganizationID != "org-acme" {
		t.Fatalf("organization_id=%q", fields.OrganizationID)
	}
	if fields.NamespaceID != "ns_1" {
		t.Fatalf("namespace_id=%q", fields.NamespaceID)
	}
	if got, ok := reloaded.Extra(DefaultProfile, "region"); !ok || got != "us" {
		t.Fatal("profile extra lost")
	}
	if _, ok := reloaded.ExtraRoot("schema_version"); !ok {
		t.Fatal("root extra lost")
	}
	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "region") {
		t.Fatal("rewritten file dropped unknown key")
	}
}

func TestSaveDoesNotLeavePartialFileOnEncodeSuccess(t *testing.T) {
	t.Parallel()
	store := Store{Dir: t.TempDir()}
	file := newFile()
	file.Set("acme-staging", KeyAPIKey, testKey)
	file.Set("acme-staging", "custom", "keep")
	if err := store.Save(file); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := Load(store.Path())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reloaded.Has("acme-staging") {
		t.Fatal("missing profile")
	}
	if got, ok := reloaded.Extra("acme-staging", "custom"); !ok || got != "keep" {
		t.Fatal("unknown key lost after rewrite")
	}
}

func TestRemoveProfile(t *testing.T) {
	t.Parallel()
	file := newFile()
	file.Set("gone", KeyAPIKey, testKey)
	file.RemoveProfile("gone")
	if file.Has("gone") {
		t.Fatal("profile still present")
	}
}

func TestKeyPrefix(t *testing.T) {
	t.Parallel()
	if got := KeyPrefix(testKey); got != testPrefix {
		t.Fatalf("prefix=%q", got)
	}
	if got := KeyPrefix("short"); got != "" {
		t.Fatalf("invalid=%q", got)
	}
}

func TestValidateAPIKey(t *testing.T) {
	t.Parallel()
	if err := ValidateAPIKey(testKey); err != nil {
		t.Fatalf("valid: %v", err)
	}
	for _, key := range []string{
		"nope",
		"sl_key_123e4567-e89b-12d3-a456-426614174000",
		"sl_key_not-a-uuid_secret",
		"sl_key_123e4567-e89b-12d3-a456-426614174000_!",
	} {
		if err := ValidateAPIKey(key); err != ErrInvalidAPIKey {
			t.Fatalf("key=%q invalid=%v", key, err)
		}
	}
}

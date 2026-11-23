package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/saleslumen/sl/internal/cli"
	"github.com/saleslumen/sl/internal/credentials"
)

const testKey = "sl_key_fixturevalue"

func storeOf(store credentials.Store) func() (credentials.Store, error) {
	return func() (credentials.Store, error) { return store, nil }
}

func TestConfigGetSetUnsetList(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	file.Set(credentials.DefaultProfile, credentials.KeyAPIKey, testKey)
	file.Set(credentials.DefaultProfile, "region", "us")
	if err := store.Save(file); err != nil {
		t.Fatalf("Save: %v", err)
	}
	stdout := &bytes.Buffer{}
	deps := Deps{Streams: Streams{Out: stdout}, Store: storeOf(store)}
	set := NewCommand(deps)
	set.SetArgs([]string{"set", "namespace_id", "ns_1"})
	if err := set.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}
	setDomain := NewCommand(deps)
	setDomain.SetArgs([]string{"set", "api_domain", "staging.saleslumenapis.com"})
	if err := setDomain.Execute(); err != nil {
		t.Fatalf("set domain: %v", err)
	}
	setOrganization := NewCommand(deps)
	setOrganization.SetArgs([]string{"set", "organization_id", "org-acme"})
	if err := setOrganization.Execute(); err != nil {
		t.Fatalf("set organization: %v", err)
	}
	stdout.Reset()
	get := NewCommand(Deps{Streams: Streams{Out: stdout}, Store: storeOf(store)})
	get.SetArgs([]string{"get", "namespace_id"})
	if err := get.Execute(); err != nil {
		t.Fatalf("get: %v", err)
	}
	if stdout.String() != "ns_1\n" {
		t.Fatalf("get stdout=%q", stdout.String())
	}
	stdout.Reset()
	list := NewCommand(Deps{Streams: Streams{Out: stdout}, Store: storeOf(store)})
	list.SetArgs([]string{"list"})
	if err := list.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}
	if stdout.String() != "organization_id=org-acme\nnamespace_id=ns_1\napi_domain=staging.saleslumenapis.com\n" {
		t.Fatalf("list stdout=%q", stdout.String())
	}
	if strings.Contains(stdout.String(), testKey) {
		t.Fatal("list printed an api key")
	}
	unset := NewCommand(Deps{Streams: Streams{Out: io.Discard}, Store: storeOf(store)})
	unset.SetArgs([]string{"unset", "namespace_id"})
	if err := unset.Execute(); err != nil {
		t.Fatalf("unset: %v", err)
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	fields := reloaded.Fields(credentials.DefaultProfile)
	if fields.OrganizationID != "org-acme" || fields.NamespaceID != "" || fields.APIDomain != "staging.saleslumenapis.com" {
		t.Fatalf("fields organization=%q ns=%q domain=%q", fields.OrganizationID, fields.NamespaceID, fields.APIDomain)
	}
	if fields.APIKey != testKey {
		t.Fatal("config edit dropped api_key")
	}
	if got, ok := reloaded.Extra(credentials.DefaultProfile, "region"); !ok || got != "us" {
		t.Fatal("unknown key lost on config rewrite")
	}
}

func TestConfigUnknownKeyIsUsageError(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	file.Set(credentials.DefaultProfile, credentials.KeyAPIKey, testKey)
	if err := store.Save(file); err != nil {
		t.Fatalf("Save: %v", err)
	}
	for _, args := range [][]string{
		{"get", "api_key"},
		{"set", "api_key", "sl_key_shouldnotwrite"},
		{"unset", "region"},
		{"get", "unknown"},
	} {
		cmd := NewCommand(Deps{Streams: Streams{Out: io.Discard}, Store: storeOf(store)})
		cmd.SetArgs(args)
		err := cmd.Execute()
		var usage *cli.UsageError
		if !errors.As(err, &usage) {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Fields(credentials.DefaultProfile).APIKey != testKey {
		t.Fatal("rejected set mutated api_key")
	}
}

func TestConfigUsesInjectedProfile(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	cmd := NewCommand(Deps{
		Streams: Streams{Out: io.Discard},
		Store:   storeOf(store),
		Profile: func() string { return "acme" },
	})
	cmd.SetArgs([]string{"set", "namespace_id", "ns_acme"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set: %v", err)
	}
	file, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if file.Fields("acme").NamespaceID != "ns_acme" {
		t.Fatal("value written to the wrong profile")
	}
	if file.Has(credentials.DefaultProfile) {
		t.Fatal("default profile was created")
	}
}

func TestConfigGetEmpty(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{Out: stdout},
		Store:   storeOf(credentials.Store{Dir: t.TempDir()}),
	})
	cmd.SetArgs([]string{"get", "namespace_id"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("get: %v", err)
	}
	if stdout.String() != "\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestConfigListEmpty(t *testing.T) {
	t.Parallel()
	stdout := &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{Out: stdout},
		Store:   storeOf(credentials.Store{Dir: t.TempDir()}),
	})
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}
	if stdout.String() != "organization_id=\nnamespace_id=\napi_domain=\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestConfigStoreResolverFailure(t *testing.T) {
	t.Parallel()
	store := credentials.Store{Dir: t.TempDir()}
	resolverErr := errors.New("store unavailable")
	stdout := &bytes.Buffer{}
	cmd := NewCommand(Deps{
		Streams: Streams{Out: stdout},
		Store:   func() (credentials.Store, error) { return credentials.Store{}, resolverErr },
	})
	cmd.SetArgs([]string{"set", "namespace_id", "ns_1"})
	err := cmd.Execute()
	if !errors.Is(err, resolverErr) {
		t.Fatalf("err=%v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if _, statErr := os.Stat(store.Path()); !os.IsNotExist(statErr) {
		t.Fatal("resolver failure wrote credentials")
	}
}

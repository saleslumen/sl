//go:build monorepo

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebsiteInstallScriptMatches(t *testing.T) {
	want, err := os.ReadFile("install.sh")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join("..", "website/public/sl/install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Fatal("website/public/sl/install.sh must be an exact copy of install.sh")
	}
}

func TestPublishSnapshotOmitsInternal(t *testing.T) {
	dest := t.TempDir()
	cmd := exec.Command("./.saleslumen/publish", dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("snapshot: %v\n%s", err, out)
	}
	for _, name := range []string{"AGENTS.md", ".saleslumen", "bin"} {
		if _, err := os.Stat(filepath.Join(dest, name)); err == nil {
			t.Fatalf("%s leaked into snapshot", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dest, "go.mod")); err != nil {
		t.Fatal("snapshot missing go.mod")
	}
	listed, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	tracked := map[string]struct{}{}
	for _, name := range strings.Split(string(listed), "\x00") {
		if name != "" {
			tracked[name] = struct{}{}
		}
	}
	err = filepath.Walk(dest, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		if _, ok := tracked[rel]; !ok {
			t.Errorf("untracked %s in snapshot", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

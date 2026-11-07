package main

import (
	"io/fs"
	"os"
	"testing"
)

func TestBundledSkillsMatchSource(t *testing.T) {
	want := []string{
		".agents/skills/sl/SKILL.md",
		".agents/skills/campaigns/SKILL.md",
		".agents/skills/emails/SKILL.md",
		".agents/skills/workflows/SKILL.md",
		".agents/skills/script/SKILL.md",
		".agents/skills/resources/SKILL.md",
		".agents/skills/marketplace/SKILL.md",
	}
	found := make(map[string]bool, len(want))
	err := fs.WalkDir(bundledSkills, ".agents/skills", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != "SKILL.md" {
			return nil
		}
		found[path] = true
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		bundled, err := fs.ReadFile(bundledSkills, path)
		if err != nil {
			return err
		}
		if string(bundled) != string(source) {
			t.Errorf("embedded skill differs from %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk bundled skills: %v", err)
	}
	for _, path := range want {
		if !found[path] {
			t.Errorf("missing embedded public skill %s", path)
		}
	}
	if len(found) != len(want) {
		t.Fatalf("embedded %d skill files; want %d public skills", len(found), len(want))
	}
}

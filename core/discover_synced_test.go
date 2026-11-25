package core

import (
	"path/filepath"
	"sort"
	"testing"
)

func entryNames(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}

func hasEntry(entries []Entry, name string) bool {
	for _, e := range entries {
		if e.Name == name {
			return true
		}
	}
	return false
}

// Regression: Claude desktop mirrors account skills into
// ~/.claude/skills/synced/<account-id>/<skill>/SKILL.md, which flooded the
// catalog with duplicate/off-project skills.
func TestDiscoverSkipsClaudeSyncedMirror(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	skills := filepath.Join(home, ".claude", "skills")
	writeSkill(t, skills, "real-user-skill", "a real user skill", "body")
	writeSkill(t, filepath.Join(skills, "synced", "eeaa7662-acct"), "mirrored-skill", "synced mirror", "body")
	writeSkill(t, filepath.Join(skills, "synced", "eeaa7662-acct", "nested"), "deep-mirrored", "synced mirror", "body")
	// A user skill literally named "synced" (SKILL.md directly under synced/) stays.
	writeCatalogDoc(t, filepath.Join(skills, "synced"), "SKILL.md", "name: synced\ndescription: sync helper", "body")
	// "synced" outside .claude/skills is not over-blocked.
	writeSkill(t, filepath.Join(home, ".cursor", "skills", "synced"), "cursor-synced-child", "cursor skill", "body")
	writeSkill(t, filepath.Join(home, "proj", "synced"), "proj-synced-child", "project skill", "body")

	got := DiscoverSkills([]string{
		skills,
		filepath.Join(home, ".cursor", "skills"),
		filepath.Join(home, "proj"),
	})
	for _, bad := range []string{"mirrored-skill", "deep-mirrored"} {
		if hasEntry(got, bad) {
			t.Fatalf("synced mirror skill %q discovered: %v", bad, entryNames(got))
		}
	}
	for _, want := range []string{"real-user-skill", "synced", "cursor-synced-child", "proj-synced-child"} {
		if !hasEntry(got, want) {
			t.Fatalf("expected %q in catalog: %v", want, entryNames(got))
		}
	}

	// A root pointing inside the mirror is denied outright.
	inner := DiscoverSkills([]string{filepath.Join(skills, "synced", "eeaa7662-acct")})
	if len(inner) != 0 {
		t.Fatalf("root inside .claude/skills/synced mirror must be denied: %v", entryNames(inner))
	}
}

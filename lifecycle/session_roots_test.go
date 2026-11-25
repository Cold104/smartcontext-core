package lifecycle_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Cold104/smartcontext-core/lifecycle"
)

func sessionRootsEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	fakeUser := t.TempDir() // keep ~/.claude etc. out of the catalog
	t.Setenv("HOME", fakeUser)
	t.Setenv("USERPROFILE", fakeUser)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	return home
}

func writeSessionSkill(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: a sufficiently long description for " + name +
		" so thin-skill refine proposals stay quiet\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

func countKind(d lifecycle.SkillDelta, kind string) int {
	n := 0
	for _, c := range d.Changes {
		if c.Kind == kind {
			n++
		}
	}
	return n
}

// Regression: a watermark taken over one root set was diffed against a
// different root set, so session_start listed every skill as added/removed.
func TestSessionStartRootsChangedResetsWatermark(t *testing.T) {
	home := sessionRootsEnv(t)
	rootA := t.TempDir()
	rootB := t.TempDir()
	writeSessionSkill(t, rootA, "alpha-skill")
	writeSessionSkill(t, rootB, "bravo-skill")
	writeSessionSkill(t, rootB, "charlie-skill")

	first := lifecycle.SessionStart(lifecycle.SessionStartRequest{
		Roots: []string{rootA}, Home: home, MarkSession: true, Source: "test",
	})
	if !first.MarkedSession {
		t.Fatalf("expected marked session: %+v", first)
	}

	other := lifecycle.SessionStart(lifecycle.SessionStartRequest{
		Roots: []string{rootB}, Home: home, Source: "test",
	})
	if !other.Delta.WatermarkReset {
		t.Fatalf("expected watermark reset on root change: %+v", other.Delta)
	}
	if other.Delta.WatermarkResetReason != "roots changed, watermark reset" {
		t.Fatalf("reset reason=%q", other.Delta.WatermarkResetReason)
	}
	if n := countKind(other.Delta, "added") + countKind(other.Delta, "removed"); n != 0 {
		t.Fatalf("root change must not list catalog as added/removed: %+v", other.Delta.Changes)
	}

	// Baseline now tracks rootB: a new skill there shows up as a normal delta.
	writeSessionSkill(t, rootB, "delta-skill")
	again := lifecycle.SessionStart(lifecycle.SessionStartRequest{
		Roots: []string{rootB}, Home: home, Source: "test",
	})
	if again.Delta.WatermarkReset {
		t.Fatalf("same roots must not reset: %+v", again.Delta)
	}
	if countKind(again.Delta, "added") != 1 || again.Delta.Changes[0].Name != "delta-skill" {
		t.Fatalf("expected only delta-skill added: %+v", again.Delta.Changes)
	}
}

func TestSessionStartLegacyWatermarkWithoutRootsResets(t *testing.T) {
	home := sessionRootsEnv(t)
	root := t.TempDir()
	writeSessionSkill(t, root, "alpha-skill")
	writeSessionSkill(t, root, "bravo-skill")

	legacy := map[string]any{
		"started_at":          "2026-01-01T00:00:00Z",
		"catalog_fingerprint": "old",
		"skill_names":         []string{"zulu-from-another-catalog"},
	}
	data, _ := json.Marshal(legacy)
	lifecycle.EnsureLayout(home)
	if err := os.WriteFile(filepath.Join(home, "session", "last_cursor_start.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	res := lifecycle.SessionStart(lifecycle.SessionStartRequest{Roots: []string{root}, Home: home, Source: "test"})
	if !res.Delta.WatermarkReset {
		t.Fatalf("legacy watermark without roots_hash must reset: %+v", res.Delta)
	}
	if n := countKind(res.Delta, "added") + countKind(res.Delta, "removed"); n != 0 {
		t.Fatalf("legacy reset must not list added/removed: %+v", res.Delta.Changes)
	}
}

func TestSessionStartSameRootsNormalDelta(t *testing.T) {
	home := sessionRootsEnv(t)
	root := t.TempDir()
	writeSessionSkill(t, root, "alpha-skill")
	lifecycle.SessionStart(lifecycle.SessionStartRequest{Roots: []string{root}, Home: home, MarkSession: true, Source: "test"})
	writeSessionSkill(t, root, "bravo-skill")
	res := lifecycle.SessionStart(lifecycle.SessionStartRequest{Roots: []string{root}, Home: home, Source: "test"})
	if res.Delta.WatermarkReset {
		t.Fatalf("unexpected reset: %+v", res.Delta)
	}
	if countKind(res.Delta, "added") != 1 {
		t.Fatalf("expected bravo-skill added: %+v", res.Delta.Changes)
	}
}

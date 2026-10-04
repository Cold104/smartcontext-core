package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRankIDFPrefersRareDomainTerms(t *testing.T) {
	entries := []Entry{
		{Name: "token-pack", Kind: "skill", Description: "pack skill context under token budget", Path: "/a", OS: "any"},
		{Name: "hermes-agent-team", Kind: "skill", Description: "docker personas for hermes", Path: "/b", OS: "any"},
		{Name: "unrelated-cooking", Kind: "skill", Description: "sourdough bread recipes", Path: "/c", OS: "any"},
	}
	ranked := rankAndSelect(entries, "token budget packing for cursor", 2)
	if len(ranked) < 1 || ranked[0].entry.Name != "token-pack" {
		t.Fatalf("got %#v", ranked)
	}
}

func TestRankIDFNameBoostBeatsDescriptionOnly(t *testing.T) {
	entries := []Entry{
		{Name: "other", Kind: "skill", Description: "mentions windows paths briefly", Path: "/a", OS: "any"},
		{Name: "windows-dev", Kind: "skill", Description: "general tooling", Path: "/b", OS: "any"},
	}
	ranked := rankAndSelect(entries, "windows-dev powershell", 2)
	if len(ranked) < 1 || ranked[0].entry.Name != "windows-dev" {
		t.Fatalf("got %#v", ranked)
	}
}

func TestRankZeroHitKeepsDiscoveryOrder(t *testing.T) {
	entries := []Entry{
		{Name: "aaa", Kind: "skill", Description: "alpha", Path: "/a", OS: "any"},
		{Name: "zzz", Kind: "skill", Description: "omega", Path: "/z", OS: "any"},
	}
	ranked := rankAndSelect(entries, "qqqqxxxx", 2)
	if len(ranked) != 2 {
		t.Fatalf("len=%d", len(ranked))
	}
	if ranked[0].entry.Name != "aaa" || ranked[1].entry.Name != "zzz" {
		t.Fatalf("expected discovery order, got %#v", ranked)
	}
}

func TestRankOSPrefersMatchingHost(t *testing.T) {
	host := HostOSCanonical()
	other := "linux"
	if host == "linux" {
		other = "windows"
	}
	entries := []Entry{
		{Name: "host-os-other", Kind: "skill", Description: "os specific paths shell", Path: "/o", OS: other},
		{Name: "host-os-match", Kind: "skill", Description: "os specific paths shell", Path: "/m", OS: host},
		{Name: "host-os-any", Kind: "skill", Description: "os specific paths shell", Path: "/a", OS: "any"},
	}
	ranked := rankAndSelect(entries, "os specific paths shell", 3)
	if len(ranked) < 2 {
		t.Fatalf("len=%d", len(ranked))
	}
	if ranked[0].entry.Name != "host-os-match" {
		t.Fatalf("expected OS match first, got %#v (host=%s)", ranked, host)
	}
	// Mismatched OS should rank below any/match when scores otherwise equal-ish.
	var otherIdx, anyIdx = -1, -1
	for i, s := range ranked {
		if s.entry.Name == "host-os-other" {
			otherIdx = i
		}
		if s.entry.Name == "host-os-any" {
			anyIdx = i
		}
	}
	if otherIdx >= 0 && anyIdx >= 0 && otherIdx < anyIdx {
		t.Fatalf("mismatched OS should not beat any: %#v", ranked)
	}
}

func TestRankAliasBoost(t *testing.T) {
	entries := []Entry{
		{Name: "reference-and-skillize", Kind: "skill", Description: "docs extract create", Path: "/r", OS: "any"},
		{Name: "token-pack", Kind: "skill", Description: "pack tokens", Path: "/t", OS: "any",
			Aliases: []string{"tf foreach", "for each invalid"}},
	}
	ranked := rankAndSelect(entries, "fix tf foreach on modules", 2)
	if len(ranked) < 1 || ranked[0].entry.Name != "token-pack" {
		t.Fatalf("alias should boost token-pack, got %#v", ranked)
	}
}

func TestRankDeterministicTieBreakNameAsc(t *testing.T) {
	entries := []Entry{
		{Name: "zeta-skill", Kind: "skill", Description: "shared topic widget", Path: "/z", OS: "any"},
		{Name: "alpha-skill", Kind: "skill", Description: "shared topic widget", Path: "/a", OS: "any"},
	}
	ranked := rankAndSelect(entries, "shared topic widget", 2)
	if len(ranked) != 2 {
		t.Fatalf("len=%d", len(ranked))
	}
	if ranked[0].score != ranked[1].score {
		// Still require alpha before zeta when scores equal; if IDF differs, skip.
		t.Skip("scores not tied")
	}
	if ranked[0].entry.Name != "alpha-skill" {
		t.Fatalf("tie-break name asc failed: %#v", ranked)
	}
}

func TestRankAttachesMatchingRules(t *testing.T) {
	entries := []Entry{
		{Name: "pack-op", Kind: "skill", Description: "pack context workflow", Path: "/p", OS: "any", Tags: []string{"packing"}},
		{Name: "other-skill", Kind: "skill", Description: "unrelated cooking", Path: "/o", OS: "any"},
		{Name: "secure-pack-hygiene", Kind: "rule", Description: "no secrets in packs", Path: "/r", OS: "any",
			AppliesTo: []string{"pack-op", "tag:packing"}},
		{Name: "unrelated-rule", Kind: "rule", Description: "finance only", Path: "/f", OS: "any",
			AppliesTo: []string{"finance-skill"}},
	}
	ranked := rankAndSelect(entries, "pack context workflow", 1)
	names := make([]string, 0, len(ranked))
	for _, s := range ranked {
		names = append(names, s.entry.Name)
	}
	if len(ranked) < 2 || ranked[0].entry.Name != "pack-op" {
		t.Fatalf("expected pack-op then rule, got %v", names)
	}
	foundSecure, foundUnrelated := false, false
	for _, s := range ranked {
		if s.entry.Name == "secure-pack-hygiene" {
			foundSecure = true
		}
		if s.entry.Name == "unrelated-rule" {
			foundUnrelated = true
		}
	}
	if !foundSecure {
		t.Fatalf("expected secure rule attached: %v", names)
	}
	if foundUnrelated {
		t.Fatalf("unrelated rule should not attach: %v", names)
	}
}

func TestCollectRelatedNames(t *testing.T) {
	items := []PackItem{
		{Name: "a", Kind: "skill", Related: []string{"b", "c"}},
		{Name: "b", Kind: "skill", Related: []string{"d"}},
		{Name: "rule-x", Kind: "rule", Related: []string{"ignored"}},
	}
	got := CollectRelatedNames(items)
	// b already selected → omit; c and d remain; rule related ignored.
	if len(got) != 2 || got[0] != "c" || got[1] != "d" {
		t.Fatalf("got %v", got)
	}
}

func TestPackExposesRelated(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "alpha-pack")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: alpha-pack\ndescription: alpha packing workflow\nos: any\nkind: skill\nrelated: [beta-fallback, gamma-alt]\naliases: [alpha pack]\n---\n# Alpha\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", filepath.Join(root, "skills"))
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")

	res := PackContext(PackRequest{
		Task:         "alpha packing workflow",
		TopN:         3,
		BudgetPreset: "super_saver",
	}, "1.1.0")
	if res.BudgetTokens != BudgetTierSuperSaver {
		t.Fatalf("budget=%d", res.BudgetTokens)
	}
	if len(res.Related) < 2 {
		t.Fatalf("related=%v selected=%v", res.Related, res.SelectedNames)
	}
	_ = runtime.GOOS
}

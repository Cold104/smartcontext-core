package lifecycle

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cold104/smartcontext-core/core"
)

// trajectoryDriftTurns are shared mock multi-turn task phrasings: turn 1 is
// goal-aligned; later turns drift toward noise so sticky vs non-sticky diverge.
var trajectoryDriftTurns = []string{
	"goal objective workflow needle carrier",
	"brief docker compose check while staying on the goal",
	"kubernetes cluster noise investigation side quest",
	"unrelated terraform aws noise only please",
	"more docker kubernetes noise — ignore the original goal wording",
}

// TestFiveTurnStickyTrajectoryDrift is the Phase-3 multi-turn harness:
// task phrasing drifts across 5 packs; sticky session keeps the needle skill
// and recall@5 / sticky names hold with SMARTCONTEXT_STICKY_SESSION on.
func TestFiveTurnStickyTrajectoryDrift(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "1")

	root := t.TempDir()
	const needle = "NEEDLE_TRAJECTORY_5turn_a7c1"
	writeLCSkill(t, root, "goal-skill", "goal objective workflow needle carrier",
		"Procedure retaining "+needle+" for five-turn recoverability.")
	writeLCSkill(t, root, "noise-skill", "docker kubernetes terraform aws noise", "noise body only")
	writeLCSkill(t, root, "filler-skill", "generic helper utility filler", "filler body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	_ = ClearStickyNames(home)
	ResetMemoryEvents()

	gold := []string{"goal-skill"}

	for i, task := range trajectoryDriftTurns {
		res := PackContext(core.PackRequest{
			Task: task, TopN: 5, BudgetTokens: 4000, IncludeBodies: true,
		})
		if !containsName(res.SelectedNames, "goal-skill") {
			t.Fatalf("turn %d: sticky dropped goal-skill; selected=%v task=%q", i+1, res.SelectedNames, task)
		}
		// recall@5: gold skill must remain among TopN≤5 selected names under sticky session.
		if len(res.SelectedNames) > 5 {
			t.Fatalf("turn %d: selected_count=%d exceeds TopN=5", i+1, len(res.SelectedNames))
		}
		if !hitAny(res.SelectedNames, gold) {
			t.Fatalf("turn %d: recall@5 miss for gold %v; selected=%v", i+1, gold, res.SelectedNames)
		}
		if !bytes.Contains([]byte(res.Markdown), []byte(needle)) {
			t.Fatalf("turn %d: needle missing from pack markdown", i+1)
		}
		sticky := LoadStickyNames(home)
		if !containsName(sticky, "goal-skill") {
			t.Fatalf("turn %d: session sticky file missing goal-skill: %v", i+1, sticky)
		}
	}
}

// TestFiveTurnStickyOffControl is the sticky-OFF contrast for the same drift
// turns: without SMARTCONTEXT_STICKY_SESSION, goal-skill is not force-pinned and
// drops once noise tasks outrank it under a tight TopN.
func TestFiveTurnStickyOffControl(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "0")

	root := t.TempDir()
	const needle = "NEEDLE_TRAJECTORY_sticky_off_b2d4"
	writeLCSkill(t, root, "goal-skill", "goal objective workflow needle carrier",
		"Procedure retaining "+needle+" for sticky-off control.")
	writeLCSkill(t, root, "noise-docker", "docker compose kubernetes noise", "docker noise body")
	writeLCSkill(t, root, "noise-k8s", "kubernetes cluster investigation noise", "k8s noise body")
	writeLCSkill(t, root, "noise-terraform", "terraform aws noise only", "tf noise body")
	writeLCSkill(t, root, "filler-skill", "generic helper utility filler", "filler body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	_ = ClearStickyNames(home)
	ResetMemoryEvents()

	const topN = 2
	dropped := false
	for i, task := range trajectoryDriftTurns {
		res := PackContext(core.PackRequest{
			Task: task, TopN: topN, BudgetTokens: 4000, IncludeBodies: true,
		})
		if len(res.SelectedNames) > topN {
			t.Fatalf("turn %d: selected_count=%d exceeds TopN=%d", i+1, len(res.SelectedNames), topN)
		}
		if i == 0 {
			if !containsName(res.SelectedNames, "goal-skill") {
				t.Fatalf("turn 1: expected goal-skill under goal-aligned task; selected=%v", res.SelectedNames)
			}
			if !bytes.Contains([]byte(res.Markdown), []byte(needle)) {
				t.Fatalf("turn 1: needle missing from pack markdown")
			}
			continue
		}
		if !containsName(res.SelectedNames, "goal-skill") {
			dropped = true
			if bytes.Contains([]byte(res.Markdown), []byte(needle)) {
				t.Fatalf("turn %d: needle present after goal-skill dropped; selected=%v", i+1, res.SelectedNames)
			}
		}
	}
	if !dropped {
		t.Fatal("sticky OFF control: goal-skill never dropped under drift (expected at least one late turn without it)")
	}
	if sticky := LoadStickyNames(home); len(sticky) != 0 {
		t.Fatalf("sticky OFF should not persist session sticky names; got %v", sticky)
	}
}

// TestFiveTurnStableHashStickyStable asserts stable_prefix_hash is identical
// across 5 packs when sticky session keeps the selected skill set unchanged.
func TestFiveTurnStableHashStickyStable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "1")

	root := t.TempDir()
	writeLCSkill(t, root, "goal-skill", "goal objective workflow needle carrier", "GOAL_BODY_stable_hash")
	writeLCSkill(t, root, "noise-skill", "docker kubernetes terraform aws noise", "noise body")
	writeLCSkill(t, root, "filler-skill", "generic helper utility filler", "filler body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	_ = ClearStickyNames(home)
	ResetMemoryEvents()

	var firstStable string
	var firstNames []string
	for i, task := range trajectoryDriftTurns {
		res := PackContext(core.PackRequest{
			Task: task, TopN: 3, BudgetTokens: 4000, IncludeBodies: true,
		})
		if i == 0 {
			firstStable = res.MarkdownStable
			firstNames = append([]string(nil), res.SelectedNames...)
			if firstStable == "" || len(firstNames) == 0 {
				t.Fatal("turn 1: empty stable markdown or selection")
			}
			continue
		}
		if !sameNameSet(firstNames, res.SelectedNames) {
			t.Fatalf("turn %d: selection drifted under sticky; want %v got %v", i+1, firstNames, res.SelectedNames)
		}
		if !bytes.Equal([]byte(firstStable), []byte(res.MarkdownStable)) {
			t.Fatalf("turn %d: markdown_stable changed despite sticky-stable selection", i+1)
		}
	}

	path := filepath.Join(home, "telemetry", "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) != len(trajectoryDriftTurns) {
		t.Fatalf("expected %d pack telemetry lines, got %d", len(trajectoryDriftTurns), len(lines))
	}
	wantHash := HashStablePrefix(firstStable)
	var firstHash string
	for i, line := range lines {
		var ev PackEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("turn %d: unmarshal pack event: %v", i+1, err)
		}
		if ev.StablePrefixHash == "" || ev.StablePrefixHash != wantHash {
			t.Fatalf("turn %d: stable_prefix_hash=%q want %q", i+1, ev.StablePrefixHash, wantHash)
		}
		if i == 0 {
			firstHash = ev.StablePrefixHash
			continue
		}
		if ev.StablePrefixHash != firstHash {
			t.Fatalf("turn %d: stable_prefix_hash diverged: %q vs turn1 %q", i+1, ev.StablePrefixHash, firstHash)
		}
		if !ev.CacheFriendly {
			t.Fatalf("turn %d: cache_friendly want true", i+1)
		}
	}
}

// TestTightBudgetTrajectoryNeedlePolicy documents and asserts fitItemsToBudget
// truncation under a small BudgetTokens mid-trajectory:
//
//  1. Skill cards are preferred; bodies shrink to remaining*4 chars with
//     "… [truncated for token budget]" when the body exceeds that cap.
//  2. Needle policy is prefix-keep: a needle near the start of a selected
//     skill body survives; a needle buried after a long body prefix is cut.
//  3. Sticky keeps the goal skill selected so the early needle remains
//     recoverable even when late body content is truncated away.
func TestTightBudgetTrajectoryNeedlePolicy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "1")

	root := t.TempDir()
	const earlyNeedle = "NEEDLE_EARLY_tight_budget_c9e2"
	const lateNeedle = "NEEDLE_LATE_tight_budget_d0f3"
	// Body stays within DefaultBodyChars (1200) so a roomy pack keeps both needles,
	// but late needle sits past the tight-budget remaining*4 body cap (prefix-keep).
	pad := strings.Repeat("PAD.", 220) // ~880 chars between needles
	body := earlyNeedle + "\n" + pad + "\n" + lateNeedle + "\n"
	writeLCSkill(t, root, "goal-skill", "goal objective workflow needle carrier", body)
	writeLCSkill(t, root, "noise-skill", "docker kubernetes terraform aws noise",
		strings.Repeat("noise padding line\n", 80))
	writeLCSkill(t, root, "filler-skill", "generic helper utility filler", "filler body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	_ = ClearStickyNames(home)
	ResetMemoryEvents()

	// Warm sticky with a roomy budget so goal is selected and persisted.
	warm := PackContext(core.PackRequest{
		Task: trajectoryDriftTurns[0], TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
	})
	if !containsName(warm.SelectedNames, "goal-skill") {
		t.Fatalf("warm pack missing goal-skill: %v", warm.SelectedNames)
	}
	if !strings.Contains(warm.Markdown, earlyNeedle) || !strings.Contains(warm.Markdown, lateNeedle) {
		t.Fatalf("warm pack should include both needles under roomy budget\nmarkdown:\n%s", warm.Markdown)
	}

	// Tight mid-trajectory turn: same sticky selection, small absolute budget.
	const tightBudget = 180
	tight := PackContext(core.PackRequest{
		Task: trajectoryDriftTurns[2], TopN: 2, BudgetTokens: tightBudget, IncludeBodies: true,
	})
	if tight.BudgetTokens != tightBudget {
		t.Fatalf("budget_tokens=%d want %d", tight.BudgetTokens, tightBudget)
	}
	if !containsName(tight.SelectedNames, "goal-skill") {
		t.Fatalf("tight turn: sticky dropped goal-skill; selected=%v", tight.SelectedNames)
	}
	truncated := false
	for _, it := range tight.Items {
		if strings.Contains(it.Body, "truncated for token budget") {
			truncated = true
		}
	}
	if !strings.Contains(tight.Markdown, earlyNeedle) {
		t.Fatal("tight budget must keep early (prefix) needle in selected goal body")
	}
	if strings.Contains(tight.Markdown, lateNeedle) {
		t.Fatalf("tight budget should truncate away late needle (prefix-keep policy)\nmarkdown:\n%s", tight.Markdown)
	}
	if !truncated {
		t.Fatal("expected at least one body with \"truncated for token budget\" under tight BudgetTokens")
	}
	if !strings.Contains(tight.Markdown, "truncated for token budget") {
		t.Fatal("truncation marker expected in pack markdown when item body was truncated")
	}
}

// TestOffloadMidTrajectoryRecallHint stores a large blob mid-trajectory and
// asserts the next pack's markdown_dynamic carries the offload path + recall hint
// without polluting markdown_stable.
func TestOffloadMidTrajectoryRecallHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "1")

	root := t.TempDir()
	const needle = "NEEDLE_TRAJECTORY_offload_e1a8"
	writeLCSkill(t, root, "goal-skill", "goal objective workflow needle carrier",
		"Procedure retaining "+needle+" across offload mid-trajectory.")
	writeLCSkill(t, root, "noise-skill", "docker kubernetes terraform aws noise", "noise body")
	writeLCSkill(t, root, "filler-skill", "generic helper utility filler", "filler body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	_ = ClearStickyNames(home)
	ResetMemoryEvents()

	// Turns 1–2: sticky trajectory warm-up.
	for i := 0; i < 2; i++ {
		res := PackContext(core.PackRequest{
			Task: trajectoryDriftTurns[i], TopN: 5, BudgetTokens: 4000, IncludeBodies: true,
		})
		if !containsName(res.SelectedNames, "goal-skill") {
			t.Fatalf("turn %d: sticky dropped goal-skill; selected=%v", i+1, res.SelectedNames)
		}
	}

	big := strings.Repeat("OFFLOAD_MID_TRAJECTORY_BLOB\n", 2000)
	if len(big) < OffloadThresholdBytes {
		t.Fatalf("fixture too small: %d < %d", len(big), OffloadThresholdBytes)
	}
	out, err := OffloadToolResult(OffloadRequest{Content: big, Label: "mid-traj-log", PreviewBytes: 80})
	if err != nil || !out.Stored {
		t.Fatalf("mid-trajectory offload failed: err=%v out=%+v", err, out)
	}

	// Next pack after offload: episodic dynamic must surface path + recall hint.
	next := PackContext(core.PackRequest{
		Task: trajectoryDriftTurns[2], TopN: 5, BudgetTokens: 4000, IncludeBodies: true,
	})
	if !containsName(next.SelectedNames, "goal-skill") {
		t.Fatalf("post-offload pack dropped goal-skill: %v", next.SelectedNames)
	}
	if !strings.Contains(next.MarkdownDynamic, "Tool-result recall") {
		t.Fatalf("markdown_dynamic missing recall hint:\n%s", next.MarkdownDynamic)
	}
	if !strings.Contains(next.MarkdownDynamic, out.Path) {
		t.Fatalf("markdown_dynamic missing offload path %s:\n%s", out.Path, next.MarkdownDynamic)
	}
	if !strings.Contains(next.MarkdownDynamic, "OFFLOAD_MID_TRAJECTORY_BLOB") {
		t.Fatalf("markdown_dynamic missing preview needle:\n%s", next.MarkdownDynamic)
	}
	if strings.Contains(next.MarkdownStable, "Tool-result recall") {
		t.Fatal("recall hint must not pollute markdown_stable")
	}
	if !bytes.Contains([]byte(next.Markdown), []byte(needle)) {
		t.Fatal("goal needle missing after mid-trajectory offload")
	}
}

// TestCacheHitProxySequentialStablePrefix asserts sequential packs with identical
// selection yield byte-equal markdown_stable, 100% prefix overlap, and matching
// stable_prefix_hash / cache_friendly telemetry.
func TestCacheHitProxySequentialStablePrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "1")

	root := t.TempDir()
	writeLCSkill(t, root, "alpha-skill", "alpha workflow helper", "ALPHA_BODY\nline-two")
	writeLCSkill(t, root, "zeta-skill", "zeta workflow helper", "ZETA_BODY")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	_ = ClearStickyNames(home)
	ResetMemoryEvents()

	a := PackContext(core.PackRequest{
		Task: "first task phrasing about alpha zeta", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
		StickyNames: []string{"alpha-skill", "zeta-skill"},
	})
	b := PackContext(core.PackRequest{
		Task: "second task phrasing totally different", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
		StickyNames: []string{"alpha-skill", "zeta-skill"},
	})

	if len(a.SelectedNames) == 0 {
		t.Fatal("expected non-empty selection")
	}
	if !bytes.Equal([]byte(a.MarkdownStable), []byte(b.MarkdownStable)) {
		t.Fatalf("markdown_stable must be byte-identical for identical selection\n---a---\n%q\n---b---\n%q",
			a.MarkdownStable, b.MarkdownStable)
	}
	overlap := core.IdenticalPrefixOverlapPct(a.MarkdownStable, b.MarkdownStable)
	if overlap != 100 {
		t.Fatalf("identical-prefix overlap%%=%v want 100", overlap)
	}

	path := filepath.Join(home, "telemetry", "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("expected ≥2 pack telemetry lines, got %d", len(lines))
	}
	var evA, evB PackEvent
	if err := json.Unmarshal(lines[len(lines)-2], &evA); err != nil {
		t.Fatalf("unmarshal first pack event: %v", err)
	}
	if err := json.Unmarshal(lines[len(lines)-1], &evB); err != nil {
		t.Fatalf("unmarshal second pack event: %v", err)
	}
	wantHash := HashStablePrefix(a.MarkdownStable)
	if evA.StablePrefixHash == "" || evA.StablePrefixHash != wantHash {
		t.Fatalf("stable_prefix_hash a=%q want %q", evA.StablePrefixHash, wantHash)
	}
	if evB.StablePrefixHash != evA.StablePrefixHash {
		t.Fatalf("stable_prefix_hash mismatch across identical selection: %q vs %q",
			evA.StablePrefixHash, evB.StablePrefixHash)
	}
	if !evA.CacheFriendly || !evB.CacheFriendly {
		t.Fatalf("cache_friendly want true; got a=%v b=%v", evA.CacheFriendly, evB.CacheFriendly)
	}
}

func hitAny(selected, gold []string) bool {
	set := map[string]struct{}{}
	for _, n := range selected {
		set[n] = struct{}{}
	}
	for _, g := range gold {
		if _, ok := set[g]; ok {
			return true
		}
	}
	return false
}

func sameNameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]struct{}{}
	for _, n := range a {
		set[n] = struct{}{}
	}
	for _, n := range b {
		if _, ok := set[n]; !ok {
			return false
		}
	}
	return true
}

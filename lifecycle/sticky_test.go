package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cold104/smartcontext-core/core"
)

func TestStickySessionPersistsAcrossPacks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "1")

	root := t.TempDir()
	writeLCSkill(t, root, "goal-skill", "goal objective workflow", "NEEDLE_STICKY_zzz")
	writeLCSkill(t, root, "noise-skill", "docker kubernetes noise", "noise")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")

	first := PackContext(core.PackRequest{
		Task: "goal objective workflow", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
	})
	if !containsName(first.SelectedNames, "goal-skill") {
		t.Fatalf("first pack missing goal-skill: %v", first.SelectedNames)
	}
	sticky := LoadStickyNames(home)
	if !containsName(sticky, "goal-skill") {
		t.Fatalf("sticky not persisted: %v", sticky)
	}

	second := PackContext(core.PackRequest{
		Task: "docker kubernetes noise only", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
	})
	if !containsName(second.SelectedNames, "goal-skill") {
		t.Fatalf("session sticky failed to retain goal-skill: %v", second.SelectedNames)
	}
	if !strings.Contains(second.Markdown, "NEEDLE_STICKY_zzz") {
		t.Fatalf("needle missing after sticky re-pack")
	}
}

func TestOffloadToolResult(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	big := strings.Repeat("LOG_LINE\n", 5000)
	if len(big) < OffloadThresholdBytes {
		t.Fatalf("fixture too small: %d < %d", len(big), OffloadThresholdBytes)
	}
	out, err := OffloadToolResult(OffloadRequest{Content: big, Label: "ci-log", PreviewBytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Stored {
		t.Fatalf("expected stored=true: %+v", out)
	}
	if out.Bytes != len(big) {
		t.Fatalf("bytes=%d want %d", out.Bytes, len(big))
	}
	if !out.Truncated {
		t.Fatalf("expected truncated preview")
	}
	if _, err := os.Stat(out.Path); err != nil {
		t.Fatalf("missing offload file: %v", err)
	}
	if out.SHA256 == "" || out.Preview == "" {
		t.Fatalf("incomplete offload result: %+v", out)
	}
	ptrs := LoadOffloadPointers(home)
	if len(ptrs) != 1 || ptrs[0].Path != out.Path {
		t.Fatalf("session pointer not persisted: %+v", ptrs)
	}
}

func TestOffloadBelowThresholdNoOp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	small := strings.Repeat("x", OffloadThresholdBytes-1)
	out, err := OffloadToolResult(OffloadRequest{Content: small, Label: "tiny"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stored {
		t.Fatalf("expected stored=false under threshold: %+v", out)
	}
	if out.Path != "" {
		t.Fatalf("no-op should not set path: %+v", out)
	}
	if out.ThresholdBytes != OffloadThresholdBytes {
		t.Fatalf("threshold_bytes=%d want %d", out.ThresholdBytes, OffloadThresholdBytes)
	}
	if !strings.Contains(out.Note, "below") {
		t.Fatalf("note should explain no-op: %q", out.Note)
	}
	entries, _ := os.ReadDir(filepath.Join(home, "offload"))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".txt") {
			t.Fatalf("no-op wrote offload file: %s", e.Name())
		}
	}

	forced, err := OffloadToolResult(OffloadRequest{Content: small, Label: "tiny-force", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !forced.Stored || forced.Path == "" {
		t.Fatalf("force=true should store under threshold: %+v", forced)
	}
}

func TestOffloadEnabledDefaultOn(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want bool
	}{
		{name: "empty", env: "", want: true},
		{name: "FALSE", env: "FALSE", want: false},
		{name: "Disabled_ws", env: " Disabled ", want: false},
		{name: "zero", env: "0", want: false},
		{name: "No", env: "No", want: false},
		{name: "OFF", env: "OFF", want: false},
		{name: "false", env: "false", want: false},
		{name: "off", env: "off", want: false},
		{name: "disabled", env: "disabled", want: false},
		{name: "garbage_on", env: "garbage", want: true},
		{name: "one", env: "1", want: true},
		{name: "true", env: "true", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SMARTCONTEXT_OFFLOAD", tc.env)
			if got := OffloadEnabled(); got != tc.want {
				t.Fatalf("OffloadEnabled()=%v want %v for %q", got, tc.want, tc.env)
			}
		})
	}
}

func TestOffloadRecallHintInMarkdownDynamic(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "0")

	root := t.TempDir()
	writeLCSkill(t, root, "recall-skill", "tool result recall offload", "needle body")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")

	big := strings.Repeat("OFFLOAD_NEEDLE\n", 2000)
	out, err := OffloadToolResult(OffloadRequest{Content: big, Label: "recall-log", PreviewBytes: 64})
	if err != nil || !out.Stored {
		t.Fatalf("offload failed: err=%v out=%+v", err, out)
	}

	pack := PackContext(core.PackRequest{
		Task: "tool result recall offload", TopN: 2, BudgetTokens: 4000, IncludeBodies: true,
	})
	if !strings.Contains(pack.MarkdownDynamic, "Tool-result recall") {
		t.Fatalf("markdown_dynamic missing recall section:\n%s", pack.MarkdownDynamic)
	}
	if !strings.Contains(pack.MarkdownDynamic, out.Path) {
		t.Fatalf("markdown_dynamic missing path %s:\n%s", out.Path, pack.MarkdownDynamic)
	}
	if !strings.Contains(pack.MarkdownDynamic, "OFFLOAD_NEEDLE") {
		t.Fatalf("markdown_dynamic missing preview needle:\n%s", pack.MarkdownDynamic)
	}
	if strings.Contains(pack.MarkdownStable, "Tool-result recall") {
		t.Fatal("recall hint must not pollute markdown_stable")
	}
	if !strings.HasPrefix(pack.Markdown, pack.MarkdownStable) {
		t.Fatal("markdown should start with stable prefix")
	}
	if !strings.Contains(pack.Markdown, pack.MarkdownDynamic) {
		t.Fatal("markdown should include dynamic including recall hint")
	}
}

// TestOffloadPreviewBudget documents the 1.2 default (IndustryAsk=200) and the
// legacy 2048 constant hosts may still pass via preview_bytes.
func TestOffloadPreviewBudget(t *testing.T) {
	if LegacyOffloadPreviewBytes != 2048 {
		t.Fatalf("LegacyOffloadPreviewBytes=%d; expected 2048", LegacyOffloadPreviewBytes)
	}
	if DefaultOffloadPreviewBytes != IndustryAskOffloadPreviewBytes {
		t.Fatalf("DefaultOffloadPreviewBytes=%d; want IndustryAsk=%d", DefaultOffloadPreviewBytes, IndustryAskOffloadPreviewBytes)
	}
	if IndustryAskOffloadPreviewBytes != 200 {
		t.Fatalf("IndustryAskOffloadPreviewBytes=%d; want 200", IndustryAskOffloadPreviewBytes)
	}
	if IndustryAskOffloadPreviewBytes >= LegacyOffloadPreviewBytes {
		t.Fatalf("industry ask (%d) should be < legacy (%d)", IndustryAskOffloadPreviewBytes, LegacyOffloadPreviewBytes)
	}

	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	// Content larger than threshold and both preview budgets.
	big := strings.Repeat("x", OffloadThresholdBytes+LegacyOffloadPreviewBytes)

	def, err := OffloadToolResult(OffloadRequest{Content: big, Label: "default-preview"})
	if err != nil {
		t.Fatal(err)
	}
	if !def.Truncated {
		t.Fatal("default preview should truncate oversized content")
	}
	if !strings.HasPrefix(def.Preview, strings.Repeat("x", DefaultOffloadPreviewBytes)) {
		t.Fatalf("default preview should start with %d content bytes", DefaultOffloadPreviewBytes)
	}

	legacy, err := OffloadToolResult(OffloadRequest{
		Content: big, Label: "legacy-preview", PreviewBytes: LegacyOffloadPreviewBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !legacy.Truncated {
		t.Fatal("legacy preview should truncate")
	}
	if !strings.HasPrefix(legacy.Preview, strings.Repeat("x", LegacyOffloadPreviewBytes)) {
		t.Fatalf("legacy preview should start with %d content bytes", LegacyOffloadPreviewBytes)
	}
	if len(def.Preview) >= len(legacy.Preview) {
		t.Fatalf("default preview (%d) should be shorter than legacy (%d)", len(def.Preview), len(legacy.Preview))
	}
}

func writeLCSkill(t *testing.T, root, name, desc, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: " + desc + "\nkind: skill\nos: any\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

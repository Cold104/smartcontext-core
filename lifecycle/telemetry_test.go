package lifecycle_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/lifecycle"
)

func TestTelemetryFileAndMemory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	lifecycle.ResetMemoryEvents()

	lifecycle.RecordEvent("session_start", map[string]any{"skill_count": 1}, "test", home)
	ev := lifecycle.MemoryEvents()
	if len(ev) != 1 {
		t.Fatalf("memory events=%d", len(ev))
	}
	path := filepath.Join(home, "telemetry", "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"type":"session_start"`) {
		t.Fatalf("events file missing session_start: %s", data)
	}
	if !strings.Contains(string(data), `"timestamp":`) {
		t.Fatalf("events file missing timestamp: %s", data)
	}
}

func TestPackTelemetryContractNoRawTask(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	lifecycle.ResetMemoryEvents()

	root := t.TempDir()
	skillDir := filepath.Join(root, "contract-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: contract-skill\ndescription: pack telemetry contract fixture\n---\n\n# Contract\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	secretTask := "UNIQUE_RAW_TASK_LEAK_PROBE_xyz_do_not_store_in_jsonl"
	res := lifecycle.PackContext(core.PackRequest{
		Task:         secretTask,
		Roots:        []string{root},
		BudgetPreset: "medium",
		TopN:         3,
	})
	if res.CatalogSize < 1 {
		t.Fatalf("catalog_size=%d note=%s", res.CatalogSize, res.Note)
	}

	path := filepath.Join(home, "telemetry", "events.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)
	if strings.Contains(raw, secretTask) {
		t.Fatal("raw task leaked into telemetry JSONL")
	}
	if strings.Contains(raw, `"task":`) {
		t.Fatalf("unexpected task field in telemetry JSONL: %s", raw)
	}

	var ev lifecycle.PackEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("unmarshal pack event: %v\n%s", err, raw)
	}
	if ev.Type != lifecycle.EventTypePack {
		t.Fatalf("type=%q", ev.Type)
	}
	if ev.Source != lifecycle.SourceMCP {
		t.Fatalf("source=%q", ev.Source)
	}
	if ev.Timestamp == "" || ev.Version == "" {
		t.Fatalf("missing timestamp/version: %+v", ev)
	}
	if ev.OS != runtime.GOOS || ev.Arch != runtime.GOARCH {
		t.Fatalf("os/arch=%s/%s want %s/%s", ev.OS, ev.Arch, runtime.GOOS, runtime.GOARCH)
	}
	if ev.RootCount < 1 || ev.CatalogSize < 1 {
		t.Fatalf("root_count=%d catalog_size=%d", ev.RootCount, ev.CatalogSize)
	}
	if ev.SelectedCount != res.SelectedCount {
		t.Fatalf("selected_count=%d want %d", ev.SelectedCount, res.SelectedCount)
	}
	if ev.BudgetTokens != res.BudgetTokens {
		t.Fatalf("budget_tokens=%d want %d", ev.BudgetTokens, res.BudgetTokens)
	}
	if ev.BudgetPreset != res.BudgetPreset {
		t.Fatalf("budget_preset=%q want %q", ev.BudgetPreset, res.BudgetPreset)
	}
	if ev.EstimatedTokens != res.EstimatedTokens {
		t.Fatalf("estimated_tokens=%d want %d", ev.EstimatedTokens, res.EstimatedTokens)
	}
	if ev.LatencyMS < 0 {
		t.Fatalf("latency_ms=%v", ev.LatencyMS)
	}
	if ev.EmptyCatalog != res.EmptyCatalog {
		t.Fatalf("empty_catalog=%v want %v", ev.EmptyCatalog, res.EmptyCatalog)
	}
	wantScrub := lifecycle.ScrubFlagCount(res.Security)
	if ev.ScrubFlagCount != wantScrub {
		t.Fatalf("scrub_flag_count=%d want %d", ev.ScrubFlagCount, wantScrub)
	}
	wantHash := lifecycle.HashTask(secretTask)
	if ev.TaskHash != wantHash {
		t.Fatalf("task_hash=%q want %q", ev.TaskHash, wantHash)
	}
	wantPrefix := lifecycle.HashStablePrefix(res.MarkdownStable)
	if ev.StablePrefixHash != wantPrefix {
		t.Fatalf("stable_prefix_hash=%q want %q", ev.StablePrefixHash, wantPrefix)
	}
	if !ev.CacheFriendly {
		t.Fatal("cache_friendly want true for non-empty markdown_stable")
	}
	if strings.Contains(raw, "# Contract") {
		t.Fatal("skill body fragment leaked into telemetry JSONL")
	}

	mem := lifecycle.MemoryEvents()
	if len(mem) != 1 {
		t.Fatalf("memory events=%d", len(mem))
	}
	if _, ok := mem[0]["task"]; ok {
		t.Fatal("memory event contains raw task key")
	}
	if strings.Contains(mustJSON(t, mem[0]), secretTask) {
		t.Fatal("raw task leaked into memory events")
	}
}

func TestNewPackEventOmitsEmptyTaskHash(t *testing.T) {
	ev := lifecycle.NewPackEvent(core.PackResult{Version: "1.1.0"}, 1.5, "test", "")
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "task_hash") {
		t.Fatalf("empty task_hash should be omitted: %s", raw)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

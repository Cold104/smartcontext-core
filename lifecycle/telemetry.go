package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/Cold104/smartcontext-core/core"
)

// Source labels telemetry origin (mcp vs future CLI).
const SourceMCP = "mcp"

// EventTypePack is the local pack sensor event (see docs/TELEMETRY_CONTRACT.md).
const EventTypePack = "pack"

var (
	memMu     sync.Mutex
	memEvents []map[string]any

	// Process-local memo for stable_prefix_hash: reuse hex when MarkdownStable
	// is byte-identical to the previous pack (skip a second full-string SHA-256).
	stableHashMu   sync.Mutex
	lastStableMD   string
	lastStableHash string
)

// PackEvent is the local-only pack telemetry contract (flat JSONL line).
// Never includes raw task text, skill bodies, or file contents.
type PackEvent struct {
	Type            string  `json:"type"`
	Source          string  `json:"source"`
	Timestamp       string  `json:"timestamp"`
	Version         string  `json:"version"`
	OS              string  `json:"os"`
	Arch            string  `json:"arch"`
	RootCount       int     `json:"root_count"`
	CatalogSize     int     `json:"catalog_size"`
	SelectedCount   int     `json:"selected_count"`
	BudgetPreset    string  `json:"budget_preset,omitempty"`
	BudgetTokens    int     `json:"budget_tokens"`
	EstimatedTokens int     `json:"estimated_tokens"`
	LatencyMS       float64 `json:"latency_ms"`
	EmptyCatalog    bool    `json:"empty_catalog"`
	ScrubFlagCount  int     `json:"scrub_flag_count"`
	TaskHash        string  `json:"task_hash,omitempty"`
	// StablePrefixHash is SHA-256 hex of markdown_stable (no bodies in the event).
	StablePrefixHash string `json:"stable_prefix_hash,omitempty"`
	// CacheFriendly is true when a non-empty procedural prefix is present for host pin.
	CacheFriendly bool `json:"cache_friendly"`
}

// HashTask returns SHA-256 hex of task (never store raw task text).
func HashTask(task string) string {
	sum := sha256.Sum256([]byte(task))
	return hex.EncodeToString(sum[:])
}

// ScrubFlagCount sums redaction + injection flags (counts only; no secret values).
func ScrubFlagCount(report core.ScrubReport) int {
	return report.TotalRedactions + report.TotalInjectionFlags
}

// HashStablePrefix returns SHA-256 hex of markdown_stable bytes (empty → "").
// Memoizes the last process-local (markdown → hex) pair so sticky-stable packs
// skip re-hashing identical MarkdownStable. Does not cache markdown rebuild.
func HashStablePrefix(markdownStable string) string {
	if markdownStable == "" {
		return ""
	}
	stableHashMu.Lock()
	defer stableHashMu.Unlock()
	if markdownStable == lastStableMD && lastStableHash != "" {
		return lastStableHash
	}
	sum := sha256.Sum256([]byte(markdownStable))
	h := hex.EncodeToString(sum[:])
	lastStableMD = markdownStable
	lastStableHash = h
	return h
}

// ResetStablePrefixHashCache clears the process-local hash memo (tests).
func ResetStablePrefixHashCache() {
	stableHashMu.Lock()
	lastStableMD = ""
	lastStableHash = ""
	stableHashMu.Unlock()
}

// IsCacheFriendly reports whether the pack has a pin-worthy procedural prefix.
func IsCacheFriendly(markdownStable string) bool {
	return markdownStable != ""
}

// NewPackEvent builds a contract pack event from a PackResult and latency.
// taskHash is optional; pass "" to omit. Never pass raw task text.
func NewPackEvent(result core.PackResult, latencyMS float64, source, taskHash string) PackEvent {
	if source == "" {
		source = SourceMCP
	}
	return PackEvent{
		Type:             EventTypePack,
		Source:           source,
		Timestamp:        time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		Version:          result.Version,
		OS:               runtime.GOOS,
		Arch:             runtime.GOARCH,
		RootCount:        result.RootCount,
		CatalogSize:      result.CatalogSize,
		SelectedCount:    result.SelectedCount,
		BudgetPreset:     result.BudgetPreset,
		BudgetTokens:     result.BudgetTokens,
		EstimatedTokens:  result.EstimatedTokens,
		LatencyMS:        roundLatencyMS(latencyMS),
		EmptyCatalog:     result.EmptyCatalog,
		ScrubFlagCount:   ScrubFlagCount(result.Security),
		TaskHash:         taskHash,
		StablePrefixHash: HashStablePrefix(result.MarkdownStable),
		CacheFriendly:    IsCacheFriendly(result.MarkdownStable),
	}
}

func roundLatencyMS(ms float64) float64 {
	if ms < 0 {
		return 0
	}
	return math.Round(ms*1000) / 1000
}

// RecordPackEvent appends one contract pack line to memory + JSONL.
func RecordPackEvent(ev PackEvent, home string) {
	if !Enabled() {
		return
	}
	if ev.Type == "" {
		ev.Type = EventTypePack
	}
	if ev.Timestamp == "" {
		ev.Timestamp = time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	}
	if ev.OS == "" {
		ev.OS = runtime.GOOS
	}
	if ev.Arch == "" {
		ev.Arch = runtime.GOARCH
	}

	var asMap map[string]any
	raw, err := json.Marshal(ev)
	if err != nil {
		return
	}
	if err := json.Unmarshal(raw, &asMap); err != nil {
		return
	}
	appendEvent(asMap, raw, home)
}

// RecordEvent appends a local-only non-pack telemetry event (memory + JSONL).
// Payloads must not include raw task text or skill bodies.
func RecordEvent(eventType string, payload map[string]any, source, home string) {
	if !Enabled() {
		return
	}
	if payload == nil {
		payload = map[string]any{}
	}
	ev := map[string]any{
		"timestamp": time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		"type":      eventType,
		"source":    source,
		"payload":   payload,
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return
	}
	appendEvent(ev, line, home)
}

func appendEvent(ev map[string]any, line []byte, home string) {
	memMu.Lock()
	memEvents = append(memEvents, ev)
	if len(memEvents) > 500 {
		memEvents = memEvents[len(memEvents)-500:]
	}
	memMu.Unlock()

	path := eventsPath(home)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(line, '\n'))
}

// MemoryEvents returns a copy of in-process events (tests / diagnostics).
func MemoryEvents() []map[string]any {
	memMu.Lock()
	defer memMu.Unlock()
	out := make([]map[string]any, len(memEvents))
	copy(out, memEvents)
	return out
}

// ResetMemoryEvents clears the in-process buffer (tests).
func ResetMemoryEvents() {
	memMu.Lock()
	memEvents = nil
	memMu.Unlock()
}

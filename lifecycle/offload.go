package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	// EnvOffload controls registration of the offload_tool_result MCP tool.
	// Default-on; opt out with SMARTCONTEXT_OFFLOAD=0|false|off|no|disabled.
	EnvOffload = "SMARTCONTEXT_OFFLOAD"
	// OffloadThresholdBytes is the minimum content size that stores to disk
	// when force=false. Below this, OffloadToolResult returns a clear JSON no-op.
	OffloadThresholdBytes = 8192
	// LegacyOffloadPreviewBytes is the pre-1.2.0 preview budget (2048).
	// Hosts may still pass preview_bytes=2048; it is not the runtime default.
	LegacyOffloadPreviewBytes = 2048
	// IndustryAskOffloadPreviewBytes is the ~50-token industry preview budget
	// (~4 bytes/token) and the 1.2+ runtime default.
	IndustryAskOffloadPreviewBytes = 200
	// DefaultOffloadPreviewBytes is the max preview when PreviewBytes is unset/≤0.
	DefaultOffloadPreviewBytes = IndustryAskOffloadPreviewBytes
	// maxOffloadSessionPointers caps how many recent offloads pack can recall.
	maxOffloadSessionPointers = 3
)

// OffloadEnabled reports whether tool-result offload MCP is armed.
// On by default; opt out with SMARTCONTEXT_OFFLOAD=0|false|off|no|disabled.
func OffloadEnabled() bool {
	return envDefaultOn(EnvOffload)
}

// Process-local cache for LoadOffloadPointers: reuse when last_offload.json is
// missing or mtime/size unchanged (avoids re-read + EnsureLayout on pack recall).
type offloadPtrCacheEntry struct {
	home    string
	path    string
	modNano int64
	size    int64
	missing bool
	ptrs    []OffloadPointer
}

var (
	offloadPtrMu    sync.Mutex
	offloadPtrCache offloadPtrCacheEntry
)

// OffloadRequest writes a large tool result to disk and returns a path + preview.
type OffloadRequest struct {
	Content      string
	Label        string // optional filename hint
	PreviewBytes int
	Home         string
	Source       string
	Force        bool // store even when content is under OffloadThresholdBytes
}

// OffloadResult is the agent-facing handle (not a Core skill pack).
type OffloadResult struct {
	Stored         bool   `json:"stored"`
	Path           string `json:"path,omitempty"`
	Preview        string `json:"preview,omitempty"`
	Bytes          int    `json:"bytes"`
	SHA256         string `json:"sha256,omitempty"`
	Truncated      bool   `json:"preview_truncated,omitempty"`
	Label          string `json:"label,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	ThresholdBytes int    `json:"threshold_bytes"`
	Note           string `json:"note"`
}

// OffloadPointer is a short session recall stub for markdown_dynamic.
type OffloadPointer struct {
	Path      string `json:"path"`
	Preview   string `json:"preview"`
	Bytes     int    `json:"bytes"`
	SHA256    string `json:"sha256,omitempty"`
	Label     string `json:"label,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// OffloadSessionState persists recent offload pointers for the next pack.
type OffloadSessionState struct {
	Pointers  []OffloadPointer `json:"pointers"`
	UpdatedAt string           `json:"updated_at,omitempty"`
}

func offloadDir(home string) string {
	return filepath.Join(EnsureLayout(home), "offload")
}

// offloadSessionPath returns the session pointer file path without EnsureLayout
// (read path stays cheap; writers call EnsureLayout explicitly).
func offloadSessionPath(home string) string {
	return filepath.Join(Home(home), "session", "last_offload.json")
}

// OffloadToolResult stores content under ~/.smartcontext/offload/ when
// len(content) ≥ OffloadThresholdBytes (or force=true) and returns a short
// preview. Under threshold with force=false returns a clear JSON no-op.
func OffloadToolResult(req OffloadRequest) (OffloadResult, error) {
	if strings.TrimSpace(req.Content) == "" {
		return OffloadResult{}, fmt.Errorf("content is required")
	}
	home := Home(req.Home)
	n := len(req.Content)
	base := OffloadResult{
		Bytes:          n,
		ThresholdBytes: OffloadThresholdBytes,
	}
	if n < OffloadThresholdBytes && !req.Force {
		base.Stored = false
		base.Note = fmt.Sprintf(
			"content below %d-byte threshold; not offloaded (pass force=true to store anyway)",
			OffloadThresholdBytes,
		)
		return base, nil
	}

	dir := offloadDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return OffloadResult{}, err
	}
	previewN := req.PreviewBytes
	if previewN <= 0 {
		previewN = DefaultOffloadPreviewBytes
	}
	sum := sha256.Sum256([]byte(req.Content))
	digest := hex.EncodeToString(sum[:])
	label := sanitizeOffloadLabel(req.Label)
	if label == "" {
		label = "tool-result"
	}
	ts := time.Now().UTC().Format("20060102T150405")
	name := fmt.Sprintf("%s-%s-%s.txt", ts, label, digest[:12])
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		return OffloadResult{}, err
	}
	preview, trunc := truncatePreview(req.Content, previewN)
	out := OffloadResult{
		Stored:         true,
		Path:           path,
		Preview:        preview,
		Bytes:          n,
		SHA256:         digest,
		Truncated:      trunc,
		Label:          label,
		CreatedAt:      time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		ThresholdBytes: OffloadThresholdBytes,
		Note:           "Filesystem offload; use tool-result-recall skill to re-read/search. Not part of Core skill pack.",
	}
	_ = writeOffloadIndex(home, out)
	_ = SaveOffloadPointers(home, OffloadPointer{
		Path:      out.Path,
		Preview:   out.Preview,
		Bytes:     out.Bytes,
		SHA256:    out.SHA256,
		Label:     out.Label,
		CreatedAt: out.CreatedAt,
	})
	src := req.Source
	if src == "" {
		src = SourceMCP
	}
	RecordEvent("offload_tool_result", map[string]any{
		"bytes":     out.Bytes,
		"sha256":    out.SHA256,
		"label":     out.Label,
		"truncated": out.Truncated,
		"stored":    true,
		"force":     req.Force,
	}, src, home)
	return out, nil
}

// LoadOffloadPointers returns recent session offload pointers (may be empty).
// Caches empty (missing file) and unchanged mtime/size results process-locally.
func LoadOffloadPointers(home string) []OffloadPointer {
	home = Home(home)
	path := offloadSessionPath(home)

	stInfo, err := os.Stat(path)
	offloadPtrMu.Lock()
	defer offloadPtrMu.Unlock()

	if err != nil {
		if offloadPtrCache.home == home && offloadPtrCache.path == path && offloadPtrCache.missing {
			return nil
		}
		offloadPtrCache = offloadPtrCacheEntry{home: home, path: path, missing: true}
		return nil
	}
	modNano := stInfo.ModTime().UnixNano()
	size := stInfo.Size()
	if offloadPtrCache.home == home && offloadPtrCache.path == path && !offloadPtrCache.missing &&
		offloadPtrCache.modNano == modNano && offloadPtrCache.size == size {
		return cloneOffloadPointers(offloadPtrCache.ptrs)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		offloadPtrCache = offloadPtrCacheEntry{home: home, path: path, missing: true}
		return nil
	}
	var st OffloadSessionState
	if err := json.Unmarshal(data, &st); err != nil {
		offloadPtrCache = offloadPtrCacheEntry{
			home: home, path: path, modNano: modNano, size: size, missing: false,
		}
		return nil
	}
	offloadPtrCache = offloadPtrCacheEntry{
		home: home, path: path, modNano: modNano, size: size, missing: false,
		ptrs: cloneOffloadPointers(st.Pointers),
	}
	return cloneOffloadPointers(st.Pointers)
}

// ResetOffloadPointerCache clears the process-local offload pointer memo (tests).
func ResetOffloadPointerCache() {
	offloadPtrMu.Lock()
	offloadPtrCache = offloadPtrCacheEntry{}
	offloadPtrMu.Unlock()
}

func cloneOffloadPointers(in []OffloadPointer) []OffloadPointer {
	if len(in) == 0 {
		return nil
	}
	out := make([]OffloadPointer, len(in))
	copy(out, in)
	return out
}

func rememberOffloadPointers(home, path string, ptrs []OffloadPointer) {
	stInfo, err := os.Stat(path)
	offloadPtrMu.Lock()
	defer offloadPtrMu.Unlock()
	if err != nil {
		offloadPtrCache = offloadPtrCacheEntry{home: home, path: path, missing: true}
		return
	}
	offloadPtrCache = offloadPtrCacheEntry{
		home: home, path: path,
		modNano: stInfo.ModTime().UnixNano(), size: stInfo.Size(),
		missing: false, ptrs: cloneOffloadPointers(ptrs),
	}
}

// SaveOffloadPointers prepends a pointer and caps the session list.
func SaveOffloadPointers(home string, ptr OffloadPointer) error {
	if strings.TrimSpace(ptr.Path) == "" {
		return nil
	}
	home = Home(home)
	EnsureLayout(home)
	existing := LoadOffloadPointers(home)
	next := make([]OffloadPointer, 0, maxOffloadSessionPointers)
	next = append(next, ptr)
	for _, p := range existing {
		if p.Path == ptr.Path {
			continue
		}
		next = append(next, p)
		if len(next) >= maxOffloadSessionPointers {
			break
		}
	}
	st := OffloadSessionState{
		Pointers:  next,
		UpdatedAt: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
	}
	payload, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	path := offloadSessionPath(home)
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
		return err
	}
	rememberOffloadPointers(home, path, next)
	return nil
}

// FormatOffloadRecallHint builds a short episodic markdown block for pack.
func FormatOffloadRecallHint(home string) string {
	ptrs := LoadOffloadPointers(home)
	if len(ptrs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Tool-result recall\n\n")
	b.WriteString("Recent offloaded tool results (path + preview). ")
	b.WriteString("Use skill **tool-result-recall** to re-read/search; do not re-ingest into Core pack.\n\n")
	for i, p := range ptrs {
		if i > 0 {
			b.WriteString("\n")
		}
		if p.Label != "" {
			b.WriteString(fmt.Sprintf("- **label**: %s\n", p.Label))
		}
		b.WriteString(fmt.Sprintf("- **path**: `%s`\n", p.Path))
		b.WriteString(fmt.Sprintf("- **bytes**: %d\n", p.Bytes))
		if p.SHA256 != "" {
			b.WriteString(fmt.Sprintf("- **sha256**: `%s`\n", p.SHA256))
		}
		if p.Preview != "" {
			b.WriteString("- **preview**:\n\n")
			b.WriteString("```\n")
			b.WriteString(p.Preview)
			if !strings.HasSuffix(p.Preview, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n")
		}
	}
	return b.String()
}

func sanitizeOffloadLabel(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' || r == '.' {
			b.WriteByte('-')
		}
	}
	out := b.String()
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}

func truncatePreview(s string, maxBytes int) (string, bool) {
	if len(s) <= maxBytes {
		return s, false
	}
	// Avoid cutting mid-rune.
	cut := maxBytes
	for cut > 0 && !utf8.ValidString(s[:cut]) {
		cut--
	}
	return s[:cut] + "\n… [preview truncated; full content at offload path]", true
}

func writeOffloadIndex(home string, latest OffloadResult) error {
	path := filepath.Join(offloadDir(home), "index.jsonl")
	payload, err := json.Marshal(latest)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(payload, '\n'))
	return err
}

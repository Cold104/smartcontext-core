package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/version"
)

// SessionStartRequest configures session_start.
type SessionStartRequest struct {
	Roots       []string
	Home        string
	MarkSession bool
	Source      string
}

// SessionStartResult is the session_start MCP payload.
type SessionStartResult struct {
	Status           string           `json:"status"`
	Version          string           `json:"version"`
	GeneratedAt      string           `json:"generated_at"`
	EmptyCatalog     bool             `json:"empty_catalog"`
	SkillCount       int              `json:"skill_count"`
	SkillNames       []string         `json:"skill_names"`
	Roots            []string         `json:"roots"`
	Delta            SkillDelta       `json:"delta"`
	Refine           RefineListResult `json:"refine"`
	Triage           TriagePrefs      `json:"triage"`
	Watermark        *Watermark       `json:"watermark,omitempty"`
	MarkedSession    bool             `json:"marked_session"`
	LifecycleEnabled bool             `json:"lifecycle_enabled"`
	Note             string           `json:"note"`
}

// Watermark is the last session-start snapshot.
type Watermark struct {
	StartedAt          string            `json:"started_at"`
	CatalogFingerprint string            `json:"catalog_fingerprint"`
	SkillNames         []string          `json:"skill_names"`
	SkillMtimes        map[string]string `json:"skill_mtimes,omitempty"`
	// RootsHash identifies the resolved root set the snapshot was taken over
	// (see rootsHash). Empty on pre-1.3.x watermarks.
	RootsHash string   `json:"roots_hash,omitempty"`
	Roots     []string `json:"roots,omitempty"`
}

// WatermarkResetRootsChanged is the delta reason when the watermark's root set
// differs from the current one.
const WatermarkResetRootsChanged = "roots changed, watermark reset"

// rootsHash is an order-independent SHA-256 over cleaned resolved roots
// (case-folded on Windows, where paths are case-insensitive).
func rootsHash(roots []string) string {
	norm := make([]string, 0, len(roots))
	for _, r := range roots {
		r = filepath.Clean(r)
		if runtime.GOOS == "windows" {
			r = strings.ToLower(r)
		}
		norm = append(norm, r)
	}
	sort.Strings(norm)
	sum := sha256.Sum256([]byte(strings.Join(norm, "\n")))
	return hex.EncodeToString(sum[:])
}

// SkillDeltaChange is one added/removed/modified/refine_proposal row.
type SkillDeltaChange struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Timestamp  string `json:"timestamp,omitempty"`
	Detail     string `json:"detail,omitempty"`
	ProposalID string `json:"proposal_id,omitempty"`
}

// SkillDelta is catalog/refine delta vs watermark.
type SkillDelta struct {
	ComputedAt         string             `json:"computed_at"`
	CatalogFingerprint string             `json:"catalog_fingerprint"`
	Changes            []SkillDeltaChange `json:"changes"`
	HasWatermark       bool               `json:"has_watermark"`
	// WatermarkReset is true when the stored watermark was taken over a
	// different resolved root set (or predates roots_hash). The catalog diff is
	// skipped and the current catalog becomes the new baseline.
	WatermarkReset       bool   `json:"watermark_reset,omitempty"`
	WatermarkResetReason string `json:"watermark_reset_reason,omitempty"`
	RootsHash            string `json:"roots_hash,omitempty"`
}

func catalogFingerprint(cat core.Catalog) string {
	parts := make([]string, 0, len(cat.Entries))
	for _, e := range cat.Entries {
		mtime := ""
		if st, err := os.Stat(e.Path); err == nil {
			mtime = st.ModTime().UTC().Truncate(time.Second).Format(time.RFC3339)
		}
		parts = append(parts, e.Kind+":"+e.Name+":"+e.Path+":"+mtime)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(stringsJoin(parts)))
	return hex.EncodeToString(sum[:])
}

func stringsJoin(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n"
		}
		out += p
	}
	return out
}

func skillNames(cat core.Catalog) []string {
	names := make([]string, 0, len(cat.Entries))
	for _, e := range cat.Entries {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}
	sort.Strings(names)
	return names
}

func skillMtimes(cat core.Catalog) map[string]string {
	out := map[string]string{}
	for _, e := range cat.Entries {
		if e.Name == "" {
			continue
		}
		if st, err := os.Stat(e.Path); err == nil {
			out[e.Name] = st.ModTime().UTC().Truncate(time.Second).Format(time.RFC3339)
		}
	}
	return out
}

// LoadWatermark loads last session watermark or nil.
func LoadWatermark(home string) *Watermark {
	path := sessionWatermarkPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var w Watermark
	if err := json.Unmarshal(data, &w); err != nil {
		return nil
	}
	return &w
}

// SaveWatermark writes the session watermark.
func SaveWatermark(cat core.Catalog, home string) (*Watermark, error) {
	w := &Watermark{
		StartedAt:          time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		CatalogFingerprint: catalogFingerprint(cat),
		SkillNames:         skillNames(cat),
		SkillMtimes:        skillMtimes(cat),
		RootsHash:          rootsHash(cat.Roots),
		Roots:              append([]string(nil), cat.Roots...),
	}
	path := sessionWatermarkPath(home)
	payload, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, append(payload, '\n'), 0o644); err != nil {
		return nil, err
	}
	return w, nil
}

func computeDelta(cat core.Catalog, home string, pending []Proposal) SkillDelta {
	now := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	fp := catalogFingerprint(cat)
	rh := rootsHash(cat.Roots)
	wm := LoadWatermark(home)
	current := map[string]core.Entry{}
	for _, e := range cat.Entries {
		if e.Name != "" {
			current[e.Name] = e
		}
	}
	var changes []SkillDeltaChange
	reset := wm != nil && wm.RootsHash != rh
	if reset {
		// Different (or unknown, pre-roots_hash) root set: a name diff would
		// report the whole catalog as added/removed. Re-baseline instead.
		_, _ = SaveWatermark(cat, home)
	} else if wm == nil {
		for _, name := range skillNames(cat) {
			changes = append(changes, SkillDeltaChange{
				Name: name, Kind: "added", Timestamp: now,
			})
		}
	} else {
		prior := map[string]bool{}
		for _, n := range wm.SkillNames {
			prior[n] = true
		}
		for _, name := range skillNames(cat) {
			if !prior[name] {
				changes = append(changes, SkillDeltaChange{
					Name: name, Kind: "added", Timestamp: now,
				})
			}
		}
		for n := range prior {
			if _, ok := current[n]; !ok {
				changes = append(changes, SkillDeltaChange{
					Name: n, Kind: "removed", Timestamp: now,
				})
			}
		}
		for name, e := range current {
			if !prior[name] {
				continue
			}
			prev := ""
			if wm.SkillMtimes != nil {
				prev = wm.SkillMtimes[name]
			}
			cur := ""
			if st, err := os.Stat(e.Path); err == nil {
				cur = st.ModTime().UTC().Truncate(time.Second).Format(time.RFC3339)
			}
			if prev != "" && cur != "" && prev != cur {
				changes = append(changes, SkillDeltaChange{
					Name: name, Kind: "modified", Timestamp: cur,
				})
			}
		}
	}
	for _, p := range pending {
		if p.Status != "" && p.Status != "pending" {
			continue
		}
		detail := ""
		for i, r := range p.Reasons {
			if i > 0 {
				detail += ", "
			}
			detail += r
		}
		changes = append(changes, SkillDeltaChange{
			Name:       p.SkillName,
			Kind:       "refine_proposal",
			Timestamp:  p.CreatedAt,
			Detail:     detail,
			ProposalID: p.ID,
		})
	}
	delta := SkillDelta{
		ComputedAt:         now,
		CatalogFingerprint: fp,
		Changes:            changes,
		HasWatermark:       wm != nil,
		WatermarkReset:     reset,
		RootsHash:          rh,
	}
	if reset {
		delta.WatermarkResetReason = WatermarkResetRootsChanged
	}
	_ = saveLastDelta(delta, home)
	return delta
}

func saveLastDelta(delta SkillDelta, home string) error {
	path := sessionDeltaPath(home)
	payload, err := json.MarshalIndent(delta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

// SessionStart builds a session snapshot: catalog delta + refine list + optional watermark.
// Canvas open is deferred (MCP hosts render JSON themselves).
func SessionStart(req SessionStartRequest) SessionStartResult {
	if req.Source == "" {
		req.Source = SourceMCP
	}
	home := Home(req.Home)
	EnsureLayout(home)

	result := SessionStartResult{
		Status:           "ok",
		Version:          version.Version,
		GeneratedAt:      time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		LifecycleEnabled: Enabled(),
		Note:             "Go POC session_start: catalog delta + refine list; Canvas open deferred",
	}

	if !Enabled() {
		result.Status = "lifecycle_disabled"
		result.Note = "SMARTCONTEXT_LIFECYCLE opted out; session_start is a no-op"
		return result
	}

	cat := core.BuildCatalog(req.Roots)
	names := skillNames(cat)
	result.EmptyCatalog = len(cat.Entries) == 0
	result.SkillCount = len(cat.Entries)
	result.SkillNames = names
	result.Roots = cat.Roots
	result.Triage = LoadTriagePrefs(home)

	_ = ProposeThinSkills(cat, home, req.Source)

	refine := ListProposals(home, "pending")
	result.Refine = refine
	result.Delta = computeDelta(cat, home, refine.Proposals)
	if result.Delta.WatermarkReset {
		result.Note += "; " + result.Delta.WatermarkResetReason
	}

	if req.MarkSession {
		if w, err := SaveWatermark(cat, home); err == nil {
			result.Watermark = w
			result.MarkedSession = true
		}
	} else {
		result.Watermark = LoadWatermark(home)
	}

	RecordEvent("session_start", map[string]any{
		"refine":        refine.Count,
		"empty_catalog": result.EmptyCatalog,
		"changes":       len(result.Delta.Changes),
		"skill_count":   result.SkillCount,
	}, req.Source, home)

	return result
}

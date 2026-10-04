package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Cold104/smartcontext-core/core"
)

var validStatuses = map[string]bool{
	"pin": true, "keep": true, "watch": true, "deprecate": true,
}

// TriagePrefs is the on-disk triage overlay document.
type TriagePrefs struct {
	Skills map[string]TriageEntry `json:"skills"`
}

// TriageEntry is one skill's triage status.
type TriageEntry struct {
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Note      string `json:"note,omitempty"`
}

// EmptyTriagePrefs returns an empty prefs document.
func EmptyTriagePrefs() TriagePrefs {
	return TriagePrefs{Skills: map[string]TriageEntry{}}
}

// LoadTriagePrefs loads triage prefs; missing/corrupt -> empty.
func LoadTriagePrefs(home string) TriagePrefs {
	path := triagePrefsPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		return EmptyTriagePrefs()
	}
	var prefs TriagePrefs
	if err := json.Unmarshal(data, &prefs); err != nil {
		return EmptyTriagePrefs()
	}
	if prefs.Skills == nil {
		prefs.Skills = map[string]TriageEntry{}
	}
	return prefs
}

// SaveTriagePrefs persists triage prefs JSON.
func SaveTriagePrefs(prefs TriagePrefs, home string) error {
	if prefs.Skills == nil {
		prefs.Skills = map[string]TriageEntry{}
	}
	path := triagePrefsPath(home)
	payload, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

// SetTriageStatus sets pin/keep/watch/deprecate for a skill name.
func SetTriageStatus(name, status, note, home, source string) (TriageEntry, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if !validStatuses[status] {
		return TriageEntry{}, fmt.Errorf("invalid triage status %q; expected pin|keep|watch|deprecate", status)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return TriageEntry{}, fmt.Errorf("skill name is required")
	}
	prefs := LoadTriagePrefs(home)
	entry := TriageEntry{
		Status:    status,
		UpdatedAt: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		Note:      note,
	}
	prefs.Skills[name] = entry
	if err := SaveTriagePrefs(prefs, home); err != nil {
		return TriageEntry{}, err
	}
	RecordEvent("triage_set", map[string]any{
		"name": name, "status": status,
	}, source, home)
	return entry, nil
}

// ListTriagePrefs returns the skills map.
func ListTriagePrefs(home string) map[string]TriageEntry {
	return LoadTriagePrefs(home).Skills
}

// ApplyTriagePrefs returns a catalog copy with deprecate excluded and pins first.
func ApplyTriagePrefs(cat core.Catalog, prefs TriagePrefs) core.Catalog {
	if prefs.Skills == nil {
		prefs.Skills = map[string]TriageEntry{}
	}
	var pinned, rest []core.Entry
	for _, e := range cat.Entries {
		st := "keep"
		if t, ok := prefs.Skills[e.Name]; ok && t.Status != "" {
			st = t.Status
		}
		if st == "deprecate" {
			continue
		}
		if st == "pin" {
			pinned = append(pinned, e)
		} else {
			rest = append(rest, e)
		}
	}
	out := cat
	out.Entries = append(pinned, rest...)
	return out
}

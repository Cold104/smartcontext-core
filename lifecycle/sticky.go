package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StickyPackState is the last pack's selected skill names for mid-task recoverability.
type StickyPackState struct {
	SelectedNames []string `json:"selected_names"`
	TaskHash      string   `json:"task_hash,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
}

func stickyPackPath(home string) string {
	return filepath.Join(EnsureLayout(home), "session", "last_pack_sticky.json")
}

// DefaultStickyTTL bounds how long persisted sticky names carry over to a pack
// whose task text differs from the pack that stored them.
const DefaultStickyTTL = 30 * time.Minute

// StickyTTL returns the carry-over window for sticky names across differing
// tasks. Override with SMARTCONTEXT_STICKY_TTL (Go duration, e.g. "10m", "1h");
// "0" disables cross-task carry-over (same-task re-packs still reuse).
// Empty or unparseable values fall back to DefaultStickyTTL.
func StickyTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SMARTCONTEXT_STICKY_TTL"))
	if raw == "" {
		return DefaultStickyTTL
	}
	if raw == "0" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return DefaultStickyTTL
	}
	return d
}

func loadStickyState(home string) (StickyPackState, bool) {
	data, err := os.ReadFile(stickyPackPath(home))
	if err != nil {
		return StickyPackState{}, false
	}
	var st StickyPackState
	if err := json.Unmarshal(data, &st); err != nil {
		return StickyPackState{}, false
	}
	return st, true
}

// LoadStickyNames returns persisted sticky skill names (may be empty) without
// any freshness check. Pack paths should use LoadStickyNamesFor.
func LoadStickyNames(home string) []string {
	st, ok := loadStickyState(home)
	if !ok {
		return nil
	}
	return st.SelectedNames
}

// LoadStickyNamesFor returns persisted sticky names only when they belong to
// the current task:
//
//   - stored task_hash == taskHash (same task re-pack): reuse, regardless of age;
//   - otherwise (phrasing drift mid-task): reuse only when updated_at is within
//     StickyTTL() of now;
//   - missing/unparseable updated_at with a different (or missing) task_hash:
//     expired (legacy state never carries over to a new task).
//
// Every pack re-saves the state, so an active task keeps its sticky set alive;
// a new task started after TTL of inactivity starts clean. Hosts should still
// pass clear_sticky at the start of a new task.
func LoadStickyNamesFor(home, taskHash string, now time.Time) []string {
	st, ok := loadStickyState(home)
	if !ok || len(st.SelectedNames) == 0 {
		return nil
	}
	if taskHash != "" && st.TaskHash == taskHash {
		return st.SelectedNames
	}
	ttl := StickyTTL()
	if ttl <= 0 || st.UpdatedAt == "" {
		return nil
	}
	updated, err := time.Parse(time.RFC3339, st.UpdatedAt)
	if err != nil {
		return nil
	}
	age := now.Sub(updated)
	if age < 0 || age > ttl {
		// Future timestamps (clock skew / hand-edited) are treated as untrusted.
		return nil
	}
	return st.SelectedNames
}

// SaveStickyNames persists selected names for the next pack in this session.
func SaveStickyNames(names []string, taskHash string, home string) error {
	st := StickyPackState{
		SelectedNames: append([]string(nil), names...),
		TaskHash:      taskHash,
		UpdatedAt:     time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
	}
	payload, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stickyPackPath(home), append(payload, '\n'), 0o644)
}

// ClearStickyNames removes persisted sticky state.
func ClearStickyNames(home string) error {
	err := os.Remove(stickyPackPath(home))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

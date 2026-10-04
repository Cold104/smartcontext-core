package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// LoadStickyNames returns persisted sticky skill names (may be empty).
func LoadStickyNames(home string) []string {
	data, err := os.ReadFile(stickyPackPath(home))
	if err != nil {
		return nil
	}
	var st StickyPackState
	if err := json.Unmarshal(data, &st); err != nil {
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

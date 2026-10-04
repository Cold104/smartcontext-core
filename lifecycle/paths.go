package lifecycle

import (
	"os"
	"path/filepath"
)

// Home resolves the local SmartContext state root.
// Override with SMARTCONTEXT_HOME; default ~/.smartcontext.
func Home(override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("SMARTCONTEXT_HOME"); env != "" {
		return env
	}
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return ".smartcontext"
	}
	return filepath.Join(h, ".smartcontext")
}

// EnsureLayout creates the standard local state directories.
func EnsureLayout(home string) string {
	root := Home(home)
	for _, rel := range []string{
		"telemetry",
		"prefs",
		"refine/queue",
		"session",
		"offload",
		"factory/quarantine",
		"factory/audit",
		"factory/sandbox/.cursor/skills",
	} {
		_ = os.MkdirAll(filepath.Join(root, rel), 0o755)
	}
	return root
}

func eventsPath(home string) string {
	return filepath.Join(EnsureLayout(home), "telemetry", "events.jsonl")
}

func triagePrefsPath(home string) string {
	return filepath.Join(EnsureLayout(home), "prefs", "triage.json")
}

func aliasPrefsPath(home string) string {
	return filepath.Join(EnsureLayout(home), "prefs", "skill_aliases.json")
}

func refineQueueDir(home string) string {
	return filepath.Join(EnsureLayout(home), "refine", "queue")
}

func sessionWatermarkPath(home string) string {
	return filepath.Join(EnsureLayout(home), "session", "last_cursor_start.json")
}

func sessionDeltaPath(home string) string {
	return filepath.Join(EnsureLayout(home), "session", "last_delta.json")
}

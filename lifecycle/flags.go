package lifecycle

import (
	"os"
	"strings"
)

// Enabled reports whether lifecycle wrappers (triage/telemetry/session) should run.
// Lifecycle is on by default. Opt out with SMARTCONTEXT_LIFECYCLE=0|false|off|no|disabled.
// Opting out does not grant factory write consent.
func Enabled() bool {
	return envDefaultOn("SMARTCONTEXT_LIFECYCLE")
}

// StickySessionEnabled controls auto sticky selected_names across packs.
// On by default when lifecycle is on. Opt out: SMARTCONTEXT_STICKY_SESSION=0|false|off|no|disabled.
func StickySessionEnabled() bool {
	if !Enabled() {
		return false
	}
	return envDefaultOn("SMARTCONTEXT_STICKY_SESSION")
}

// envDefaultOn is the shared default-on env helper: explicit opt-out values
// (0|false|off|no|disabled, trimmed + lowercased) disable; empty/garbage stay on.
func envDefaultOn(key string) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch raw {
	case "0", "false", "off", "no", "disabled":
		return false
	default:
		return true
	}
}

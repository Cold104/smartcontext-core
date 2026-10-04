//go:build !windows

package winproc

// HasConsoleWindow always false off Windows.
func HasConsoleWindow(pid int) bool { return false }

// WindowClasses always empty off Windows.
func WindowClasses(pid int) []string { return nil }

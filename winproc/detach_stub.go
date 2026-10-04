//go:build !windows

package winproc

// DetachConsole is a no-op on non-Windows.
func DetachConsole() {}

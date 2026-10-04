//go:build !windows

package winproc

import "os/exec"

// HideConsole is a no-op on non-Windows.
func HideConsole(cmd *exec.Cmd) {}

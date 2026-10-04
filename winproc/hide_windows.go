//go:build windows

package winproc

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW prevents a console window for console-subsystem children.
// Combined with HideWindow for GUI children that might still show a frame.
const createNoWindow = 0x08000000

// HideConsole configures cmd so CreateProcess does not show a console window.
// Safe with redirected stdin/stdout (MCP stdio pipes still work).
func HideConsole(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

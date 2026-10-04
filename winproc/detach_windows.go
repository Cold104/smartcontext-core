//go:build windows

package winproc

import "syscall"

var (
	modKernel32     = syscall.NewLazyDLL("kernel32.dll")
	procFreeConsole = modKernel32.NewProc("FreeConsole")
)

// DetachConsole drops any console attached to this process (no-op if none).
// Used at MCP stdio entry so a GUI host never leaves a visible console attached.
func DetachConsole() {
	_, _, _ = procFreeConsole.Call()
}

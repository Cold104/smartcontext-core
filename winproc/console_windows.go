//go:build windows

package winproc

import (
	"syscall"
	"unsafe"
)

// HasConsoleWindow reports whether pid owns a console / Windows Terminal host window.
func HasConsoleWindow(pid int) bool {
	for _, c := range WindowClasses(pid) {
		if c == "ConsoleWindowClass" || c == "CASCADIA_HOSTING_WINDOW_CLASS" {
			return true
		}
	}
	return false
}

// WindowClasses lists Win32 class names for top-level windows owned by pid.
func WindowClasses(pid int) []string {
	user32 := syscall.NewLazyDLL("user32.dll")
	enumWindows := user32.NewProc("EnumWindows")
	getWindowThreadProcessId := user32.NewProc("GetWindowThreadProcessId")
	getClassNameW := user32.NewProc("GetClassNameW")

	want := uint32(pid)
	var classes []string
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		var wpid uint32
		getWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&wpid)))
		if wpid != want {
			return 1
		}
		buf := make([]uint16, 256)
		getClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		classes = append(classes, syscall.UTF16ToString(buf))
		return 1
	})
	enumWindows.Call(cb, 0)
	return classes
}

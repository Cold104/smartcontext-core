//go:build windows

package core

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// MEMORYSTATUSEX — not exported by golang.org/x/sys/windows; call kernel32 directly.
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var (
	modkernel32              = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modkernel32.NewProc("GlobalMemoryStatusEx")
)

func memoryBytes() (total, available uint64, ok bool) {
	var status memoryStatusEx
	status.Length = uint32(unsafe.Sizeof(status))
	r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if r1 == 0 {
		return 0, 0, false
	}
	return status.TotalPhys, status.AvailPhys, true
}

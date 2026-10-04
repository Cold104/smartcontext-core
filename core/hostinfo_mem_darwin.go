//go:build darwin

package core

import (
	"golang.org/x/sys/unix"
)

func memoryBytes() (total, available uint64, ok bool) {
	// hw.memsize is total physical RAM.
	mem, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0, 0, false
	}
	total = mem
	// Free+inactive pages ≈ usable; page size via unix.Getpagesize.
	pageSize := uint64(unix.Getpagesize())
	free, err1 := unix.SysctlUint64("vm.page_free_count")
	inactive, err2 := unix.SysctlUint64("vm.page_inactive_count")
	if err1 == nil && err2 == nil {
		available = (free + inactive) * pageSize
	}
	return total, available, true
}

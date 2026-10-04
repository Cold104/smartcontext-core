//go:build linux

package core

import "golang.org/x/sys/unix"

func memoryBytes() (total, available uint64, ok bool) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, 0, false
	}
	// unit is mem_unit bytes on modern Linux (often 1).
	unit := uint64(info.Unit)
	if unit == 0 {
		unit = 1
	}
	total = uint64(info.Totalram) * unit
	available = uint64(info.Freeram) * unit
	return total, available, true
}

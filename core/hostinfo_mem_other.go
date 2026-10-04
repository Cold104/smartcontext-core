//go:build !windows && !linux && !darwin

package core

// memoryBytes is unavailable on this GOOS (POC: omit memory fields).
func memoryBytes() (total, available uint64, ok bool) {
	return 0, 0, false
}

package core

import (
	"runtime"
	"testing"
)

func TestCollectHostInfoFieldsPresent(t *testing.T) {
	info := CollectHostInfo()
	if info.OS == "" {
		t.Fatal("os empty")
	}
	if info.OS != runtime.GOOS {
		t.Fatalf("os=%q want %q", info.OS, runtime.GOOS)
	}
	if info.Arch == "" {
		t.Fatal("arch empty")
	}
	if info.Arch != runtime.GOARCH {
		t.Fatalf("arch=%q want %q", info.Arch, runtime.GOARCH)
	}
	if info.NumCPU < 1 {
		t.Fatalf("num_cpu=%d want >=1", info.NumCPU)
	}
	if info.WorkingDir == "" {
		t.Fatal("working_dir empty")
	}
	// Memory is best-effort on windows/linux/darwin; require on those platforms.
	switch runtime.GOOS {
	case "windows", "linux", "darwin":
		if info.MemoryTotalBytes == 0 {
			t.Fatalf("memory_total_bytes unset on %s", runtime.GOOS)
		}
	}
	sum := SummarizeHost()
	if sum.OS != info.OS || sum.Arch != info.Arch || sum.NumCPU != info.NumCPU {
		t.Fatalf("summary mismatch: %+v vs %+v", sum, info)
	}
}

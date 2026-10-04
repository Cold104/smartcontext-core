package core

import (
	"os"
	"os/user"
	"runtime"
)

// HostInfo is a POC-grade snapshot of the machine running the MCP server.
// Values are best-effort; omitted/zero fields mean unavailable on this OS.
type HostInfo struct {
	OS                   string `json:"os"`   // runtime.GOOS: windows|linux|darwin|…
	Arch                 string `json:"arch"` // runtime.GOARCH
	Hostname             string `json:"hostname,omitempty"`
	NumCPU               int    `json:"num_cpu"`
	MemoryTotalBytes     uint64 `json:"memory_total_bytes,omitempty"`
	MemoryAvailableBytes uint64 `json:"memory_available_bytes,omitempty"`
	WorkingDir           string `json:"working_dir,omitempty"`
	HomeDir              string `json:"home_dir,omitempty"`
	Username             string `json:"username,omitempty"`
	CwdIsHome            bool   `json:"cwd_is_home"`
	WorkspaceSeedSource  string `json:"workspace_seed_source,omitempty"`
	Warning              string `json:"warning,omitempty"`
}

// HostSummary is a short OS hint embedded in discover_skills so agents see
// the host before packing (without a separate round-trip).
type HostSummary struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	NumCPU int    `json:"num_cpu"`
}

// CollectHostInfo gathers OS/arch, CPU, memory (when available), cwd, home, user.
func CollectHostInfo() HostInfo {
	info := HostInfo{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		NumCPU: runtime.NumCPU(),
	}
	if h, err := os.Hostname(); err == nil {
		info.Hostname = h
	}
	if wd, err := os.Getwd(); err == nil {
		info.WorkingDir = wd
	}
	if home, err := os.UserHomeDir(); err == nil {
		info.HomeDir = home
	}
	if u, err := user.Current(); err == nil && u != nil {
		info.Username = u.Username
	} else if u := os.Getenv("USER"); u != "" {
		info.Username = u
	} else if u := os.Getenv("USERNAME"); u != "" {
		info.Username = u
	}
	total, avail, ok := memoryBytes()
	if ok {
		info.MemoryTotalBytes = total
		info.MemoryAvailableBytes = avail
	}
	diag := DiagnoseRootResolution(nil)
	info.CwdIsHome = diag.CwdIsHome
	info.WorkspaceSeedSource = diag.WorkspaceSeedSource
	info.Warning = diag.Warning
	return info
}

// SummarizeHost returns the short host block for discover_skills.
func SummarizeHost() HostSummary {
	return HostSummary{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		NumCPU: runtime.NumCPU(),
	}
}

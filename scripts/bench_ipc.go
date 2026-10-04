// Bench in-process pack vs MCP stdio round-trip on the lifecycle-ON product path.
// Usage (from go/): go build -o smartcontext.exe ./cmd/smartcontext && go run ./scripts/bench_ipc.go
//
//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/lifecycle"
	"github.com/Cold104/smartcontext-core/winproc"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	exe := filepath.Join(wd, "smartcontext.exe")
	if _, err := os.Stat(exe); err != nil {
		fail(fmt.Errorf("missing %s — run go build first", exe))
	}

	root := filepath.Join(os.TempDir(), "sc-bench-ipc-skills")
	skillDir := filepath.Join(root, "bench-skill")
	_ = os.MkdirAll(skillDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(
		"---\nname: bench-skill\ndescription: Bench skill for IPC overhead pack\nkind: skill\nos: any\n---\n\n# Bench\n"+
			"Body text for a modest pack payload.\n",
	), 0o644)

	homeEmpty := filepath.Join(os.TempDir(), "sc-bench-ipc-home-empty")
	homePop := filepath.Join(os.TempDir(), "sc-bench-ipc-home-pop")
	_ = os.RemoveAll(homeEmpty)
	_ = os.RemoveAll(homePop)
	_ = os.MkdirAll(homeEmpty, 0o755)
	_ = os.MkdirAll(homePop, 0o755)

	// Lifecycle ON = product default. Isolate home so telemetry/offload stay hermetic.
	_ = os.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	_ = os.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	_ = os.Unsetenv("SMARTCONTEXT_LIFECYCLE") // default-on
	_ = os.Setenv("SMARTCONTEXT_HOME", homeEmpty)

	const iters = 20
	const wsPackN = 30
	task := "bench skill for IPC overhead pack"
	req := core.PackRequest{Task: task, TopN: 5, BudgetTokens: 2000, IncludeBodies: true, Roots: []string{root}}

	// --- In-process: empty offload session (missing last_offload.json) ---
	lifecycle.ResetOffloadPointerCache()
	emptyMS := timePacks(req, iters)

	// --- In-process: populated offload session ---
	_ = os.Setenv("SMARTCONTEXT_HOME", homePop)
	lifecycle.ResetOffloadPointerCache()
	lifecycle.EnsureLayout(homePop)
	if err := lifecycle.SaveOffloadPointers(homePop, lifecycle.OffloadPointer{
		Path:      filepath.Join(homePop, "offload", "bench-blob.txt"),
		Preview:   "bench preview line for populated session recall",
		Bytes:     12000,
		SHA256:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Label:     "bench-blob",
		CreatedAt: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
	}); err != nil {
		fail(err)
	}
	lifecycle.ResetOffloadPointerCache()
	popMS := timePacks(req, iters)

	// Primary in-process figure: empty session (cold product path after cache warm-up).
	_ = os.Setenv("SMARTCONTEXT_HOME", homeEmpty)
	lifecycle.ResetOffloadPointerCache()
	inProc := timePacks(req, iters)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "ipc-bench", Version: "0.0.1"}, nil)
	cmd := exec.CommandContext(ctx, exe, "mcp")
	cmd.Dir = wd
	cmd.Env = append(os.Environ(),
		"SMARTCONTEXT_SKILL_ROOTS="+root,
		"SMARTCONTEXT_HOME="+homeEmpty,
		// Explicit lifecycle-on for the child even if parent env was mutated.
		"SMARTCONTEXT_LIFECYCLE=1",
	)
	winproc.HideConsole(cmd)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		fail(err)
	}
	defer session.Close()

	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	hostInfo := map[string]any{}
	hostRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "host_info",
		Arguments: map[string]any{},
	})
	if err != nil {
		fail(err)
	}
	if hostRes.IsError {
		fail(fmt.Errorf("host_info error: %+v", hostRes.Content))
	}
	if err := json.Unmarshal([]byte(firstText(hostRes)), &hostInfo); err != nil {
		fail(fmt.Errorf("host_info JSON: %w", err))
	}

	wsBeforeMB, wsBeforeErr := workingSetMB(pid)

	mcpMS := make([]float64, 0, iters)
	for i := 0; i < iters; i++ {
		start := time.Now()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "pack_context",
			Arguments: map[string]any{
				"task":           task,
				"budget_tokens":  2000,
				"include_bodies": true,
				"roots":          []string{root},
			},
		})
		if err != nil {
			fail(err)
		}
		if res.IsError {
			fail(fmt.Errorf("pack_context error: %+v", res.Content))
		}
		mcpMS = append(mcpMS, float64(time.Since(start).Microseconds())/1000.0)
	}

	// Extra packs for WS delta (same MCP child).
	for i := 0; i < wsPackN; i++ {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "pack_context",
			Arguments: map[string]any{
				"task":           task,
				"budget_tokens":  2000,
				"include_bodies": true,
				"roots":          []string{root},
			},
		})
		if err != nil {
			fail(err)
		}
		if res.IsError {
			fail(fmt.Errorf("pack_context (ws sample) error: %+v", res.Content))
		}
	}
	wsAfterMB, wsAfterErr := workingSetMB(pid)

	// One json-only sample for size comparison.
	resJSON, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "pack_context",
		Arguments: map[string]any{
			"task": task, "budget_tokens": 2000, "include_bodies": true,
			"roots": []string{root}, "format": "json",
		},
	})
	if err != nil {
		fail(err)
	}
	jsonOnlyBytes := contentBytes(resJSON)

	resFull, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "pack_context",
		Arguments: map[string]any{
			"task": task, "budget_tokens": 2000, "include_bodies": true,
			"roots": []string{root},
		},
	})
	if err != nil {
		fail(err)
	}
	fullBytes := contentBytes(resFull)

	ip50 := round3(percentile(inProc, 50))
	emptyP50 := round3(percentile(emptyMS, 50))
	popP50 := round3(percentile(popMS, 50))
	mp50 := round3(percentile(mcpMS, 50))

	out := map[string]any{
		"lifecycle":                           "on",
		"iters":                               iters,
		"in_process_pack_p50_ms":              ip50,
		"in_process_empty_offload_p50_ms":     emptyP50,
		"in_process_populated_offload_p50_ms": popP50,
		"mcp_roundtrip_p50_ms":                mp50,
		"stdio_delta_p50_ms":                  round3(mp50 - ip50),
		"content_bytes_full":                  fullBytes,
		"content_bytes_json":                  jsonOnlyBytes,
		"mcp_pid":                             pid,
		"ws_pack_count":                       wsPackN,
		"ws_before_packs_mb":                  wsBeforeMB,
		"ws_after_n_packs_mb":                 wsAfterMB,
		"host_info_memory_total_bytes":        hostInfo["memory_total_bytes"],
		"host_info_memory_available_bytes":    hostInfo["memory_available_bytes"],
		"host_info_num_cpu":                   hostInfo["num_cpu"],
		"host_info_os":                        hostInfo["os"],
		"host_info_arch":                      hostInfo["arch"],
		"note":                                "Lifecycle-ON product path. See go/docs/IPC_OVERHEAD.md",
		"captured_at":                         time.Now().UTC().Format(time.RFC3339),
	}
	if wsBeforeErr != nil {
		out["ws_before_error"] = wsBeforeErr.Error()
	}
	if wsAfterErr != nil {
		out["ws_after_error"] = wsAfterErr.Error()
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
	fmt.Println("BENCH_IPC_OK")
}

func timePacks(req core.PackRequest, iters int) []float64 {
	out := make([]float64, 0, iters)
	for i := 0; i < iters; i++ {
		start := time.Now()
		_ = lifecycle.PackContext(req)
		out = append(out, float64(time.Since(start).Microseconds())/1000.0)
	}
	return out
}

func contentBytes(res *mcp.CallToolResult) int {
	n := 0
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			n += len(tc.Text)
		}
	}
	return n
}

func firstText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	idx := int(float64(len(cp)-1) * p / 100.0)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

// workingSetMB samples process WorkingSet via PowerShell Get-Process (Windows).
// Returns 0 with error on non-Windows or when PID is unavailable.
func workingSetMB(pid int) (float64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("no mcp pid")
	}
	if runtime.GOOS != "windows" {
		return 0, fmt.Errorf("ws sample is Windows-only")
	}
	script := fmt.Sprintf(
		"[math]::Round((Get-Process -Id %d).WorkingSet64/1MB, 2)",
		pid,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(out))
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse ws %q: %w", s, err)
	}
	return v, nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}

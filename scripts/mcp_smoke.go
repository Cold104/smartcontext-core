// Smoke helper: drive smartcontext.exe mcp over stdio (CommandTransport).
// Usage (from go/): go run ./scripts/mcp_smoke.go
//
//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Cold104/smartcontext-core/winproc"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getcwd: %v\n", err)
		os.Exit(1)
	}
	exe := filepath.Join(wd, "smartcontext.exe")
	if _, err := os.Stat(exe); err != nil {
		fmt.Fprintf(os.Stderr, "missing %s — run go build first\n", exe)
		os.Exit(1)
	}

	root := filepath.Join(os.TempDir(), "sc-smoke-skills")
	skillDir := filepath.Join(root, "demo-skill")
	_ = os.MkdirAll(skillDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(
		"---\nname: demo-skill\ndescription: Demo skill for MCP binary smoke pack\n---\n\n# Demo\n",
	), 0o644)
	_ = os.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)

	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-smoke", Version: "0.0.1"}, nil)
	cmd := exec.CommandContext(ctx, exe, "mcp")
	cmd.Dir = wd
	cmd.Env = append(os.Environ(), "SMARTCONTEXT_SKILL_ROOTS="+root)
	// Windows: CREATE_NO_WINDOW + HideWindow so the MCP child never flashes a console
	// (covers CONSOLE-subsystem binaries and odd GUI-host spawn paths).
	winproc.HideConsole(cmd)
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tools/list: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("tools/list:")
	haveDiscover := false
	haveHost := false
	for _, tool := range tools.Tools {
		fmt.Printf("  - %s\n", tool.Name)
		switch tool.Name {
		case "discover_skills":
			haveDiscover = true
		case "host_info":
			haveHost = true
		}
	}
	if !haveDiscover {
		fmt.Fprintf(os.Stderr, "discover_skills missing from tools/list\n")
		os.Exit(1)
	}
	if !haveHost {
		fmt.Fprintf(os.Stderr, "host_info missing from tools/list\n")
		os.Exit(1)
	}

	hostRes, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "host_info",
		Arguments: map[string]any{},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "host_info: %v\n", err)
		os.Exit(1)
	}
	if hostRes.IsError {
		fmt.Fprintf(os.Stderr, "host_info error result\n")
		os.Exit(1)
	}
	fmt.Println("host_info: ok")
	for _, c := range hostRes.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			fmt.Println(tc.Text)
			break
		}
	}

	disc, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "discover_skills",
		Arguments: map[string]any{},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "discover_skills: %v\n", err)
		os.Exit(1)
	}
	if disc.IsError {
		fmt.Fprintf(os.Stderr, "discover_skills error result\n")
		os.Exit(1)
	}
	fmt.Println("discover_skills: ok")
	for _, c := range disc.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			var pretty map[string]any
			if err := json.Unmarshal([]byte(tc.Text), &pretty); err == nil {
				if host, ok := pretty["host"].(map[string]any); ok {
					b, _ := json.Marshal(host)
					fmt.Printf("discover_skills.host: %s\n", string(b))
				}
			}
			break
		}
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "pack_context",
		Arguments: map[string]any{
			"task": "demo skill smoke pack",
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pack_context: %v\n", err)
		os.Exit(1)
	}
	if res.IsError {
		fmt.Fprintf(os.Stderr, "pack_context error result\n")
		os.Exit(1)
	}

	fmt.Println("pack_context:")
	var selected float64
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			var pretty map[string]any
			if err := json.Unmarshal([]byte(tc.Text), &pretty); err == nil {
				if n, ok := pretty["selected_count"].(float64); ok {
					selected = n
				}
				b, _ := json.MarshalIndent(pretty, "", "  ")
				fmt.Println(string(b))
				break
			}
			fmt.Println(tc.Text)
			break
		}
	}
	if selected < 1 {
		fmt.Fprintf(os.Stderr, "expected selected_count >= 1, got %v\n", selected)
		os.Exit(1)
	}
	fmt.Println("SMOKE_OK")
}

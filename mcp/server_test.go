package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	scmcp "github.com/Cold104/smartcontext-core/mcp"
	"github.com/Cold104/smartcontext-core/version"
)

func TestPackContextToolSmoke(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := "---\nname: demo-skill\ndescription: Demo skill for MCP smoke pack\n---\n\n# Demo\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")
	t.Setenv("SMARTCONTEXT_OFFLOAD", "") // default-on

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := scmcp.NewServer()
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke-client", Version: "0.0.1"}, nil)

	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()

	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	wantTools := map[string]bool{
		"pack_context": false, "discover_skills": false, "host_info": false,
		"session_start": false, "skills_refine_list": false,
		"offload_tool_result": false,
	}
	for _, tool := range tools.Tools {
		if strings.HasPrefix(tool.Name, "factory_") || tool.Name == "pack_at_boundary" || tool.Name == "pro_loop" {
			t.Fatalf("Pro-only tool %q must not appear in Core tools/list", tool.Name)
		}
	}
	for _, tool := range tools.Tools {
		if _, ok := wantTools[tool.Name]; ok {
			wantTools[tool.Name] = true
		}
	}
	for name, ok := range wantTools {
		if !ok {
			t.Fatalf("%s not in tools/list: %+v", name, tools.Tools)
		}
	}

	hostRes, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "host_info",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("tools/call host_info: %v", err)
	}
	if hostRes.IsError {
		t.Fatalf("host_info returned error: %+v", hostRes.Content)
	}
	hostText, ok := hostRes.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("host_info expected TextContent, got %T", hostRes.Content[0])
	}
	var hostPayload map[string]any
	if err := json.Unmarshal([]byte(hostText.Text), &hostPayload); err != nil {
		t.Fatalf("host_info JSON: %v\n%s", err, hostText.Text)
	}
	if hostPayload["os"] != runtime.GOOS {
		t.Fatalf("host_info os=%v want %s", hostPayload["os"], runtime.GOOS)
	}
	if hostPayload["arch"] != runtime.GOARCH {
		t.Fatalf("host_info arch=%v want %s", hostPayload["arch"], runtime.GOARCH)
	}
	if n, _ := hostPayload["num_cpu"].(float64); n < 1 {
		t.Fatalf("host_info num_cpu=%v", hostPayload["num_cpu"])
	}

	disc, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "discover_skills",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("tools/call discover_skills: %v", err)
	}
	if disc.IsError {
		t.Fatalf("discover_skills returned error: %+v", disc.Content)
	}
	discText, ok := disc.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("discover_skills expected TextContent, got %T", disc.Content[0])
	}
	if !strings.Contains(discText.Text, "demo-skill") {
		t.Fatalf("discover_skills missing demo-skill:\n%s", discText.Text)
	}
	var discPayload map[string]any
	if err := json.Unmarshal([]byte(discText.Text), &discPayload); err != nil {
		t.Fatalf("discover_skills JSON: %v\n%s", err, discText.Text)
	}
	hostSum, _ := discPayload["host"].(map[string]any)
	if hostSum["os"] != runtime.GOOS {
		t.Fatalf("discover_skills host.os=%v want %s", hostSum["os"], runtime.GOOS)
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "pack_context",
		Arguments: map[string]any{
			"task":  "demo skill smoke pack",
			"roots": []string{root},
		},
	})
	if err != nil {
		t.Fatalf("tools/call pack_context: %v", err)
	}
	if res.IsError {
		t.Fatalf("pack_context returned error: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatal("expected content from pack_context")
	}

	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		t.Fatalf("pack JSON: %v\n%s", err, text.Text)
	}
	if payload["empty_catalog"] != false {
		t.Fatalf("empty_catalog=%v want false", payload["empty_catalog"])
	}
	if payload["task"] != "demo skill smoke pack" {
		t.Fatalf("task=%v", payload["task"])
	}
	if payload["version"] != version.Version {
		t.Fatalf("version=%v want %s", payload["version"], version.Version)
	}
	count, _ := payload["selected_count"].(float64)
	if count < 1 {
		t.Fatalf("selected_count=%v want >=1\n%s", payload["selected_count"], text.Text)
	}
	if !strings.Contains(text.Text, "demo-skill") {
		t.Fatalf("missing demo-skill in:\n%s", text.Text)
	}
}

func TestSessionTools(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", "")
	t.Setenv("SMARTCONTEXT_ROOT", "")
	t.Setenv("SMARTCONTEXT_FRIEND_ROOT", "")

	root := t.TempDir()
	skillDir := filepath.Join(root, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := "---\nname: demo-skill\ndescription: Demo skill for MCP session smoke\n---\n\n# Demo\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := scmcp.NewServer()
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke-client", Version: "0.0.1"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	sess, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "session_start",
		Arguments: map[string]any{
			"roots":        []string{root},
			"mark_session": true,
		},
	})
	if err != nil {
		t.Fatalf("session_start: %v", err)
	}
	if sess.IsError {
		t.Fatalf("session_start error: %+v", sess.Content)
	}
	sessText := sess.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(sessText, `"status": "ok"`) {
		t.Fatalf("session_start:\n%s", sessText)
	}

	refine, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "skills_refine_list",
		Arguments: map[string]any{"status": "pending"},
	})
	if err != nil {
		t.Fatalf("skills_refine_list: %v", err)
	}
	if refine.IsError {
		t.Fatalf("skills_refine_list error: %+v", refine.Content)
	}
}

func TestOffloadToolResultSmoke(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	t.Setenv("SMARTCONTEXT_OFFLOAD", "") // default-on
	t.Setenv("SMARTCONTEXT_LIFECYCLE", "1")
	t.Setenv("SMARTCONTEXT_STICKY_SESSION", "0")

	root := t.TempDir()
	skillDir := filepath.Join(root, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := "---\nname: demo-skill\ndescription: Demo skill for offload smoke\n---\n\n# Demo\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := scmcp.NewServer()
	client := mcp.NewClient(&mcp.Implementation{Name: "offload-client", Version: "0.0.1"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	found := false
	for _, tool := range tools.Tools {
		if tool.Name == "offload_tool_result" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("offload_tool_result not registered by default")
	}

	small, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "offload_tool_result",
		Arguments: map[string]any{
			"content": "tiny",
			"label":   "noop",
		},
	})
	if err != nil {
		t.Fatalf("offload small: %v", err)
	}
	if small.IsError {
		t.Fatalf("under-threshold should be JSON no-op, not tool error: %+v", small.Content)
	}
	smallText := small.Content[0].(*mcp.TextContent).Text
	var smallPayload map[string]any
	if err := json.Unmarshal([]byte(smallText), &smallPayload); err != nil {
		t.Fatalf("small JSON: %v\n%s", err, smallText)
	}
	if smallPayload["stored"] != false {
		t.Fatalf("stored=%v want false\n%s", smallPayload["stored"], smallText)
	}

	big := strings.Repeat("MCP_OFFLOAD_LINE\n", 2000)
	off, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "offload_tool_result",
		Arguments: map[string]any{
			"content": big,
			"label":   "mcp-log",
		},
	})
	if err != nil {
		t.Fatalf("offload big: %v", err)
	}
	if off.IsError {
		t.Fatalf("offload error: %+v", off.Content)
	}
	offText := off.Content[0].(*mcp.TextContent).Text
	var offPayload map[string]any
	if err := json.Unmarshal([]byte(offText), &offPayload); err != nil {
		t.Fatalf("offload JSON: %v\n%s", err, offText)
	}
	if offPayload["stored"] != true {
		t.Fatalf("stored=%v want true\n%s", offPayload["stored"], offText)
	}
	path, _ := offPayload["path"].(string)
	if path == "" {
		t.Fatalf("missing path:\n%s", offText)
	}

	pack, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "pack_context",
		Arguments: map[string]any{
			"task":   "demo offload recall",
			"roots":  []string{root},
			"format": "json",
		},
	})
	if err != nil {
		t.Fatalf("pack_context: %v", err)
	}
	if pack.IsError {
		t.Fatalf("pack_context error: %+v", pack.Content)
	}
	packText := pack.Content[0].(*mcp.TextContent).Text
	var packPayload map[string]any
	if err := json.Unmarshal([]byte(packText), &packPayload); err != nil {
		t.Fatalf("pack JSON: %v\n%s", err, packText)
	}
	dyn, _ := packPayload["markdown_dynamic"].(string)
	if !strings.Contains(dyn, "Tool-result recall") || !strings.Contains(dyn, path) {
		t.Fatalf("pack markdown_dynamic missing offload recall:\n%s", dyn)
	}
}

func TestOffloadDisabledWhenOptOut(t *testing.T) {
	t.Setenv("SMARTCONTEXT_OFFLOAD", "0")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := scmcp.NewServer()
	client := mcp.NewClient(&mcp.Implementation{Name: "offload-off-client", Version: "0.0.1"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "offload_tool_result" {
			t.Fatal("offload_tool_result should not register when SMARTCONTEXT_OFFLOAD=0")
		}
	}
}

func TestPackContextRequiresTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server := scmcp.NewServer()
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke-client", Version: "0.0.1"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "pack_context",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected IsError when task missing")
	}
}

// Orch / pointer-handoff MCP coverage lives in smartcontext-pro/mcp.

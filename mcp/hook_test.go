package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Cold104/smartcontext-core/core"
	scmcp "github.com/Cold104/smartcontext-core/mcp"
)

// Pro hook contract: EnrichPack sees selected items (rank/sticky/tokens) and
// the optional PostProcessStable hook rewrites markdown_stable before
// enrichment with markdown/estimated_tokens kept consistent.
func TestPackHooksReceiveItemsAndPostProcessStable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SMARTCONTEXT_HOME", home)
	root := t.TempDir()
	for _, s := range []struct{ name, desc string }{
		{"goal-skill", "goal objective workflow"},
		{"noise-skill", "docker kubernetes noise"},
	} {
		dir := filepath.Join(root, s.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		md := "---\nname: " + s.name + "\ndescription: " + s.desc + "\n---\n\n# body\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SMARTCONTEXT_SKILL_ROOTS", root)
	t.Setenv("SMARTCONTEXT_WORKSPACE_ROOTS", "")
	t.Setenv("SMARTCONTEXT_OFFLOAD", "0")
	core.ResetMarkdownStableCache()

	var seen core.PackResult
	var postCalls int
	server := scmcp.NewServer(scmcp.ServerOptions{
		PostProcessStable: func(req core.PackRequest, plain core.PackResult) string {
			postCalls++
			return "<!-- renderer:test -->\n" + plain.MarkdownStable
		},
		EnrichPack: func(req core.PackRequest, plain core.PackResult) (any, string) {
			seen = plain
			return plain, plain.Markdown
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "hook-client", Version: "0.0.1"}, nil)
	st, ct := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "pack_context",
		Arguments: map[string]any{
			"task": "docker kubernetes noise", "top_n": 2, "budget_tokens": 4000,
			"sticky_names": []string{"goal-skill"}, "clear_sticky": true,
		},
	})
	if err != nil || res.IsError {
		t.Fatalf("pack_context: %v %+v", err, res)
	}
	if postCalls != 1 {
		t.Fatalf("post-process calls=%d", postCalls)
	}
	if !strings.HasPrefix(seen.MarkdownStable, "<!-- renderer:test -->\n") {
		t.Fatalf("enricher did not see post-processed stable:\n%s", seen.MarkdownStable)
	}
	if seen.Markdown != seen.MarkdownStable+seen.MarkdownDynamic {
		t.Fatal("markdown not recomposed from stable+dynamic")
	}
	if seen.EstimatedTokens != core.EstimateTokens(seen.Markdown) {
		t.Fatalf("estimated_tokens stale: %d", seen.EstimatedTokens)
	}
	if len(seen.Items) != 2 {
		t.Fatalf("items=%+v", seen.Items)
	}
	for i, it := range seen.Items {
		if it.Rank != i+1 || it.EstimatedTokens <= 0 || it.Path == "" || it.Kind == "" {
			t.Fatalf("incomplete hook item: %+v", it)
		}
	}
	if !seen.Items[0].Sticky || seen.Items[0].Name != "goal-skill" || seen.Items[1].Sticky {
		t.Fatalf("sticky flags wrong: %+v", seen.Items)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &payload); err != nil {
		t.Fatal(err)
	}
	items, _ := payload["items"].([]any)
	first, _ := items[0].(map[string]any)
	for _, k := range []string{"rank", "estimated_tokens", "sticky", "score"} {
		if _, ok := first[k]; !ok {
			t.Fatalf("JSON item missing %q: %v", k, first)
		}
	}
}

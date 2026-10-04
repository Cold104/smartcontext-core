// Package mcp implements the PRIMARY SmartContext Core entry: stdio MCP server.
// Pro tools (factory / orch) live in github.com/Cold104/smartcontext-pro.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/lifecycle"
	"github.com/Cold104/smartcontext-core/version"
)

// PackEnricher optionally wraps a plain pack result (Pro sets this for
// pointer/AST meta). When nil, pack_context returns the plain PackResult.
type PackEnricher func(req core.PackRequest, plain core.PackResult) (result any, markdown string)

// ServerOptions configures optional Pro hooks without importing Pro packages.
type ServerOptions struct {
	// EnrichPack, when non-nil, wraps every pack_context result (Pro may no-op
	// internally when handoff/AST flags are off).
	EnrichPack PackEnricher
}

type packArgs struct {
	Task           string   `json:"task" jsonschema:"Task description to pack context for"`
	Kind           string   `json:"kind,omitempty" jsonschema:"all|skill|rule (default all)"`
	TopN           int      `json:"top_n,omitempty" jsonschema:"Max items to select (default 5)"`
	BudgetPreset   string   `json:"budget_preset,omitempty" jsonschema:"super_saver|balanced|max|auto|small|medium|large|minimum|maximum"`
	BudgetPct      float64  `json:"budget_pct,omitempty" jsonschema:"Budget as percent of naive corpus"`
	BudgetTokens   int      `json:"budget_tokens,omitempty" jsonschema:"Absolute token budget"`
	IncludeBodies  bool     `json:"include_bodies,omitempty" jsonschema:"Include skill bodies when packing"`
	Roots          []string `json:"roots,omitempty" jsonschema:"Optional workspace or skill roots for this call"`
	StickyNames    []string `json:"sticky_names,omitempty" jsonschema:"Force-include these skill names (mid-task recoverability)"`
	ClearSticky    bool     `json:"clear_sticky,omitempty" jsonschema:"Clear persisted session sticky names before packing"`
	Format         string   `json:"format,omitempty" jsonschema:"full (default: JSON+markdown texts) or json (structured JSON only; less stdio)"`
	PointerHandoff bool     `json:"pointer_handoff,omitempty" jsonschema:"Attach hash/pointer handoff envelope (Pro enricher / SMARTCONTEXT_POINTER_HANDOFF=1)"`
	ASTFilter      bool     `json:"ast_filter,omitempty" jsonschema:"Attach Core AST stub hop meta (Pro enricher / SMARTCONTEXT_AST_FILTER=1)"`
	ASTPaths       []string `json:"ast_paths,omitempty" jsonschema:"Optional source paths for AST stub"`
	ASTLanguage    string   `json:"ast_language,omitempty" jsonschema:"Optional language hint for AST stub"`
}

type offloadArgs struct {
	Content      string `json:"content" jsonschema:"Full tool result text to store on disk"`
	Label        string `json:"label,omitempty" jsonschema:"Optional filename label"`
	PreviewBytes int    `json:"preview_bytes,omitempty" jsonschema:"Max preview bytes (default 200 / industry ~50-token ask; pass 2048 for legacy)"`
	Force        bool   `json:"force,omitempty" jsonschema:"Store even when content is under the 8192-byte threshold"`
}

type discoverArgs struct {
	Roots []string `json:"roots,omitempty" jsonschema:"Optional workspace or skill roots to resolve"`
}

type hostInfoArgs struct{}

type sessionStartArgs struct {
	Roots       []string `json:"roots,omitempty" jsonschema:"Optional workspace or skill roots for catalog snapshot"`
	MarkSession bool     `json:"mark_session,omitempty" jsonschema:"Advance session watermark after snapshot (default false)"`
}

type refineListArgs struct {
	Status string `json:"status,omitempty" jsonschema:"pending|all (default pending)"`
}

// NewServer builds the SmartContext Core MCP server with pack/discover/host_info,
// session/refine, and optional offload. Factory/orch tools are not registered here.
func NewServer(opts ...ServerOptions) *mcp.Server {
	var opt ServerOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "smartcontext",
		Version: version.Version,
	}, nil)

	RegisterCoreTools(server, opt)
	return server
}

// RegisterCoreTools adds Core-only MCP tools onto an existing server.
// Pro calls this, then registers factory/orch tools on the same server.
func RegisterCoreTools(server *mcp.Server, opt ServerOptions) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "pack_context",
		Description: "Cache-friendly pack: markdown_stable is the procedural skill prefix " +
			"(pin across turns for provider KV-cache hits); markdown_dynamic is the episodic " +
			"task/meta tail (mutable per call). " +
			"Packs relevant skills for a task (OS-aware IDF rank, budget tiers, pack-time scrub). " +
			"budget_preset: super_saver|balanced|max|auto (1000/2000/8000/heuristic tokens; aliases small|medium|large); " +
			"legacy minimum|maximum (30/90% of naive); or budget_pct / budget_tokens. " +
			"Plan-mode: ask Super saver/Balanced/Max; 60s no answer → auto. " +
			"Session sticky_names keep goal skills across mid-task re-packs (clear_sticky to reset). " +
			"format=json omits duplicate markdown text content (less stdio). " +
			"Optional roots; pointer_handoff / ast_filter require a Pro enricher hook.",
	}, makeHandlePackContext(opt.EnrichPack))

	mcp.AddTool(server, &mcp.Tool{
		Name: "discover_skills",
		Description: "Read-only: resolve skill roots (env / walk-up / optional roots) and list " +
			"discovered skill names before pack. Includes a short host (os/arch/cpu) summary. " +
			"POC-grade list_roots companion.",
	}, handleDiscoverSkills)

	mcp.AddTool(server, &mcp.Tool{
		Name: "host_info",
		Description: "Read-only: report the MCP host machine (OS, arch, hostname, CPU count, " +
			"memory total/available when available, working dir, home, username). " +
			"Call before or alongside discover_skills when packing needs OS/resource context.",
	}, handleHostInfo)

	mcp.AddTool(server, &mcp.Tool{
		Name: "session_start",
		Description: "Lifecycle session snapshot: catalog skill delta vs watermark, pending refine " +
			"proposals (read-only), triage prefs. Optional mark_session advances the watermark. " +
			"Does not write SKILL.md files.",
	}, handleSessionStart)

	mcp.AddTool(server, &mcp.Tool{
		Name: "skills_refine_list",
		Description: "Read-only: list refine proposals from the local queue (~/.smartcontext/refine/queue). " +
			"Proposals never rewrite skills; Pro factory_apply with consent writes.",
	}, handleRefineList)

	if lifecycle.OffloadEnabled() {
		mcp.AddTool(server, &mcp.Tool{
			Name: "offload_tool_result",
			Description: "Lifecycle observation masking (default-on): store tool results ≥8192 bytes under " +
				"~/.smartcontext/offload/ and return path + short preview + sha256. Under threshold with " +
				"force=false returns a clear JSON no-op. Persists pointers for the next pack markdown_dynamic. " +
				"Disable with SMARTCONTEXT_OFFLOAD=0|false|off|no|disabled. Pair with tool-result-recall skill.",
		}, handleOffloadToolResult)
	}
}

func makeHandlePackContext(enrich PackEnricher) func(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args packArgs,
) (*mcp.CallToolResult, any, error) {
	return func(
		ctx context.Context,
		req *mcp.CallToolRequest,
		args packArgs,
	) (*mcp.CallToolResult, any, error) {
		_ = ctx
		_ = req

		if args.Task == "" {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "task is required"},
				},
			}, nil, nil
		}

		if args.ClearSticky {
			_ = lifecycle.ClearStickyNames(lifecycle.Home(""))
		}

		packReq := core.PackRequest{
			Task:           args.Task,
			Kind:           args.Kind,
			TopN:           args.TopN,
			BudgetPreset:   args.BudgetPreset,
			BudgetPct:      args.BudgetPct,
			BudgetTokens:   args.BudgetTokens,
			IncludeBodies:  args.IncludeBodies,
			Roots:          args.Roots,
			StickyNames:    args.StickyNames,
			PointerHandoff: args.PointerHandoff,
			ASTFilter:      args.ASTFilter,
			ASTPaths:       args.ASTPaths,
			ASTLanguage:    args.ASTLanguage,
		}

		plain := lifecycle.PackContext(packReq)
		var (
			payload  []byte
			err      error
			markdown string
			result   any
		)
		if enrich != nil {
			result, markdown = enrich(packReq, plain)
			payload, err = json.MarshalIndent(result, "", "  ")
		} else {
			markdown = plain.Markdown
			result = plain
			payload, err = json.MarshalIndent(plain, "", "  ")
		}
		if err != nil {
			return nil, nil, fmt.Errorf("marshal pack result: %w", err)
		}

		contents := []mcp.Content{
			&mcp.TextContent{Text: string(payload)},
		}
		if strings.ToLower(strings.TrimSpace(args.Format)) != "json" {
			contents = append(contents, &mcp.TextContent{Text: markdown})
		}

		return &mcp.CallToolResult{
			Content: contents,
		}, result, nil
	}
}

func handleOffloadToolResult(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args offloadArgs,
) (*mcp.CallToolResult, any, error) {
	_ = ctx
	_ = req
	out, err := lifecycle.OffloadToolResult(lifecycle.OffloadRequest{
		Content:      args.Content,
		Label:        args.Label,
		PreviewBytes: args.PreviewBytes,
		Force:        args.Force,
		Source:       lifecycle.SourceMCP,
	})
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
		}, nil, nil
	}
	payload, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
	}, out, nil
}

func handleDiscoverSkills(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args discoverArgs,
) (*mcp.CallToolResult, any, error) {
	_ = ctx
	_ = req

	report := core.ReportDiscovery(args.Roots)
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal discover result: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(payload)},
		},
	}, report, nil
}

func handleHostInfo(
	ctx context.Context,
	req *mcp.CallToolRequest,
	_ hostInfoArgs,
) (*mcp.CallToolResult, any, error) {
	_ = ctx
	_ = req

	info := core.CollectHostInfo()
	payload, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal host_info: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(payload)},
		},
	}, info, nil
}

func handleSessionStart(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args sessionStartArgs,
) (*mcp.CallToolResult, any, error) {
	_ = ctx
	_ = req

	result := lifecycle.SessionStart(lifecycle.SessionStartRequest{
		Roots:       args.Roots,
		MarkSession: args.MarkSession,
		Source:      lifecycle.SourceMCP,
	})
	return jsonToolResult(result)
}

func handleRefineList(
	ctx context.Context,
	req *mcp.CallToolRequest,
	args refineListArgs,
) (*mcp.CallToolResult, any, error) {
	_ = ctx
	_ = req

	result := lifecycle.ListProposals("", args.Status)
	return jsonToolResult(result)
}

func jsonToolResult(v any) (*mcp.CallToolResult, any, error) {
	payload, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal tool result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(payload)},
		},
	}, v, nil
}

// RunStdio starts the MCP server on stdin/stdout until the client disconnects.
func RunStdio(ctx context.Context) error {
	return NewServer().Run(ctx, &mcp.StdioTransport{})
}

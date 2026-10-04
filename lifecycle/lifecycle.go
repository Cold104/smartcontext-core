// Package lifecycle holds triage, aliases, telemetry, session, and refine
// glue between MCP tools and Core. Forge/Lathe stays out of this package.
// Skill factory writes live in internal/factory (consent-gated).
package lifecycle

import (
	"strings"
	"time"

	"github.com/Cold104/smartcontext-core/core"
	"github.com/Cold104/smartcontext-core/version"
)

// PackContext wraps Core pack with triage prefs, alias expansion, sticky names, and telemetry.
func PackContext(req core.PackRequest) core.PackResult {
	ApplyBudgetTier(&req)
	home := Home("")
	if Enabled() {
		EnsureLayout(home)
		aliases := LoadAliasMap(home)
		req.Task = ExpandTaskTokens(req.Task, aliases)
		if StickySessionEnabled() && len(req.StickyNames) == 0 {
			req.StickyNames = LoadStickyNames(home)
		}

		start := time.Now()
		result := core.PackContext(req, version.Version)
		result = applyTriageToPack(result, LoadTriagePrefs(home))
		result = applyOffloadRecallHint(result, home)
		latencyMS := float64(time.Since(start).Microseconds()) / 1000.0

		RecordPackEvent(NewPackEvent(result, latencyMS, SourceMCP, HashTask(req.Task)), home)
		if StickySessionEnabled() && len(result.SelectedNames) > 0 {
			_ = SaveStickyNames(result.SelectedNames, HashTask(req.Task), home)
		}

		if result.Note == "" {
			result.Note = "lifecycle: triage + aliases + sticky + local telemetry"
		} else {
			result.Note = result.Note + "; lifecycle: triage+aliases+sticky+telemetry"
		}
		return result
	}

	result := core.PackContext(req, version.Version)
	if result.Note == "" {
		result.Note = "lifecycle disabled (SMARTCONTEXT_LIFECYCLE)"
	}
	return result
}

func applyTriageToPack(result core.PackResult, prefs TriagePrefs) core.PackResult {
	if prefs.Skills == nil || len(result.Items) == 0 {
		return result
	}
	var pinned, rest []core.PackItem
	for _, item := range result.Items {
		st := "keep"
		if t, ok := prefs.Skills[item.Name]; ok && t.Status != "" {
			st = t.Status
		}
		if st == "deprecate" {
			continue
		}
		if st == "pin" {
			pinned = append(pinned, item)
		} else {
			rest = append(rest, item)
		}
	}
	items := append(pinned, rest...)
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	result.Items = items
	result.SelectedNames = names
	result.SelectedCount = len(items)
	return result
}

// applyOffloadRecallHint injects last offload path+preview into markdown_dynamic.
func applyOffloadRecallHint(result core.PackResult, home string) core.PackResult {
	hint := FormatOffloadRecallHint(home)
	if hint == "" {
		return result
	}
	dyn := result.MarkdownDynamic
	if dyn != "" && !strings.HasSuffix(dyn, "\n") {
		dyn += "\n"
	}
	dyn += hint
	result.MarkdownDynamic = dyn
	result.Markdown = result.MarkdownStable + dyn
	result.EstimatedTokens = core.EstimateTokens(result.Markdown)
	return result
}

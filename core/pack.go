package core

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// PackRequest is the thin pack_context input surface (MCP-facing).
type PackRequest struct {
	Task          string
	Kind          string
	TopN          int
	BudgetPreset  string
	BudgetPct     float64
	BudgetTokens  int
	IncludeBodies bool
	RedactSecrets *bool // nil = default on
	// Roots are optional workspace or skill roots for this call (host-provided).
	// When empty, ResolveRoots uses env / cwd walk-up discovery.
	Roots []string
	// StickyNames are skill/rule names to force-include across mid-task re-packs
	// (goal recoverability). Merged ahead of fresh rank hits.
	StickyNames []string
	// PointerHandoff opts into WS7 hash/pointer pack meta (also env SMARTCONTEXT_POINTER_HANDOFF).
	PointerHandoff bool
	// ASTFilter opts into WS10 Core AST stub hop (also env SMARTCONTEXT_AST_FILTER).
	ASTFilter bool
	// ASTPaths are optional source paths for the AST stub (ignored when AST off).
	ASTPaths []string
	// ASTLanguage is an optional language hint for the AST stub.
	ASTLanguage string
}

// PackItem is one selected skill/rule card in a pack.
type PackItem struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Path        string   `json:"path"`
	Description string   `json:"description,omitempty"`
	Score       float64  `json:"score"`
	Body        string   `json:"body,omitempty"`
	Related     []string `json:"related,omitempty"`
}

// PackResult is the pack_context response (IDF rank + budget dial + scrub).
type PackResult struct {
	Task            string     `json:"task"`
	BudgetTokens    int        `json:"budget_tokens"`
	BudgetPreset    string     `json:"budget_preset,omitempty"`
	BudgetPct       float64    `json:"budget_pct,omitempty"`
	NaiveTokens     int        `json:"naive_tokens,omitempty"`
	EstimatedTokens int        `json:"estimated_tokens"`
	SelectedCount   int        `json:"selected_count"`
	CatalogSize     int        `json:"catalog_size"`
	EmptyCatalog    bool       `json:"empty_catalog"`
	SelectedNames   []string   `json:"selected_names"`
	Related         []string   `json:"related,omitempty"`
	Items           []PackItem `json:"items"`
	Roots           []string   `json:"roots"`
	RootCount       int        `json:"root_count"`
	Markdown        string     `json:"markdown"`
	// MarkdownStable is the cache-friendly skills block (no task/meta); hosts may pin it.
	MarkdownStable string `json:"markdown_stable,omitempty"`
	// MarkdownDynamic is per-call task/budget/security (may change every pack).
	MarkdownDynamic string      `json:"markdown_dynamic,omitempty"`
	Security        ScrubReport `json:"security"`
	Version         string      `json:"version,omitempty"`
	Note            string      `json:"note"`
}

type scoredEntry struct {
	entry Entry
	score float64
}

// PackContext discovers skills, ranks by thin IDF (+ OS/alias), packs under a token budget, scrubs bodies.
func PackContext(req PackRequest, version string) PackResult {
	if req.Kind == "" {
		req.Kind = "all"
	}
	if req.TopN <= 0 {
		req.TopN = 5
	}
	redact := true
	if req.RedactSecrets != nil {
		redact = *req.RedactSecrets
	}

	cat := BuildCatalog(req.Roots)
	entries := filterKind(cat.Entries, req.Kind)

	budget, err := ResolveBudget(entries, req.Task, req.BudgetPreset, req.BudgetPct, req.BudgetTokens)
	if err != nil {
		return PackResult{
			Task:         req.Task,
			CatalogSize:  len(entries),
			EmptyCatalog: len(entries) == 0,
			Roots:        cat.Roots,
			RootCount:    len(cat.Roots),
			Version:      version,
			Note:         "budget error: " + err.Error(),
			Markdown:     "# SmartContext pack\n\nbudget error: " + err.Error() + "\n",
		}
	}

	selected := rankAndSelect(entries, req.Task, req.TopN)
	selected = mergeStickySelections(entries, selected, req.StickyNames, req.TopN)
	items, used, scrub := fitItemsToBudget(req.Task, selected, budget.BudgetTokens, req.IncludeBodies, DefaultBodyChars, redact)

	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	related := CollectRelatedNames(items)

	empty := len(entries) == 0
	// G-1 Prompt-Cache Aligned: reuse markdown_stable when selected_names set
	// (+ version) is unchanged; always rebuild markdown_dynamic for the task.
	memoKey := StableMemoKey(names, version)
	var stable, dynamic string
	if cached, ok := lookupStableMemo(memoKey); ok {
		stable = cached
		dynamic = formatPackMarkdownDynamic(req.Task, items, related, cat.Roots, empty, budget, used, scrub)
	} else {
		stable = formatPackMarkdownStable(items, version, empty)
		dynamic = formatPackMarkdownDynamic(req.Task, items, related, cat.Roots, empty, budget, used, scrub)
		storeStableMemo(memoKey, stable)
	}
	md := stable + dynamic

	note := "v1.3: Prompt-Cache Aligned stable reuse + OS-aware IDF + budget tiers + scrub"
	if empty {
		note = "v1.3: empty catalog (no SKILL.md under roots)"
	}

	return PackResult{
		Task:            req.Task,
		BudgetTokens:    budget.BudgetTokens,
		BudgetPreset:    budget.BudgetPreset,
		BudgetPct:       budget.BudgetPct,
		NaiveTokens:     budget.NaiveTokens,
		EstimatedTokens: EstimateTokens(md),
		SelectedCount:   len(items),
		CatalogSize:     len(entries),
		EmptyCatalog:    empty,
		SelectedNames:   names,
		Related:         related,
		Items:           items,
		Roots:           cat.Roots,
		RootCount:       len(cat.Roots),
		Markdown:        md,
		MarkdownStable:  stable,
		MarkdownDynamic: dynamic,
		Security:        scrub,
		Version:         version,
		Note:            note,
	}
}

// mergeStickySelections forces sticky names (when present in the catalog) ahead of
// ranked hits so mid-task re-packs do not drop goal skills when phrasing drifts.
func mergeStickySelections(entries []Entry, ranked []scoredEntry, sticky []string, topN int) []scoredEntry {
	if len(sticky) == 0 {
		return ranked
	}
	byName := make(map[string]Entry, len(entries))
	for _, e := range entries {
		byName[strings.ToLower(e.Name)] = e
	}
	seen := map[string]struct{}{}
	var out []scoredEntry
	for _, name := range sticky {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		e, ok := byName[key]
		if !ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, scoredEntry{entry: e, score: 1e9}) // pin ahead of IDF
	}
	skillSlots := topN
	if skillSlots <= 0 {
		skillSlots = 5
	}
	stickySkills := 0
	for _, s := range out {
		if strings.ToLower(s.entry.Kind) != "rule" {
			stickySkills++
		}
	}
	for _, s := range ranked {
		key := strings.ToLower(s.entry.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		isRule := strings.ToLower(s.entry.Kind) == "rule"
		if !isRule {
			if stickySkills >= skillSlots {
				continue
			}
			stickySkills++
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

func filterKind(entries []Entry, kind string) []Entry {
	if kind == "" || kind == "all" {
		return entries
	}
	var out []Entry
	for _, e := range entries {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

func readBodySnippet(path string, maxBytes int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := string(data)
	// Strip frontmatter for the card body.
	if strings.HasPrefix(text, "---") {
		rest := text[3:]
		if i := strings.Index(rest, "\n---"); i >= 0 {
			text = strings.TrimSpace(rest[i+4:])
		}
	}
	if maxBytes > 0 && len(text) > maxBytes {
		return text[:maxBytes] + "\n…"
	}
	return text
}

// IdenticalPrefixOverlapPct returns how much of two strings share an identical
// byte prefix, as a percentage of the longer length (0–100). Equal → 100.
// Used as a cache-hit proxy: identical selected skill sets should score 100 on
// markdown_stable (provider KV-cache % remains host-measured).
func IdenticalPrefixOverlapPct(a, b string) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	den := len(a)
	if len(b) > den {
		den = len(b)
	}
	if den == 0 {
		return 100
	}
	return 100 * float64(i) / float64(den)
}

// formatPackMarkdownParts splits the pack into a cache-friendly stable prefix
// (skills, name-sorted, no task) and a dynamic suffix (task/budget/security).
// Hosts should pin markdown_stable across turns when the skill set is unchanged;
// mutating only markdown_dynamic preserves provider prefix-cache friendliness.
func formatPackMarkdownParts(task string, items []PackItem, related []string, roots []string, version string, empty bool, budget BudgetResolution, used int, scrub ScrubReport) (stable, dynamic string) {
	return formatPackMarkdownStable(items, version, empty),
		formatPackMarkdownDynamic(task, items, related, roots, empty, budget, used, scrub)
}

// formatPackMarkdownStable builds the procedural (Prompt-Cache Aligned) prefix.
func formatPackMarkdownStable(items []PackItem, version string, empty bool) string {
	var stab strings.Builder
	stab.WriteString("# SmartContext pack\n\n")
	stab.WriteString(fmt.Sprintf("- **version**: %s\n\n", version))
	stab.WriteString("## Skills\n\n")
	if empty {
		stab.WriteString("No SKILL.md files found under skill roots.\n")
		stab.WriteString("Install skills under `.cursor/skills` or set `SMARTCONTEXT_SKILL_ROOTS`.\n\n")
	} else if len(items) == 0 {
		stab.WriteString("(none selected)\n\n")
	} else {
		sorted := append([]PackItem(nil), items...)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Name == sorted[j].Name {
				return sorted[i].Kind < sorted[j].Kind
			}
			return sorted[i].Name < sorted[j].Name
		})
		for _, item := range sorted {
			stab.WriteString(fmt.Sprintf("### %s\n\n", item.Name))
			stab.WriteString(fmt.Sprintf("- **kind**: %s\n", item.Kind))
			stab.WriteString(fmt.Sprintf("- **path**: `%s`\n", item.Path))
			if item.Description != "" {
				stab.WriteString(fmt.Sprintf("- **description**: %s\n", item.Description))
			}
			if len(item.Related) > 0 {
				stab.WriteString(fmt.Sprintf("- **related**: %s\n", strings.Join(item.Related, ", ")))
			}
			if item.Body != "" {
				stab.WriteString("\n")
				stab.WriteString(item.Body)
				stab.WriteString("\n")
			}
			stab.WriteString("\n")
		}
	}
	return stab.String()
}

// formatPackMarkdownDynamic builds the episodic (per-call) suffix.
func formatPackMarkdownDynamic(task string, items []PackItem, related []string, roots []string, empty bool, budget BudgetResolution, used int, scrub ScrubReport) string {
	var dyn strings.Builder
	dyn.WriteString("## Call context\n\n")
	dyn.WriteString(fmt.Sprintf("- **task**: %s\n", task))
	dyn.WriteString(fmt.Sprintf("- **budget_tokens**: %d\n", budget.BudgetTokens))
	if budget.BudgetPreset != "" {
		dyn.WriteString(fmt.Sprintf("- **budget_preset**: %s\n", budget.BudgetPreset))
	}
	if budget.NaiveTokens > 0 {
		dyn.WriteString(fmt.Sprintf("- **naive_tokens**: %d\n", budget.NaiveTokens))
	}
	if used > 0 {
		dyn.WriteString(fmt.Sprintf("- **estimated_used**: %d\n", used))
	}
	dyn.WriteString(fmt.Sprintf("- **empty_catalog**: %v\n", empty))
	dyn.WriteString(fmt.Sprintf("- **selected_count**: %d\n", len(items)))
	dyn.WriteString(fmt.Sprintf("- **root_count**: %d\n", len(roots)))
	if len(items) > 0 {
		names := make([]string, 0, len(items))
		for _, it := range items {
			names = append(names, it.Name)
		}
		dyn.WriteString(fmt.Sprintf("- **selected_names**: %s\n", strings.Join(names, ", ")))
		// Scores live only in the dynamic tail so stable skill bodies stay byte-stable.
		for _, it := range items {
			dyn.WriteString(fmt.Sprintf("- **score:%s**: %.3f\n", it.Name, it.Score))
		}
	}
	if len(related) > 0 {
		dyn.WriteString(fmt.Sprintf("- **related**: %s\n", strings.Join(related, ", ")))
	}
	dyn.WriteString("\n")
	dyn.WriteString(formatSecurityMarkdown(scrub))
	return dyn.String()
}

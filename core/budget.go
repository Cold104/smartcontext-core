package core

import (
	"fmt"
	"math"
	"strings"
)

// Absolute plan-mode / lifecycle tier token caps (hard budgets).
// Aliases: small→super_saver, medium→balanced, large→max.
const (
	BudgetTierSuperSaver = 1000
	BudgetTierBalanced   = 2000
	BudgetTierMax        = 8000
)

// Fraction-of-naive dial (legacy). Mirrors Python NAIVE_BUDGET_PRESETS for
// minimum/maximum. Prefer absolute tiers (super_saver|balanced|max|auto) for
// plan-mode UX; use budget_pct for an explicit naive fraction.
var NaiveBudgetPresets = map[string]int{
	"minimum": 30,
	"maximum": 90,
}

// AbsoluteBudgetTiers maps canonical + alias preset names to hard token caps.
// "auto" is handled separately (heuristic clamped to [SuperSaver, Max]).
var AbsoluteBudgetTiers = map[string]int{
	"super_saver": BudgetTierSuperSaver,
	"small":       BudgetTierSuperSaver,
	"balanced":    BudgetTierBalanced,
	"medium":      BudgetTierBalanced,
	"max":         BudgetTierMax,
	"large":       BudgetTierMax,
}

const (
	BudgetFloorTokens          = 256
	EmptyCatalogBudgetFallback = 2000
	DefaultBodyChars           = 1200
	NaiveBodyChars             = 50_000
	DefaultBudgetPreset        = "balanced"
)

// BudgetResolution is the resolved hard token gate for a pack.
type BudgetResolution struct {
	BudgetTokens int     `json:"budget_tokens"`
	NaiveTokens  int     `json:"naive_tokens"`
	BudgetPct    float64 `json:"budget_pct,omitempty"`
	BudgetPreset string  `json:"budget_preset,omitempty"`
	EmptyCatalog bool    `json:"empty_catalog"`
	Mode         string  `json:"mode"`
}

// EstimateTokens is a heuristic (~4 chars/token). No model tokenizer required.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return max(1, (len(text)+3)/4)
}

// CanonicalBudgetPreset normalizes tier / legacy aliases to a stable name.
func CanonicalBudgetPreset(preset string) string {
	switch strings.TrimSpace(strings.ToLower(preset)) {
	case "super_saver", "small":
		return "super_saver"
	case "balanced", "medium", "":
		return "balanced"
	case "max", "large":
		return "max"
	case "auto":
		return "auto"
	case "minimum":
		return "minimum"
	case "maximum":
		return "maximum"
	default:
		return strings.TrimSpace(strings.ToLower(preset))
	}
}

// IsAbsoluteBudgetTier reports whether preset is an absolute tier (incl. auto/aliases).
func IsAbsoluteBudgetTier(preset string) bool {
	p := strings.TrimSpace(strings.ToLower(preset))
	if p == "auto" {
		return true
	}
	_, ok := AbsoluteBudgetTiers[p]
	return ok
}

// AutoBudgetTokens heuristics from task length + catalog size, clamped to tier min/max.
func AutoBudgetTokens(task string, catalogSize int) int {
	taskTok := EstimateTokens(task)
	if catalogSize < 0 {
		catalogSize = 0
	}
	// Short task + small catalog → near super_saver; grow toward max.
	raw := BudgetTierSuperSaver + taskTok*4 + catalogSize*25
	if raw < BudgetTierSuperSaver {
		raw = BudgetTierSuperSaver
	}
	if raw > BudgetTierMax {
		raw = BudgetTierMax
	}
	return raw
}

// ResolveBudget picks an absolute budget_tokens from preset / pct / absolute.
// Absolute budget_tokens is mutually exclusive with preset and budget_pct.
//
// Absolute tiers: super_saver|balanced|max|auto (+ small|medium|large aliases).
// Legacy naive fractions: minimum|maximum (or budget_pct).
func ResolveBudget(entries []Entry, task string, preset string, budgetPct float64, budgetTokens int) (BudgetResolution, error) {
	absoluteSet := budgetTokens > 0
	pctSet := budgetPct > 0
	preset = strings.TrimSpace(strings.ToLower(preset))
	presetSet := preset != ""

	if absoluteSet && (pctSet || presetSet) {
		return BudgetResolution{}, fmt.Errorf("budget_tokens is mutually exclusive with preset and budget_pct")
	}
	if pctSet && presetSet {
		return BudgetResolution{}, fmt.Errorf("preset and budget_pct are mutually exclusive")
	}

	if absoluteSet {
		return BudgetResolution{
			BudgetTokens: budgetTokens,
			Mode:         "absolute",
			EmptyCatalog: len(entries) == 0,
		}, nil
	}

	if len(entries) == 0 {
		if preset == "" {
			preset = DefaultBudgetPreset
		}
		canon := CanonicalBudgetPreset(preset)
		tokens := EmptyCatalogBudgetFallback
		switch {
		case canon == "auto":
			tokens = AutoBudgetTokens(task, 0)
		case IsAbsoluteBudgetTier(preset):
			tokens = AbsoluteBudgetTiers[preset]
		case canon == "super_saver" || canon == "balanced" || canon == "max":
			tokens = AbsoluteBudgetTiers[canon]
		}
		return BudgetResolution{
			BudgetTokens: tokens,
			NaiveTokens:  0,
			BudgetPreset: canon,
			EmptyCatalog: true,
			Mode:         "empty_fallback",
		}, nil
	}

	// Absolute tier presets (incl. auto). Default empty preset → balanced.
	if !pctSet {
		if preset == "" {
			preset = DefaultBudgetPreset
		}
		if IsAbsoluteBudgetTier(preset) {
			canon := CanonicalBudgetPreset(preset)
			tokens := BudgetTierBalanced
			if canon == "auto" {
				tokens = AutoBudgetTokens(task, len(entries))
			} else if t, ok := AbsoluteBudgetTiers[preset]; ok {
				tokens = t
			}
			if tokens < BudgetFloorTokens {
				tokens = BudgetFloorTokens
			}
			return BudgetResolution{
				BudgetTokens: tokens,
				NaiveTokens:  0,
				BudgetPreset: canon,
				EmptyCatalog: false,
				Mode:         "tier_absolute",
			}, nil
		}
	}

	var pct float64
	var resolvedPreset string
	if pctSet {
		pct = budgetPct
	} else {
		p, ok := NaiveBudgetPresets[preset]
		if !ok {
			return BudgetResolution{}, fmt.Errorf("invalid budget preset %q", preset)
		}
		resolvedPreset = preset
		pct = float64(p)
	}
	if pct <= 0 || pct > 100 {
		return BudgetResolution{}, fmt.Errorf("budget_pct must be in (0, 100]; got %v", pct)
	}

	naiveTokens := estimateNaiveTokens(entries, task)
	resolved := int(math.Floor(float64(naiveTokens) * pct / 100.0))
	if resolved < BudgetFloorTokens {
		resolved = BudgetFloorTokens
	}
	return BudgetResolution{
		BudgetTokens: resolved,
		NaiveTokens:  naiveTokens,
		BudgetPct:    pct,
		BudgetPreset: resolvedPreset,
		EmptyCatalog: false,
		Mode:         "naive_fraction",
	}, nil
}

func estimateNaiveTokens(entries []Entry, task string) int {
	var b strings.Builder
	b.WriteString("# Naive full corpus pack\nTask: ")
	b.WriteString(task)
	b.WriteString("\n")
	for _, e := range entries {
		body := readBodySnippet(e.Path, NaiveBodyChars)
		b.WriteString(fmt.Sprintf("## [%s] %s\n", e.Kind, e.Name))
		b.WriteString(fmt.Sprintf("Path: %s\n", e.Path))
		b.WriteString(body)
		b.WriteString("\n")
	}
	return EstimateTokens(b.String())
}

// fitItemsToBudget keeps ranked cards (and optional bodies) under budget_tokens.
func fitItemsToBudget(task string, selected []scoredEntry, budgetTokens int, includeBodies bool, bodyChars int, redact bool) ([]PackItem, int, ScrubReport) {
	if bodyChars <= 0 {
		bodyChars = DefaultBodyChars
	}
	report := emptyScrubReport(redact)
	header := fmt.Sprintf("# SmartContext pack\nTask: %s\nBudget: %d tokens\n", task, budgetTokens)
	used := EstimateTokens(header)
	items := make([]PackItem, 0, len(selected))

	for _, s := range selected {
		cardText := fmt.Sprintf("## [%s] %s\n%s\nPath: %s\n",
			s.entry.Kind, s.entry.Name, s.entry.Description, s.entry.Path)
		cardTokens := EstimateTokens(cardText)
		if used+cardTokens > budgetTokens && len(items) > 0 {
			break
		}
		item := PackItem{
			Name:        s.entry.Name,
			Kind:        s.entry.Kind,
			Path:        s.entry.Path,
			Description: s.entry.Description,
			Score:       s.score,
			Related:     append([]string(nil), s.entry.Related...),
		}
		if used+cardTokens > budgetTokens {
			items = append(items, item)
			used += cardTokens
			break
		}

		remaining := budgetTokens - used - cardTokens
		if includeBodies && remaining > 40 {
			raw := readBodySnippet(s.entry.Path, bodyChars)
			body, bodyReport := ScrubBody(raw, s.entry.Name, s.entry.Path, redact)
			mergeScrubReports(&report, bodyReport)
			maxBodyChars := min(bodyChars, remaining*4)
			if len(body) > maxBodyChars {
				body = body[:maxBodyChars] + "\n… [truncated for token budget]"
			}
			item.Body = body
			cardTokens += EstimateTokens(body)
		}
		items = append(items, item)
		used += cardTokens
		if used >= budgetTokens {
			break
		}
	}
	return items, used, report
}

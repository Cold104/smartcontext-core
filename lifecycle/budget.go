package lifecycle

import (
	"strings"
	"time"

	"github.com/Cold104/smartcontext-core/core"
)

// PlanModeBudgetAskTimeout is how long the host agent should wait for a
// Super saver / Balanced / Max answer before selecting auto.
const PlanModeBudgetAskTimeout = 60 * time.Second

// ResolvePlanModeBudget maps plan-mode / MCP budget_preset names onto Core.
// Absolute tiers (super_saver|balanced|max|auto + small|medium|large) are
// passed through; Core resolves them to hard token caps.
//
// The 60s no-answer → auto policy is enforced by the host agent / plan UX
// (see go/docs/BUDGET_TIERS.md). Callers that time out should pass "auto".
func ResolvePlanModeBudget(preset string) string {
	p := strings.TrimSpace(strings.ToLower(preset))
	if p == "" {
		return ""
	}
	return core.CanonicalBudgetPreset(p)
}

// ApplyBudgetTier normalizes req.BudgetPreset for Core when using tier names.
// No-op when absolute budget_tokens or budget_pct is already set.
func ApplyBudgetTier(req *core.PackRequest) {
	if req == nil {
		return
	}
	if req.BudgetTokens > 0 || req.BudgetPct > 0 {
		return
	}
	if req.BudgetPreset == "" {
		return
	}
	raw := strings.TrimSpace(strings.ToLower(req.BudgetPreset))
	if core.IsAbsoluteBudgetTier(raw) || raw == "minimum" || raw == "maximum" {
		req.BudgetPreset = ResolvePlanModeBudget(raw)
	}
}

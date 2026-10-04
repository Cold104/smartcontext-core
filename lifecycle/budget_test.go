package lifecycle

import (
	"testing"
	"time"

	"github.com/Cold104/smartcontext-core/core"
)

func TestPlanModeBudgetAskTimeout(t *testing.T) {
	if PlanModeBudgetAskTimeout != 60*time.Second {
		t.Fatalf("timeout=%v", PlanModeBudgetAskTimeout)
	}
}

func TestResolvePlanModeBudgetAliases(t *testing.T) {
	if got := ResolvePlanModeBudget("small"); got != "super_saver" {
		t.Fatalf("small→%s", got)
	}
	if got := ResolvePlanModeBudget("MEDIUM"); got != "balanced" {
		t.Fatalf("medium→%s", got)
	}
	if got := ResolvePlanModeBudget("large"); got != "max" {
		t.Fatalf("large→%s", got)
	}
}

func TestApplyBudgetTier(t *testing.T) {
	req := core.PackRequest{BudgetPreset: "small"}
	ApplyBudgetTier(&req)
	if req.BudgetPreset != "super_saver" {
		t.Fatalf("preset=%q", req.BudgetPreset)
	}
	req2 := core.PackRequest{BudgetPreset: "balanced", BudgetTokens: 500}
	ApplyBudgetTier(&req2)
	if req2.BudgetPreset != "balanced" {
		t.Fatal("should not rewrite when budget_tokens set")
	}
}

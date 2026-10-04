package core

import (
	"os"
	"strings"
)

// LatencyBudget is the Go pack/discover wall-clock gate (ns for p50).
// Product defaults leave ~3× headroom over local corpus p50 (~6–12 ms pack,
// ~3–7 ms BuildCatalog). CI uses ForCI (default 2×) so shared runners do not
// force loosening the product claim — mirrors Python Budget.for_ci.
type LatencyBudget struct {
	PackNSP50  int64  // max allowed PackContext p50 (nanoseconds)
	BuildNSP50 int64  // max allowed BuildCatalog p50 (nanoseconds)
	Label      string // "product" | "ci"
}

// ProductLatencyBudget is the buyer-facing / local gate.
func ProductLatencyBudget() LatencyBudget {
	return LatencyBudget{
		PackNSP50:  36_000_000, // 36 ms
		BuildNSP50: 21_000_000, // 21 ms
		Label:      "product",
	}
}

// LatencyBudgetForCI returns looser budgets for shared CI runners.
func LatencyBudgetForCI(multiplier float64) LatencyBudget {
	if multiplier <= 0 {
		multiplier = 2.0
	}
	base := ProductLatencyBudget()
	return LatencyBudget{
		PackNSP50:  int64(float64(base.PackNSP50) * multiplier),
		BuildNSP50: int64(float64(base.BuildNSP50) * multiplier),
		Label:      "ci",
	}
}

// ResolveLatencyBudget picks product or CI budgets.
// CI when ci=true, or env CI / SMARTCONTEXT_BENCH_CI is truthy.
func ResolveLatencyBudget(ci bool) LatencyBudget {
	envCI := strings.EqualFold(strings.TrimSpace(os.Getenv("SMARTCONTEXT_BENCH_CI")), "1") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("SMARTCONTEXT_BENCH_CI")), "true") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("SMARTCONTEXT_BENCH_CI")), "yes")
	githubCI := strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "1") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "true") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("CI")), "yes")
	if ci || envCI || githubCI {
		return LatencyBudgetForCI(2.0)
	}
	return ProductLatencyBudget()
}

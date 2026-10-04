package eval

import (
	"math"
	"sort"
	"strings"
)

func HitAny(selected, gold []string) bool {
	if len(gold) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, n := range selected {
		set[strings.ToLower(n)] = struct{}{}
	}
	for _, g := range gold {
		if _, ok := set[strings.ToLower(g)]; ok {
			return true
		}
	}
	return false
}

func RecallAtK(selected, gold []string, k int) float64 {
	if len(gold) == 0 {
		return 0
	}
	top := selected
	if len(top) > k {
		top = top[:k]
	}
	set := map[string]struct{}{}
	for _, n := range top {
		set[strings.ToLower(n)] = struct{}{}
	}
	hits := 0
	for _, g := range gold {
		if _, ok := set[strings.ToLower(g)]; ok {
			hits++
		}
	}
	return float64(hits) / float64(len(gold))
}

func Reduction(naiveTokens, selectedTokens int) float64 {
	if naiveTokens <= 0 {
		return 0
	}
	return float64(naiveTokens-selectedTokens) / float64(naiveTokens)
}

func Percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func Mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func Median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	return Percentile(cp, 50)
}

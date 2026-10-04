package core

import (
	"sort"
	"strings"
	"sync"
)

var (
	stableMemoMu   sync.Mutex
	lastStableKey  string
	lastStableMD   string
	stableMemoHits int
)

// StableMemoKey is the process-local G-1 cache key for markdown_stable reuse:
// version + NUL + sorted selected_names (order-independent set).
// When the key is unchanged across packs, PackContext reuses prior stable bytes
// and only rebuilds markdown_dynamic (Prompt-Cache Aligned).
func StableMemoKey(selectedNames []string, version string) string {
	names := append([]string(nil), selectedNames...)
	sort.Strings(names)
	return version + "\x00" + strings.Join(names, "\x00")
}

func lookupStableMemo(key string) (string, bool) {
	stableMemoMu.Lock()
	defer stableMemoMu.Unlock()
	if key != "" && key == lastStableKey && lastStableMD != "" {
		stableMemoHits++
		return lastStableMD, true
	}
	return "", false
}

func storeStableMemo(key, stable string) {
	if key == "" || stable == "" {
		return
	}
	stableMemoMu.Lock()
	lastStableKey = key
	lastStableMD = stable
	stableMemoMu.Unlock()
}

// ResetMarkdownStableCache clears the process-local G-1 markdown_stable memo (tests).
func ResetMarkdownStableCache() {
	stableMemoMu.Lock()
	lastStableKey = ""
	lastStableMD = ""
	stableMemoHits = 0
	stableMemoMu.Unlock()
}

// MarkdownStableCacheHits returns how many packs reused cached markdown_stable bytes.
func MarkdownStableCacheHits() int {
	stableMemoMu.Lock()
	defer stableMemoMu.Unlock()
	return stableMemoHits
}

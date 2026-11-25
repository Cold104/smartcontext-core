package core

import (
	"math"
	"runtime"
	"sort"
	"strings"
	"unicode"
)

// Pure function words carry no selection signal (aligned with Python catalog).
var stopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "the": {}, "to": {}, "of": {}, "for": {},
	"in": {}, "on": {}, "it": {}, "is": {}, "be": {}, "by": {}, "with": {},
	"as": {}, "at": {}, "or": {}, "from": {}, "into": {}, "than": {}, "then": {},
	"this": {}, "that": {}, "these": {}, "those": {}, "i": {}, "we": {},
	"you": {}, "my": {}, "our": {},
	// v1.3.x: common English function words / conversational filler.
	"are": {}, "was": {}, "were": {}, "been": {}, "being": {}, "am": {},
	"do": {}, "does": {}, "did": {}, "can": {}, "could": {}, "should": {},
	"would": {}, "will": {}, "shall": {}, "may": {}, "might": {}, "must": {},
	"have": {}, "has": {}, "had": {}, "not": {}, "but": {}, "if": {}, "so": {},
	"its": {}, "their": {}, "there": {}, "here": {}, "what": {}, "which": {},
	"who": {}, "when": {}, "where": {}, "how": {}, "why": {}, "me": {}, "us": {},
	"your": {}, "they": {}, "them": {}, "he": {}, "she": {}, "his": {}, "her": {},
	"also": {}, "just": {}, "very": {}, "some": {}, "about": {}, "please": {},
	"want": {}, "need": {}, "let": {}, "lets": {}, "now": {}, "again": {},
	"etc": {}, "eg": {}, "ie": {},
}

// genericTaskWords are common dev verbs/adjectives that appear in most task
// phrasings ("add a new X", "update/fix Y"). They are not dropped (a skill
// named "fix-flaky-tests" should still match "fix") but carry only
// genericWordFactor of their IDF weight.
var genericTaskWords = map[string]struct{}{
	"add": {}, "make": {}, "use": {}, "using": {}, "new": {}, "update": {},
	"fix": {}, "create": {}, "change": {}, "get": {}, "run": {}, "implement": {},
	"write": {}, "work": {}, "thing": {}, "stuff": {},
}

// Local trade-secret weights stay simple: name > description, no remote model.
const (
	nameBoost         = 2.0
	aliasMatchBoost   = 3.0
	triggerMatchBoost = 1.5
	osMatchBoost      = 1.5
	osMismatchFactor  = 0.35
	// pathOnlyFactor scales a match found only in the file path (directory
	// names such as the user, repo or ".cursor/skills"), not in name /
	// description / tags.
	pathOnlyFactor = 0.25
	// genericWordFactor scales genericTaskWords matches.
	genericWordFactor = 0.3
	// highDFRatio: query tokens present in more than this fraction of the
	// catalog carry no selection signal and get weight 0. Applied only when
	// the catalog has at least highDFMinCatalog entries (tiny catalogs keep
	// plain IDF so a 2-of-3 match still discriminates).
	highDFRatio      = 0.5
	highDFMinCatalog = 8
)

func normalizeToken(token string) string {
	if len(token) > 3 && strings.HasSuffix(token, "s") && !strings.HasSuffix(token, "ss") {
		return token[:len(token)-1]
	}
	return token
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tok := b.String()
		b.Reset()
		if len(tok) < 2 {
			return
		}
		if _, stop := stopwords[tok]; stop {
			return
		}
		tokens = append(tokens, normalizeToken(tok))
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func tokenSet(tokens []string) map[string]struct{} {
	out := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		out[t] = struct{}{}
	}
	return out
}

func scoreEntryIDF(queryTokens map[string]struct{}, weights map[string]float64, e Entry) float64 {
	if len(queryTokens) == 0 {
		return 0
	}
	nameToks := tokenSet(tokenize(e.Name))
	semToks := tokenSet(tokenize(semanticText(e)))
	pathToks := tokenSet(tokenize(e.Path))
	var score float64
	for tok := range queryTokens {
		w, ok := weights[tok]
		if !ok || w <= 0 {
			continue
		}
		if _, inName := nameToks[tok]; inName {
			score += w * nameBoost
			continue
		}
		if _, inSem := semToks[tok]; inSem {
			score += w
			continue
		}
		if _, inPath := pathToks[tok]; inPath {
			score += w * pathOnlyFactor
		}
	}
	return score
}

// semanticText is the selection text a skill author controls: name,
// description and tags (path tokens are scored separately and down-weighted).
func semanticText(e Entry) string {
	return e.Name + " " + e.Description + " " + strings.Join(e.Tags, " ")
}

func aliasBoostForTask(task string, e Entry) float64 {
	if task == "" || len(e.Aliases) == 0 {
		return 0
	}
	lower := strings.ToLower(task)
	for _, a := range e.Aliases {
		a = strings.TrimSpace(strings.ToLower(a))
		if a == "" {
			continue
		}
		if strings.Contains(lower, a) {
			return aliasMatchBoost
		}
	}
	return 0
}

func triggerBoostForTask(task string, e Entry) float64 {
	if task == "" || len(e.Triggers) == 0 {
		return 0
	}
	lower := strings.ToLower(task)
	var boost float64
	for _, tr := range e.Triggers {
		tr = strings.TrimSpace(strings.ToLower(tr))
		if tr == "" {
			continue
		}
		if strings.Contains(lower, tr) {
			boost += triggerMatchBoost
		}
	}
	return boost
}

// HostOSCanonical maps runtime.GOOS into the skill frontmatter OS vocabulary.
func HostOSCanonical() string {
	return NormalizeOS(runtime.GOOS)
}

func applyOSPreference(score float64, entryOS, hostOS string) float64 {
	entryOS = NormalizeOS(entryOS)
	hostOS = NormalizeOS(hostOS)
	if entryOS == "" || entryOS == "any" {
		return score
	}
	if hostOS == "" || hostOS == "any" {
		return score
	}
	if entryOS == hostOS {
		if score <= 0 {
			return osMatchBoost * 0.5
		}
		return score + osMatchBoost
	}
	return score * osMismatchFactor
}

func compareScored(i, j scoredEntry) bool {
	if i.score != j.score {
		return i.score > j.score
	}
	ni := strings.ToLower(i.entry.Name)
	nj := strings.ToLower(j.entry.Name)
	if ni != nj {
		return ni < nj
	}
	return strings.ToLower(i.entry.Path) < strings.ToLower(j.entry.Path)
}

func scoreAll(entries []Entry, task string, hostOS string) []scoredEntry {
	qTokens := tokenSet(tokenize(task))
	docFreq := make(map[string]int, len(qTokens))
	for _, e := range entries {
		hay := tokenSet(tokenize(semanticText(e) + " " + e.Path))
		for tok := range qTokens {
			if _, ok := hay[tok]; ok {
				docFreq[tok]++
			}
		}
	}
	total := float64(len(entries))
	if total < 1 {
		total = 1
	}
	weights := make(map[string]float64, len(qTokens))
	for tok := range qTokens {
		df := float64(docFreq[tok])
		if len(entries) >= highDFMinCatalog && df/total > highDFRatio {
			weights[tok] = 0
			continue
		}
		w := math.Log(1 + total/(1+df))
		if _, generic := genericTaskWords[tok]; generic {
			w *= genericWordFactor
		}
		weights[tok] = w
	}

	scored := make([]scoredEntry, len(entries))
	for i, e := range entries {
		sc := scoreEntryIDF(qTokens, weights, e)
		sc += aliasBoostForTask(task, e)
		sc += triggerBoostForTask(task, e)
		sc = applyOSPreference(sc, e.OS, hostOS)
		scored[i] = scoredEntry{entry: e, score: sc}
	}
	return scored
}

func partitionSkillsRules(entries []Entry) (skills, rules []Entry) {
	for _, e := range entries {
		switch strings.ToLower(strings.TrimSpace(e.Kind)) {
		case "rule":
			rules = append(rules, e)
		default:
			skills = append(skills, e)
		}
	}
	return skills, rules
}

func ruleAppliesTo(rule Entry, selected []Entry) bool {
	if len(rule.AppliesTo) == 0 {
		// No applies_to → global guardrail; attach when any skill selected.
		return len(selected) > 0
	}
	for _, a := range rule.AppliesTo {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		lower := strings.ToLower(a)
		if lower == "*" || lower == "all" {
			return true
		}
		if strings.HasPrefix(lower, "tag:") {
			tag := strings.TrimSpace(a[len("tag:"):])
			for _, s := range selected {
				for _, t := range s.Tags {
					if strings.EqualFold(t, tag) {
						return true
					}
				}
			}
			continue
		}
		for _, s := range selected {
			if strings.EqualFold(s.Name, a) {
				return true
			}
		}
	}
	return false
}

// rankAndSelect ranks skills for the workflow (IDF + alias/trigger/OS), then
// attaches rules whose applies_to matches selected skills/tags (or global).
// Deterministic tie-break: score ↓, name ↑, path ↑.
// When every skill score is 0, keeps discovery order (demo-friendly zero-hit packs).
func rankAndSelect(entries []Entry, task string, topN int) []scoredEntry {
	if topN <= 0 || len(entries) == 0 {
		return nil
	}
	hostOS := HostOSCanonical()
	skills, rules := partitionSkillsRules(entries)

	var rankedSkills []scoredEntry
	if len(skills) > 0 {
		scored := scoreAll(skills, task, hostOS)
		anyHit := false
		for _, s := range scored {
			if s.score > 0 {
				anyHit = true
				break
			}
		}
		if anyHit {
			sort.SliceStable(scored, func(i, j int) bool {
				return compareScored(scored[i], scored[j])
			})
		}
		if len(scored) > topN {
			scored = scored[:topN]
		}
		rankedSkills = scored
	}

	selectedEntries := make([]Entry, 0, len(rankedSkills))
	for _, s := range rankedSkills {
		selectedEntries = append(selectedEntries, s.entry)
	}

	out := append([]scoredEntry(nil), rankedSkills...)
	if len(rules) > 0 && len(selectedEntries) > 0 {
		var matched []scoredEntry
		for _, r := range rules {
			if !ruleAppliesTo(r, selectedEntries) {
				continue
			}
			sc := scoreAll([]Entry{r}, task, hostOS)[0].score
			// Keep attached rules discoverable but below primary skills unless strong hit.
			if sc < 0.01 {
				sc = 0.01
			}
			matched = append(matched, scoredEntry{entry: r, score: sc})
		}
		sort.SliceStable(matched, func(i, j int) bool {
			return compareScored(matched[i], matched[j])
		})
		out = append(out, matched...)
	}
	return out
}

// CollectRelatedNames returns deduped related skill names from selected items
// (excluding names already selected) for agent-side failure recovery.
func CollectRelatedNames(items []PackItem) []string {
	selected := make(map[string]struct{}, len(items))
	for _, it := range items {
		selected[strings.ToLower(it.Name)] = struct{}{}
	}
	var out []string
	seen := make(map[string]struct{})
	for _, it := range items {
		if strings.ToLower(it.Kind) == "rule" {
			continue
		}
		for _, r := range it.Related {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			key := strings.ToLower(r)
			if _, ok := selected[key]; ok {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, r)
		}
	}
	sort.Strings(out)
	return out
}

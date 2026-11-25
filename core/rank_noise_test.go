package core

import (
	"fmt"
	"testing"
)

func rankIndex(ranked []scoredEntry, name string) int {
	for i, s := range ranked {
		if s.entry.Name == name {
			return i
		}
	}
	return -1
}

// Regression: a skill whose only overlap with the task is directory names in
// its path outranked a skill whose description/tags name the task topic.
func TestRankPathOnlyMatchBelowSemanticMatch(t *testing.T) {
	entries := []Entry{
		{Name: "path-noise", Kind: "skill", Description: "cooking recipes",
			Path: "/work/terraform/module/.cursor/skills/path-noise/SKILL.md", OS: "any"},
		{Name: "infra-layout", Kind: "skill", Description: "terraform repository layout",
			Tags: []string{"module"}, Path: "/home/u/.cursor/skills/infra-layout/SKILL.md", OS: "any"},
		{Name: "unrelated", Kind: "skill", Description: "sourdough bread",
			Path: "/home/u/.cursor/skills/unrelated/SKILL.md", OS: "any"},
	}
	ranked := rankAndSelect(entries, "terraform module", 3)
	if pi, si := rankIndex(ranked, "path-noise"), rankIndex(ranked, "infra-layout"); si < 0 || pi < si {
		t.Fatalf("semantic (description/tags) match must outrank path-only match: %#v", ranked)
	}
}

// Regression: generic dev verbs ("add", "new", "update", "fix") dominated IDF
// when a skill's description happened to use them.
func TestRankGenericWordsBelowDomainMatch(t *testing.T) {
	entries := []Entry{
		{Name: "generic-noise", Kind: "skill", Description: "add new things, update and fix stuff",
			Path: "/s/generic-noise/SKILL.md", OS: "any"},
		{Name: "terraform-guide", Kind: "skill", Description: "infrastructure as code guide",
			Path: "/s/terraform-guide/SKILL.md", OS: "any"},
		{Name: "bread", Kind: "skill", Description: "sourdough", Path: "/s/bread/SKILL.md", OS: "any"},
	}
	ranked := rankAndSelect(entries, "please add a new terraform resource, update and fix the plan", 3)
	if ranked[0].entry.Name != "terraform-guide" {
		t.Fatalf("domain match must outrank generic-word match: %#v", ranked)
	}
}

// Regression: tokens present in most of the catalog carried weight and let
// many near-duplicates outrank the one skill naming the rare domain term.
func TestRankHighDocFreqTokensIgnored(t *testing.T) {
	var entries []Entry
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("deploy-workflow-%02d", i)
		entries = append(entries, Entry{Name: name, Kind: "skill", Description: "deploy workflow",
			Path: "/s/" + name + "/SKILL.md", OS: "any"})
	}
	entries = append(entries, Entry{Name: "k8s-ops", Kind: "skill", Description: "kubernetes operations",
		Path: "/s/k8s-ops/SKILL.md", OS: "any"})
	ranked := rankAndSelect(entries, "deploy workflow kubernetes", 3)
	if ranked[0].entry.Name != "k8s-ops" {
		t.Fatalf("rare-term match must outrank tokens present in >50%% of catalog: %#v", ranked[:3])
	}
}

func TestTokenizeDropsFunctionWordsKeepsDomain(t *testing.T) {
	got := tokenSet(tokenize("Should we please update the docker build cache?"))
	for _, stop := range []string{"should", "we", "please", "the"} {
		if _, ok := got[stop]; ok {
			t.Fatalf("stopword %q kept: %v", stop, got)
		}
	}
	for _, keep := range []string{"update", "docker", "build", "cache"} {
		if _, ok := got[keep]; !ok {
			t.Fatalf("token %q dropped: %v", keep, got)
		}
	}
}

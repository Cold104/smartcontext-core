package core

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var agentSkillParents = map[string]struct{}{
	".cursor": {},
	".agents": {},
	".claude": {},
	".codex":  {},
}

var denyPathSegments = map[string]struct{}{
	".git":             {},
	".hg":              {},
	".svn":             {},
	".venv":            {},
	"venv":             {},
	"node_modules":     {},
	"__pycache__":      {},
	".hermes-work":     {},
	".smoke-lifecycle": {},
	"site-packages":    {},
	"eval-repos":       {},
	"dist":             {},
	"build":            {},
}

func pathDenied(path string) bool {
	for _, part := range strings.Split(path, string(os.PathSeparator)) {
		if _, ok := denyPathSegments[part]; ok {
			return true
		}
		if strings.HasPrefix(part, ".smoke") {
			return true
		}
	}
	// Also check forward slashes (normalized / mixed paths on Windows).
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if _, ok := denyPathSegments[part]; ok {
			return true
		}
		if strings.HasPrefix(part, ".smoke") {
			return true
		}
	}
	return false
}

func isAgentSkillPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := 0; i < len(parts)-1; i++ {
		if _, ok := agentSkillParents[parts[i]]; ok && i+1 < len(parts) && parts[i+1] == "skills" {
			return true
		}
	}
	return false
}

func isAgentRulePath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := 0; i < len(parts)-1; i++ {
		if _, ok := agentSkillParents[parts[i]]; ok && i+1 < len(parts) && parts[i+1] == "rules" {
			return true
		}
	}
	return false
}

func isCatalogDoc(name string) bool {
	return name == "SKILL.md" || name == "RULE.md"
}

func defaultKindForFile(filename string) string {
	if filename == "RULE.md" {
		return "rule"
	}
	return "skill"
}

func allowShallowCatalogPath(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return len(strings.Split(filepath.ToSlash(rel), "/")) <= 3
}

// DiscoverSkills walks roots for SKILL.md and RULE.md files and returns catalog entries.
// Thin POC port of Python discover_skills, extended for the locked skill/rule schema.
//
// Dedupe key is CatalogKey(kind, name). Tie-break: first wins in discovery order —
// roots in ResolveRoots order; within a root, absolute path ascending (sorted).
func DiscoverSkills(roots []string) []Entry {
	var entries []Entry
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		if pathDenied(root) {
			continue
		}
		rootIsAgentSkills := filepath.Base(root) == "skills"
		if rootIsAgentSkills {
			_, rootIsAgentSkills = agentSkillParents[filepath.Base(filepath.Dir(root))]
		}
		rootIsAgentRules := filepath.Base(root) == "rules"
		if rootIsAgentRules {
			_, rootIsAgentRules = agentSkillParents[filepath.Base(filepath.Dir(root))]
		}
		rootIsBaseline := filepath.Base(root) == "baseline" ||
			(filepath.Base(root) == "skills" && filepath.Base(filepath.Dir(root)) == "baseline") ||
			(filepath.Base(root) == "rules" && filepath.Base(filepath.Dir(root)) == "baseline")

		var catalogFiles []string
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if path != root && shouldSkipNestedDir(name) {
					return filepath.SkipDir
				}
				return nil
			}
			if !isCatalogDoc(d.Name()) {
				return nil
			}
			if pathDenied(path) {
				return nil
			}
			inAgentTree := rootIsAgentSkills || rootIsAgentRules || isAgentSkillPath(path) || isAgentRulePath(path) || rootIsBaseline
			if !inAgentTree && !allowShallowCatalogPath(root, path) {
				return nil
			}
			catalogFiles = append(catalogFiles, path)
			return nil
		})
		sort.Strings(catalogFiles)

		for _, catalogFile := range catalogFiles {
			entries = append(entries, entryFromFile(root, catalogFile))
		}
	}
	return dedupeEntries(entries)
}

func entryFromFile(root, path string) Entry {
	filename := filepath.Base(path)
	name := filepath.Base(filepath.Dir(path))
	kind := defaultKindForFile(filename)
	e := Entry{
		Kind: kind,
		Name: name,
		Path: path,
		Root: root,
		OS:   "any",
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return e
	}
	fm := ParseSkillFrontmatter(string(data))
	if fm.Name != "" {
		e.Name = fm.Name
	}
	e.Description = fm.Description
	e.Tags = fm.Tags
	e.Triggers = fm.Triggers
	e.Related = fm.Related
	e.Aliases = fm.Aliases
	e.AppliesTo = fm.AppliesTo
	e.GovernedBy = fm.GovernedBy
	if fm.OS != "" {
		e.OS = fm.OS
	}
	if fm.Kind == "skill" || fm.Kind == "rule" {
		e.Kind = fm.Kind
	}
	return e
}

func dedupeEntries(entries []Entry) []Entry {
	seen := make(map[string]struct{})
	var out []Entry
	for _, e := range entries {
		key := CatalogKey(e.Kind, e.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	return out
}

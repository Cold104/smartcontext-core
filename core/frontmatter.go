package core

import (
	"strings"
)

// Frontmatter is the locked SKILL.md / RULE.md schema (scalars + list fields).
type Frontmatter struct {
	Name        string
	Description string
	Tags        []string
	Triggers    []string
	OS          string // windows|linux|macos|any
	Kind        string // skill|rule
	Related     []string
	Aliases     []string
	AppliesTo   []string // rules: which skills/tags this limits
	GovernedBy  []string // skills: which rules govern this workflow
	Raw         map[string]string
}

// ParseFrontmatter extracts simple YAML frontmatter key: value pairs from a
// SKILL.md / RULE.md body. List values are flattened to a single string
// (inline [a, b] or block "- a" lines joined). Prefer ParseSkillFrontmatter
// for typed lists.
//
// Thin POC port of Python smartcontext.core.skill_index.parse_frontmatter.
func ParseFrontmatter(text string) map[string]string {
	fields := make(map[string]string)
	body, ok := frontmatterBody(text)
	if !ok {
		return fields
	}
	var key string
	for _, line := range strings.Split(body, "\n") {
		if line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(line, ":") {
			k, v, _ := strings.Cut(line, ":")
			key = strings.TrimSpace(k)
			if key == "" {
				continue
			}
			v = strings.TrimSpace(v)
			v = strings.TrimLeft(v, ">|-")
			v = strings.TrimSpace(v)
			fields[key] = v
		} else if key != "" && strings.TrimSpace(line) != "" {
			fields[key] = strings.TrimSpace(fields[key] + " " + strings.TrimSpace(line))
		}
	}
	return fields
}

// ParseSkillFrontmatter parses locked schema fields (including YAML lists).
func ParseSkillFrontmatter(text string) Frontmatter {
	raw := ParseFrontmatter(text)
	fm := Frontmatter{Raw: raw}
	fm.Name = strings.TrimSpace(raw["name"])
	fm.Description = strings.TrimSpace(raw["description"])
	fm.Tags = ParseStringList(raw["tags"])
	fm.Triggers = ParseStringList(raw["triggers"])
	fm.OS = NormalizeOS(raw["os"])
	fm.Kind = NormalizeKind(firstNonEmpty(raw["kind"], raw["type"]))
	fm.Related = ParseStringList(raw["related"])
	fm.Aliases = ParseStringList(raw["aliases"])
	fm.AppliesTo = ParseStringList(raw["applies_to"])
	fm.GovernedBy = ParseStringList(raw["governed_by"])
	return fm
}

func frontmatterBody(text string) (string, bool) {
	if !strings.HasPrefix(text, "---") {
		return "", false
	}
	rest := text[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return rest, true
	}
	return rest[:end], true
}

// ParseStringList parses a frontmatter list value:
//   - inline YAML: [a, b, "c d"]
//   - block flattened by ParseFrontmatter: "- a - b" or "- a"
//   - comma-separated: a, b
//   - whitespace-separated kebab tokens when no commas/brackets
func ParseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		inner := strings.TrimSpace(raw[1 : len(raw)-1])
		if inner == "" {
			return nil
		}
		return splitCSVRespectingQuotes(inner)
	}
	if strings.Contains(raw, " - ") || strings.HasPrefix(raw, "- ") {
		parts := strings.Split(raw, " - ")
		var out []string
		for i, p := range parts {
			p = strings.TrimSpace(p)
			if i == 0 {
				p = strings.TrimPrefix(p, "- ")
				p = strings.TrimSpace(p)
			}
			if p == "" {
				continue
			}
			out = append(out, unquote(p))
		}
		return uniqueNonEmpty(out)
	}
	if strings.Contains(raw, ",") {
		return splitCSVRespectingQuotes(raw)
	}
	// Single token or space-separated simple ids.
	fields := strings.Fields(raw)
	return uniqueNonEmpty(fields)
}

func splitCSVRespectingQuotes(s string) []string {
	var out []string
	var b strings.Builder
	inQuote := false
	quote := rune(0)
	flush := func() {
		v := strings.TrimSpace(b.String())
		b.Reset()
		if v == "" {
			return
		}
		out = append(out, unquote(v))
	}
	for _, r := range s {
		switch {
		case (r == '"' || r == '\'') && !inQuote:
			inQuote = true
			quote = r
			b.WriteRune(r)
		case inQuote && r == quote:
			inQuote = false
			quote = 0
			b.WriteRune(r)
		case r == ',' && !inQuote:
			flush()
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return uniqueNonEmpty(out)
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	return s
}

func uniqueNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

// NormalizeKind maps kind/type to skill|rule. Empty stays empty (caller defaults).
func NormalizeKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "skill", "skills", "workflow":
		return "skill"
	case "rule", "rules", "limitation", "guardrail":
		return "rule"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

// NormalizeOS maps os to windows|linux|macos|any. Empty → any.
func NormalizeOS(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "any", "*":
		return "any"
	case "windows", "win", "win32", "windows_nt":
		return "windows"
	case "linux":
		return "linux"
	case "macos", "darwin", "osx", "mac":
		return "macos"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

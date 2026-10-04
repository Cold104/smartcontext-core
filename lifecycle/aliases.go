package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// AliasPrefs is user skill-alias prefs under prefs/skill_aliases.json.
type AliasPrefs struct {
	Version int                 `json:"version"`
	Aliases map[string][]string `json:"aliases"`
}

// DefaultAliases are a minimal packaged expansion set for triage/pack demos.
var DefaultAliases = map[string][]string{
	"tf":         {"terraform"},
	"terraform":  {"tf", "iac", "hcl"},
	"k8s":        {"kubernetes"},
	"kubernetes": {"k8s"},
	"go":         {"golang"},
	"golang":     {"go"},
}

// EmptyAliasPrefs returns empty user prefs.
func EmptyAliasPrefs() AliasPrefs {
	return AliasPrefs{Version: 1, Aliases: map[string][]string{}}
}

// LoadAliasPrefs loads user alias prefs; missing/corrupt -> empty.
func LoadAliasPrefs(home string) AliasPrefs {
	path := aliasPrefsPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		return EmptyAliasPrefs()
	}
	var prefs AliasPrefs
	if err := json.Unmarshal(data, &prefs); err != nil {
		return EmptyAliasPrefs()
	}
	if prefs.Aliases == nil {
		prefs.Aliases = map[string][]string{}
	}
	if prefs.Version == 0 {
		prefs.Version = 1
	}
	return prefs
}

// SaveAliasPrefs persists user alias prefs.
func SaveAliasPrefs(prefs AliasPrefs, home string) error {
	if prefs.Aliases == nil {
		prefs.Aliases = map[string][]string{}
	}
	if prefs.Version == 0 {
		prefs.Version = 1
	}
	path := aliasPrefsPath(home)
	payload, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

// LoadAliasMap merges defaults with user overrides (user wins).
func LoadAliasMap(home string) map[string][]string {
	out := map[string][]string{}
	for k, v := range DefaultAliases {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	user := LoadAliasPrefs(home)
	for k, v := range user.Aliases {
		token := strings.ToLower(strings.TrimSpace(k))
		if token == "" {
			continue
		}
		cp := make([]string, 0, len(v))
		for _, e := range v {
			e = strings.ToLower(strings.TrimSpace(e))
			if e != "" && e != token {
				cp = append(cp, e)
			}
		}
		if len(cp) > 0 {
			out[token] = cp
		}
	}
	return out
}

// ExpandTaskTokens appends alias expansions for tokens found in task (deduped).
func ExpandTaskTokens(task string, aliases map[string][]string) string {
	if len(aliases) == 0 {
		return task
	}
	lower := strings.ToLower(task)
	seen := map[string]bool{}
	var extra []string
	for token, expansions := range aliases {
		if !strings.Contains(lower, token) {
			continue
		}
		for _, e := range expansions {
			if seen[e] || strings.Contains(lower, e) {
				continue
			}
			seen[e] = true
			extra = append(extra, e)
		}
	}
	if len(extra) == 0 {
		return task
	}
	return task + " " + strings.Join(extra, " ")
}

// SetAlias sets a user alias override.
func SetAlias(token string, expansions []string, home string) (map[string][]string, error) {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return nil, fmt.Errorf("alias token must be non-empty")
	}
	norm := make([]string, 0, len(expansions))
	for _, e := range expansions {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" && e != token {
			norm = append(norm, e)
		}
	}
	if len(norm) == 0 {
		return nil, fmt.Errorf("alias expansions must include at least one other token")
	}
	prefs := LoadAliasPrefs(home)
	prefs.Aliases[token] = norm
	if err := SaveAliasPrefs(prefs, home); err != nil {
		return nil, err
	}
	return map[string][]string{token: norm}, nil
}

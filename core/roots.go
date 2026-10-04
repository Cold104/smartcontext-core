package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Agent skill directory markers (Cursor Agent Skills + Claude/Codex compat).
// Ported from Python smartcontext.core.catalog._AGENT_SKILL_DIR_MARKERS.
var agentSkillDirMarkers = []string{".cursor", ".agents", ".claude", ".codex"}

// Workspace markers used when walking up from cwd to find project/host roots.
var workspaceUpMarkers = []string{".git", ".cursor", ".agents"}

// Max relative depth for nested **/{marker}/skills discovery under a workspace root.
const maxNestedSkillDepth = 6

var nestedSkillSkipDirNames = map[string]struct{}{
	".git":             {},
	".hg":              {},
	".svn":             {},
	".venv":            {},
	"venv":             {},
	"node_modules":     {},
	"__pycache__":      {},
	".tox":             {},
	".mypy_cache":      {},
	".pytest_cache":    {},
	".ruff_cache":      {},
	".hermes-work":     {},
	".smoke-lifecycle": {},
	"site-packages":    {},
	"dist":             {},
	"build":            {},
	"eval-repos":       {},
	".cache":           {},
	".npm":             {},
	".yarn":            {},
	".pnpm-store":      {},
	"target":           {},
	"vendor":           {},
}

func agentSkillDir(parent, marker string) string {
	return filepath.Join(parent, marker, "skills")
}

func shouldSkipNestedDir(name string) bool {
	if _, ok := nestedSkillSkipDirNames[name]; ok {
		return true
	}
	return strings.HasPrefix(name, ".smoke")
}

func parseRootList(env string) []string {
	var roots []string
	for _, part := range strings.Split(env, ";") {
		part = strings.TrimSpace(part)
		if part != "" {
			roots = append(roots, part)
		}
	}
	return roots
}

func hasDirMarker(dir, marker string) bool {
	st, err := os.Stat(filepath.Join(dir, marker))
	return err == nil && st.IsDir()
}

// walkUpWorkspaceRoots walks from start toward filesystem root, collecting
// directories that contain .git / .cursor / .agents. Stops after the first
// ancestor with .git (typical project root).
func walkUpWorkspaceRoots(start string) []string {
	cur, err := filepath.Abs(start)
	if err != nil {
		cur = start
	}
	var roots []string
	seen := map[string]struct{}{}
	for i := 0; i < 16; i++ {
		hit := false
		hasGit := hasDirMarker(cur, ".git")
		for _, m := range workspaceUpMarkers {
			if hasDirMarker(cur, m) {
				hit = true
				break
			}
		}
		if hit {
			if _, ok := seen[cur]; !ok {
				seen[cur] = struct{}{}
				roots = append(roots, cur)
			}
			if hasGit {
				break
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return roots
}

func relDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return 0
	}
	return len(strings.Split(rel, "/"))
}

// iterNestedAgentSkillDirs finds **/{.cursor,.agents,.claude,.codex}/skills
// under project, pruning junk trees and capping depth. Thin port of Python
// _iter_nested_agent_skill_dirs with POC depth/deny caps for multi-app hosts.
func iterNestedAgentSkillDirs(project string) []string {
	info, err := os.Stat(project)
	if err != nil || !info.IsDir() {
		return nil
	}
	absProject, err := filepath.Abs(project)
	if err != nil {
		absProject = project
	}
	var found []string
	_ = filepath.WalkDir(absProject, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != absProject {
			if shouldSkipNestedDir(name) {
				return filepath.SkipDir
			}
			if relDepth(absProject, path) > maxNestedSkillDepth {
				return filepath.SkipDir
			}
		}
		for _, marker := range agentSkillDirMarkers {
			if name == marker {
				skills := filepath.Join(path, "skills")
				if st, e := os.Stat(skills); e == nil && st.IsDir() {
					found = append(found, skills)
				}
			}
		}
		return nil
	})
	return found
}

func samePath(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return pathEqual(filepath.Clean(a), filepath.Clean(b))
	}
	return pathEqual(filepath.Clean(aa), filepath.Clean(bb))
}

// pathEqual compares cleaned paths; case-insensitive only on Windows.
func pathEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// pathDedupeKey normalizes a root for uniqueness. Case-fold on Windows only.
func pathDedupeKey(path string) string {
	key := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(key)
	}
	return key
}

func isUserHome(path string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	return samePath(path, home)
}

func expandWorkspaceRoot(ws string) []string {
	info, err := os.Stat(ws)
	if err != nil || !info.IsDir() {
		// Still emit conventional agent skill paths so callers see intent.
		var roots []string
		for _, marker := range agentSkillDirMarkers {
			roots = append(roots, agentSkillDir(ws, marker))
		}
		roots = append(roots, filepath.Join(ws, ".cursor"))
		return roots
	}
	var roots []string
	for _, marker := range agentSkillDirMarkers {
		roots = append(roots, agentSkillDir(ws, marker))
	}
	// Nested monorepo scan — skip user home (direct ~/.cursor/skills is enough).
	if !isUserHome(ws) {
		roots = append(roots, iterNestedAgentSkillDirs(ws)...)
	}
	roots = append(roots, filepath.Join(ws, ".cursor"))
	if fileExists(filepath.Join(ws, "AGENTS.md")) || fileExists(filepath.Join(ws, "RULE.md")) {
		roots = append(roots, ws)
	}
	return roots
}

func appendHomeAndEnvRoots(roots []string) []string {
	home, _ := os.UserHomeDir()
	if home != "" {
		for _, marker := range agentSkillDirMarkers {
			roots = append(roots, agentSkillDir(home, marker))
		}
	}
	if envRoot := strings.TrimSpace(os.Getenv("SMARTCONTEXT_ROOT")); envRoot != "" {
		for _, marker := range agentSkillDirMarkers {
			roots = append(roots, agentSkillDir(envRoot, marker))
		}
		roots = append(roots, filepath.Join(envRoot, ".cursor"))
	}
	if friend := strings.TrimSpace(os.Getenv("SMARTCONTEXT_FRIEND_ROOT")); friend != "" {
		for _, marker := range agentSkillDirMarkers {
			roots = append(roots, agentSkillDir(friend, marker))
		}
		roots = append(roots, filepath.Join(friend, ".cursor"))
	}
	return roots
}

// expandExplicitRoots treats each path as a discover root and as a workspace
// seed (so hosts can pass either .cursor/skills or a project folder).
func expandExplicitRoots(explicit []string) []string {
	var roots []string
	for _, r := range explicit {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		roots = append(roots, r)
		roots = append(roots, expandWorkspaceRoot(r)...)
	}
	return roots
}

// hostIDEWorkspaceEnvKeys are common IDE/host env vars that point at open
// workspace folders. Honored when SMARTCONTEXT_WORKSPACE_ROOTS is unset so
// Cursor and similar apps need less manual mcp.json wiring.
//
// Values may be a single path or a ;-separated list (same contract as
// SMARTCONTEXT_WORKSPACE_ROOTS). Empty / whitespace-only values are ignored.
var hostIDEWorkspaceEnvKeys = []string{
	"CURSOR_WORKSPACE",
	"CURSOR_WORKSPACE_ROOT",
	"WORKSPACE_FOLDER",
	"WORKSPACE_FOLDERS",
	"VSCODE_WORKSPACE_FOLDER",
	"VSCODE_CWD",
	"PROJECT_ROOT",
	"PROJECT_DIR",
	"INIT_CWD", // npm / yarn often set this to the project that launched the process
	"PWD",      // POSIX shells; useful when Getwd differs or is unavailable
}

func appendUniqueWorkspace(dst []string, seen map[string]struct{}, path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return dst
	}
	// Expand ~ only when it is a lone home prefix (filepath does not expand ~).
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err == nil && home != "" {
			rest := strings.TrimPrefix(strings.TrimPrefix(path[1:], `/`), `\`)
			path = filepath.Join(home, rest)
		}
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	key := pathDedupeKey(path)
	if _, ok := seen[key]; ok {
		return dst
	}
	seen[key] = struct{}{}
	return append(dst, path)
}

// collectHostAutoWorkspaceSeeds gathers workspace folders from IDE/host env
// vars, then cwd + walk-up markers. Does not read SMARTCONTEXT_* (caller
// handles those at higher precedence).
func collectHostAutoWorkspaceSeeds() []string {
	seen := map[string]struct{}{}
	var seeds []string

	for _, key := range hostIDEWorkspaceEnvKeys {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		for _, part := range parseRootList(raw) {
			seeds = appendUniqueWorkspace(seeds, seen, part)
		}
	}

	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd = "."
	}
	seeds = appendUniqueWorkspace(seeds, seen, cwd)
	for _, walked := range walkUpWorkspaceRoots(cwd) {
		seeds = appendUniqueWorkspace(seeds, seen, walked)
	}

	// Also walk up from each IDE seed (MCP cwd may be home while CURSOR_WORKSPACE
	// points at a nested package folder).
	n := len(seeds)
	for i := 0; i < n; i++ {
		for _, walked := range walkUpWorkspaceRoots(seeds[i]) {
			seeds = appendUniqueWorkspace(seeds, seen, walked)
		}
	}
	return seeds
}

// Workspace seed source labels for discover_skills / host_info diagnostics.
const (
	SeedSourceExplicit      = "explicit"
	SeedSourceSkillRootsEnv = "skill_roots_env"
	SeedSourceWorkspaceEnv  = "workspace_env"
	SeedSourceIDEEnv        = "ide_env"
	SeedSourceCwdWalk       = "cwd_walk"
	SeedSourceHomeOnly      = "home_only"
)

// RootDiagnostics describes how workspace seeds were chosen and whether cwd is home.
type RootDiagnostics struct {
	CwdIsHome           bool   `json:"cwd_is_home"`
	WorkspaceSeedSource string `json:"workspace_seed_source"`
	Warning             string `json:"warning,omitempty"`
}

func cwdIsUserHome() bool {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		return false
	}
	return isUserHome(cwd)
}

func hostIDEEnvSet() bool {
	for _, key := range hostIDEWorkspaceEnvKeys {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

// isHomeAgentSkillRoot reports conventional per-user agent skill/rule roots
// (~/{.cursor,.agents,.claude,.codex}/skills and ~/.cursor), not every path
// under the home directory (Temp/AppData project checkouts must not count).
func isHomeAgentSkillRoot(path string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	for _, marker := range agentSkillDirMarkers {
		if samePath(path, agentSkillDir(home, marker)) {
			return true
		}
	}
	return samePath(path, filepath.Join(home, ".cursor"))
}

// onlyHomeSkillRoots is true when every existing resolved root is a
// conventional home agent skill dir (or no roots exist on disk).
func onlyHomeSkillRoots(roots []string) bool {
	for _, r := range roots {
		st, err := os.Stat(r)
		if err != nil || !st.IsDir() {
			continue
		}
		if !isHomeAgentSkillRoot(r) {
			return false
		}
	}
	return true
}

// DiagnoseRootResolution reports cwd_is_home / workspace_seed_source / warning
// for the same precedence as ResolveRoots (without changing discovery).
func DiagnoseRootResolution(explicit []string) RootDiagnostics {
	diag := RootDiagnostics{CwdIsHome: cwdIsUserHome()}

	switch {
	case len(explicit) > 0:
		diag.WorkspaceSeedSource = SeedSourceExplicit
	case strings.TrimSpace(os.Getenv("SMARTCONTEXT_SKILL_ROOTS")) != "":
		diag.WorkspaceSeedSource = SeedSourceSkillRootsEnv
	case strings.TrimSpace(os.Getenv("SMARTCONTEXT_WORKSPACE_ROOTS")) != "":
		diag.WorkspaceSeedSource = SeedSourceWorkspaceEnv
	case hostIDEEnvSet():
		diag.WorkspaceSeedSource = SeedSourceIDEEnv
	default:
		cwd, err := os.Getwd()
		if err != nil || cwd == "" {
			cwd = "."
		}
		nonHomeSeed := !isUserHome(cwd)
		if !nonHomeSeed {
			for _, walked := range walkUpWorkspaceRoots(cwd) {
				if !isUserHome(walked) {
					nonHomeSeed = true
					break
				}
			}
		}
		if nonHomeSeed {
			diag.WorkspaceSeedSource = SeedSourceCwdWalk
		} else {
			diag.WorkspaceSeedSource = SeedSourceHomeOnly
		}
	}

	roots := ResolveRoots(explicit)
	if diag.CwdIsHome && onlyHomeSkillRoots(roots) {
		diag.Warning = "process cwd is user home and only home skill dirs were resolved; " +
			"set SMARTCONTEXT_WORKSPACE_ROOTS or CURSOR_WORKSPACE in mcp.json " +
			"(see docs/SKILL_ROOT_DISCOVERY.md)"
	}
	return diag
}

// ResolveRoots returns skill/rule roots for catalog discovery.
//
// Precedence:
//  1. explicit roots (pack_context / discover_skills tool arg) — expanded
//  2. SMARTCONTEXT_SKILL_ROOTS (; -separated) — full override (existing)
//  3. SMARTCONTEXT_WORKSPACE_ROOTS (; -separated) — explicit host/workspace seeds
//  4. host-auto seeds — IDE env (CURSOR_WORKSPACE, WORKSPACE_FOLDER, PWD, …)
//     + cwd + walk-up markers (.git / .cursor / .agents) + capped nested glob
//  5. user-global home skill dirs + SMARTCONTEXT_ROOT / FRIEND_ROOT
//
// MCP client roots/list (go-sdk ServerSession.ListRoots) is deprecated
// (SEP-2577); hosts should prefer env / tool args. See docs/SKILL_ROOT_DISCOVERY.md.
func ResolveRoots(explicit []string) []string {
	if len(explicit) > 0 {
		return uniqueRoots(expandExplicitRoots(explicit))
	}

	if env := strings.TrimSpace(os.Getenv("SMARTCONTEXT_SKILL_ROOTS")); env != "" {
		return parseRootList(env)
	}

	var workspaces []string
	if ws := strings.TrimSpace(os.Getenv("SMARTCONTEXT_WORKSPACE_ROOTS")); ws != "" {
		workspaces = parseRootList(ws)
	} else {
		workspaces = collectHostAutoWorkspaceSeeds()
	}

	var roots []string
	for _, ws := range workspaces {
		roots = append(roots, expandWorkspaceRoot(ws)...)
	}
	roots = appendHomeAndEnvRoots(roots)
	return uniqueRoots(roots)
}

// DefaultRoots returns portable agent skill/rule roots (not the whole project tree).
// Contract port of Python smartcontext.core.catalog.default_roots, extended for
// multi-app hosts (workspace env + walk-up + depth-capped nested discovery).
func DefaultRoots() []string {
	return ResolveRoots(nil)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func uniqueRoots(roots []string) []string {
	seen := make(map[string]struct{})
	var unique []string
	for _, root := range roots {
		key := root
		if abs, err := filepath.Abs(root); err == nil {
			if _, err := os.Stat(abs); err == nil {
				key = abs
			}
		}
		key = pathDedupeKey(key)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, root)
	}
	return unique
}

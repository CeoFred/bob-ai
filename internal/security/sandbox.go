package security

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type sessionContextKeyType struct{}

// SessionContextKey is the context key for attaching session security context.
var SessionContextKey = sessionContextKeyType{}

// SessionContext encapsulates dynamic session parameters for security & path scoping.
type SessionContext struct {
	SessionID   string
	Type        string // "project" or "conversation"
	ProjectPath string // For project sessions: the absolute root folder
	ProjectName string
	ReadOnly    bool   // True if conversation mode (blocks writes)
}

// ContextWithSession attaches a SessionContext to context.Context.
func ContextWithSession(ctx context.Context, sc SessionContext) context.Context {
	return context.WithValue(ctx, SessionContextKey, sc)
}

// SessionFromContext retrieves the SessionContext from context.Context.
func SessionFromContext(ctx context.Context) (SessionContext, bool) {
	if ctx == nil {
		return SessionContext{}, false
	}
	sc, ok := ctx.Value(SessionContextKey).(SessionContext)
	return sc, ok
}

var (
	ErrAccessDenied  = errors.New("access denied: path outside allowed workspace")
	ErrInvalidPath   = errors.New("invalid path")
	ErrSymlinkEscape = errors.New("symlink escapes allowed workspace")
)

// PathValidator verifies that filesystem access stays strictly inside authorized folders
// and provides smart fuzzy project path discovery.
type PathValidator struct {
	allowedRoots []string
}

// NewPathValidator initializes path validator with allowed root paths.
func NewPathValidator(allowedRoots []string) *PathValidator {
	cleanRoots := make([]string, 0, len(allowedRoots))
	for _, root := range allowedRoots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		expanded := expandHome(root)
		if abs, err := filepath.Abs(expanded); err == nil {
			if real, err := filepath.EvalSymlinks(abs); err == nil {
				cleanRoots = append(cleanRoots, real)
			} else {
				cleanRoots = append(cleanRoots, abs)
			}
		}
	}
	return &PathValidator{
		allowedRoots: cleanRoots,
	}
}

// AllowedRoots returns the list of authorized root paths.
func (pv *PathValidator) AllowedRoots() []string {
	return pv.allowedRoots
}

func (pv *PathValidator) effectiveRoots(ctx context.Context) []string {
	if sc, ok := SessionFromContext(ctx); ok {
		if sc.Type == "project" && sc.ProjectPath != "" {
			expanded := expandHome(sc.ProjectPath)
			if abs, err := filepath.Abs(expanded); err == nil {
				if real, err := filepath.EvalSymlinks(abs); err == nil {
					return []string{real}
				}
				return []string{abs}
			}
			return []string{sc.ProjectPath}
		}
		if sc.Type == "conversation" {
			home, _ := os.UserHomeDir()
			if home != "" {
				if real, err := filepath.EvalSymlinks(home); err == nil {
					return []string{real}
				}
				return []string{home}
			}
		}
	}
	return pv.allowedRoots
}

// ValidatePath checks if targetPath is safe and strictly inside one of the allowed roots.
func (pv *PathValidator) ValidatePath(targetPath string) (string, error) {
	return pv.ValidatePathWithContext(context.Background(), targetPath)
}

// ValidatePathWithContext checks if targetPath is safe with respect to the session context (if any).
func (pv *PathValidator) ValidatePathWithContext(ctx context.Context, targetPath string) (string, error) {
	trimmed := strings.TrimSpace(targetPath)
	if trimmed == "" {
		return "", ErrInvalidPath
	}

	roots := pv.effectiveRoots(ctx)

	// In project mode, if path is relative, resolve relative to the project root
	sc, hasSession := SessionFromContext(ctx)
	if hasSession && sc.Type == "project" && sc.ProjectPath != "" {
		if !filepath.IsAbs(trimmed) && !strings.HasPrefix(trimmed, "~") {
			trimmed = filepath.Join(sc.ProjectPath, trimmed)
		}
	}

	// 1. First, attempt standard direct resolution
	expanded := expandHome(trimmed)
	absPath, err := filepath.Abs(expanded)
	if err == nil {
		cleanPath := filepath.Clean(absPath)
		if resolved, ok := isInsideRoots(cleanPath, roots); ok {
			return resolved, nil
		}
	}

	// 2. If direct resolution failed, check if targetPath is a placeholder (e.g. /path/to/xyz) or relative project name
	cleanQuery := stripPlaceholderPrefixes(trimmed)

	// 3. Search across effective workspace roots
	matches := pv.FindMatchingPathsWithContext(ctx, cleanQuery)
	if len(matches) == 1 {
		return matches[0], nil
	} else if len(matches) > 1 {
		// Check if there is an exact basename/relative match among the matches
		lowerClean := strings.ToLower(cleanQuery)
		var exactMatches []string
		for _, m := range matches {
			base := strings.ToLower(filepath.Base(m))
			if base == lowerClean {
				exactMatches = append(exactMatches, m)
			}
		}
		if len(exactMatches) == 1 {
			return exactMatches[0], nil
		}

		return "", fmt.Errorf("multiple matching projects found for %q: %v. Please specify the target directory explicitly", cleanQuery, matches)
	}

	return "", fmt.Errorf("%w: %s (allowed roots: %v)", ErrAccessDenied, targetPath, roots)
}

func (pv *PathValidator) isInsideAllowedRoots(cleanPath string) (string, bool) {
	return isInsideRoots(cleanPath, pv.allowedRoots)
}

func isInsideRoots(cleanPath string, roots []string) (string, bool) {
	// If file or directory exists, evaluate symlinks
	target := cleanPath
	if _, err := os.Stat(cleanPath); err == nil {
		if realPath, err := filepath.EvalSymlinks(cleanPath); err == nil {
			target = realPath
		}
	} else {
		// If creating a new file, evaluate parent
		parent := filepath.Dir(cleanPath)
		if realParent, err := filepath.EvalSymlinks(parent); err == nil {
			target = filepath.Join(realParent, filepath.Base(cleanPath))
		}
	}

	for _, root := range roots {
		rel, err := filepath.Rel(root, target)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			return target, true
		}
	}
	return "", false
}

// FindMatchingPaths searches allowed workspace roots for folders matching query using exact, prefix, substring, token, and fuzzy similarity.
func (pv *PathValidator) FindMatchingPaths(query string) []string {
	return pv.FindMatchingPathsWithContext(context.Background(), query)
}

// FindMatchingPathsWithContext searches effective workspace roots for folders matching query.
func (pv *PathValidator) FindMatchingPathsWithContext(ctx context.Context, query string) []string {
	cleanQuery := strings.TrimSpace(query)
	cleanQuery = stripPlaceholderPrefixes(cleanQuery)
	cleanQuery = cleanQueryWords(cleanQuery)
	if cleanQuery == "" {
		return nil
	}

	lowerQuery := strings.ToLower(cleanQuery)
	queryTokens := tokenize(lowerQuery)

	type scoredMatch struct {
		path  string
		score int
	}

	var scored []scoredMatch
	seenPaths := make(map[string]bool)

	roots := pv.effectiveRoots(ctx)
	for _, root := range roots {
		// A. Check direct join (e.g. root/jeroidpay or root/jeroidpay/server)
		direct := filepath.Join(root, cleanQuery)
		if info, err := os.Stat(direct); err == nil && info.IsDir() {
			realPath := direct
			if real, err := filepath.EvalSymlinks(direct); err == nil {
				realPath = real
			}
			if !seenPaths[realPath] {
				seenPaths[realPath] = true
				scored = append(scored, scoredMatch{path: realPath, score: 1000})
			}
		}

		// B. Walk workspace directory tree up to depth 4
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}

			name := d.Name()
			if isIgnoredDir(name) {
				return filepath.SkipDir
			}

			rel, _ := filepath.Rel(root, path)
			if rel == "." {
				return nil
			}

			// Limit search depth to 4
			depth := strings.Count(rel, string(filepath.Separator)) + 1
			if depth > 4 {
				return filepath.SkipDir
			}

			realPath := path
			if real, err := filepath.EvalSymlinks(path); err == nil {
				realPath = real
			}

			if seenPaths[realPath] {
				return nil
			}

			lowerRel := strings.ToLower(rel)
			lowerName := strings.ToLower(name)
			nameTokens := tokenize(lowerName)

			score := calculateMatchScore(lowerQuery, queryTokens, lowerName, lowerRel, nameTokens)

			// Boost score if this directory contains a project indicator file (.git, package.json, go.mod, etc.)
			if score > 0 {
				if isProjectDirectory(realPath) {
					score += 50
				}
				seenPaths[realPath] = true
				scored = append(scored, scoredMatch{path: realPath, score: score})
			}

			return nil
		})
	}

	if len(scored) == 0 {
		return nil
	}

	// Sort matches by score descending
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Extract top matches (up to 10)
	limit := 10
	if len(scored) < limit {
		limit = len(scored)
	}

	matches := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		matches = append(matches, scored[i].path)
	}

	return matches
}

func calculateMatchScore(query string, queryTokens []string, name string, rel string, nameTokens []string) int {
	// 1. Exact match on directory name or relative path
	if name == query || rel == query {
		return 1000
	}
	if strings.HasSuffix(rel, "/"+query) {
		return 950
	}

	// 2. Prefix match with separator (e.g. query "verxa" -> "verxa-backend", "verxa_api")
	if strings.HasPrefix(name, query+"-") || strings.HasPrefix(name, query+"_") || strings.HasPrefix(name, query+".") {
		return 850
	}

	// 3. Suffix match with separator (e.g. query "backend" -> "verxa-backend", query "server" -> "jeroid-server")
	if strings.HasSuffix(name, "-"+query) || strings.HasSuffix(name, "_"+query) {
		return 800
	}

	// 4. Simple prefix match
	if strings.HasPrefix(name, query) {
		return 750
	}

	// 5. Substring match (e.g. query "verxa" in "my-verxa-app")
	if strings.Contains(name, query) {
		return 600
	}
	if strings.Contains(rel, "/"+query) || strings.Contains(rel, query) {
		return 500
	}

	// 6. Token matching: check if all query tokens appear in name tokens
	if len(queryTokens) > 0 {
		allMatch := true
		for _, qt := range queryTokens {
			matched := false
			for _, nt := range nameTokens {
				if nt == qt || strings.HasPrefix(nt, qt) {
					matched = true
					break
				}
			}
			if !matched {
				allMatch = false
				break
			}
		}
		if allMatch {
			return 450
		}
	}

	// 7. Fuzzy / Typo tolerance (distance <= 2 for queries with length >= 4)
	if len(query) >= 4 {
		dist := levenshteinDistance(query, name)
		if dist <= 2 {
			return 300 - (dist * 50)
		}

		// Also check distance against individual name tokens (e.g. "vexra" vs "verxa" in "verxa-backend")
		for _, nt := range nameTokens {
			if len(nt) >= 4 {
				d := levenshteinDistance(query, nt)
				if d <= 2 {
					return 280 - (d * 50)
				}
			}
		}
	}

	return 0
}

func isProjectDirectory(dir string) bool {
	indicators := []string{
		".git", "go.mod", "package.json", "Cargo.toml", "pyproject.toml",
		"requirements.txt", "Makefile", "pom.xml", "build.gradle", "composer.json",
	}
	for _, ind := range indicators {
		if _, err := os.Stat(filepath.Join(dir, ind)); err == nil {
			return true
		}
	}
	return false
}

func isIgnoredDir(name string) bool {
	if strings.HasPrefix(name, ".") && name != "." {
		return true
	}
	ignored := []string{
		"node_modules", "vendor", "dist", "build", "target", "Pods",
		".next", ".turbo", ".venv", "__pycache__", ".cache", ".bob",
		".gemini", "Library", "Applications", "System",
	}
	lower := strings.ToLower(name)
	for _, ig := range ignored {
		if lower == ig {
			return true
		}
	}
	return false
}

func cleanSearchQuery(query string) string {
	res := strings.TrimSpace(query)
	res = stripPlaceholderPrefixes(res)
	res = cleanQueryWords(res)
	return res
}

func cleanQueryWords(query string) string {
	// Strip common extraneous conversational words if user typed "verxa project" or "the jeroidpay repo"
	words := strings.Fields(query)
	if len(words) > 1 {
		fillerWords := map[string]bool{
			"project": true, "repo": true, "repository": true,
			"codebase": true, "app": true, "the": true, "a": true, "an": true,
		}
		var kept []string
		for _, w := range words {
			lower := strings.ToLower(strings.Trim(w, `"'.,`))
			if !fillerWords[lower] && lower != "" {
				kept = append(kept, w)
			}
		}
		if len(kept) > 0 {
			return strings.Join(kept, " ")
		}
	}
	return query
}

func tokenize(s string) []string {
	f := func(c rune) bool {
		return c == '-' || c == '_' || c == '.' || c == ' ' || c == '/' || c == '\\'
	}
	parts := strings.FieldsFunc(s, f)
	var tokens []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			tokens = append(tokens, trimmed)
		}
	}
	return tokens
}

func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)
	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
		dp[i][0] = i
	}
	for j := 0; j <= m; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			dp[i][j] = min3(
				dp[i-1][j]+1,      // deletion
				dp[i][j-1]+1,      // insertion
				dp[i-1][j-1]+cost, // substitution
			)
			// Transposition (Damerau-Levenshtein)
			if i > 1 && j > 1 && r1[i-1] == r2[j-2] && r1[i-2] == r2[j-1] {
				if dp[i-2][j-2]+1 < dp[i][j] {
					dp[i][j] = dp[i-2][j-2] + 1
				}
			}
		}
	}
	return dp[n][m]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func stripPlaceholderPrefixes(path string) string {
	prefixes := []string{
		"/path/to/", "path/to/",
		"/your/project/path/", "your/project/path/",
		"/example/", "example/",
		"/Users/user/", "/home/user/",
	}

	res := path
	for _, p := range prefixes {
		if strings.HasPrefix(strings.ToLower(res), strings.ToLower(p)) {
			res = res[len(p):]
			break
		}
	}
	return strings.Trim(res, "/")
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

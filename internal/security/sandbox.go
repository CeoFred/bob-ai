package security

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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

// ValidatePath checks if targetPath is safe and strictly inside one of the allowed roots.
// If targetPath is a relative folder name, project name, or contains generic placeholders (e.g. /path/to/project),
// it automatically attempts to resolve and locate the matching project inside allowed workspaces.
func (pv *PathValidator) ValidatePath(targetPath string) (string, error) {
	trimmed := strings.TrimSpace(targetPath)
	if trimmed == "" {
		return "", ErrInvalidPath
	}

	// 1. First, attempt standard direct resolution
	expanded := expandHome(trimmed)
	absPath, err := filepath.Abs(expanded)
	if err == nil {
		cleanPath := filepath.Clean(absPath)
		if resolved, ok := pv.isInsideAllowedRoots(cleanPath); ok {
			return resolved, nil
		}
	}

	// 2. If direct resolution failed, check if targetPath is a placeholder (e.g. /path/to/xyz) or relative project name
	cleanQuery := stripPlaceholderPrefixes(trimmed)

	// 3. Search across allowed workspace roots
	matches := pv.FindMatchingPaths(cleanQuery)
	if len(matches) == 1 {
		return matches[0], nil
	} else if len(matches) > 1 {
		return "", fmt.Errorf("multiple matching projects found for %q: %v. Please specify the target directory explicitly", cleanQuery, matches)
	}

	return "", fmt.Errorf("%w: %s (allowed roots: %v)", ErrAccessDenied, targetPath, pv.allowedRoots)
}

func (pv *PathValidator) isInsideAllowedRoots(cleanPath string) (string, bool) {
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

	for _, root := range pv.allowedRoots {
		rel, err := filepath.Rel(root, target)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			return target, true
		}
	}
	return "", false
}

// FindMatchingPaths searches allowed workspace roots for folders matching query (by suffix, relative path, or name).
func (pv *PathValidator) FindMatchingPaths(query string) []string {
	var matches []string
	cleanQuery := strings.TrimSpace(query)
	cleanQuery = strings.TrimPrefix(cleanQuery, "/")
	cleanQuery = strings.TrimSuffix(cleanQuery, "/")
	if cleanQuery == "" {
		return matches
	}

	lowerQuery := strings.ToLower(cleanQuery)

	for _, root := range pv.allowedRoots {
		// A. Check direct join (e.g. root/jeroidpay or root/jeroidpay/server)
		direct := filepath.Join(root, cleanQuery)
		if info, err := os.Stat(direct); err == nil {
			if real, err := filepath.EvalSymlinks(direct); err == nil {
				matches = appendUnique(matches, real)
			} else {
				matches = appendUnique(matches, direct)
			}
			_ = info
		}

		// B. Walk top-level directories up to depth 4
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}

			// Skip hidden directories like .git, .node_modules, etc.
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
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

			lowerRel := strings.ToLower(rel)
			lowerName := strings.ToLower(d.Name())

			// Check matching conditions
			if lowerRel == lowerQuery || strings.HasSuffix(lowerRel, "/"+lowerQuery) || lowerName == lowerQuery {
				realPath := path
				if real, err := filepath.EvalSymlinks(path); err == nil {
					realPath = real
				}
				matches = appendUnique(matches, realPath)
			}

			return nil
		})
	}

	return matches
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

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
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

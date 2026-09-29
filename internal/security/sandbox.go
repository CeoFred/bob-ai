package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrAccessDenied    = errors.New("access denied: path outside allowed workspace")
	ErrInvalidPath     = errors.New("invalid path")
	ErrSymlinkEscape   = errors.New("symlink escapes allowed workspace")
)

// PathValidator verifies that filesystem access stays strictly inside authorized folders.
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

// ValidatePath checks if targetPath is safe and strictly inside one of the allowed roots.
func (pv *PathValidator) ValidatePath(targetPath string) (string, error) {
	if strings.TrimSpace(targetPath) == "" {
		return "", ErrInvalidPath
	}

	expanded := expandHome(targetPath)
	absPath, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}

	cleanPath := filepath.Clean(absPath)

	// If file exists, evaluate symlinks to prevent symlink traversal attacks
	if _, err := os.Stat(cleanPath); err == nil {
		if realPath, err := filepath.EvalSymlinks(cleanPath); err == nil {
			cleanPath = realPath
		}
	} else {
		// If creating a new file, evaluate the parent directory's symlinks
		parent := filepath.Dir(cleanPath)
		if realParent, err := filepath.EvalSymlinks(parent); err == nil {
			cleanPath = filepath.Join(realParent, filepath.Base(cleanPath))
		}
	}

	for _, root := range pv.allowedRoots {
		// Check if cleanPath is root or a subpath of root
		rel, err := filepath.Rel(root, cleanPath)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			return cleanPath, nil
		}
	}

	return "", fmt.Errorf("%w: %s (allowed: %v)", ErrAccessDenied, targetPath, pv.allowedRoots)
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

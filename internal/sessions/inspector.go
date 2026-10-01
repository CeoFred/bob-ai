package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// InspectProject analyzes a project root folder to extract its tech stack and overview.
func InspectProject(projectPath string) (summary string, techStack []string) {
	stackMap := make(map[string]bool)

	// 1. Check for Go
	if _, err := os.Stat(filepath.Join(projectPath, "go.mod")); err == nil {
		stackMap["Go"] = true
	}

	// 2. Check for JavaScript / TypeScript / Node / Frontend frameworks
	pkgPath := filepath.Join(projectPath, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			allDeps := make(map[string]bool)
			for k := range pkg.Dependencies {
				allDeps[strings.ToLower(k)] = true
			}
			for k := range pkg.DevDependencies {
				allDeps[strings.ToLower(k)] = true
			}

			if allDeps["typescript"] {
				stackMap["TypeScript"] = true
			} else {
				stackMap["JavaScript"] = true
			}
			if allDeps["react"] || allDeps["react-dom"] {
				stackMap["React"] = true
			}
			if allDeps["next"] {
				stackMap["Next.js"] = true
			}
			if allDeps["vue"] {
				stackMap["Vue"] = true
			}
			if allDeps["tailwindcss"] {
				stackMap["Tailwind CSS"] = true
			}
			if allDeps["vite"] {
				stackMap["Vite"] = true
			}
			if allDeps["express"] {
				stackMap["Express"] = true
			}
			if allDeps["electron"] {
				stackMap["Electron"] = true
			}
		}
	}

	// 3. Check for Rust
	if _, err := os.Stat(filepath.Join(projectPath, "Cargo.toml")); err == nil {
		stackMap["Rust"] = true
	}

	// 4. Check for Python
	if _, err := os.Stat(filepath.Join(projectPath, "pyproject.toml")); err == nil {
		stackMap["Python"] = true
	} else if _, err := os.Stat(filepath.Join(projectPath, "requirements.txt")); err == nil {
		stackMap["Python"] = true
	}

	// 5. Check for Docker & Infra
	if _, err := os.Stat(filepath.Join(projectPath, "Dockerfile")); err == nil {
		stackMap["Docker"] = true
	}
	if _, err := os.Stat(filepath.Join(projectPath, "docker-compose.yml")); err == nil {
		stackMap["Docker Compose"] = true
	} else if _, err := os.Stat(filepath.Join(projectPath, "compose.yaml")); err == nil {
		stackMap["Docker Compose"] = true
	}
	if _, err := os.Stat(filepath.Join(projectPath, "Makefile")); err == nil {
		stackMap["Makefile"] = true
	}

	// Convert stack to slice
	for s := range stackMap {
		techStack = append(techStack, s)
	}

	// 6. Extract Readme Summary
	readmeFiles := []string{"README.md", "readme.md", "README", "readme.txt"}
	for _, rf := range readmeFiles {
		fullPath := filepath.Join(projectPath, rf)
		if data, err := os.ReadFile(fullPath); err == nil {
			summary = extractReadmeSummary(string(data))
			if summary != "" {
				break
			}
		}
	}

	if summary == "" {
		if len(techStack) > 0 {
			summary = "Project built with " + strings.Join(techStack, ", ")
		} else {
			summary = "Project workspace in " + filepath.Base(projectPath)
		}
	}

	return summary, techStack
}

func extractReadmeSummary(content string) string {
	lines := strings.Split(content, "\n")
	var meaningful []string
	count := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip image/badge lines
		if strings.HasPrefix(trimmed, "[![") || strings.HasPrefix(trimmed, "![") {
			continue
		}
		// Strip leading markdown headers
		trimmed = strings.TrimLeft(trimmed, "# ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			meaningful = append(meaningful, trimmed)
			count++
			if count >= 4 {
				break
			}
		}
	}

	res := strings.Join(meaningful, " — ")
	if len(res) > 300 {
		res = res[:297] + "..."
	}
	return res
}

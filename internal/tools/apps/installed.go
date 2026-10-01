package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"bob/internal/tools/registry"
)

// InstalledAppInfo describes an installed application on macOS.
type InstalledAppInfo struct {
	Name         string `json:"name"`
	BundleID     string `json:"bundle_id,omitempty"`
	Version      string `json:"version,omitempty"`
	Path         string `json:"path"`
	Category     string `json:"category"` // "User", "System", "Local"
	DisplayName  string `json:"display_name,omitempty"`
}

// ListInstalledAppsInput defines parameters for list_installed_apps.
type ListInstalledAppsInput struct {
	Search   string `json:"search,omitempty"`
	Category string `json:"category,omitempty"` // "all", "user", "system"
	Limit    int    `json:"limit,omitempty"`
}

// ListInstalledAppsTool scans and lists installed macOS applications.
type ListInstalledAppsTool struct{}

// NewListInstalledAppsTool creates a new ListInstalledAppsTool.
func NewListInstalledAppsTool() *ListInstalledAppsTool {
	return &ListInstalledAppsTool{}
}

func (t *ListInstalledAppsTool) Name() string {
	return "list_installed_apps"
}

func (t *ListInstalledAppsTool) Description() string {
	return "Lists all applications installed on the Mac across /Applications, /System/Applications, and ~/Applications. Supports filtering by name and category."
}

func (t *ListInstalledAppsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"search": map[string]any{
				"type":        "string",
				"description": "Optional keyword to filter installed applications by name, bundle ID, or path (e.g. 'slack', 'chrome', 'code').",
			},
			"category": map[string]any{
				"type":        "string",
				"description": "Filter by category: 'all' (default), 'user' (user-installed apps in /Applications and ~/Applications), or 'system' (macOS system apps).",
				"enum":        []string{"all", "user", "system"},
				"default":     "all",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of applications to return (default 100).",
				"default":     100,
			},
		},
	}
}

type ListInstalledAppsResult struct {
	Count       int                `json:"count"`
	TotalFound  int                `json:"total_found"`
	Timestamp   string             `json:"timestamp"`
	SearchQuery string             `json:"search_query,omitempty"`
	Category    string             `json:"category"`
	Apps        []InstalledAppInfo `json:"apps"`
}

func (t *ListInstalledAppsTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	search := ""
	category := "all"
	limit := 100

	if len(rawInput) > 0 && string(rawInput) != "{}" {
		var in ListInstalledAppsInput
		if err := json.Unmarshal(rawInput, &in); err == nil {
			search = strings.TrimSpace(in.Search)
			if in.Category != "" {
				category = strings.ToLower(strings.TrimSpace(in.Category))
			}
			if in.Limit > 0 {
				limit = in.Limit
			}
		}
	}

	apps := t.scanInstalledApps(category)

	// Filter by search query
	var filtered []InstalledAppInfo
	searchLower := strings.ToLower(search)
	for _, app := range apps {
		if searchLower != "" {
			nameMatch := strings.Contains(strings.ToLower(app.Name), searchLower)
			dispMatch := strings.Contains(strings.ToLower(app.DisplayName), searchLower)
			bundleMatch := strings.Contains(strings.ToLower(app.BundleID), searchLower)
			pathMatch := strings.Contains(strings.ToLower(app.Path), searchLower)
			if !nameMatch && !dispMatch && !bundleMatch && !pathMatch {
				continue
			}
		}
		filtered = append(filtered, app)
	}

	// Sort alphabetically by Name
	sort.Slice(filtered, func(i, j int) bool {
		return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
	})

	totalFound := len(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	resultData := ListInstalledAppsResult{
		Count:       len(filtered),
		TotalFound:  totalFound,
		Timestamp:   time.Now().Format(time.RFC3339),
		SearchQuery: search,
		Category:    category,
		Apps:        filtered,
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Installed Applications on Mac (%d found", totalFound))
	if search != "" {
		sb.WriteString(fmt.Sprintf(", matching %q", search))
	}
	sb.WriteString("):\n\n")

	for idx, app := range filtered {
		extra := ""
		if app.Version != "" {
			extra += fmt.Sprintf(" (v%s)", app.Version)
		}
		if app.BundleID != "" {
			extra += fmt.Sprintf(" [%s]", app.BundleID)
		}
		sb.WriteString(fmt.Sprintf("%d. %s%s — %s\n", idx+1, app.Name, extra, app.Path))
	}

	return registry.ToolResult{
		Success: true,
		Output:  strings.TrimSpace(sb.String()),
		Data:    resultData,
	}, nil
}

func (t *ListInstalledAppsTool) scanInstalledApps(category string) []InstalledAppInfo {
	homeDir, _ := os.UserHomeDir()
	var targets []struct {
		dir      string
		category string
	}

	if category == "all" || category == "user" {
		targets = append(targets,
			struct {
				dir      string
				category string
			}{"/Applications", "User"},
			struct {
				dir      string
				category string
			}{filepath.Join(homeDir, "Applications"), "Local"},
		)
	}

	if category == "all" || category == "system" {
		targets = append(targets,
			struct {
				dir      string
				category string
			}{"/System/Applications", "System"},
			struct {
				dir      string
				category string
			}{"/System/Applications/Utilities", "System"},
			struct {
				dir      string
				category string
			}{"/System/Library/CoreServices/Applications", "System"},
		)
	}

	var apps []InstalledAppInfo
	seenPaths := make(map[string]bool)

	for _, target := range targets {
		entries, err := os.ReadDir(target.dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".app") {
				appPath := filepath.Join(target.dir, entry.Name())
				if seenPaths[appPath] {
					continue
				}
				seenPaths[appPath] = true

				info := parseAppBundle(appPath, target.category)
				apps = append(apps, info)
			}
		}
	}

	return apps
}

var (
	bundleNameRegex    = regexp.MustCompile(`<key>CFBundleName</key>\s*<string>([^<]+)</string>`)
	displayNameRegex   = regexp.MustCompile(`<key>CFBundleDisplayName</key>\s*<string>([^<]+)</string>`)
	bundleIDRegex      = regexp.MustCompile(`<key>CFBundleIdentifier</key>\s*<string>([^<]+)</string>`)
	bundleVersionRegex = regexp.MustCompile(`<key>CFBundleShortVersionString</key>\s*<string>([^<]+)</string>`)
)

func parseAppBundle(appPath, category string) InstalledAppInfo {
	baseName := strings.TrimSuffix(filepath.Base(appPath), ".app")
	info := InstalledAppInfo{
		Name:     baseName,
		Path:     appPath,
		Category: category,
	}

	plistPath := filepath.Join(appPath, "Contents", "Info.plist")
	data, err := os.ReadFile(plistPath)
	if err == nil {
		strData := string(data)
		if match := displayNameRegex.FindStringSubmatch(strData); len(match) > 1 {
			info.DisplayName = match[1]
			info.Name = match[1]
		} else if match := bundleNameRegex.FindStringSubmatch(strData); len(match) > 1 {
			info.Name = match[1]
		}
		if match := bundleIDRegex.FindStringSubmatch(strData); len(match) > 1 {
			info.BundleID = match[1]
		}
		if match := bundleVersionRegex.FindStringSubmatch(strData); len(match) > 1 {
			info.Version = match[1]
		}
	}

	return info
}

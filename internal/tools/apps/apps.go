package apps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"bob/internal/tools/registry"
)

// AppInfo encapsulates information about a running application or process on the Mac.
type AppInfo struct {
	Name             string  `json:"name"`
	BundleID         string  `json:"bundle_id,omitempty"`
	PID              int     `json:"pid"`
	IsFrontmost      bool    `json:"is_frontmost,omitempty"`
	IsGUIApp         bool    `json:"is_gui_app"`
	CPUPercent       float64 `json:"cpu_percent,omitempty"`
	MemoryPercent    float64 `json:"memory_percent,omitempty"`
	ExecutablePath   string  `json:"executable_path,omitempty"`
}

// ListRunningAppsInput defines the input parameters for list_running_apps.
type ListRunningAppsInput struct {
	GUIOnly       *bool  `json:"gui_only,omitempty"`
	Search        string `json:"search,omitempty"`
	IncludeSystem bool   `json:"include_system,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

// ListRunningAppsTool inspects and reports running applications on macOS.
type ListRunningAppsTool struct{}

// NewListRunningAppsTool creates a new ListRunningAppsTool instance.
func NewListRunningAppsTool() *ListRunningAppsTool {
	return &ListRunningAppsTool{}
}

func (t *ListRunningAppsTool) Name() string {
	return "list_running_apps"
}

func (t *ListRunningAppsTool) Description() string {
	return "Reports and lists applications currently running on the Mac. Returns application names, bundle identifiers, frontmost status, PIDs, and resource metrics. Supports filtering by name and toggling GUI vs background processes."
}

func (t *ListRunningAppsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"gui_only": map[string]any{
				"type":        "boolean",
				"description": "If true (default), returns only active GUI applications (e.g. Chrome, VS Code, Slack, Ghostty, Finder). If false, includes background processes.",
				"default":     true,
			},
			"search": map[string]any{
				"type":        "string",
				"description": "Optional search term to filter applications by name or bundle identifier (e.g. 'code', 'chrome', 'slack').",
			},
			"include_system": map[string]any{
				"type":        "boolean",
				"description": "If true, includes low-level system daemons and background agents.",
				"default":     false,
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of applications/processes to return (default 50).",
				"default":     50,
			},
		},
	}
}

// ListRunningAppsResult is the structured payload returned in ToolResult.Data.
type ListRunningAppsResult struct {
	Count       int       `json:"count"`
	TotalFound  int       `json:"total_found"`
	GUIOnly     bool      `json:"gui_only"`
	Timestamp   string    `json:"timestamp"`
	Frontmost   string    `json:"frontmost_app,omitempty"`
	Apps        []AppInfo `json:"apps"`
	SearchQuery string    `json:"search_query,omitempty"`
	Note        string    `json:"note,omitempty"`
}

func (t *ListRunningAppsTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	guiOnly := true
	limit := 50
	search := ""
	includeSystem := false

	if len(rawInput) > 0 && string(rawInput) != "{}" {
		var in ListRunningAppsInput
		if err := json.Unmarshal(rawInput, &in); err == nil {
			if in.GUIOnly != nil {
				guiOnly = *in.GUIOnly
			}
			if in.Limit > 0 {
				limit = in.Limit
			}
			search = strings.TrimSpace(in.Search)
			includeSystem = in.IncludeSystem
		}
	}

	apps, frontmost, note, err := t.collectRunningApps(ctx, guiOnly, includeSystem)
	if err != nil && len(apps) == 0 {
		return registry.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to list running applications: %v", err),
		}, err
	}

	// Apply search filter if provided
	var filtered []AppInfo
	searchLower := strings.ToLower(search)
	for _, app := range apps {
		if searchLower != "" {
			nameMatch := strings.Contains(strings.ToLower(app.Name), searchLower)
			bundleMatch := strings.Contains(strings.ToLower(app.BundleID), searchLower)
			execMatch := strings.Contains(strings.ToLower(app.ExecutablePath), searchLower)
			if !nameMatch && !bundleMatch && !execMatch {
				continue
			}
		}
		filtered = append(filtered, app)
	}

	// Sort: frontmost app first, then GUI apps alphabetically, then other processes
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].IsFrontmost != filtered[j].IsFrontmost {
			return filtered[i].IsFrontmost
		}
		if filtered[i].IsGUIApp != filtered[j].IsGUIApp {
			return filtered[i].IsGUIApp
		}
		return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
	})

	totalFound := len(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	resultData := ListRunningAppsResult{
		Count:       len(filtered),
		TotalFound:  totalFound,
		GUIOnly:     guiOnly,
		Timestamp:   time.Now().Format(time.RFC3339),
		Frontmost:   frontmost,
		Apps:        filtered,
		SearchQuery: search,
		Note:        note,
	}

	// Build clean, human-readable summary output for LLM
	var sb strings.Builder
	if guiOnly {
		sb.WriteString(fmt.Sprintf("Currently Running GUI Applications (%d found):\n", totalFound))
	} else {
		sb.WriteString(fmt.Sprintf("Currently Running Applications & Processes (%d found):\n", totalFound))
	}

	if frontmost != "" {
		sb.WriteString(fmt.Sprintf("• Active (Frontmost) Application: %s\n\n", frontmost))
	}

	for idx, app := range filtered {
		prefix := "•"
		if app.IsFrontmost {
			prefix = "★ [Active]"
		}
		extra := ""
		if app.BundleID != "" {
			extra = fmt.Sprintf(" (Bundle: %s, PID: %d)", app.BundleID, app.PID)
		} else if app.PID > 0 {
			extra = fmt.Sprintf(" (PID: %d)", app.PID)
		}
		if app.CPUPercent > 0 || app.MemoryPercent > 0 {
			extra += fmt.Sprintf(" [CPU: %.1f%%, MEM: %.1f%%]", app.CPUPercent, app.MemoryPercent)
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s%s\n", idx+1, prefix, app.Name, extra))
	}

	if note != "" {
		sb.WriteString(fmt.Sprintf("\nNote: %s\n", note))
	}

	return registry.ToolResult{
		Success: true,
		Output:  strings.TrimSpace(sb.String()),
		Data:    resultData,
	}, nil
}

// collectRunningApps combines AppleScript System Events and PS process table for comprehensive detection.
func (t *ListRunningAppsTool) collectRunningApps(ctx context.Context, guiOnly bool, includeSystem bool) ([]AppInfo, string, string, error) {
	var apps []AppInfo
	var frontmostApp string
	var notes []string

	// Method 1: Query System Events via osascript for rich GUI app metadata
	script := `tell application "System Events"
		set appList to every application process where background only is false
		set outList to {}
		repeat with p in appList
			set pName to ""
			set pBundle to ""
			set pFront to false
			set pPID to 0
			try
				set pName to name of p
			end try
			try
				set pBundle to bundle identifier of p
			end try
			try
				set pFront to frontmost of p
			end try
			try
				set pPID to unix id of p
			end try
			set end of outList to (pPID as text) & ":::" & pName & ":::" & (pBundle as text) & ":::" & (pFront as text)
		end repeat
		set AppleScript's text item delimiters to "|||"
		return outList as text
	end tell`

	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		rawStr := strings.TrimSpace(string(out))
		items := strings.Split(rawStr, "|||")
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			parts := strings.Split(item, ":::")
			if len(parts) >= 4 {
				pid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
				name := strings.TrimSpace(parts[1])
				bundle := strings.TrimSpace(parts[2])
				isFront := strings.TrimSpace(parts[3]) == "true"

				if name == "" {
					continue
				}

				if isFront {
					frontmostApp = name
				}

				apps = append(apps, AppInfo{
					Name:        name,
					BundleID:    bundle,
					PID:         pid,
					IsFrontmost: isFront,
					IsGUIApp:    true,
				})
			}
		}
	} else {
		notes = append(notes, "AppleScript query skipped or restricted, using process scan")
	}

	// Method 2: Process table inspection via `ps` - ALWAYS merge to capture all apps
	psApps, errPs := parseProcessTable(ctx, guiOnly, includeSystem)
	if errPs == nil && len(psApps) > 0 {
		existingPIDs := make(map[int]int)
		existingNames := make(map[string]int)
		for idx, a := range apps {
			if a.PID > 0 {
				existingPIDs[a.PID] = idx
			}
			existingNames[strings.ToLower(a.Name)] = idx
		}

		for _, p := range psApps {
			if idx, exists := existingPIDs[p.PID]; exists {
				apps[idx].CPUPercent = p.CPUPercent
				apps[idx].MemoryPercent = p.MemoryPercent
				if apps[idx].ExecutablePath == "" {
					apps[idx].ExecutablePath = p.ExecutablePath
				}
			} else if idx, nameExists := existingNames[strings.ToLower(p.Name)]; nameExists && apps[idx].PID == 0 {
				apps[idx].PID = p.PID
				apps[idx].CPUPercent = p.CPUPercent
				apps[idx].MemoryPercent = p.MemoryPercent
				if apps[idx].ExecutablePath == "" {
					apps[idx].ExecutablePath = p.ExecutablePath
				}
			} else {
				// If not in AppleScript results, add it if it matches GUI filter
				if !guiOnly || p.IsGUIApp {
					apps = append(apps, p)
					existingNames[strings.ToLower(p.Name)] = len(apps) - 1
					if p.PID > 0 {
						existingPIDs[p.PID] = len(apps) - 1
					}
				}
			}
		}
	} else if len(apps) == 0 {
		return nil, "", "", fmt.Errorf("could not collect processes: %v", errPs)
	}

	return apps, frontmostApp, strings.Join(notes, "; "), nil
}

var appBundleRegex = regexp.MustCompile(`/(?:[^/]+/)*([^/]+)\.app(?:/|$)`)

func parseProcessTable(ctx context.Context, guiOnly bool, includeSystem bool) ([]AppInfo, error) {
	// Query ps with PID, %CPU, %MEM, Command
	cmd := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid,%cpu,%mem,command")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var results []AppInfo
	lines := strings.Split(string(out), "\n")
	if len(lines) <= 1 {
		return results, nil
	}

	seenNames := make(map[string]bool)

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		cpu, _ := strconv.ParseFloat(fields[1], 64)
		mem, _ := strconv.ParseFloat(fields[2], 64)
		commandPath := strings.Join(fields[3:], " ")

		isGUI := false
		appName := ""

		// Check if process is part of a macOS .app bundle
		if match := appBundleRegex.FindStringSubmatch(commandPath); len(match) > 1 {
			appName = match[1]
			isGUI = true
		} else if strings.Contains(commandPath, ".app/Contents/MacOS/") {
			parts := strings.Split(commandPath, ".app/Contents/MacOS/")
			appName = filepath.Base(parts[0])
			isGUI = true
		} else {
			appName = filepath.Base(fields[3])
		}

		// Clean appName
		appName = strings.TrimSpace(appName)
		if appName == "" {
			continue
		}

		// Avoid helper process naming duplicates for main apps
		if strings.HasSuffix(appName, " Helper") || strings.HasSuffix(appName, " Helper (Renderer)") || strings.HasSuffix(appName, " Helper (GPU)") {
			base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(appName, " Helper (GPU)"), " Helper (Renderer)"), " Helper")
			if base != "" {
				appName = base
			}
		}

		if guiOnly && !isGUI {
			continue
		}

		if !includeSystem && isSystemDaemon(appName, commandPath) {
			continue
		}

		// Avoid spammy duplicates for multi-process apps (like Chrome/Electron/WhatsApp helpers) when guiOnly is true
		if guiOnly && seenNames[strings.ToLower(appName)] {
			continue
		}
		seenNames[strings.ToLower(appName)] = true

		results = append(results, AppInfo{
			Name:           appName,
			PID:            pid,
			IsGUIApp:       isGUI,
			CPUPercent:     cpu,
			MemoryPercent:  mem,
			ExecutablePath: commandPath,
		})
	}

	return results, nil
}

func isSystemDaemon(name string, command string) bool {
	lowerName := strings.ToLower(name)
	lowerCmd := strings.ToLower(command)

	// Filter common system background daemons unless requested
	systemPrefixes := []string{"launchd", "kernel_task", "kextd", "logd", "syslogd", "distnoted", "fseventsd", "mds", "mds_stores", "diskarbitrationd", "notifyd", "cfprefsd", "powerd", "opendirectoryd", "trustd", "securityd", "usbd"}
	for _, p := range systemPrefixes {
		if lowerName == p || strings.HasPrefix(lowerName, p) {
			return true
		}
	}

	if strings.HasPrefix(lowerCmd, "/usr/libexec/") || strings.HasPrefix(lowerCmd, "/system/library/") {
		return true
	}

	return false
}

// GenerateFormattedSummary produces a human-readable list of running apps.
func GenerateFormattedSummary(apps []AppInfo, frontmost string) string {
	var buf bytes.Buffer
	if frontmost != "" {
		buf.WriteString(fmt.Sprintf("• Active / Frontmost Application: %s\n\n", frontmost))
	}
	for i, app := range apps {
		frontTag := ""
		if app.IsFrontmost {
			frontTag = " ★ [Active]"
		}
		buf.WriteString(fmt.Sprintf("%d. %s%s (PID: %d)\n", i+1, app.Name, frontTag, app.PID))
	}
	return buf.String()
}

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"bob/internal/tools/registry"
)

// OpenAppInput defines the input schema for open_app.
type OpenAppInput struct {
	AppName     string `json:"app_name"`
	Target      string `json:"target,omitempty"`
	BundleID    string `json:"bundle_id,omitempty"`
	NewInstance bool   `json:"new_instance,omitempty"`
}

// OpenAppTool launches an application on macOS.
type OpenAppTool struct{}

// NewOpenAppTool creates a new OpenAppTool instance.
func NewOpenAppTool() *OpenAppTool {
	return &OpenAppTool{}
}

func (t *OpenAppTool) Name() string {
	return "open_app"
}

func (t *OpenAppTool) Description() string {
	return "Opens or launches an application on macOS by name or bundle ID, optionally opening a file, folder, or URL. Automatically verifies that the application is running."
}

func (t *OpenAppTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"app_name": map[string]any{
				"type":        "string",
				"description": "Name of the application to open (e.g. 'Slack', 'Brave Browser', 'Visual Studio Code', 'Calculator', 'Notes').",
			},
			"target": map[string]any{
				"type":        "string",
				"description": "Optional file path, directory, or URL to open with the application.",
			},
			"bundle_id": map[string]any{
				"type":        "string",
				"description": "Optional macOS bundle identifier (e.g. 'com.tinyspeck.slackmacgap', 'com.brave.Browser').",
			},
			"new_instance": map[string]any{
				"type":        "boolean",
				"description": "If true, opens a new instance of the application even if one is already running.",
				"default":     false,
			},
		},
		"required": []string{"app_name"},
	}
}

type OpenAppResult struct {
	AppName    string `json:"app_name"`
	ResolvedPath string `json:"resolved_path,omitempty"`
	Target     string `json:"target,omitempty"`
	BundleID   string `json:"bundle_id,omitempty"`
	Timestamp  string `json:"timestamp"`
	Launched   bool   `json:"launched"`
	Verified   bool   `json:"verified"`
	PID        int    `json:"pid,omitempty"`
}

func (t *OpenAppTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in OpenAppInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid input for open_app: %v", err),
		}, err
	}

	appName := strings.TrimSpace(in.AppName)
	bundleID := strings.TrimSpace(in.BundleID)
	target := strings.TrimSpace(in.Target)

	if appName == "" && bundleID == "" {
		return registry.ToolResult{
			Success: false,
			Error:   "Either app_name or bundle_id must be provided to open an application",
		}, fmt.Errorf("missing app_name or bundle_id")
	}

	// 1. Locate app path if possible
	resolvedPath := FindAppPath(appName)

	// 2. Build open arguments
	var args []string
	if in.NewInstance {
		args = append(args, "-n")
	}

	if resolvedPath != "" {
		args = append(args, resolvedPath)
	} else if bundleID != "" {
		args = append(args, "-b", bundleID)
	} else {
		args = append(args, "-a", appName)
	}

	if target != "" {
		args = append(args, target)
	}

	// Execute open command
	cmd := exec.CommandContext(ctx, "/usr/bin/open", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback: try AppleScript activate
		script := fmt.Sprintf(`tell application "%s" to activate`, appName)
		asCmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
		if asOut, asErr := asCmd.CombinedOutput(); asErr != nil {
			errStr := strings.TrimSpace(string(out))
			if errStr == "" {
				errStr = string(asOut)
			}
			return registry.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to open %q: %s", appName, errStr),
			}, fmt.Errorf("failed to open app %s: %w", appName, err)
		}
	}

	// 3. Post-launch Verification: Poll for app running status (up to 2 seconds)
	verified, pid := WaitForAppState(ctx, appName, bundleID, 0, true, 2*time.Second)

	displayApp := appName
	if displayApp == "" {
		displayApp = bundleID
	}

	var sb strings.Builder
	if verified {
		sb.WriteString(fmt.Sprintf("Successfully opened application %q.", displayApp))
		if pid > 0 {
			sb.WriteString(fmt.Sprintf(" [Verified: Process is active with PID %d]", pid))
		} else {
			sb.WriteString(" [Verified: Application process is now running]")
		}
	} else {
		sb.WriteString(fmt.Sprintf("Launch command issued for %q, but process was not immediately confirmed active. It may still be initializing.", displayApp))
	}

	if target != "" {
		sb.WriteString(fmt.Sprintf(" Opened with target %q.", target))
	}

	return registry.ToolResult{
		Success: true,
		Output:  sb.String(),
		Data: OpenAppResult{
			AppName:      displayApp,
			ResolvedPath: resolvedPath,
			Target:       target,
			BundleID:     bundleID,
			Timestamp:    time.Now().Format(time.RFC3339),
			Launched:     true,
			Verified:     verified,
			PID:          pid,
		},
	}, nil
}

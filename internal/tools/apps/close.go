package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"bob/internal/tools/registry"
)

// CloseAppInput defines the input schema for close_app.
type CloseAppInput struct {
	AppName  string `json:"app_name"`
	Force    bool   `json:"force,omitempty"`
	PID      int    `json:"pid,omitempty"`
	BundleID string `json:"bundle_id,omitempty"`
}

// CloseAppTool closes or quits a running application on macOS.
type CloseAppTool struct{}

// NewCloseAppTool creates a new CloseAppTool instance.
func NewCloseAppTool() *CloseAppTool {
	return &CloseAppTool{}
}

func (t *CloseAppTool) Name() string {
	return "close_app"
}

func (t *CloseAppTool) Description() string {
	return "Closes or quits a running application on macOS. Gracefully quits via AppleScript and SIGTERM by default, or force-kills (SIGKILL) if requested. Automatically verifies that the process has completely terminated."
}

func (t *CloseAppTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"app_name": map[string]any{
				"type":        "string",
				"description": "Name of the running application to close (e.g. 'WhatsApp', 'Slack', 'Brave Browser', 'Visual Studio Code', 'Spotify').",
			},
			"force": map[string]any{
				"type":        "boolean",
				"description": "If true, immediately force-kills the application process (SIGKILL). Defaults to false for graceful application quit.",
				"default":     false,
			},
			"pid": map[string]any{
				"type":        "integer",
				"description": "Optional process ID (PID) of the application to terminate specifically.",
			},
			"bundle_id": map[string]any{
				"type":        "string",
				"description": "Optional bundle identifier of the application (e.g. 'net.whatsapp.WhatsApp', 'com.tinyspeck.slackmacgap').",
			},
		},
		"required": []string{"app_name"},
	}
}

type CloseAppResult struct {
	AppName        string `json:"app_name"`
	PID            int    `json:"pid,omitempty"`
	TerminatedPIDs []int  `json:"terminated_pids,omitempty"`
	Force          bool   `json:"force"`
	Timestamp      string `json:"timestamp"`
	Closed         bool   `json:"closed"`
	AlreadyClosed  bool   `json:"already_closed,omitempty"`
	Verified       bool   `json:"verified"`
	Method         string `json:"method"`
}

func (t *CloseAppTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in CloseAppInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Invalid input for close_app: %v", err),
		}, err
	}

	appName := strings.TrimSpace(in.AppName)
	bundleID := strings.TrimSpace(in.BundleID)
	pid := in.PID
	force := in.Force

	if appName == "" && bundleID == "" && pid <= 0 {
		return registry.ToolResult{
			Success: false,
			Error:   "Must provide app_name, bundle_id, or pid to close an application",
		}, fmt.Errorf("missing app identification")
	}

	displayApp := appName
	if displayApp == "" {
		if bundleID != "" {
			displayApp = bundleID
		} else {
			displayApp = fmt.Sprintf("PID %d", pid)
		}
	}

	// 1. Discover all target PIDs
	var targetPIDs []int
	if pid > 0 {
		targetPIDs = append(targetPIDs, pid)
	} else {
		procs := FindAppProcesses(ctx, appName, bundleID)
		for _, p := range procs {
			targetPIDs = append(targetPIDs, p.PID)
		}
	}

	// Check if already not running
	if len(targetPIDs) == 0 {
		running, foundPID := CheckAppRunning(ctx, appName, bundleID, pid)
		if !running {
			return registry.ToolResult{
				Success: true,
				Output:  fmt.Sprintf("Application %q is not currently running.", displayApp),
				Data: CloseAppResult{
					AppName:       displayApp,
					Force:         force,
					Timestamp:     time.Now().Format(time.RFC3339),
					Closed:        false,
					AlreadyClosed: true,
					Verified:      true,
					Method:        "noop_already_closed",
				},
			}, nil
		}
		if foundPID > 0 {
			targetPIDs = append(targetPIDs, foundPID)
		}
	}

	primaryPID := 0
	if len(targetPIDs) > 0 {
		primaryPID = targetPIDs[0]
	}

	method := "graceful"

	if force {
		// FORCE QUIT (SIGKILL)
		method = "sigkill"
		for _, p := range targetPIDs {
			_ = exec.CommandContext(ctx, "/bin/kill", "-9", strconv.Itoa(p)).Run()
		}
		if appName != "" {
			_ = exec.CommandContext(ctx, "/usr/bin/pkill", "-9", "-f", "-i", appName).Run()
			_ = exec.CommandContext(ctx, "/usr/bin/killall", "-9", appName).Run()
		}
	} else {
		// GRACEFUL QUIT: Multi-tiered (AppleScript + SIGTERM fallback)
		method = "applescript_quit"

		// Step 1: Send AppleScript quit command
		script := ""
		if bundleID != "" {
			script = fmt.Sprintf(`tell application id "%s" to quit`, bundleID)
		} else {
			script = fmt.Sprintf(`tell application "%s" to quit`, appName)
		}
		_ = exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script).Run()

		// Allow brief moment for app to respond to AppleEvent
		time.Sleep(300 * time.Millisecond)

		// Step 2: Check if app exited or is still running
		stillRunning, _ := CheckAppRunning(ctx, appName, bundleID, primaryPID)
		if stillRunning {
			// Step 3: Send graceful SIGTERM to all matched PIDs
			method = "sigterm"
			for _, p := range targetPIDs {
				_ = exec.CommandContext(ctx, "/bin/kill", "-15", strconv.Itoa(p)).Run()
			}
			if appName != "" {
				_ = exec.CommandContext(ctx, "/usr/bin/pkill", "-15", "-f", "-i", appName).Run()
			}
		}
	}

	// Post-close Verification: Wait up to 2 seconds for all processes to exit
	verifiedClosed, remainingPID := WaitForAppState(ctx, appName, bundleID, primaryPID, false, 2*time.Second)

	var sb strings.Builder
	if verifiedClosed {
		if len(targetPIDs) > 1 {
			sb.WriteString(fmt.Sprintf("Application %q (PIDs: %v) was successfully closed (%s). [Verified: All processes have exited]", displayApp, targetPIDs, method))
		} else if primaryPID > 0 {
			sb.WriteString(fmt.Sprintf("Application %q (PID %d) was successfully closed (%s). [Verified: Process has exited and is no longer running]", displayApp, primaryPID, method))
		} else {
			sb.WriteString(fmt.Sprintf("Application %q was successfully closed (%s). [Verified: Process has exited and is no longer running]", displayApp, method))
		}
	} else {
		if remainingPID > 0 {
			sb.WriteString(fmt.Sprintf("Notice: Application %q (PID %d) was sent close signal (%s), but is STILL running. The application may have unsaved changes or require a force quit. Call close_app with force: true to terminate immediately.", displayApp, remainingPID, method))
		} else {
			sb.WriteString(fmt.Sprintf("Notice: Application %q was sent close signal (%s), but process is STILL detected running. Call close_app with force: true to force terminate immediately.", displayApp, method))
		}
	}

	return registry.ToolResult{
		Success: true,
		Output:  sb.String(),
		Data: CloseAppResult{
			AppName:        displayApp,
			PID:            primaryPID,
			TerminatedPIDs: targetPIDs,
			Force:          force,
			Timestamp:      time.Now().Format(time.RFC3339),
			Closed:         verifiedClosed,
			Verified:       verifiedClosed,
			Method:         method,
		},
	}, nil
}

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
	return "Closes or quits a running application on macOS. Performs a graceful quit by default via macOS application scripting, or force terminates if requested. Automatically verifies that the process has exited."
}

func (t *CloseAppTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"app_name": map[string]any{
				"type":        "string",
				"description": "Name of the running application to close (e.g. 'Slack', 'Brave Browser', 'Visual Studio Code', 'WhatsApp', 'Spotify').",
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
				"description": "Optional bundle identifier of the application (e.g. 'com.tinyspeck.slackmacgap').",
			},
		},
		"required": []string{"app_name"},
	}
}

type CloseAppResult struct {
	AppName   string `json:"app_name"`
	PID       int    `json:"pid,omitempty"`
	Force     bool   `json:"force"`
	Timestamp string `json:"timestamp"`
	Closed    bool   `json:"closed"`
	Verified  bool   `json:"verified"`
	Method    string `json:"method"`
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

	method := "graceful"
	var closeErr error

	if pid > 0 && force {
		// Force kill by PID
		cmd := exec.CommandContext(ctx, "/bin/kill", "-9", strconv.Itoa(pid))
		out, err := cmd.CombinedOutput()
		if err != nil {
			closeErr = fmt.Errorf("kill -9 pid %d failed: %s (%v)", pid, string(out), err)
		}
		method = "sigkill_pid"
	} else if pid > 0 && !force {
		// Graceful signal by PID
		cmd := exec.CommandContext(ctx, "/bin/kill", "-15", strconv.Itoa(pid))
		out, err := cmd.CombinedOutput()
		if err != nil {
			closeErr = fmt.Errorf("kill -15 pid %d failed: %s (%v)", pid, string(out), err)
		}
		method = "sigterm_pid"
	} else if !force {
		// Method 1: Graceful AppleScript quit
		script := ""
		if bundleID != "" {
			script = fmt.Sprintf(`tell application id "%s" to quit`, bundleID)
		} else {
			script = fmt.Sprintf(`tell application "%s" to quit`, appName)
		}

		cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Fallback: try pkill if AppleScript failed
			pkillCmd := exec.CommandContext(ctx, "/usr/bin/pkill", "-x", appName)
			pkillOut, pkillErr := pkillCmd.CombinedOutput()
			if pkillErr != nil {
				// Try case-insensitive pkill
				pkillF := exec.CommandContext(ctx, "/usr/bin/pkill", "-f", "-i", appName)
				if fOut, fErr := pkillF.CombinedOutput(); fErr != nil {
					closeErr = fmt.Errorf("could not close app %q (osascript: %s, pkill: %s, %s)", appName, string(out), string(pkillOut), string(fOut))
				} else {
					method = "pkill_fallback"
				}
			} else {
				method = "pkill_graceful"
			}
		} else {
			method = "applescript_quit"
		}
	} else {
		// Force kill by app name
		killallCmd := exec.CommandContext(ctx, "/usr/bin/killall", "-9", appName)
		out, err := killallCmd.CombinedOutput()
		if err != nil {
			pkillCmd := exec.CommandContext(ctx, "/usr/bin/pkill", "-9", "-f", "-i", appName)
			pkillOut, pkillErr := pkillCmd.CombinedOutput()
			if pkillErr != nil {
				closeErr = fmt.Errorf("force kill failed for %q: %s (%s)", appName, string(out), string(pkillOut))
			} else {
				method = "pkill_sigkill"
			}
		} else {
			method = "killall_sigkill"
		}
	}

	if closeErr != nil {
		return registry.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to close application %q: %v", appName, closeErr),
		}, closeErr
	}

	displayApp := appName
	if displayApp == "" {
		displayApp = fmt.Sprintf("PID %d", pid)
	}

	// Post-close Verification: Check if app has indeed exited (up to 2 seconds)
	verifiedClosed, _ := WaitForAppState(ctx, appName, bundleID, pid, false, 2*time.Second)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Application %q close command executed (%s).", displayApp, method))
	if verifiedClosed {
		sb.WriteString(" [Verified: Process has exited and is no longer running]")
	} else {
		sb.WriteString(" [Notice: Process was still detected after 2s. The app may be waiting on an unsaved prompt or require force=true].")
	}

	return registry.ToolResult{
		Success: true,
		Output:  sb.String(),
		Data: CloseAppResult{
			AppName:   displayApp,
			PID:       pid,
			Force:     force,
			Timestamp: time.Now().Format(time.RFC3339),
			Closed:    true,
			Verified:  verifiedClosed,
			Method:    method,
		},
	}, nil
}

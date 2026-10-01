package apps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FindAppPath attempts to locate the .app bundle on the macOS filesystem.
func FindAppPath(appName string) string {
	cleanName := strings.TrimSpace(appName)
	if cleanName == "" {
		return ""
	}

	// If already a full path to .app
	if strings.HasSuffix(cleanName, ".app") && fileOrDirExists(cleanName) {
		return cleanName
	}

	homeDir, _ := os.UserHomeDir()
	searchDirs := []string{
		"/Applications",
		filepath.Join(homeDir, "Applications"),
		"/System/Applications",
		"/System/Applications/Utilities",
		"/System/Library/CoreServices/Applications",
	}

	withSuffix := cleanName
	if !strings.HasSuffix(strings.ToLower(withSuffix), ".app") {
		withSuffix = withSuffix + ".app"
	}

	// Direct check in standard directories
	for _, dir := range searchDirs {
		candidate := filepath.Join(dir, withSuffix)
		if fileOrDirExists(candidate) {
			return candidate
		}
	}

	// Case-insensitive directory scan
	cleanLower := strings.ToLower(cleanName)
	cleanSuffixLower := strings.ToLower(withSuffix)
	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			eNameLower := strings.ToLower(e.Name())
			if eNameLower == cleanSuffixLower || strings.TrimSuffix(eNameLower, ".app") == cleanLower {
				return filepath.Join(dir, e.Name())
			}
		}
	}

	return ""
}

// CheckAppRunning returns whether the specified application/PID is active and its PID.
func CheckAppRunning(ctx context.Context, appName, bundleID string, pid int) (bool, int) {
	if pid > 0 {
		// Check PID
		cmd := exec.CommandContext(ctx, "/bin/kill", "-0", strconv.Itoa(pid))
		if err := cmd.Run(); err == nil {
			return true, pid
		}
		return false, 0
	}

	cleanName := strings.TrimSpace(appName)
	cleanName = strings.TrimSuffix(cleanName, ".app")

	// 1. Try pgrep (fastest)
	if cleanName != "" {
		cmd := exec.CommandContext(ctx, "/usr/bin/pgrep", "-f", "-i", cleanName)
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) > 0 {
				if parsedPID, err := strconv.Atoi(lines[0]); err == nil {
					return true, parsedPID
				}
			}
			return true, 0
		}
	}

	// 2. Try osascript for exact GUI app name
	if cleanName != "" {
		script := fmt.Sprintf(`tell application "System Events" to (name of processes) contains "%s"`, cleanName)
		cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script)
		out, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(out)) == "true" {
			return true, 0
		}
	}

	return false, 0
}

// WaitForAppState polls until the application reaches the desired state (running or closed).
func WaitForAppState(ctx context.Context, appName, bundleID string, pid int, shouldBeRunning bool, timeout time.Duration) (bool, int) {
	deadline := time.Now().Add(timeout)
	lastPID := 0

	for time.Now().Before(deadline) {
		running, foundPID := CheckAppRunning(ctx, appName, bundleID, pid)
		if foundPID > 0 {
			lastPID = foundPID
		}
		if running == shouldBeRunning {
			return true, lastPID
		}
		time.Sleep(200 * time.Millisecond)
	}

	running, foundPID := CheckAppRunning(ctx, appName, bundleID, pid)
	if foundPID > 0 {
		lastPID = foundPID
	}
	return running == shouldBeRunning, lastPID
}

func fileOrDirExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

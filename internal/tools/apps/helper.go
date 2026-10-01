package apps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ProcessInfo holds information about a discovered running process.
type ProcessInfo struct {
	PID            int
	PPID           int
	Name           string
	ExecutablePath string
	IsAppBundle    bool
	BundleName     string
}

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
		"/Applications/Utilities",
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

	// Spotlight mdfind fallback for non-standard install locations
	if out, err := exec.Command("/usr/bin/mdfind", fmt.Sprintf("kMDItemFSName == '%s' || kMDItemDisplayName == '%s'", withSuffix, cleanName)).Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasSuffix(line, ".app") && fileOrDirExists(line) {
				return line
			}
		}
	}

	return ""
}

var appBundleRegexAll = regexp.MustCompile(`/(?:[^/]+/)*([^/]+)\.app(?:/|$)`)

// FindAppProcesses scans running processes to find all PIDs matching an app name or bundle ID.
func FindAppProcesses(ctx context.Context, appName, bundleID string) []ProcessInfo {
	cleanName := strings.TrimSpace(appName)
	cleanNameLower := strings.ToLower(strings.TrimSuffix(cleanName, ".app"))
	bundleIDLower := strings.ToLower(strings.TrimSpace(bundleID))

	if cleanNameLower == "" && bundleIDLower == "" {
		return nil
	}

	var matched []ProcessInfo
	seenPIDs := make(map[int]bool)

	// Strategy 1: Parse PS table with full command lines
	cmd := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid,ppid,comm,command")
	if out, err := cmd.Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}

			pid, err1 := strconv.Atoi(fields[0])
			ppid, err2 := strconv.Atoi(fields[1])
			if err1 != nil || err2 != nil || pid <= 0 {
				continue
			}

			comm := fields[2]
			commandPath := strings.Join(fields[3:], " ")
			commBase := filepath.Base(comm)

			isBundle := false
			bundleName := ""
			if m := appBundleRegexAll.FindStringSubmatch(commandPath); len(m) > 1 {
				bundleName = m[1]
				isBundle = true
			}

			bundleNameLower := strings.ToLower(bundleName)
			commLower := strings.ToLower(commBase)

			isMatch := false
			if cleanNameLower != "" {
				if commLower == cleanNameLower || bundleNameLower == cleanNameLower {
					isMatch = true
				} else if strings.Contains(strings.ToLower(commandPath), "/"+cleanNameLower+".app/") {
					isMatch = true
				} else if strings.Contains(commLower, cleanNameLower) && !isSystemDaemon(commBase, commandPath) {
					isMatch = true
				}
			}

			if isMatch && !seenPIDs[pid] {
				seenPIDs[pid] = true
				matched = append(matched, ProcessInfo{
					PID:            pid,
					PPID:           ppid,
					Name:           commBase,
					ExecutablePath: commandPath,
					IsAppBundle:    isBundle,
					BundleName:     bundleName,
				})
			}
		}
	}

	// Strategy 2: pgrep fallback if PS missed something
	if len(matched) == 0 && cleanNameLower != "" {
		pgrepCmd := exec.CommandContext(ctx, "/usr/bin/pgrep", "-i", "-f", cleanNameLower)
		if pOut, pErr := pgrepCmd.Output(); pErr == nil {
			lines := strings.Split(strings.TrimSpace(string(pOut)), "\n")
			for _, l := range lines {
				if pid, err := strconv.Atoi(strings.TrimSpace(l)); err == nil && pid > 0 && !seenPIDs[pid] {
					seenPIDs[pid] = true
					matched = append(matched, ProcessInfo{
						PID:  pid,
						Name: cleanName,
					})
				}
			}
		}
	}

	// Strategy 3: AppleScript System Events PID lookup
	if len(matched) == 0 && cleanName != "" {
		script := fmt.Sprintf(`tell application "System Events" to get unix id of every application process whose name is "%s"`, cleanName)
		if asOut, asErr := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script).Output(); asErr == nil {
			parts := strings.Split(strings.TrimSpace(string(asOut)), ",")
			for _, p := range parts {
				if pid, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && pid > 0 && !seenPIDs[pid] {
					seenPIDs[pid] = true
					matched = append(matched, ProcessInfo{
						PID:  pid,
						Name: cleanName,
					})
				}
			}
		}
	}

	return matched
}

// CheckAppRunning returns whether the specified application/PID is active and its main PID.
func CheckAppRunning(ctx context.Context, appName, bundleID string, pid int) (bool, int) {
	if pid > 0 {
		cmd := exec.CommandContext(ctx, "/bin/kill", "-0", strconv.Itoa(pid))
		if err := cmd.Run(); err == nil {
			return true, pid
		}
		return false, 0
	}

	procs := FindAppProcesses(ctx, appName, bundleID)
	for _, p := range procs {
		cmd := exec.CommandContext(ctx, "/bin/kill", "-0", strconv.Itoa(p.PID))
		if err := cmd.Run(); err == nil {
			return true, p.PID
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
		time.Sleep(100 * time.Millisecond)
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

package coding

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	ErrAntiGravityDisabled    = errors.New("AntiGravity integration is currently disabled or unconfigured")
	ErrAntiGravityCLINotFound = errors.New("AntiGravity official CLI/API not detected on host")
)

// CodingTask represents a delegated software engineering task.
type CodingTask struct {
	Prompt      string   `json:"prompt"`
	RepoPath    string   `json:"repo_path"`
	TargetFiles []string `json:"target_files,omitempty"`
	TimeoutSec  int      `json:"timeout_seconds,omitempty"`
}

// CodingResult represents the outcome of a delegated coding task.
type CodingResult struct {
	Success      bool     `json:"success"`
	Summary      string   `json:"summary"`
	ModifiedFiles []string `json:"modified_files,omitempty"`
	Logs         string   `json:"logs,omitempty"`
	DurationMs   int64    `json:"duration_ms"`
}

// CodingAgent is the abstraction for specialized coding subagents (like AntiGravity).
type CodingAgent interface {
	Name() string
	Available() (bool, string)
	Execute(ctx context.Context, request CodingTask) (CodingResult, error)
}

// AntiGravityCodingAgent integrates Bob with AntiGravity if an official CLI/daemon exists.
type AntiGravityCodingAgent struct {
	enabled bool
	cliPath string
}

func NewAntiGravityCodingAgent(enabled bool) *AntiGravityCodingAgent {
	cliPath, _ := exec.LookPath("agy")
	if cliPath == "" {
		cliPath, _ = exec.LookPath("antigravity")
	}
	return &AntiGravityCodingAgent{
		enabled: enabled,
		cliPath: cliPath,
	}
}

func (a *AntiGravityCodingAgent) Name() string {
	return "antigravity"
}

// Available checks whether AntiGravity app or CLI is present on the Mac.
func (a *AntiGravityCodingAgent) Available() (bool, string) {
	if !a.enabled {
		return false, "Integration disabled in Bob configuration"
	}

	if a.cliPath != "" {
		return true, fmt.Sprintf("AntiGravity CLI found at %s", a.cliPath)
	}

	appPath := "/Applications/Antigravity.app"
	if _, err := os.Stat(appPath); err == nil {
		return false, "AntiGravity.app bundle installed, but no public headless CLI exposed in PATH"
	}

	home, _ := os.UserHomeDir()
	appSupport := filepath.Join(home, "Library", "Application Support", "Antigravity")
	if _, err := os.Stat(appSupport); err == nil {
		return false, "AntiGravity application data exists, but CLI automation endpoint is unconfigured"
	}

	return false, "AntiGravity installation not detected"
}

// Execute delegates the coding task to AntiGravity if CLI is available, or returns a clear boundary message.
func (a *AntiGravityCodingAgent) Execute(ctx context.Context, request CodingTask) (CodingResult, error) {
	start := time.Now()
	avail, reason := a.Available()
	if !avail {
		return CodingResult{
			Success:    false,
			Summary:    fmt.Sprintf("AntiGravity coding agent unavailable: %s", reason),
			DurationMs: time.Since(start).Milliseconds(),
		}, fmt.Errorf("%w: %s", ErrAntiGravityCLINotFound, reason)
	}

	// When official CLI becomes available, execute sub-process
	cmd := exec.CommandContext(ctx, a.cliPath, "--repo", request.RepoPath, "--task", request.Prompt)
	out, err := cmd.CombinedOutput()
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		return CodingResult{
			Success:    false,
			Summary:    fmt.Sprintf("AntiGravity task failed: %v", err),
			Logs:       string(out),
			DurationMs: durationMs,
		}, err
	}

	return CodingResult{
		Success:    true,
		Summary:    "AntiGravity task completed successfully.",
		Logs:       string(out),
		DurationMs: durationMs,
	}, nil
}

package terminal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"bob/internal/security"
	"bob/internal/tools/registry"
)

type TerminalTool struct {
	policy         *security.CommandPolicy
	pathValidator  *security.PathValidator
	defaultWorkDir string
	timeout        time.Duration
	maxOutputBytes int64
}

type TerminalInput struct {
	Command string `json:"command"`
	WorkDir string `json:"work_dir,omitempty"`
	Timeout int    `json:"timeout_seconds,omitempty"`
}

type TerminalOutput struct {
	Command    string `json:"command"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
}

func NewTerminalTool(policy *security.CommandPolicy, pathValidator *security.PathValidator, defaultWorkDir string, defaultTimeout time.Duration, maxOutputBytes int64) *TerminalTool {
	if defaultTimeout <= 0 {
		defaultTimeout = 60 * time.Second
	}
	if maxOutputBytes <= 0 {
		maxOutputBytes = 1024 * 1024 // 1MB
	}
	if defaultWorkDir == "" {
		defaultWorkDir, _ = os.Getwd()
	}

	return &TerminalTool{
		policy:         policy,
		pathValidator:  pathValidator,
		defaultWorkDir: defaultWorkDir,
		timeout:        defaultTimeout,
		maxOutputBytes: maxOutputBytes,
	}
}

func (t *TerminalTool) Name() string {
	return "terminal_exec"
}

func (t *TerminalTool) Description() string {
	return "Executes shell commands in a controlled terminal environment. Returns command, stdout, stderr, exit code, and execution duration."
}

func (t *TerminalTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute (e.g. 'pwd', 'ls -la', 'git status', 'go test ./...')",
			},
			"work_dir": map[string]any{
				"type":        "string",
				"description": "Optional working directory. Must be within allowed workspace paths.",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"description": "Optional execution timeout in seconds (default 60s).",
			},
		},
		"required": []string{"command"},
	}
}

func (t *TerminalTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in TerminalInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid input format: %v", err),
		}, err
	}

	commandStr := strings.TrimSpace(in.Command)
	if commandStr == "" {
		return registry.ToolResult{
			Success: false,
			Error:   "command cannot be empty",
		}, fmt.Errorf("empty command")
	}

	// 1. Policy check
	level, reason := t.policy.EvaluateCommandWithContext(ctx, commandStr)
	if level == security.PolicyBlocked {
		return registry.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("command blocked by security policy: %s", reason),
		}, fmt.Errorf("command blocked: %s", reason)
	}

	// 2. Working Directory validation
	workDir := t.defaultWorkDir
	if sc, ok := security.SessionFromContext(ctx); ok && sc.ProjectPath != "" {
		workDir = sc.ProjectPath
	}

	if in.WorkDir != "" {
		validDir, err := t.pathValidator.ValidatePathWithContext(ctx, in.WorkDir)
		if err != nil {
			return registry.ToolResult{
				Success: false,
				Error:   fmt.Sprintf("invalid working directory: %v", err),
			}, err
		}
		workDir = validDir
	}

	// 3. Timeout setup
	timeout := t.timeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 4. Execute via zsh (macOS standard)
	cmd := exec.CommandContext(execCtx, "/bin/zsh", "-c", commandStr)
	cmd.Dir = workDir
	// Filter sensitive environment variables
	cmd.Env = []string{
		"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin",
		"HOME=" + os.Getenv("HOME"),
		"USER=" + os.Getenv("USER"),
		"LANG=en_US.UTF-8",
		"SHELL=/bin/zsh",
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	runErr := cmd.Run()
	durationMs := time.Since(start).Milliseconds()

	exitCode := 0
	if runErr != nil {
		if exitError, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else if execCtx.Err() == context.DeadlineExceeded {
			exitCode = 124 // Standard timeout code
			stderrBuf.WriteString(fmt.Sprintf("\n[Process timed out after %v]", timeout))
		} else {
			exitCode = 1
			stderrBuf.WriteString(fmt.Sprintf("\n[Execution error: %v]", runErr))
		}
	}

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	// Truncate if exceeds maxOutputBytes
	if int64(len(stdoutStr)) > t.maxOutputBytes {
		stdoutStr = stdoutStr[:t.maxOutputBytes] + "\n...[output truncated due to size limit]"
	}
	if int64(len(stderrStr)) > t.maxOutputBytes {
		stderrStr = stderrStr[:t.maxOutputBytes] + "\n...[stderr truncated due to size limit]"
	}

	output := TerminalOutput{
		Command:    commandStr,
		Stdout:     stdoutStr,
		Stderr:     stderrStr,
		ExitCode:   exitCode,
		DurationMs: durationMs,
	}

	formattedSummary := fmt.Sprintf("Command: %s\nExit Code: %d\nDuration: %dms\n\nStdout:\n%s",
		commandStr, exitCode, durationMs, stdoutStr)
	if stderrStr != "" {
		formattedSummary += fmt.Sprintf("\n\nStderr:\n%s", stderrStr)
	}

	return registry.ToolResult{
		Success:    exitCode == 0,
		Output:     formattedSummary,
		Data:       output,
		DurationMs: durationMs,
		Error:      mapError(runErr, exitCode, stderrStr),
	}, nil
}

func mapError(err error, exitCode int, stderr string) string {
	if exitCode == 0 {
		return ""
	}
	if stderr != "" {
		return fmt.Sprintf("exit code %d: %s", exitCode, strings.TrimSpace(stderr))
	}
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("exit code %d", exitCode)
}

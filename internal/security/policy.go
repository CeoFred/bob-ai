package security

import (
	"context"
	"fmt"
	"strings"
)

// PolicyLevel represents security classifications for operations.
type PolicyLevel string

const (
	PolicySafe             PolicyLevel = "SAFE"
	PolicyApprovalRequired PolicyLevel = "APPROVAL_REQUIRED"
	PolicyBlocked          PolicyLevel = "BLOCKED"
)

// CommandPolicy defines evaluation rules for shell execution.
type CommandPolicy struct {
	AllowedCommands          []string
	ApprovalRequiredCommands []string
	BlockedCommands          []string
}

// NewCommandPolicy creates a policy validator from configurations.
func NewCommandPolicy(allowed, approvalReq, blocked []string) *CommandPolicy {
	return &CommandPolicy{
		AllowedCommands:          allowed,
		ApprovalRequiredCommands: approvalReq,
		BlockedCommands:          blocked,
	}
}

// EvaluateCommandWithContext evaluates command security with session awareness.
func (p *CommandPolicy) EvaluateCommandWithContext(ctx context.Context, commandStr string) (PolicyLevel, string) {
	cmd := strings.TrimSpace(commandStr)
	if cmd == "" {
		return PolicyBlocked, "Empty command is not allowed"
	}

	if sc, ok := SessionFromContext(ctx); ok && (sc.Type == "conversation" || sc.ReadOnly) {
		lowerCmd := strings.ToLower(cmd)

		// Check explicit blocked commands and catastrophic patterns first
		for _, blocked := range p.BlockedCommands {
			blocked = strings.TrimSpace(strings.ToLower(blocked))
			if blocked != "" && (lowerCmd == blocked || strings.HasPrefix(lowerCmd, blocked+" ") || strings.Contains(lowerCmd, blocked)) {
				return PolicyBlocked, "Command matches blocked security policy rule: " + blocked
			}
		}
		for _, dp := range dangerousPatterns {
			if strings.Contains(lowerCmd, dp) {
				return PolicyBlocked, "Dangerous system destruction pattern detected"
			}
		}

		// State-changing commands in conversation mode require human approval
		mutatingPrefixes := []string{
			"rm", "mv", "cp", "touch", "mkdir", "rmdir", "chmod", "chown", "chgrp",
			"sed -i", "truncate", "dd", "nano", "vim", "vi", "emacs",
			"git commit", "git push", "git checkout -b", "git branch -d", "git reset",
			"git revert", "git merge", "git rebase", "git clean", "git stash pop",
			"npm install", "npm i", "npm uninstall", "yarn add", "yarn remove", "pnpm add",
			"pip install", "pip uninstall", "brew install", "brew uninstall",
			"go install", "kill", "pkill", "killall",
		}
		for _, mp := range mutatingPrefixes {
			if lowerCmd == mp || strings.HasPrefix(lowerCmd, mp+" ") || containsPipeOrSubcommand(lowerCmd, mp) {
				return PolicyApprovalRequired, fmt.Sprintf("Command %q modifies system state and requires explicit approval", mp)
			}
		}
		if strings.Contains(lowerCmd, " >") || strings.Contains(lowerCmd, " >>") || strings.HasPrefix(lowerCmd, ">") || strings.HasPrefix(lowerCmd, ">>") {
			return PolicyApprovalRequired, "Command redirects output/modifies files and requires explicit approval"
		}
	}

	return p.EvaluateCommand(commandStr)
}

var dangerousPatterns = []string{
	"rm -rf /", "rm -rf ~", "rm -rf /*",
	":(){ :|:& };:",
	"> /dev/sda", "> /dev/disk",
	"mkfs", "dd if=/dev",
	"shutdown", "halt",
}

// EvaluateCommand inspects a shell command and returns its security classification.
func (p *CommandPolicy) EvaluateCommand(commandStr string) (PolicyLevel, string) {
	cmd := strings.TrimSpace(commandStr)
	if cmd == "" {
		return PolicyBlocked, "Empty command is not allowed"
	}

	lowerCmd := strings.ToLower(cmd)

	// 1. Check explicit blocked list
	for _, blocked := range p.BlockedCommands {
		blocked = strings.TrimSpace(strings.ToLower(blocked))
		if blocked != "" && (lowerCmd == blocked || strings.HasPrefix(lowerCmd, blocked+" ") || strings.Contains(lowerCmd, blocked)) {
			return PolicyBlocked, "Command matches blocked security policy rule: " + blocked
		}
	}

	// 2. Additional hardcoded dangerous patterns
	for _, dp := range dangerousPatterns {
		if strings.Contains(lowerCmd, dp) {
			return PolicyBlocked, "Dangerous system destruction pattern detected"
		}
	}

	// 3. Check approval-required commands
	for _, req := range p.ApprovalRequiredCommands {
		req = strings.TrimSpace(strings.ToLower(req))
		if req != "" {
			if lowerCmd == req || strings.HasPrefix(lowerCmd, req+" ") || containsPipeOrSubcommand(lowerCmd, req) {
				return PolicyApprovalRequired, "Command requires explicit user approval: " + req
			}
		}
	}

	// 4. Check safe commands
	for _, safe := range p.AllowedCommands {
		safe = strings.TrimSpace(strings.ToLower(safe))
		if safe != "" {
			if lowerCmd == safe || strings.HasPrefix(lowerCmd, safe+" ") {
				// Ensure no chained dangerous operators like && sudo or ; rm
				if containsDangerousChains(lowerCmd) {
					return PolicyApprovalRequired, "Chained command requires verification"
				}
				return PolicySafe, "Command classified as safe"
			}
		}
	}

	// Default fallback: require approval for any unknown command
	return PolicyApprovalRequired, "Unclassified command requires user approval"
}

func containsPipeOrSubcommand(cmd string, target string) bool {
	parts := strings.FieldsFunc(cmd, func(r rune) bool {
		return r == '|' || r == ';' || r == '&'
	})
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == target || strings.HasPrefix(p, target+" ") {
			return true
		}
	}
	return false
}

func containsDangerousChains(cmd string) bool {
	dangerousTokens := []string{";", "&&", "||", "`", "$("}
	for _, dt := range dangerousTokens {
		if strings.Contains(cmd, dt) {
			return true
		}
	}
	return false
}

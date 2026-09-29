package security_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bob/internal/security"
)

func TestCommandPolicy_EvaluateCommand(t *testing.T) {
	allowed := []string{"pwd", "ls", "git status", "go test"}
	approval := []string{"rm", "chmod", "sudo", "brew install"}
	blocked := []string{"rm -rf /", "mkfs", "dd if=/dev/zero"}

	policy := security.NewCommandPolicy(allowed, approval, blocked)

	tests := []struct {
		cmd      string
		expected security.PolicyLevel
	}{
		{"pwd", security.PolicySafe},
		{"ls -la /tmp", security.PolicySafe},
		{"git status", security.PolicySafe},
		{"go test ./...", security.PolicySafe},
		{"rm file.txt", security.PolicyApprovalRequired},
		{"sudo apt-get install", security.PolicyApprovalRequired},
		{"chmod +x script.sh", security.PolicyApprovalRequired},
		{"rm -rf /", security.PolicyBlocked},
		{"rm -rf / --no-preserve-root", security.PolicyBlocked},
		{"dd if=/dev/zero of=/dev/sda", security.PolicyBlocked},
		{":(){ :|:& };:", security.PolicyBlocked},
		{"unknown_script.sh", security.PolicyApprovalRequired},
		{"ls; sudo rm -rf /", security.PolicyBlocked},
	}

	for _, tt := range tests {
		level, reason := policy.EvaluateCommand(tt.cmd)
		if level != tt.expected {
			t.Errorf("EvaluateCommand(%q) = %v (reason: %s); want %v", tt.cmd, level, reason, tt.expected)
		}
	}
}

func TestPathValidator_ValidatePath(t *testing.T) {
	tempDir := t.TempDir()
	allowedSubdir := filepath.Join(tempDir, "workspace")
	_ = os.MkdirAll(allowedSubdir, 0755)

	forbiddenDir := filepath.Join(tempDir, "private")
	_ = os.MkdirAll(forbiddenDir, 0755)

	validator := security.NewPathValidator([]string{allowedSubdir})

	// 1. Safe inside workspace
	safeFile := filepath.Join(allowedSubdir, "test.txt")
	valid, err := validator.ValidatePath(safeFile)
	if err != nil {
		t.Fatalf("expected safe path to succeed, got %v", err)
	}
	realDir, _ := filepath.EvalSymlinks(filepath.Dir(safeFile))
	expected := filepath.Join(realDir, filepath.Base(safeFile))
	if valid != expected {
		t.Errorf("got %s, want %s", valid, expected)
	}

	// 2. Traversal attempt with ..
	escapePath := filepath.Join(allowedSubdir, "..", "private", "secret.txt")
	_, err = validator.ValidatePath(escapePath)
	if err == nil {
		t.Fatalf("expected .. escape to fail, but succeeded")
	}

	// 3. Absolute path outside allowed
	_, err = validator.ValidatePath(filepath.Join(forbiddenDir, "key.pem"))
	if err == nil {
		t.Fatalf("expected outside path to fail, but succeeded")
	}
}

func TestRedactor_Redact(t *testing.T) {
	token := "bob_super_secret_token_12345"
	redactor := security.NewRedactor(token)

	input := "Connecting with Authorization: Bearer secret_bearer_token_xyz and token=" + token + " and api_key: key12345678"
	redacted := redactor.Redact(input)

	if redacted == input {
		t.Errorf("expected string to be redacted, got unchanged string")
	}
	if strings.Contains(redacted, token) {
		t.Errorf("redacted output still contains secret token: %s", redacted)
	}
}

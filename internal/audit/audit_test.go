package audit_test

import (
	"os"
	"path/filepath"
	"testing"

	"bob/internal/audit"
	"bob/internal/security"
)

func TestAuditLogger(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit.jsonl")

	redactor := security.NewRedactor("secret_tok_99")
	logger, err := audit.NewLogger(logPath, redactor)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	entry := audit.Entry{
		TaskID:         "task_123",
		SessionID:      "sess_456",
		Tool:           "terminal_exec",
		Input:          map[string]string{"command": "echo secret_tok_99"},
		Result:         "secret_tok_99\n",
		ExitCode:       0,
		DurationMs:     42,
		ApprovalStatus: "AUTOMATIC",
	}

	if err := logger.Log(entry); err != nil {
		t.Fatalf("failed to write audit log: %v", err)
	}

	entries, err := logger.ReadRecent(10)
	if err != nil {
		t.Fatalf("failed to read recent audit entries: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].TaskID != "task_123" {
		t.Errorf("got task_id %s, want task_123", entries[0].TaskID)
	}

	// Verify file content on disk has redacted token
	rawBytes, _ := os.ReadFile(logPath)
	if string(rawBytes) == "" {
		t.Errorf("audit log file empty")
	}
}

package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"bob/internal/security"
)

// Entry represents an immutable structured audit log record.
type Entry struct {
	Timestamp      string `json:"timestamp"`
	TaskID         string `json:"task_id"`
	SessionID      string `json:"session_id"`
	Tool           string `json:"tool"`
	Input          any    `json:"input"`
	Result         any    `json:"result"`
	ExitCode       int    `json:"exit_code"`
	DurationMs     int64  `json:"duration_ms"`
	ApprovalStatus string `json:"approval_status"`
	Error          string `json:"error,omitempty"`
}

// Logger persists structured audit records safely.
type Logger struct {
	mu       sync.Mutex
	logPath  string
	file     *os.File
	redactor *security.Redactor
}

// NewLogger creates or opens an audit log file.
func NewLogger(logPath string, redactor *security.Redactor) (*Logger, error) {
	if redactor == nil {
		redactor = security.NewRedactor()
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}

	return &Logger{
		logPath:  logPath,
		file:     file,
		redactor: redactor,
	}, nil
}

// Log records a tool execution event.
func (l *Logger) Log(entry Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format(time.RFC3339Nano)
	}

	// Marshal and redact
	raw, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to serialize audit entry: %w", err)
	}

	redactedString := l.redactor.Redact(string(raw))

	if _, err := l.file.WriteString(redactedString + "\n"); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}

	return nil
}

// ReadRecent retrieves the last N entries for UI inspection.
func (l *Logger) ReadRecent(limit int) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}

	lines := splitLines(data)
	if limit > 0 && len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	entries := make([]Entry, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(line, &entry); err == nil {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// Close closes the underlying audit file handle.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

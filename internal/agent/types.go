package agent

import (
	"context"
	"time"

	"bob/internal/tools/registry"
)

// TaskStatus defines current execution state of a user request.
type TaskStatus string

const (
	StatusQueued             TaskStatus = "queued"
	StatusRunning            TaskStatus = "running"
	StatusWaitingForApproval TaskStatus = "waiting_for_approval"
	StatusCompleted          TaskStatus = "completed"
	StatusFailed             TaskStatus = "failed"
	StatusCancelled          TaskStatus = "cancelled"
)

// EventType defines streaming event types emitted during execution.
type EventType string

const (
	EventAgentThinking         EventType = "agent.thinking"
	EventToolStarted           EventType = "tool.started"
	EventToolApprovalRequired  EventType = "tool.approval_required"
	EventToolCompleted         EventType = "tool.completed"
	EventAgentMessage          EventType = "agent.message"
	EventAgentError            EventType = "agent.error"
	EventTaskStatusChanged     EventType = "task.status"
	EventTaskCompleted         EventType = "task.completed"
)

// Event is emitted in real time to WebSocket/SSE clients.
type Event struct {
	Type       EventType           `json:"type"`
	TaskID     string              `json:"task_id"`
	SessionID  string              `json:"session_id,omitempty"`
	Timestamp  string              `json:"timestamp"`
	Message    string              `json:"message,omitempty"`
	Tool       string              `json:"tool,omitempty"`
	Input      any                 `json:"input,omitempty"`
	Output     string              `json:"output,omitempty"`
	ToolResult *registry.ToolResult `json:"tool_result,omitempty"`
	DurationMs int64               `json:"duration_ms,omitempty"`
	Status     TaskStatus          `json:"status,omitempty"`
	Error      string              `json:"error,omitempty"`
}

// PendingApproval holds state for a tool execution awaiting human confirmation.
type PendingApproval struct {
	ToolName   string          `json:"tool_name"`
	Input      any             `json:"input"`
	Reason     string          `json:"reason"`
	ResponseCh chan bool       `json:"-"`
}

// Task represents a run request from a user.
type Task struct {
	ID              string           `json:"id"`
	SessionID       string           `json:"session_id"`
	Prompt          string           `json:"prompt"`
	Status          TaskStatus       `json:"status"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
	Result          string           `json:"result,omitempty"`
	Error           string           `json:"error,omitempty"`
	Events          []Event          `json:"events"`
	PendingApproval *PendingApproval `json:"pending_approval,omitempty"`
	cancelFunc      context.CancelFunc
}

type AgentConfig struct {
	MaxSteps       int
	MaxToolCalls   int
	TimeoutSeconds int
}

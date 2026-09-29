package llm

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	ErrLLMUnavailable = errors.New("local LLM service unavailable")
	ErrMaxTokens      = errors.New("maximum token limit reached")
	ErrInvalidModel   = errors.New("invalid or empty model specified")
)

// Role defines the participant role in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message represents a single message in the LLM conversation.
type Message struct {
	Role       Role        `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function name and arguments.
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolDefinition defines the schema of a tool presented to the LLM.
type ToolDefinition struct {
	Type     string             `json:"type"` // "function"
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// ChatRequest encapsulates all parameters sent to the LLM.
type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
}

// ChatResponse contains the LLM's response.
type ChatResponse struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
	Model        string  `json:"model"`
	TotalTokens  int     `json:"total_tokens,omitempty"`
}

// StreamEvent represents a chunk emitted during streaming inference.
type StreamEvent struct {
	Delta        string     `json:"delta,omitempty"`
	ToolCall     *ToolCall  `json:"tool_call,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
	Done         bool       `json:"done"`
	Error        error      `json:"error,omitempty"`
}

// LLM is the core abstraction for local and alternative language models.
type LLM interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
	Stream(ctx context.Context, request ChatRequest) (<-chan StreamEvent, error)
	Name() string
}

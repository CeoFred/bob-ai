package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"bob/internal/llm"
)

// ToolResult represents the standardized outcome of a tool execution.
type ToolResult struct {
	Success    bool   `json:"success"`
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	Data       any    `json:"data,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// Tool defines the contract for all tools in Bob.
type Tool interface {
	Name() string
	Description() string
	InputSchema() any
	Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}

// Registry manages the set of available tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry initializes an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool must have a non-empty name")
	}
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}

	r.tools[name] = tool
	return nil
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// ToToolDefinitions generates LLM-compatible tool definitions.
func (r *Registry) ToToolDefinitions() []llm.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]llm.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.InputSchema(),
			},
		})
	}
	return defs
}

// Execute safely runs a tool by name with input JSON and duration tracking.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (ToolResult, error) {
	tool, exists := r.Get(name)
	if !exists {
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("tool not found: %s", name),
		}, fmt.Errorf("tool not found: %s", name)
	}

	start := time.Now()
	res, err := tool.Execute(ctx, input)
	res.DurationMs = time.Since(start).Milliseconds()

	if err != nil && res.Error == "" {
		res.Error = err.Error()
	}

	return res, err
}

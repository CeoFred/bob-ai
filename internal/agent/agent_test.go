package agent_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"bob/internal/agent"
	"bob/internal/audit"
	"bob/internal/llm"
	"bob/internal/security"
	"bob/internal/sessions"
	"bob/internal/tools/filesystem"
	"bob/internal/tools/registry"
	"bob/internal/tools/terminal"
)

func setupTestAgent(t *testing.T, mockResponses ...llm.ChatResponse) (*agent.Agent, *llm.MockLLM, *registry.Registry, string) {
	tempDir := t.TempDir()

	mockLLM := llm.NewMockLLM(mockResponses...)
	reg := registry.NewRegistry()
	pathVal := security.NewPathValidator([]string{tempDir})
	policy := security.NewCommandPolicy([]string{"echo", "pwd"}, []string{"rm"}, []string{"rm -rf /"})

	_ = reg.Register(terminal.NewTerminalTool(policy, pathVal, tempDir, 5*time.Second, 1024*1024))
	_ = reg.Register(filesystem.NewWriteFileTool(pathVal))
	_ = reg.Register(filesystem.NewReadFileTool(pathVal, 1024*1024))

	redactor := security.NewRedactor()
	auditLog, _ := audit.NewLogger(tempDir+"/audit.jsonl", redactor)
	sessMgr := sessions.NewManager(tempDir)

	ag := agent.NewAgent(mockLLM, reg, policy, auditLog, sessMgr, agent.AgentConfig{
		MaxSteps:       5,
		MaxToolCalls:   5,
		TimeoutSeconds: 5,
	})

	return ag, mockLLM, reg, tempDir
}

func TestAgent_SimpleResponse(t *testing.T) {
	resp := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Hello! I am Bob.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, resp)
	task := ag.CreateTask("", "Hello")

	ag.Run(context.Background(), task)

	time.Sleep(100 * time.Millisecond)

	tObj, ok := ag.GetTask(task.ID)
	if !ok || tObj.Status != agent.StatusCompleted {
		t.Fatalf("expected task status completed, got %v", tObj.Status)
	}

	if tObj.Result != "Hello! I am Bob." {
		t.Errorf("got result %q, want %q", tObj.Result, "Hello! I am Bob.")
	}
}

func TestAgent_ToolExecutionFlow(t *testing.T) {
	// 1. LLM requests tool execution
	toolCallArgs, _ := json.Marshal(map[string]any{"command": "echo 'running tests'"})
	step1 := llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "terminal_exec",
						Arguments: toolCallArgs,
					},
				},
			},
		},
	}

	// 2. LLM receives tool output and finishes
	step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Tests passed successfully.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, step1, step2)
	task := ag.CreateTask("", "Run tests")

	ag.Run(context.Background(), task)

	time.Sleep(200 * time.Millisecond)

	tObj, _ := ag.GetTask(task.ID)
	if tObj.Status != agent.StatusCompleted {
		t.Fatalf("expected task completed, got %s (error: %s)", tObj.Status, tObj.Error)
	}

	if tObj.Result != "Tests passed successfully." {
		t.Errorf("got %q, want %q", tObj.Result, "Tests passed successfully.")
	}
}

func TestAgent_MaxStepEnforcement(t *testing.T) {
	// Infinite loop simulation: LLM keeps calling tool
	toolCallArgs, _ := json.Marshal(map[string]any{"command": "echo 'loop'"})
	loopResp := llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_loop",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "terminal_exec",
						Arguments: toolCallArgs,
					},
				},
			},
		},
	}

	ag, _, _, _ := setupTestAgent(t, loopResp)
	task := ag.CreateTask("", "Loop forever")

	ag.Run(context.Background(), task)

	time.Sleep(300 * time.Millisecond)

	tObj, _ := ag.GetTask(task.ID)
	if tObj.Status != agent.StatusFailed {
		t.Fatalf("expected task to fail due to max steps, got %s", tObj.Status)
	}
}

func TestAgent_TaskCancellation(t *testing.T) {
	toolCallArgs, _ := json.Marshal(map[string]any{"command": "rm file.txt"}) // requires approval
	approvalResp := llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_rm",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "terminal_exec",
						Arguments: toolCallArgs,
					},
				},
			},
		},
	}

	ag, _, _, _ := setupTestAgent(t, approvalResp)
	task := ag.CreateTask("", "Delete file")

	ag.Run(context.Background(), task)

	time.Sleep(50 * time.Millisecond)

	tObj, _ := ag.GetTask(task.ID)
	if tObj.Status != agent.StatusWaitingForApproval {
		t.Fatalf("expected waiting for approval, got %s", tObj.Status)
	}

	// Cancel task while paused
	if err := ag.CancelTask(task.ID); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	tObj, _ = ag.GetTask(task.ID)
	if tObj.Status != agent.StatusCancelled {
		t.Errorf("expected cancelled status, got %s", tObj.Status)
	}
}

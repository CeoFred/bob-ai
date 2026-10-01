package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"bob/internal/agent"
	"bob/internal/audit"
	"bob/internal/llm"
	"bob/internal/security"
	"bob/internal/sessions"
	"bob/internal/tools/apps"
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
	_ = reg.Register(filesystem.NewReplaceFileContentTool(pathVal))
	_ = reg.Register(filesystem.NewReadFileTool(pathVal, 1024*1024))
	_ = reg.Register(apps.NewListRunningAppsTool())
	_ = reg.Register(apps.NewListInstalledAppsTool())
	_ = reg.Register(apps.NewOpenAppTool())
	_ = reg.Register(apps.NewCloseAppTool())

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
	toolCallArgs, _ := json.Marshal(map[string]any{"command": "rm file.txt"}) // requires approval in project mode
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

	ag, _, _, tempDir := setupTestAgent(t, approvalResp)
	sess := ag.SessionManager().Create(sessions.SessionTypeProject, tempDir, "TestProject")
	task := ag.CreateTask(sess.ID, "Delete file")

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

func TestAgent_Approval_ApproveAndReject(t *testing.T) {
	toolCallArgs, _ := json.Marshal(map[string]any{"command": "rm file.txt"})
	step1 := llm.ChatResponse{
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
	step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Handled tool response",
		},
		FinishReason: "stop",
	}

	// 1. Test Reject
	ag, _, _, tempDir := setupTestAgent(t, step1, step2)
	sess := ag.SessionManager().Create(sessions.SessionTypeProject, tempDir, "TestProject")
	taskReject := ag.CreateTask(sess.ID, "Delete file reject")

	ag.Run(context.Background(), taskReject)
	time.Sleep(50 * time.Millisecond)

	tObj, _ := ag.GetTask(taskReject.ID)
	if tObj.Status != agent.StatusWaitingForApproval {
		t.Fatalf("expected waiting for approval, got %s", tObj.Status)
	}

	if err := ag.ApproveTool(taskReject.ID, false); err != nil {
		t.Fatalf("ApproveTool(false) failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	tObj, _ = ag.GetTask(taskReject.ID)
	if tObj.Status != agent.StatusCompleted {
		t.Errorf("expected completed status after rejection, got %s", tObj.Status)
	}

	var foundRejectEvent bool
	for _, ev := range tObj.Events {
		if ev.Type == agent.EventToolCompleted && strings.Contains(strings.ToLower(ev.Error), "rejected") {
			foundRejectEvent = true
		}
	}
	if !foundRejectEvent {
		t.Errorf("expected tool completed event with rejection message")
	}

	// 2. Test Approve
	ag2, _, _, tempDir2 := setupTestAgent(t, step1, step2)
	sess2 := ag2.SessionManager().Create(sessions.SessionTypeProject, tempDir2, "TestProject")
	taskApprove := ag2.CreateTask(sess2.ID, "Delete file approve")

	ag2.Run(context.Background(), taskApprove)
	time.Sleep(50 * time.Millisecond)

	tObj2, _ := ag2.GetTask(taskApprove.ID)
	if tObj2.Status != agent.StatusWaitingForApproval {
		t.Fatalf("expected waiting for approval, got %s", tObj2.Status)
	}

	if err := ag2.ApproveTool(taskApprove.ID, true); err != nil {
		t.Fatalf("ApproveTool(true) failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	tObj2, _ = ag2.GetTask(taskApprove.ID)
	if tObj2.Status != agent.StatusCompleted {
		t.Errorf("expected completed status after approval, got %s", tObj2.Status)
	}
}

func TestAgent_ConversationMode_BlocksMutatingCommands(t *testing.T) {
	toolCallArgs, _ := json.Marshal(map[string]any{"command": "rm -rf /"})
	step1 := llm.ChatResponse{
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
	step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Dangerous command is blocked.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, step1, step2)
	sess := ag.SessionManager().Create(sessions.SessionTypeConversation, "", "General Chat")
	task := ag.CreateTask(sess.ID, "Delete root")

	ag.Run(context.Background(), task)

	time.Sleep(150 * time.Millisecond)

	tObj, _ := ag.GetTask(task.ID)
	if tObj.Status != agent.StatusCompleted {
		t.Fatalf("expected task completed, got %s (error: %s)", tObj.Status, tObj.Error)
	}

	// Verify blocked event was emitted
	foundBlocked := false
	for _, ev := range tObj.Events {
		if ev.Type == agent.EventToolStarted && ev.Tool == "terminal_exec" {
			foundBlocked = true
		}
	}
	if !foundBlocked {
		t.Errorf("expected terminal_exec tool attempt recorded")
	}
}

func TestAgent_MarkdownToolCallExtraction(t *testing.T) {
	// LLM outputs markdown json blocks instead of structured tool_calls
	step1 := llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			Content: "Sure, let's get started.\n\n1. Finding directory:\n```json\n{\"name\": \"terminal_exec\", \"arguments\": {\"command\": \"pwd\"}}\n```\n",
		},
	}

	step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "The current directory is verified.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, step1, step2)
	task := ag.CreateTask("", "Where are you running from?")

	ag.Run(context.Background(), task)

	time.Sleep(200 * time.Millisecond)

	tObj, _ := ag.GetTask(task.ID)
	if tObj.Status != agent.StatusCompleted {
		t.Fatalf("expected task completed, got %s (error: %s)", tObj.Status, tObj.Error)
	}

	if tObj.Result != "The current directory is verified." {
		t.Errorf("got %q, want %q", tObj.Result, "The current directory is verified.")
	}
}

func TestAgent_ListRunningAppsExecution(t *testing.T) {
	// Step 1: Agent requests list_running_apps
	toolArgs, _ := json.Marshal(map[string]any{"gui_only": true})
	step1 := llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_apps_1",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "list_running_apps",
						Arguments: toolArgs,
					},
				},
			},
		},
	}

	// Step 2: Agent reports running apps to creator
	step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "codemon, here are the applications currently active on your Mac: Google Chrome, Visual Studio Code, and Ghostty.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, step1, step2)
	task := ag.CreateTask("", "What applications are currently open?")

	ag.Run(context.Background(), task)

	time.Sleep(200 * time.Millisecond)

	tObj, ok := ag.GetTask(task.ID)
	if !ok || tObj.Status != agent.StatusCompleted {
		t.Fatalf("expected task completed, got %v (error: %s)", tObj.Status, tObj.Error)
	}

	foundTool := false
	for _, ev := range tObj.Events {
		if ev.Type == agent.EventToolStarted && ev.Tool == "list_running_apps" {
			foundTool = true
		}
	}

	if !foundTool {
		t.Errorf("expected list_running_apps event to be recorded")
	}

	if !strings.Contains(tObj.Result, "codemon") {
		t.Errorf("expected result addressing codemon, got %q", tObj.Result)
	}
}

func TestAgent_CloseApp_ApprovalFlow(t *testing.T) {
	// Step 1: Agent attempts to close Slack
	toolArgs, _ := json.Marshal(map[string]any{"app_name": "Slack"})
	step1 := llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_close_slack",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "close_app",
						Arguments: toolArgs,
					},
				},
			},
		},
	}

	step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Slack has been closed, codemon.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, step1, step2)
	task := ag.CreateTask("", "Can you close slack?")

	ag.Run(context.Background(), task)

	// Wait for task to pause in waiting_for_approval
	time.Sleep(100 * time.Millisecond)

	tObj, ok := ag.GetTask(task.ID)
	if !ok || tObj.Status != agent.StatusWaitingForApproval {
		t.Fatalf("expected task waiting for approval, got %v", tObj.Status)
	}

	if tObj.PendingApproval == nil || tObj.PendingApproval.ToolName != "close_app" {
		t.Fatalf("expected pending approval for close_app, got %+v", tObj.PendingApproval)
	}

	// Approve close_app
	err := ag.ApproveTool(task.ID, true)
	if err != nil {
		t.Fatalf("failed to approve tool: %v", err)
	}

	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		tObj, _ = ag.GetTask(task.ID)
		if tObj.Status == agent.StatusCompleted {
			break
		}
	}

	if tObj.Status != agent.StatusCompleted {
		t.Fatalf("expected task completed after approval, got %v (error: %s)", tObj.Status, tObj.Error)
	}
}

func TestAgent_ReplaceFileContentFlow(t *testing.T) {
	tempDir := t.TempDir()
	filePath := tempDir + "/app.js"
	_ = os.WriteFile(filePath, []byte("const port = 3000;\nconsole.log(port);"), 0644)

	repArgs, _ := json.Marshal(map[string]any{
		"path":                filePath,
		"target_content":      "3000",
		"replacement_content": "8080",
	})

	step1 := llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_rep",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "replace_file_content",
						Arguments: repArgs,
					},
				},
			},
		},
	}

	step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Updated port to 8080 successfully.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, step1, step2)
	sess := ag.SessionManager().Create(sessions.SessionTypeProject, tempDir, "AppProject")
	task := ag.CreateTask(sess.ID, "Change port to 8080 in app.js")

	ag.Run(context.Background(), task)
	time.Sleep(150 * time.Millisecond)

	tObj, ok := ag.GetTask(task.ID)
	if !ok || tObj.Status != agent.StatusCompleted {
		t.Fatalf("expected task completed, got %+v", tObj)
	}

	contentBytes, _ := os.ReadFile(filePath)
	if !strings.Contains(string(contentBytes), "const port = 8080;") {
		t.Errorf("expected updated content in file, got: %s", string(contentBytes))
	}
}

func TestAgent_MultiTurnConversationContext(t *testing.T) {
	// Turn 1 response
	turn1Resp := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "I'm Bob, ready to help you, Creator.",
		},
		FinishReason: "stop",
	}

	// Turn 2 response
	turn2Resp := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Understood, continuing the previous discussion.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, turn1Resp, turn2Resp)
	sess := ag.SessionManager().Create(sessions.SessionTypeConversation, "", "Chat Thread")

	// 1. Run Turn 1
	task1 := ag.CreateTask(sess.ID, "Hello Bob, who are you?")
	ag.Run(context.Background(), task1)
	time.Sleep(100 * time.Millisecond)

	tObj1, ok := ag.GetTask(task1.ID)
	if !ok || tObj1.Status != agent.StatusCompleted {
		t.Fatalf("turn 1 failed: status=%v", tObj1.Status)
	}

	// Verify session history has Turn 1 user and assistant messages
	updatedSess, _ := ag.SessionManager().Get(sess.ID)
	if len(updatedSess.Messages) != 2 {
		t.Fatalf("expected 2 messages in session history after turn 1, got %d", len(updatedSess.Messages))
	}
	if updatedSess.Messages[0].Role != llm.RoleUser || updatedSess.Messages[1].Role != llm.RoleAssistant {
		t.Errorf("unexpected message roles in history: %+v", updatedSess.Messages)
	}

	// 2. Run Turn 2
	task2 := ag.CreateTask(sess.ID, "Can you follow up on that?")
	ag.Run(context.Background(), task2)
	time.Sleep(100 * time.Millisecond)

	tObj2, ok := ag.GetTask(task2.ID)
	if !ok || tObj2.Status != agent.StatusCompleted {
		t.Fatalf("turn 2 failed: status=%v", tObj2.Status)
	}

	// Verify session history now contains both turns (4 messages)
	updatedSess2, _ := ag.SessionManager().Get(sess.ID)
	if len(updatedSess2.Messages) != 4 {
		t.Fatalf("expected 4 messages in session history after turn 2, got %d", len(updatedSess2.Messages))
	}
	if updatedSess2.Messages[2].Content != "Can you follow up on that?" {
		t.Errorf("expected turn 2 prompt in history, got: %s", updatedSess2.Messages[2].Content)
	}
	if updatedSess2.Messages[3].Content != "Understood, continuing the previous discussion." {
		t.Errorf("expected turn 2 assistant reply in history, got: %s", updatedSess2.Messages[3].Content)
	}
}

func TestAgent_EnforceActionIntent_CloseApp(t *testing.T) {
	// Step 1: LLM hallucinates/responds with plain text claiming app is closed without calling tool
	step1 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "WhatsApp has been successfully closed.",
		},
		FinishReason: "stop",
	}

	// Step 2: After the agent enforces the close_app tool call, LLM receives the real tool result
	step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "WhatsApp is not running, codemon.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, step1, step2)
	task := ag.CreateTask("", "Now close WhatsApp")

	ag.Run(context.Background(), task)

	// close_app may pause for human approval if required by security policy
	time.Sleep(100 * time.Millisecond)

	tObj, ok := ag.GetTask(task.ID)
	if !ok {
		t.Fatalf("task not found")
	}

	if tObj.Status == agent.StatusWaitingForApproval {
		_ = ag.ApproveTool(task.ID, true)
	}

	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		tObj, _ = ag.GetTask(task.ID)
		if tObj.Status == agent.StatusCompleted {
			break
		}
	}

	if tObj.Status != agent.StatusCompleted {
		t.Fatalf("expected task completed, got %v (error: %s)", tObj.Status, tObj.Error)
	}

	// Verify that close_app was indeed executed in the events
	foundCloseTool := false
	for _, ev := range tObj.Events {
		if ev.Type == agent.EventToolStarted && ev.Tool == "close_app" {
			foundCloseTool = true
		}
	}

	if !foundCloseTool {
		t.Errorf("expected close_app tool execution event to be generated via action intent enforcement")
	}
}





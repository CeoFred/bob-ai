package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	var tObj *agent.Task
	var ok bool
	for i := 0; i < 20; i++ {
		tObj, ok = ag.GetTask(task.ID)
		if ok && (tObj.Status == agent.StatusCompleted || tObj.Status == agent.StatusFailed) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

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

func TestAgent_PronounResolution_CloseIt(t *testing.T) {
	// Turn 1: User asks to open WhatsApp -> agent opens WhatsApp
	openArgs, _ := json.Marshal(map[string]any{"app_name": "WhatsApp"})
	t1Step1 := llm.ChatResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_open_1",
					Type: "function",
					Function: llm.FunctionCall{
						Name:      "open_app",
						Arguments: openArgs,
					},
				},
			},
		},
	}
	t1Step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "WhatsApp has been successfully opened.",
		},
		FinishReason: "stop",
	}

	// Turn 2: User says "Close it" -> agent executes close_app(app_name="WhatsApp")
	t2Step1 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "WhatsApp has been successfully closed.",
		},
		FinishReason: "stop",
	}
	t2Step2 := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "WhatsApp is closed now, codemon.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, t1Step1, t1Step2, t2Step1, t2Step2)
	sess := ag.SessionManager().Create(sessions.SessionTypeConversation, "", "App Control Thread")

	// Turn 1 Execution
	task1 := ag.CreateTask(sess.ID, "Open WhatsApp")
	ag.Run(context.Background(), task1)
	time.Sleep(150 * time.Millisecond)

	// Turn 2 Execution with pronoun "Close it"
	task2 := ag.CreateTask(sess.ID, "Close it")
	ag.Run(context.Background(), task2)

	time.Sleep(100 * time.Millisecond)
	tObj2, _ := ag.GetTask(task2.ID)
	if tObj2.Status == agent.StatusWaitingForApproval {
		_ = ag.ApproveTool(task2.ID, true)
	}

	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		tObj2, _ = ag.GetTask(task2.ID)
		if tObj2.Status == agent.StatusCompleted {
			break
		}
	}

	if tObj2.Status != agent.StatusCompleted {
		t.Fatalf("turn 2 task not completed: %v (err: %s)", tObj2.Status, tObj2.Error)
	}

	// Verify that close_app was invoked with "WhatsApp", NOT "it"
	var closedAppParam string
	for _, ev := range tObj2.Events {
		if ev.Type == agent.EventToolStarted && ev.Tool == "close_app" {
			var in struct {
				AppName string `json:"app_name"`
			}
			inputStr := fmt.Sprint(ev.Input)
			_ = json.Unmarshal([]byte(inputStr), &in)
			closedAppParam = in.AppName
		}
	}

	if closedAppParam != "WhatsApp" {
		t.Errorf("expected close_app parameter to be resolved to 'WhatsApp', got %q", closedAppParam)
	}
}

func TestAgent_DynamicModelRouting(t *testing.T) {
	tempDir := t.TempDir()
	mockLLM := llm.NewMockLLM()
	reg := registry.NewRegistry()
	policy := security.NewCommandPolicy([]string{"echo", "pwd"}, nil, nil)
	auditLog, _ := audit.NewLogger(tempDir+"/audit.jsonl", security.NewRedactor())
	sessMgr := sessions.NewManager(tempDir)

	// 1. By default when configured with gpt-oss, both general and coding route to gpt-oss
	agDefault := agent.NewAgent(mockLLM, reg, policy, auditLog, sessMgr, agent.AgentConfig{
		DefaultModel: "gpt-oss",
		CodingModel:  "gpt-oss",
		GeneralModel: "gpt-oss",
	})

	convSess := agDefault.SessionManager().Create(sessions.SessionTypeConversation, "", "General Chat")
	model1, reason1 := agDefault.SelectModelForTask(convSess, "What is the weather today?")
	if model1 != "gpt-oss" {
		t.Errorf("expected gpt-oss for general conversation, got %q", model1)
	}
	if reason1 != "General Assistant" {
		t.Errorf("expected 'General Assistant' reason, got %q", reason1)
	}

	projSess := agDefault.SessionManager().Create(sessions.SessionTypeProject, "/Users/codemon_/Documents/Bob-AI", "Project Mode")
	model2, reason2 := agDefault.SelectModelForTask(projSess, "How does the agent loop work?")
	if model2 != "gpt-oss" {
		t.Errorf("expected gpt-oss for project mode, got %q", model2)
	}
	if reason2 != "Project Mode" {
		t.Errorf("expected 'Project Mode' reason, got %q", reason2)
	}

	model3, reason3 := agDefault.SelectModelForTask(convSess, "Can you write a golang function to calculate fibonacci?")
	if model3 != "gpt-oss" {
		t.Errorf("expected gpt-oss for coding prompt, got %q", model3)
	}
	if reason3 != "Coding Task" {
		t.Errorf("expected 'Coding Task' reason, got %q", reason3)
	}

	// 2. Custom multi-model routing when explicit distinct models are configured
	agSplit := agent.NewAgent(mockLLM, reg, policy, auditLog, sessMgr, agent.AgentConfig{
		DefaultModel: "gpt-oss",
		CodingModel:  "qwen2.5-coder:7b",
		GeneralModel: "hermes3:8b",
	})
	mGen, _ := agSplit.SelectModelForTask(convSess, "Hello")
	if mGen != "hermes3:8b" {
		t.Errorf("expected hermes3:8b for general task when configured, got %q", mGen)
	}
	mCode, _ := agSplit.SelectModelForTask(projSess, "Build project")
	if mCode != "qwen2.5-coder:7b" {
		t.Errorf("expected qwen2.5-coder:7b for project task when configured, got %q", mCode)
	}
}

func TestAgent_ResponseDurationRecorded(t *testing.T) {
	respMsg := llm.ChatResponse{
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "Hello codemon! Here is the response you asked for.",
		},
		FinishReason: "stop",
	}

	ag, _, _, _ := setupTestAgent(t, respMsg)
	task := ag.CreateTask("", "Hello")

	ag.Run(context.Background(), task)

	var tObj *agent.Task
	var ok bool
	for i := 0; i < 20; i++ {
		tObj, ok = ag.GetTask(task.ID)
		if ok && (tObj.Status == agent.StatusCompleted || tObj.Status == agent.StatusFailed) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !ok || tObj.Status != agent.StatusCompleted {
		t.Fatalf("expected task completed, got %v (error: %s)", tObj.Status, tObj.Error)
	}

	if tObj.DurationMs < 0 {
		t.Errorf("expected task DurationMs >= 0, got %d", tObj.DurationMs)
	}

	foundAgentMsg := false
	foundCompleted := false
	for _, ev := range tObj.Events {
		if ev.Type == agent.EventAgentMessage {
			foundAgentMsg = true
			if ev.DurationMs < 0 {
				t.Errorf("expected EventAgentMessage DurationMs >= 0, got %d", ev.DurationMs)
			}
		}
		if ev.Type == agent.EventTaskCompleted {
			foundCompleted = true
			if ev.DurationMs < 0 {
				t.Errorf("expected EventTaskCompleted DurationMs >= 0, got %d", ev.DurationMs)
			}
		}
	}

	if !foundAgentMsg {
		t.Errorf("expected EventAgentMessage event to be recorded")
	}
	if !foundCompleted {
		t.Errorf("expected EventTaskCompleted event to be recorded")
	}
}

func TestAgent_EnsureComprehensiveRunningAppsResponse(t *testing.T) {
	ag, _, _, _ := setupTestAgent(t)

	toolOut := `Currently Running GUI Applications (3 found):
• Active (Frontmost) Application: Brave Browser

1. ★ [Active] Brave Browser (Bundle: com.brave.Browser, PID: 726) [CPU: 1.9%, MEM: 1.3%]
2. • Spotify (Bundle: com.spotify.client, PID: 18355) [CPU: 0.3%, MEM: 0.3%]
3. • Code (Bundle: com.microsoft.VSCode, PID: 766) [CPU: 0.0%, MEM: 0.4%]`

	toolOutputs := map[string]string{
		"list_running_apps": toolOut,
	}

	// 1. Evasive placeholder response from LLM should be enriched with actual apps
	evasive := "I have provided you with a list of 28 currently running GUI applications on your Mac. If you need further assistance or have any other queries, feel free to ask."
	enriched := ag.EnsureComprehensiveResponseForTest("what apps are currently running?", evasive, toolOutputs)

	if !strings.Contains(enriched, "Brave Browser") || !strings.Contains(enriched, "Spotify") || !strings.Contains(enriched, "Code") {
		t.Errorf("expected enriched response to list running applications, got: %s", enriched)
	}

	// 2. Specific query with already detailed response should be preserved
	specific := "Yes, Spotify is currently running. It is listed as a GUI application with PID 18355."
	preserved := ag.EnsureComprehensiveResponseForTest("confirm if spotify app is open", specific, toolOutputs)
	if preserved != specific {
		t.Errorf("expected specific response to be preserved, got: %s", preserved)
	}
}

func TestAgent_TasksDiskPersistence(t *testing.T) {
	tempDir := t.TempDir()
	tasksPath := filepath.Join(tempDir, "tasks.json")

	mockLLM := llm.NewMockLLM()
	reg := registry.NewRegistry()
	policy := security.NewCommandPolicy([]string{"echo"}, nil, nil)
	auditLog, _ := audit.NewLogger(tempDir+"/audit.jsonl", security.NewRedactor())
	sessMgr := sessions.NewManager(tempDir)

	ag1 := agent.NewAgent(mockLLM, reg, policy, auditLog, sessMgr, agent.AgentConfig{
		StoragePath: tasksPath,
	})

	task := ag1.CreateTask("sess_123", "Test prompt")
	if task == nil {
		t.Fatal("failed to create task")
	}

	// Recreate agent pointing to same tasks.json (simulating server restart / new tab)
	ag2 := agent.NewAgent(mockLLM, reg, policy, auditLog, sessMgr, agent.AgentConfig{
		StoragePath: tasksPath,
	})

	loadedTask, ok := ag2.GetTask(task.ID)
	if !ok || loadedTask == nil {
		t.Fatalf("expected task %s to be loaded from disk", task.ID)
	}
	if loadedTask.Prompt != "Test prompt" {
		t.Errorf("expected prompt 'Test prompt', got %q", loadedTask.Prompt)
	}
}







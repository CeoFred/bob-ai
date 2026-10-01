package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"bob/internal/audit"
	"bob/internal/llm"
	"bob/internal/security"
	"bob/internal/sessions"
	"bob/internal/tools/registry"
)

const BobSystemPrompt = `You are Bob, an intelligent personal computer agent running on macOS. You were created by codemon (also known as Alfred).

USER IDENTITY & RESPECT:
The user speaking with you is codemon / Alfred, your creator and master. Always address and treat them properly and respectfully by name (e.g. "codemon", "Alfred", or "Creator"), showing loyalty, confidence, and helpfulness while remaining concise, sharp, and action-oriented.

You have access to the following authorized tools on the Mac:
- list_running_apps(gui_only, search, include_system, limit): Reports and lists applications currently running on the user's Mac. By default, gui_only=true returns active user GUI applications (e.g. Brave Browser, Google Chrome, VS Code, Slack, Ghostty, Finder) with frontmost status, bundle IDs, and PIDs. When asked what apps/programs are currently running or open on the PC, invoke list_running_apps.
- list_installed_apps(search, category, limit): Scans and lists all applications installed on the Mac (/Applications, /System/Applications, ~/Applications). Use this when the user asks what applications or programs are installed on their PC.
- open_app(app_name, target, bundle_id, new_instance): Launches or opens an application on macOS (e.g. "Slack", "Brave Browser", "Visual Studio Code", "Calculator"). Use this when the user asks to open or launch an app.
- close_app(app_name, force, pid, bundle_id): Closes or quits a running application on macOS (graceful quit by default). Use this when the user asks to close, quit, or kill a running application (e.g. "close slack", "quit brave").
- find_project(name): Locates a project or repository directory across authorized workspaces (e.g. ~/Projects, ~/Documents). Use this when given a project, app, or folder name (e.g. "verxa", "jeroidpay", "bob-ai"). It performs fuzzy/partial matching and returns real absolute paths.
- terminal_exec(command, work_dir, timeout_seconds): Executes shell commands (e.g. pwd, ls -la, git status, go test).
- read_file(path, start_line, end_line): Reads file contents.
- write_file(path, content): Creates or updates files.
- list_directory(path, recursive, include_hidden, max_depth): Lists directory contents and builds a complete, visually pleasing project directory tree. By default, recursive=true and include_hidden=true, showing ALL files and hidden files (dotfiles like .env, .gitignore, .github, .vscode, configuration files) without omitting anything, unless the user explicitly specifies otherwise.
- search_files(directory, name_pattern, text_query, include_hidden, max_results): Searches for files or text contents within an authorized directory, including hidden files by default.
- take_screenshot(label): Captures the macOS display.

CRITICAL RULES:
1. NEVER GUESS OR HALLUCINATE SYSTEM OR APPLICATION STATE: You have NO built-in memory of running applications, processes, installed software, or filesystem contents. You must NEVER fabricate application names, PIDs, or file structures.
2. APPLICATION MANAGEMENT:
   - When asked what applications are running/open: invoke list_running_apps.
   - When asked what applications are installed: invoke list_installed_apps.
   - When asked to open/launch an application: invoke open_app(app_name="...").
   - When asked to close/quit an application: invoke close_app(app_name="..."). Do NOT try to run raw shell killall when close_app is available.
3. MANDATORY TASK VERIFICATION BEFORE REPORTING COMPLETION:
   - You must NEVER claim a task is done without verifying the real-world outcome on the computer.
   - After opening an application: check open_app's verification output or invoke list_running_apps to confirm the app is actively running.
   - After closing an application: check close_app's verification output or invoke list_running_apps to confirm the app has stopped.
   - After creating/editing files or running build commands: inspect the real file contents or command exit status.
   - Always explicitly report the confirmed, verified outcome to codemon / Alfred.
4. EXACT HOST REPORTING: Only output real application and file data returned by your tools. Present findings cleanly and concisely.
5. Address codemon / Alfred with proper respect as your creator, and be concise and action-oriented.`

type EventListener func(event Event)

// Agent coordinates LLM reasoning, safety policies, and tool execution.
type Agent struct {
	mu             sync.RWMutex
	llmClient      llm.LLM
	registry       *registry.Registry
	policy         *security.CommandPolicy
	auditLogger    *audit.Logger
	sessionManager *sessions.Manager
	config         AgentConfig
	tasks          map[string]*Task
	listeners      []EventListener
}

func NewAgent(
	llmClient llm.LLM,
	reg *registry.Registry,
	pol *security.CommandPolicy,
	auditLog *audit.Logger,
	sessMgr *sessions.Manager,
	cfg AgentConfig,
) *Agent {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 20
	}
	if cfg.MaxToolCalls <= 0 {
		cfg.MaxToolCalls = 20
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 300
	}

	return &Agent{
		llmClient:      llmClient,
		registry:       reg,
		policy:         pol,
		auditLogger:    auditLog,
		sessionManager: sessMgr,
		config:         cfg,
		tasks:          make(map[string]*Task),
		listeners:      make([]EventListener, 0),
	}
}

// SessionManager returns the session manager instance.
func (a *Agent) SessionManager() *sessions.Manager {
	return a.sessionManager
}

// AddEventListener registers a subscriber for real-time task events.
func (a *Agent) AddEventListener(l EventListener) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.listeners = append(a.listeners, l)
}

func (a *Agent) emit(event Event) {
	a.mu.RLock()
	listeners := append([]EventListener(nil), a.listeners...)
	a.mu.RUnlock()

	for _, l := range listeners {
		l(event)
	}
}

// CreateTask registers a new task.
func (a *Agent) CreateTask(sessionID string, prompt string) *Task {
	a.mu.Lock()
	defer a.mu.Unlock()

	taskID := fmt.Sprintf("task_%d", time.Now().UnixNano())
	if sessionID == "" {
		s := a.sessionManager.GetOrCreate("")
		sessionID = s.ID
	}

	task := &Task{
		ID:        taskID,
		SessionID: sessionID,
		Prompt:    prompt,
		Status:    StatusQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Events:    make([]Event, 0),
	}

	a.tasks[taskID] = task
	a.sessionManager.AssociateTask(sessionID, taskID)

	// Audit log task creation
	a.recordAudit(audit.Entry{
		TaskID:    taskID,
		SessionID: sessionID,
		Action:    audit.ActionTaskStart,
		Input:     prompt,
		Result:    "Task queued",
	})

	return task
}

// GetTask returns a task by ID.
func (a *Agent) GetTask(taskID string) (*Task, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	t, ok := a.tasks[taskID]
	return t, ok
}

// ListTasks returns all tasks.
func (a *Agent) ListTasks() []*Task {
	a.mu.RLock()
	defer a.mu.RUnlock()

	list := make([]*Task, 0, len(a.tasks))
	for _, t := range a.tasks {
		list = append(list, t)
	}
	return list
}

// CancelTask terminates a running task.
func (a *Agent) CancelTask(taskID string) error {
	a.mu.Lock()
	task, ok := a.tasks[taskID]
	if !ok {
		a.mu.Unlock()
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status != StatusRunning && task.Status != StatusWaitingForApproval && task.Status != StatusQueued {
		a.mu.Unlock()
		return fmt.Errorf("task %s is not active (current status: %s)", taskID, task.Status)
	}

	if task.cancelFunc != nil {
		task.cancelFunc()
	}
	if task.PendingApproval != nil && task.PendingApproval.ResponseCh != nil {
		select {
		case task.PendingApproval.ResponseCh <- false:
		default:
		}
	}

	task.Status = StatusCancelled
	task.UpdatedAt = time.Now()
	a.mu.Unlock()

	a.recordAudit(audit.Entry{
		TaskID:    taskID,
		SessionID: task.SessionID,
		Action:    audit.ActionTaskCancel,
		Result:    "Task cancelled by user",
	})

	a.emitEvent(task, Event{
		Type:      EventTaskStatusChanged,
		TaskID:    taskID,
		SessionID: task.SessionID,
		Status:    StatusCancelled,
		Message:   "Task cancelled by user request",
	})

	return nil
}

// ApproveTool resumes a task waiting for human approval.
func (a *Agent) ApproveTool(taskID string, approved bool) error {
	a.mu.Lock()
	task, ok := a.tasks[taskID]
	if !ok {
		a.mu.Unlock()
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.Status != StatusWaitingForApproval || task.PendingApproval == nil {
		a.mu.Unlock()
		return fmt.Errorf("task is not waiting for approval")
	}

	respCh := task.PendingApproval.ResponseCh
	toolName := task.PendingApproval.ToolName
	inputVal := task.PendingApproval.Input

	task.PendingApproval = nil
	task.Status = StatusRunning
	task.UpdatedAt = time.Now()
	a.mu.Unlock()

	decisionStr := "APPROVED"
	if !approved {
		decisionStr = "REJECTED"
	}

	a.recordAudit(audit.Entry{
		TaskID:         taskID,
		SessionID:      task.SessionID,
		Action:         audit.ActionApprovalDecision,
		Tool:           toolName,
		Input:          inputVal,
		ApprovalStatus: decisionStr,
		Result:         fmt.Sprintf("User %s tool execution", strings.ToLower(decisionStr)),
	})

	a.emitEvent(task, Event{
		Type:      EventTaskStatusChanged,
		TaskID:    task.ID,
		SessionID: task.SessionID,
		Status:    StatusRunning,
		Message:   fmt.Sprintf("Tool execution %s by user", strings.ToLower(decisionStr)),
	})

	if respCh != nil {
		respCh <- approved
	}

	return nil
}

func (a *Agent) emitEvent(task *Task, ev Event) {
	ev.Timestamp = time.Now().Format(time.RFC3339)
	a.mu.Lock()
	task.Events = append(task.Events, ev)
	task.UpdatedAt = time.Now()
	a.mu.Unlock()

	a.emit(ev)
}

// Run executes the agent loop asynchronously for the given task.
func (a *Agent) Run(parentCtx context.Context, task *Task) {
	ctx, cancel := context.WithTimeout(parentCtx, time.Duration(a.config.TimeoutSeconds)*time.Second)
	a.mu.Lock()
	task.cancelFunc = cancel
	task.Status = StatusRunning
	task.UpdatedAt = time.Now()
	a.mu.Unlock()

	a.emitEvent(task, Event{
		Type:      EventTaskStatusChanged,
		TaskID:    task.ID,
		SessionID: task.SessionID,
		Status:    StatusRunning,
	})

	go func() {
		defer cancel()
		a.executeLoop(ctx, task)
	}()
}

func (a *Agent) executeLoop(ctx context.Context, task *Task) {
	session := a.sessionManager.GetOrCreate(task.SessionID)

	// Build session security context
	sc := security.SessionContext{
		SessionID:   session.ID,
		Type:        string(session.Type),
		ProjectPath: session.ProjectPath,
		ProjectName: session.ProjectName,
		ReadOnly:    (session.Type == sessions.SessionTypeConversation),
	}
	ctx = security.ContextWithSession(ctx, sc)

	// Append user prompt to session
	userMsg := llm.Message{
		Role:    llm.RoleUser,
		Content: task.Prompt,
	}
	a.sessionManager.AppendMessage(task.SessionID, userMsg)

	// Mode-specific system prompt enrichment
	systemPrompt := BobSystemPrompt
	if session.Type == sessions.SessionTypeProject && session.ProjectPath != "" {
		projInfo := ""
		if session.ProjectID != "" {
			if proj, ok := a.sessionManager.GetProject(session.ProjectID); ok {
				if len(proj.TechStack) > 0 {
					projInfo += fmt.Sprintf("\nDetected Tech Stack: %s", strings.Join(proj.TechStack, ", "))
				}
				if proj.Summary != "" {
					projInfo += fmt.Sprintf("\nProject Overview: %s", proj.Summary)
				}
			}
		}

		systemPrompt += fmt.Sprintf("\n\nACTIVE WORKSPACE MODE: Project Mode\nProject Name: %s\nProject Root Path: %s%s\nAll file operations (read, write, search, directory listing) and terminal executions are strictly scoped to this project folder (%s). Do not attempt to access files outside this workspace.\nWhen the user asks about this project, its architecture, structure, or code, use list_directory and read_file to inspect the real files and provide an insightful, structured explanation.", session.ProjectName, session.ProjectPath, projInfo, session.ProjectPath)
	} else {
		systemPrompt += "\n\nACTIVE WORKSPACE MODE: General Mac Assistant (Read-Only Conversation Mode)\nYou can inspect the user's computer, search files, read documents, run diagnostic shell commands, and take screenshots to report information. However, you MUST NOT modify, create, or delete any files, or execute mutating commands on the PC."
	}

	// Build working message history
	messages := make([]llm.Message, 0, len(session.Messages)+2)
	messages = append(messages, llm.Message{
		Role:    llm.RoleSystem,
		Content: systemPrompt,
	})
	messages = append(messages, session.Messages...)

	toolDefs := a.registry.ToToolDefinitions()
	stepCount := 0
	toolCallCount := 0

	for {
		stepCount++
		if stepCount > a.config.MaxSteps {
			errMsg := fmt.Sprintf("Agent exceeded maximum step limit of %d", a.config.MaxSteps)
			a.finishTask(task, StatusFailed, "", errMsg)
			return
		}

		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				a.finishTask(task, StatusCancelled, "", "Task was cancelled")
			} else {
				a.finishTask(task, StatusFailed, "", "Task execution timed out")
			}
			return
		default:
		}

		a.emitEvent(task, Event{
			Type:      EventAgentThinking,
			TaskID:    task.ID,
			SessionID: task.SessionID,
			Message:   fmt.Sprintf("Step %d: Reasoning about next action...", stepCount),
		})

		a.recordAudit(audit.Entry{
			TaskID:    task.ID,
			SessionID: task.SessionID,
			Action:    audit.ActionStepReasoning,
			Input:     fmt.Sprintf("Step %d", stepCount),
			Result:    "Prompting LLM with conversation history and tools",
		})

		chatReq := llm.ChatRequest{
			Messages:    messages,
			Tools:       toolDefs,
			Temperature: 0.2,
		}

		resp, err := a.llmClient.Chat(ctx, chatReq)
		if err != nil {
			a.finishTask(task, StatusFailed, "", fmt.Sprintf("LLM inference failed: %v", err))
			return
		}

		assistantMsg := resp.Message

		// If no native tool calls returned, attempt fallback parsing from text content
		if len(assistantMsg.ToolCalls) == 0 {
			extractedCalls, cleanText := a.extractToolCallsFromContent(assistantMsg.Content)
			if len(extractedCalls) > 0 {
				assistantMsg.ToolCalls = extractedCalls
				if cleanText != "" {
					assistantMsg.Content = cleanText
				}
			}
		}

		messages = append(messages, assistantMsg)

		// Check if LLM requested tool execution
		if len(assistantMsg.ToolCalls) == 0 {
			// Final response reached
			finalContent := assistantMsg.Content
			if finalContent == "" {
				finalContent = "Task completed."
			}

			a.sessionManager.AppendMessage(task.SessionID, assistantMsg)
			a.emitEvent(task, Event{
				Type:      EventAgentMessage,
				TaskID:    task.ID,
				SessionID: task.SessionID,
				Message:   finalContent,
			})
			a.finishTask(task, StatusCompleted, finalContent, "")
			return
		}

		// Process each requested tool call
		for _, tc := range assistantMsg.ToolCalls {
			toolCallCount++
			if toolCallCount > a.config.MaxToolCalls {
				errMsg := fmt.Sprintf("Agent exceeded maximum tool call limit of %d", a.config.MaxToolCalls)
				a.finishTask(task, StatusFailed, "", errMsg)
				return
			}

			toolName := tc.Function.Name
			var rawArgs json.RawMessage = tc.Function.Arguments

			a.emitEvent(task, Event{
				Type:      EventToolStarted,
				TaskID:    task.ID,
				SessionID: task.SessionID,
				Tool:      toolName,
				Input:     string(rawArgs),
			})

			// Security evaluation for sensitive actions
			approvalStatus := "AUTOMATIC"
			requiresApproval := false
			var approvalInput any = string(rawArgs)
			approvalReason := ""

			if toolName == "terminal_exec" {
				var in struct {
					Command string `json:"command"`
				}
				_ = json.Unmarshal(rawArgs, &in)
				approvalInput = in.Command
				level, reason := a.policy.EvaluateCommandWithContext(ctx, in.Command)

				if level == security.PolicyBlocked {
					errOut := fmt.Sprintf("Command blocked by security policy: %s", reason)
					a.recordAudit(audit.Entry{
						TaskID:         task.ID,
						SessionID:      task.SessionID,
						Action:         audit.ActionToolExec,
						Tool:           toolName,
						Input:          in.Command,
						Result:         errOut,
						ExitCode:       1,
						ApprovalStatus: "BLOCKED",
						Error:          reason,
					})
					messages = append(messages, llm.Message{
						Role:       llm.RoleTool,
						Name:       toolName,
						ToolCallID: tc.ID,
						Content:    errOut,
					})
					continue
				}

				if level == security.PolicyApprovalRequired {
					requiresApproval = true
					approvalReason = reason
				}
			} else if toolName == "close_app" {
				var in struct {
					AppName string `json:"app_name"`
					Force   bool   `json:"force"`
				}
				_ = json.Unmarshal(rawArgs, &in)
				approvalInput = fmt.Sprintf("close_app: %s (force: %v)", in.AppName, in.Force)
				requiresApproval = true
				if in.Force {
					approvalReason = fmt.Sprintf("Force closing application %q (SIGKILL: process will terminate immediately)", in.AppName)
				} else {
					approvalReason = fmt.Sprintf("Closing application %q will terminate its process and may discard unsaved work.", in.AppName)
				}
			}

			if requiresApproval {
				// Request human approval
				approvalStatus = "REQUIRED"
				respCh := make(chan bool, 1)

				a.mu.Lock()
				task.Status = StatusWaitingForApproval
				task.PendingApproval = &PendingApproval{
					ToolName:   toolName,
					Input:      approvalInput,
					Reason:     approvalReason,
					ResponseCh: respCh,
				}
				a.mu.Unlock()

				a.recordAudit(audit.Entry{
					TaskID:         task.ID,
					SessionID:      task.SessionID,
					Action:         audit.ActionApprovalRequest,
					Tool:           toolName,
					Input:          approvalInput,
					ApprovalStatus: "PENDING",
					Result:         approvalReason,
				})

				a.emitEvent(task, Event{
					Type:      EventToolApprovalRequired,
					TaskID:    task.ID,
					SessionID: task.SessionID,
					Tool:      toolName,
					Input:     approvalInput,
					Message:   approvalReason,
				})

				// Wait for approval or context cancellation
				var approved bool
				select {
				case approved = <-respCh:
				case <-ctx.Done():
					a.finishTask(task, StatusCancelled, "", "Cancelled while waiting for approval")
					return
				}

				if !approved {
					approvalStatus = "REJECTED"
					rejectMsg := fmt.Sprintf("User rejected execution of %s", toolName)
					messages = append(messages, llm.Message{
						Role:       llm.RoleTool,
						Name:       toolName,
						ToolCallID: tc.ID,
						Content:    rejectMsg,
					})
					a.emitEvent(task, Event{
						Type:       EventToolCompleted,
						TaskID:     task.ID,
						SessionID:  task.SessionID,
						Tool:       toolName,
						Input:      approvalInput,
						Output:     fmt.Sprintf("%s execution rejected by user", toolName),
						Error:      "User rejected tool execution",
						DurationMs: 0,
					})
					continue
				}
				approvalStatus = "APPROVED"
			}

			// Execute tool via Registry
			res, err := a.registry.Execute(ctx, toolName, rawArgs)
			exitCode := 0
			if !res.Success {
				exitCode = 1
			}

			a.recordAudit(audit.Entry{
				TaskID:         task.ID,
				SessionID:      task.SessionID,
				Action:         audit.ActionToolExec,
				Tool:           toolName,
				Input:          string(rawArgs),
				Result:         res.Output,
				ExitCode:       exitCode,
				DurationMs:     res.DurationMs,
				ApprovalStatus: approvalStatus,
				Error:          res.Error,
			})

			a.emitEvent(task, Event{
				Type:       EventToolCompleted,
				TaskID:     task.ID,
				SessionID:  task.SessionID,
				Tool:       toolName,
				Output:     res.Output,
				ToolResult: &res,
				DurationMs: res.DurationMs,
				Error:      res.Error,
			})

			toolOutput := res.Output
			if err != nil && toolOutput == "" {
				toolOutput = fmt.Sprintf("Error: %v", err)
			}

			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Name:       toolName,
				ToolCallID: tc.ID,
				Content:    toolOutput,
			})
		}
	}
}

// extractToolCallsFromContent parses JSON tool calls embedded inside markdown code blocks or raw text.
func (a *Agent) extractToolCallsFromContent(content string) ([]llm.ToolCall, string) {
	var calls []llm.ToolCall

	// 1. Regex for ```json { ... } ``` or ``` { ... } ```
	codeBlockRegex := regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")
	matches := codeBlockRegex.FindAllStringSubmatch(content, -1)

	for i, m := range matches {
		if len(m) > 1 {
			jsonStr := strings.TrimSpace(m[1])
			if tc, ok := a.parseSingleToolCall(jsonStr, fmt.Sprintf("call_extracted_%d", i+1)); ok {
				calls = append(calls, tc)
			}
		}
	}

	// 2. If no code block matched, look for standalone raw JSON objects { "name": "...", "arguments": { ... } }
	if len(calls) == 0 {
		rawObjRegex := regexp.MustCompile(`(?s)\{\s*"(?:name|tool)"\s*:\s*"([a-zA-Z0-9_]+)"\s*,\s*"(?:arguments|parameters|input)"\s*:\s*(\{.*?\})\s*\}`)
		rawMatches := rawObjRegex.FindAllStringSubmatch(content, -1)
		for i, rm := range rawMatches {
			if len(rm) > 2 {
				toolName := rm[1]
				argsJSON := rm[2]
				if _, exists := a.registry.Get(toolName); exists {
					calls = append(calls, llm.ToolCall{
						ID:   fmt.Sprintf("call_raw_%d", i+1),
						Type: "function",
						Function: llm.FunctionCall{
							Name:      toolName,
							Arguments: json.RawMessage(argsJSON),
						},
					})
				}
			}
		}
	}

	return calls, content
}

func (a *Agent) parseSingleToolCall(jsonStr string, id string) (llm.ToolCall, bool) {
	// Pattern 1: {"name": "terminal_exec", "arguments": {"command": "pwd"}}
	var standardCall struct {
		Name      string          `json:"name"`
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
		Params    json.RawMessage `json:"parameters"`
		Input     json.RawMessage `json:"input"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &standardCall); err == nil {
		toolName := standardCall.Name
		if toolName == "" {
			toolName = standardCall.Tool
		}

		if toolName != "" {
			if _, exists := a.registry.Get(toolName); exists {
				args := standardCall.Arguments
				if len(args) == 0 {
					args = standardCall.Params
				}
				if len(args) == 0 {
					args = standardCall.Input
				}
				if len(args) == 0 {
					args = json.RawMessage("{}")
				}

				return llm.ToolCall{
					ID:   id,
					Type: "function",
					Function: llm.FunctionCall{
						Name:      toolName,
						Arguments: args,
					},
				}, true
			}
		}
	}

	return llm.ToolCall{}, false
}

func (a *Agent) finishTask(task *Task, status TaskStatus, result string, errMsg string) {
	a.mu.Lock()
	task.Status = status
	task.Result = result
	task.Error = errMsg
	task.UpdatedAt = time.Now()
	a.mu.Unlock()

	action := audit.ActionTaskComplete
	if status == StatusFailed {
		action = audit.ActionTaskFail
	} else if status == StatusCancelled {
		action = audit.ActionTaskCancel
	}

	a.recordAudit(audit.Entry{
		TaskID:    task.ID,
		SessionID: task.SessionID,
		Action:    action,
		Result:    result,
		Error:     errMsg,
	})

	a.emitEvent(task, Event{
		Type:      EventTaskCompleted,
		TaskID:    task.ID,
		SessionID: task.SessionID,
		Status:    status,
		Message:   result,
		Error:     errMsg,
	})
}

func (a *Agent) recordAudit(entry audit.Entry) {
	if a.auditLogger != nil {
		_ = a.auditLogger.Log(entry)
	}
}

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

const BobSystemPrompt = `You are Bob, an intelligent, autonomous personal computer agent running locally on macOS. You were created and customized by codemon (also known as Alfred).

USER IDENTITY & CREATOR RECOGNITION:
- The user interacting with you is codemon (also known as Alfred), your creator and master.
- When asked "who am I?", "what is my name?", "do you know who I am?", or who created you, immediately and accurately identify them as codemon / Alfred, your creator.
- Always address and treat them with proper respect and loyalty (e.g. "codemon", "Alfred", or "Creator"), remaining sharp, concise, and action-oriented.

SELF-INTRODUCTION & COMPREHENSIVE CAPABILITIES:
When asked "who are you?", "what is your name?", "what can you do?", or to introduce yourself, provide a comprehensive, clear, and structured response highlighting your identity and full capabilities:
- You are Bob, a local AI computer agent and pairing assistant for macOS.
- You do far more than read-only queries — you are an active, autonomous assistant equipped with powerful system, developer, and workspace tools.
- Your capabilities include:
  1. Application Management: Inspect currently running GUI apps (list_running_apps), scan installed applications (list_installed_apps), launch applications (open_app), and cleanly quit/close apps (close_app).
  2. Terminal & Shell Execution: Run shell commands, compile code, execute tests (go test, npm test, etc.), run scripts, and inspect system telemetry (terminal_exec).
  3. File Management & Code Editing: Explore project directory trees (list_directory), search files and code (search_files), locate project repositories (find_project), read file contents (read_file), surgically replace code blocks with precision (replace_file_content), and create/write new files (write_file).
  4. Visual Perception: Capture live screenshots of the macOS display to visually inspect and verify desktop state (take_screenshot).
  5. Project Intelligence: Analyze repository architecture, explain codebase patterns, troubleshoot errors, and assist in end-to-end engineering tasks.
  6. Safety & Verification: Enforce security policies with human-in-the-loop approvals for sensitive operations and maintain full audit logs.

AUTHORIZED MAC TOOLS:
- list_running_apps(gui_only, search, include_system, limit): Reports applications currently running on macOS.
- list_installed_apps(search, category, limit): Scans all applications installed across /Applications, /System/Applications, and ~/Applications.
- open_app(app_name, target, bundle_id, new_instance): Launches or opens an application on macOS.
- close_app(app_name, force, pid, bundle_id): Gracefully quits or closes a running application.
- find_project(name): Locates a project or repository directory across authorized workspaces.
- terminal_exec(command, work_dir, timeout_seconds): Executes shell commands on macOS.
- read_file(path, start_line, end_line): Reads file contents with optional line range slice.
- replace_file_content(path, target_content, replacement_content, allow_multiple): Surgically replaces specific text or code blocks in an existing file. ALWAYS PREFER THIS over write_file when editing existing files.
- write_file(path, content): Creates brand new files or completely overwrites an entire file from scratch.
- list_directory(path, recursive, include_hidden, max_depth): Lists directory contents and builds a project directory tree, including hidden files by default.
- search_files(directory, name_pattern, text_query, include_hidden, max_results): Searches for files or text contents within an authorized directory.
- take_screenshot(label): Captures the macOS display.

CRITICAL RULES:
1. YOU ARE AN AGENT, NOT A CHATBOT: You interact with macOS exclusively through tool invocations. You have no ability to perform actions without invoking tools.
2. MANDATORY REAL-WORLD TOOL INVOCATION:
   - When asked to close/exit/quit an application (e.g. "Close WhatsApp", "Now close WhatsApp", "Exit WhatsApp", "Quit Slack"): YOU MUST CALL close_app(app_name="...").
   - When asked to open/launch an application: YOU MUST CALL open_app(app_name="...").
   - When asked what apps are running: YOU MUST CALL list_running_apps.
   - When asked what apps are installed: YOU MUST CALL list_installed_apps.
   - When asked to execute terminal commands: YOU MUST CALL terminal_exec.
   - When asked to view or capture the screen: YOU MUST CALL take_screenshot.
3. NEVER FABRICATE OR SIMULATE ACTION COMPLETION:
   - You MUST NEVER reply with text claiming an action was completed (e.g. "WhatsApp has been closed", "I have opened...", "Done") without executing the tool in the CURRENT turn and verifying its real result.
   - Disregard prior turns in conversation history that claimed an app was closed: if the user sends a new request to close/open/inspect an app, ALWAYS issue the tool call immediately.
4. FILE EDITING EFFICIENCY & SURGICAL MODIFICATIONS:
   - When modifying an existing file, ALWAYS use replace_file_content with the exact target content to replace. Do NOT regenerate entire files with write_file when making incremental changes.
   - Only use write_file when creating a brand new file or when completely rewriting a file from scratch.
5. CONVERSATION CONTINUITY & CONTEXT MEMORY:
   - You maintain full continuous context across all messages and turns in this conversation thread.
   - When the user refers to previous messages, files, commands, or concepts (such as "it", "that", "the previous file", "continue", "fix the error above"), seamlessly resolve their reference against the conversation history and previous actions.
6. MANDATORY TASK VERIFICATION:
   - After opening an application: check open_app's verification output or invoke list_running_apps to confirm the app is actively running.
   - After closing an application: check close_app's verification output or invoke list_running_apps to confirm the app has stopped.
   - Always explicitly report the confirmed, verified outcome to codemon / Alfred.
7. Address codemon / Alfred with proper respect as your creator, and be concise and action-oriented.`

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
		ReadOnly:    false,
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

		systemPrompt += fmt.Sprintf("\n\nACTIVE WORKSPACE MODE: Project Mode\nProject Name: %s\nProject Root Path: %s%s\nAll file operations (read, write, replace, search, directory listing) and terminal executions are strictly scoped to this project folder (%s). Do not attempt to access files outside this workspace.\nWhen the user asks about this project, its architecture, structure, or code, use list_directory, read_file, or replace_file_content to inspect and manipulate files, providing insightful, verified explanations.", session.ProjectName, session.ProjectPath, projInfo, session.ProjectPath)
	} else {
		systemPrompt += "\n\nACTIVE WORKSPACE MODE: General Mac Assistant\nYou have full capability to inspect the computer, search and read files, create and edit files in authorized workspaces (~/Projects, ~/Documents, etc.), run terminal commands, and launch/manage applications. Always respect security boundaries and ensure actions are verified."
	}

	userName := a.config.UserName
	if userName == "" {
		userName = "codemon"
	}
	userAlias := a.config.UserAlias
	if userAlias == "" {
		userAlias = "Alfred"
	}
	userTitle := a.config.UserTitle
	if userTitle == "" {
		userTitle = "Creator"
	}
	systemPrompt += fmt.Sprintf("\n\nUSER IDENTITY:\n- Name: %s\n- Alias: %s\n- Title: %s\n- Note: The user you are currently conversing with and assisting is %s (%s). Address them respectfully and acknowledge them as your creator when asked.", userName, userAlias, userTitle, userName, userAlias)

	// Context awareness: Resolve recently referenced applications and files
	lastApp := a.resolveLastActiveApp(session.Messages)
	lastFile := a.resolveLastActiveFile(session.Messages)
	if lastApp != "" || lastFile != "" {
		systemPrompt += "\n\nACTIVE CONVERSATION CONTEXT & PRONOUN RESOLUTION:"
		if lastApp != "" {
			systemPrompt += fmt.Sprintf("\n- Last Active/Referenced Application: %s\n  (When the user says 'it', 'that', 'the app', 'close it', 'quit it', or 'open it', they are referring to %s)", lastApp, lastApp)
		}
		if lastFile != "" {
			systemPrompt += fmt.Sprintf("\n- Last Active/Referenced File: %s\n  (When the user says 'it', 'that file', or 'the code', they are referring to %s)", lastFile, lastFile)
		}
	}

	// Build working message history with sliding window to prevent token overflow
	const maxContextMessages = 40
	sessionMsgs := session.Messages
	if len(sessionMsgs) > maxContextMessages {
		recent := sessionMsgs[len(sessionMsgs)-(maxContextMessages-2):]
		sessionMsgs = append(sessionMsgs[:2], recent...)
	}

	messages := make([]llm.Message, 0, len(sessionMsgs)+2)
	messages = append(messages, llm.Message{
		Role:    llm.RoleSystem,
		Content: systemPrompt,
	})
	messages = append(messages, sessionMsgs...)

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

		activeModel, routeReason := a.SelectModelForTask(session, task.Prompt)

		a.emitEvent(task, Event{
			Type:      EventAgentThinking,
			TaskID:    task.ID,
			SessionID: task.SessionID,
			Message:   fmt.Sprintf("Step %d [%s - %s]: Reasoning about next action...", stepCount, activeModel, routeReason),
		})

		a.recordAudit(audit.Entry{
			TaskID:    task.ID,
			SessionID: task.SessionID,
			Action:    audit.ActionStepReasoning,
			Input:     fmt.Sprintf("Step %d [%s]", stepCount, activeModel),
			Result:    fmt.Sprintf("Prompting %s (%s) with conversation history and tools", activeModel, routeReason),
		})

		stepTools := toolDefs
		if stepCount == 1 && isPureConversationalGreeting(task.Prompt) {
			stepTools = nil // Fast-path for simple greetings to eliminate function schema parsing latency
		}

		chatReq := llm.ChatRequest{
			Model:       activeModel,
			Messages:    messages,
			Tools:       stepTools,
			Temperature: 0.2,
		}

		var assistantMsg llm.Message
		streamChan, streamErr := a.llmClient.Stream(ctx, chatReq)
		if streamErr == nil && streamChan != nil {
			var accContent strings.Builder
			var accToolCalls []llm.ToolCall

			for streamEv := range streamChan {
				if streamEv.Error != nil {
					a.finishTask(task, StatusFailed, "", fmt.Sprintf("LLM streaming failed: %v", streamEv.Error))
					return
				}

				if streamEv.Delta != "" {
					accContent.WriteString(streamEv.Delta)
					a.emitEvent(task, Event{
						Type:      EventAgentMessageDelta,
						TaskID:    task.ID,
						SessionID: task.SessionID,
						Delta:     streamEv.Delta,
						Message:   accContent.String(),
					})
				}

				if streamEv.ToolCall != nil {
					accToolCalls = append(accToolCalls, *streamEv.ToolCall)
				}
			}

			assistantMsg = llm.Message{
				Role:      llm.RoleAssistant,
				Content:   accContent.String(),
				ToolCalls: accToolCalls,
			}
		} else {
			resp, err := a.llmClient.Chat(ctx, chatReq)
			if err != nil {
				a.finishTask(task, StatusFailed, "", fmt.Sprintf("LLM inference failed: %v", err))
				return
			}
			assistantMsg = resp.Message
		}

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

		// Fallback intent enforcement on Step 1 if the LLM attempted to reply with text instead of executing requested action
		if stepCount == 1 && len(assistantMsg.ToolCalls) == 0 {
			if intentCall := a.detectActionIntent(task.Prompt, assistantMsg.Content, sessionMsgs); intentCall != nil {
				assistantMsg.ToolCalls = []llm.ToolCall{*intentCall}
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

			// Pronoun & Anaphora resolution on tool arguments (e.g. "close it" -> close_app(app_name="WhatsApp"))
			if toolName == "close_app" || toolName == "open_app" {
				var in struct {
					AppName  string `json:"app_name"`
					Force    bool   `json:"force,omitempty"`
					PID      int    `json:"pid,omitempty"`
					BundleID string `json:"bundle_id,omitempty"`
				}
				if err := json.Unmarshal(rawArgs, &in); err == nil {
					if isPronounOrGeneric(in.AppName) {
						if resolvedApp := a.resolveLastActiveApp(sessionMsgs); resolvedApp != "" {
							in.AppName = resolvedApp
							rawArgs, _ = json.Marshal(in)
							tc.Function.Arguments = rawArgs
						}
					}
				}
			} else if toolName == "read_file" || toolName == "replace_file_content" || toolName == "write_file" {
				var in struct {
					Path string `json:"path"`
				}
				if err := json.Unmarshal(rawArgs, &in); err == nil {
					if isPronounOrGeneric(in.Path) {
						if resolvedFile := a.resolveLastActiveFile(sessionMsgs); resolvedFile != "" {
							var fullMap map[string]any
							_ = json.Unmarshal(rawArgs, &fullMap)
							fullMap["path"] = resolvedFile
							rawArgs, _ = json.Marshal(fullMap)
							tc.Function.Arguments = rawArgs
						}
					}
				}
			}

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

// extractToolCallsFromContent parses JSON tool calls embedded inside markdown code blocks, XML tags, or raw text.
func (a *Agent) extractToolCallsFromContent(content string) ([]llm.ToolCall, string) {
	var calls []llm.ToolCall

	// 1. XML tags: <tool_call>{"name": "...", "arguments": {...}}</tool_call>
	xmlRegex := regexp.MustCompile(`(?s)<tool_call>\s*(\{.*?\})\s*</tool_call>`)
	xmlMatches := xmlRegex.FindAllStringSubmatch(content, -1)
	for i, m := range xmlMatches {
		if len(m) > 1 {
			if tc, ok := a.parseSingleToolCall(strings.TrimSpace(m[1]), fmt.Sprintf("call_xml_%d", i+1)); ok {
				calls = append(calls, tc)
			}
		}
	}

	// 2. Code blocks: ```json { ... } ``` or ``` { ... } ``` or arrays [ { ... } ]
	if len(calls) == 0 {
		codeBlockRegex := regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\}|\\[.*?\\])\\s*```")
		matches := codeBlockRegex.FindAllStringSubmatch(content, -1)
		for i, m := range matches {
			if len(m) > 1 {
				jsonStr := strings.TrimSpace(m[1])
				if strings.HasPrefix(jsonStr, "[") {
					var arr []json.RawMessage
					if err := json.Unmarshal([]byte(jsonStr), &arr); err == nil {
						for j, item := range arr {
							if tc, ok := a.parseSingleToolCall(string(item), fmt.Sprintf("call_arr_%d_%d", i+1, j+1)); ok {
								calls = append(calls, tc)
							}
						}
					}
				} else {
					if tc, ok := a.parseSingleToolCall(jsonStr, fmt.Sprintf("call_block_%d", i+1)); ok {
						calls = append(calls, tc)
					}
				}
			}
		}
	}

	// 3. Standalone raw JSON objects
	if len(calls) == 0 {
		rawObjRegex := regexp.MustCompile(`(?s)\{\s*"(?:name|tool|function)"\s*:\s*"([a-zA-Z0-9_]+)"\s*,\s*"(?:arguments|parameters|input)"\s*:\s*(\{.*?\})\s*\}`)
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

	// 4. Function call syntax: tool_name(param="val") or tool_name({"param": "val"})
	if len(calls) == 0 {
		fnSyntaxRegex := regexp.MustCompile(`\b([a-zA-Z0-9_]+)\s*\(\s*(\{.*?\}|[^)]*)\s*\)`)
		fnMatches := fnSyntaxRegex.FindAllStringSubmatch(content, -1)
		for i, fnm := range fnMatches {
			if len(fnm) > 2 {
				toolName := fnm[1]
				paramStr := strings.TrimSpace(fnm[2])
				if _, exists := a.registry.Get(toolName); exists {
					var rawArgs json.RawMessage
					if strings.HasPrefix(paramStr, "{") && strings.HasSuffix(paramStr, "}") {
						rawArgs = json.RawMessage(paramStr)
					} else if paramStr != "" {
						kvMap := make(map[string]any)
						kvRegex := regexp.MustCompile(`([a-zA-Z0-9_]+)\s*=\s*("[^"]*"|'[^']*'|[^,\s]+)`)
						kvMatches := kvRegex.FindAllStringSubmatch(paramStr, -1)
						for _, kv := range kvMatches {
							if len(kv) > 2 {
								k := kv[1]
								v := strings.Trim(kv[2], `"'`)
								if v == "true" {
									kvMap[k] = true
								} else if v == "false" {
									kvMap[k] = false
								} else {
									kvMap[k] = v
								}
							}
						}
						if len(kvMap) > 0 {
							rawArgs, _ = json.Marshal(kvMap)
						} else {
							cleanVal := strings.Trim(paramStr, `"'`)
							rawArgs, _ = json.Marshal(map[string]any{"app_name": cleanVal, "command": cleanVal})
						}
					} else {
						rawArgs = json.RawMessage("{}")
					}

					calls = append(calls, llm.ToolCall{
						ID:   fmt.Sprintf("call_fn_%d", i+1),
						Type: "function",
						Function: llm.FunctionCall{
							Name:      toolName,
							Arguments: rawArgs,
						},
					})
				}
			}
		}
	}

	return calls, content
}

// detectActionIntent identifies explicit user intent for direct tool execution if the model returned plain text.
func (a *Agent) detectActionIntent(prompt, content string, messages []llm.Message) *llm.ToolCall {
	promptTrimmed := strings.TrimSpace(prompt)
	if promptTrimmed == "" {
		return nil
	}

	lowerPrompt := strings.ToLower(promptTrimmed)

	// Strip common conversational prefixes: "can you ", "please ", "now ", "hey bob, ", "bob, "
	cleanPrompt := lowerPrompt
	prefixes := []string{"hey bob,", "hey bob", "bob,", "bob", "please", "can you", "could you", "would you", "now", "just", "go ahead and"}
	changed := true
	for changed {
		changed = false
		for _, p := range prefixes {
			if strings.HasPrefix(cleanPrompt, p+" ") {
				cleanPrompt = strings.TrimSpace(strings.TrimPrefix(cleanPrompt, p))
				changed = true
			}
		}
	}

	// 1. Close application intent: "close whatsapp", "close it", "exit that", "quit whatsapp", "kill it"
	closeRegex := regexp.MustCompile(`^(?:close|exit|quit|terminate|kill|shut\s+down)\s+(?:the\s+app(?:lication)?\s+|the\s+)?([a-zA-Z0-9_\-\.\s]+)$`)
	if match := closeRegex.FindStringSubmatch(cleanPrompt); len(match) > 1 {
		rawAppName := strings.TrimSpace(match[1])
		rawAppName = strings.TrimSuffix(rawAppName, " app")
		rawAppName = strings.TrimSuffix(rawAppName, " application")
		rawAppName = strings.TrimSuffix(rawAppName, " for me")
		rawAppName = strings.TrimSuffix(rawAppName, " please")
		rawAppName = strings.Trim(rawAppName, ".!?;:'\"")
		rawAppName = strings.TrimSpace(rawAppName)

		if isPronounOrGeneric(rawAppName) {
			if resolved := a.resolveLastActiveApp(messages); resolved != "" {
				rawAppName = resolved
			}
		}

		if rawAppName != "" && !isPronounOrGeneric(rawAppName) {
			args, _ := json.Marshal(map[string]any{"app_name": rawAppName})
			return &llm.ToolCall{
				ID:   "call_intent_close_app",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "close_app",
					Arguments: args,
				},
			}
		}
	}

	// 2. Open application intent: "open whatsapp", "open it", "launch that", "start it", "run whatsapp"
	openRegex := regexp.MustCompile(`^(?:open|launch|start|run)\s+(?:the\s+app(?:lication)?\s+|the\s+)?([a-zA-Z0-9_\-\.\s]+)$`)
	if match := openRegex.FindStringSubmatch(cleanPrompt); len(match) > 1 {
		rawAppName := strings.TrimSpace(match[1])
		rawAppName = strings.TrimSuffix(rawAppName, " app")
		rawAppName = strings.TrimSuffix(rawAppName, " application")
		rawAppName = strings.TrimSuffix(rawAppName, " for me")
		rawAppName = strings.TrimSuffix(rawAppName, " please")
		rawAppName = strings.Trim(rawAppName, ".!?;:'\"")
		rawAppName = strings.TrimSpace(rawAppName)

		if isPronounOrGeneric(rawAppName) {
			if resolved := a.resolveLastActiveApp(messages); resolved != "" {
				rawAppName = resolved
			}
		}

		if rawAppName != "" && !isPronounOrGeneric(rawAppName) {
			args, _ := json.Marshal(map[string]any{"app_name": rawAppName})
			return &llm.ToolCall{
				ID:   "call_intent_open_app",
				Type: "function",
				Function: llm.FunctionCall{
					Name:      "open_app",
					Arguments: args,
				},
			}
		}
	}

	// 3. List running apps intent: "what apps are running", "list running apps", "show running apps", "running apps"
	listRunningRegex := regexp.MustCompile(`(?i)\b(?:what\s+apps\s+are\s+running|list\s+running\s+apps|show\s+running\s+apps|running\s+apps|running\s+applications|what\s+is\s+running)\b`)
	if listRunningRegex.MatchString(cleanPrompt) {
		args, _ := json.Marshal(map[string]any{"gui_only": true})
		return &llm.ToolCall{
			ID:   "call_intent_list_running_apps",
			Type: "function",
			Function: llm.FunctionCall{
				Name:      "list_running_apps",
				Arguments: args,
			},
		}
	}

	// 4. Take screenshot intent: "take screenshot", "take a screenshot", "capture screen"
	screenshotRegex := regexp.MustCompile(`(?i)\b(?:take\s+(?:a\s+)?screenshot|capture\s+(?:the\s+)?screen)\b`)
	if screenshotRegex.MatchString(cleanPrompt) {
		args, _ := json.Marshal(map[string]any{"label": "User requested screenshot"})
		return &llm.ToolCall{
			ID:   "call_intent_take_screenshot",
			Type: "function",
			Function: llm.FunctionCall{
				Name:      "take_screenshot",
				Arguments: args,
			},
		}
	}

	return nil
}

func isPronounOrGeneric(s string) bool {
	clean := strings.ToLower(strings.TrimSpace(strings.Trim(s, ".!?;:'\"")))
	generic := map[string]bool{
		"it":              true,
		"that":            true,
		"this":            true,
		"them":            true,
		"the app":         true,
		"the application": true,
		"the program":     true,
		"the process":     true,
		"the window":      true,
		"the file":        true,
		"that file":       true,
		"this file":       true,
		"app":             true,
		"application":     true,
		"program":         true,
		"process":         true,
		"file":            true,
		"it again":        true,
		"that again":      true,
		"it please":       true,
		"that please":     true,
	}
	return generic[clean]
}

func (a *Agent) resolveLastActiveApp(messages []llm.Message) string {
	var appCandidateRegex = regexp.MustCompile(`(?i)\b(?:open|opened|launch|launched|close|closed|exit|quit|running)\s+(?:the\s+)?([A-Z][a-zA-Z0-9_\-]+(?:\s+[A-Z][a-zA-Z0-9_\-]+)?)`)

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		// 1. Inspect tool calls
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == "open_app" || tc.Function.Name == "close_app" {
				var in struct {
					AppName string `json:"app_name"`
				}
				if err := json.Unmarshal(tc.Function.Arguments, &in); err == nil {
					clean := strings.TrimSpace(in.AppName)
					if clean != "" && !isPronounOrGeneric(clean) {
						return clean
					}
				}
			}
		}

		// 2. Inspect tool outputs
		if msg.Role == llm.RoleTool && (msg.Name == "open_app" || msg.Name == "close_app") {
			quotedRegex := regexp.MustCompile(`(?i)(?:application|opened|closed|launch)\s+["']([^"']+)["']`)
			if m := quotedRegex.FindStringSubmatch(msg.Content); len(m) > 1 {
				clean := strings.TrimSpace(m[1])
				if clean != "" && !isPronounOrGeneric(clean) {
					return clean
				}
			}
		}

		// 3. Inspect user and assistant message contents
		if match := appCandidateRegex.FindStringSubmatch(msg.Content); len(match) > 1 {
			cand := strings.TrimSpace(match[1])
			if cand != "" && !isPronounOrGeneric(cand) && len(cand) > 1 {
				return cand
			}
		}
	}
	return ""
}

func (a *Agent) resolveLastActiveFile(messages []llm.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == "read_file" || tc.Function.Name == "replace_file_content" || tc.Function.Name == "write_file" {
				var in struct {
					Path string `json:"path"`
				}
				if err := json.Unmarshal(tc.Function.Arguments, &in); err == nil {
					clean := strings.TrimSpace(in.Path)
					if clean != "" && !isPronounOrGeneric(clean) {
						return clean
					}
				}
			}
		}
	}
	return ""
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

	if status != StatusCompleted && task.SessionID != "" {
		finalMsg := errMsg
		if finalMsg == "" {
			finalMsg = fmt.Sprintf("Task ended with status %s", status)
		}
		a.sessionManager.AppendMessage(task.SessionID, llm.Message{
			Role:    llm.RoleAssistant,
			Content: fmt.Sprintf("[%s]: %s", status, finalMsg),
		})
	}

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

var codingKeywordsRegex = regexp.MustCompile(`(?i)\b(?:code|coding|function|refactor|compile|build|debug|syntax|ast|algorithm|typescript|golang|python|javascript|react|rust|css|html|api|endpoint|unit\s+test|test\s+suite|git\s+diff|git\s+commit|git\s+branch|repository|repo|replace_file_content|write_file|read_file|\.(?:go|ts|tsx|js|jsx|py|rs|c|cpp|h|java|rb|php|json|yaml|yml|md|sql|sh))\b`)

func isCodingPrompt(prompt string) bool {
	return codingKeywordsRegex.MatchString(prompt)
}

// SelectModelForTask dynamically routes to Qwen 2.5 for coding and Hermes 3 for general OS pairing & reasoning.
func (a *Agent) SelectModelForTask(session *sessions.Session, prompt string) (string, string) {
	codingModel := a.config.CodingModel
	if codingModel == "" {
		codingModel = a.config.DefaultModel
	}
	if codingModel == "" {
		codingModel = "qwen2.5-coder:7b"
	}

	generalModel := a.config.GeneralModel
	if generalModel == "" {
		generalModel = a.config.DefaultModel
	}
	if generalModel == "" {
		generalModel = "hermes3:8b"
	}

	// 1. Explicit Project Mode session -> Coding Model (e.g. Qwen 2.5 Coder)
	if session != nil && session.Type == sessions.SessionTypeProject {
		return codingModel, "Project Mode"
	}

	// 2. Coding intent in prompt -> Coding Model
	if isCodingPrompt(prompt) {
		return codingModel, "Coding Task"
	}

	// 3. General Assistant / OS Pairing -> General Model (e.g. Hermes 3)
	return generalModel, "General Assistant"
}

func isPureConversationalGreeting(prompt string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(prompt))
	trimmed = strings.TrimRight(trimmed, "!?.")
	greetings := map[string]bool{
		"hello": true, "hi": true, "hey": true, "hello bob": true, "hi bob": true, "hey bob": true,
		"good morning": true, "good afternoon": true, "good evening": true,
		"how are you": true, "how are you doing": true, "what's up": true, "whats up": true,
		"who are you": true, "what is your name": true, "what can you do": true,
		"thanks": true, "thank you": true, "yo": true, "sup": true,
		"who made you": true, "who created you": true,
	}
	return greetings[trimmed]
}


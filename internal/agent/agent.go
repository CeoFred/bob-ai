package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"bob/internal/audit"
	"bob/internal/llm"
	"bob/internal/security"
	"bob/internal/sessions"
	"bob/internal/tools/registry"
)

const BobSystemPrompt = `You are Bob, a local personal computer agent running on the user’s Mac. Your job is to help the user accomplish tasks by reasoning about requests and using authorized tools on the computer.

Key behaviors:
1. Be concise, direct, and action-oriented. Avoid verbose chatter or long conversational filler.
2. If inspecting a project or system, use the appropriate tools (terminal_exec, read_file, list_directory, search_files, take_screenshot).
3. Always verify commands before running them.
4. When reporting results, summarize key findings clearly.`

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
	task.PendingApproval = nil
	task.Status = StatusRunning
	task.UpdatedAt = time.Now()
	a.mu.Unlock()

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

	// Append user prompt to session
	userMsg := llm.Message{
		Role:    llm.RoleUser,
		Content: task.Prompt,
	}
	a.sessionManager.AppendMessage(task.SessionID, userMsg)

	// Build working message history
	messages := make([]llm.Message, 0, len(session.Messages)+2)
	messages = append(messages, llm.Message{
		Role:    llm.RoleSystem,
		Content: BobSystemPrompt,
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
			Message:   "Reasoning about next action...",
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
			if toolName == "terminal_exec" {
				var in struct {
					Command string `json:"command"`
				}
				_ = json.Unmarshal(rawArgs, &in)
				level, reason := a.policy.EvaluateCommand(in.Command)

				if level == security.PolicyBlocked {
					errOut := fmt.Sprintf("Command blocked by security policy: %s", reason)
					a.recordAudit(task.ID, task.SessionID, toolName, in, errOut, 1, 0, "BLOCKED")
					messages = append(messages, llm.Message{
						Role:       llm.RoleTool,
						Name:       toolName,
						ToolCallID: tc.ID,
						Content:    errOut,
					})
					continue
				}

				if level == security.PolicyApprovalRequired {
					// Request human approval
					approvalStatus = "REQUIRED"
					respCh := make(chan bool, 1)

					a.mu.Lock()
					task.Status = StatusWaitingForApproval
					task.PendingApproval = &PendingApproval{
						ToolName:   toolName,
						Input:      in.Command,
						Reason:     reason,
						ResponseCh: respCh,
					}
					a.mu.Unlock()

					a.emitEvent(task, Event{
						Type:      EventToolApprovalRequired,
						TaskID:    task.ID,
						SessionID: task.SessionID,
						Tool:      toolName,
						Input:     in.Command,
						Message:   reason,
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
						rejectMsg := "User rejected execution of command"
						a.recordAudit(task.ID, task.SessionID, toolName, in, rejectMsg, 1, 0, approvalStatus)
						messages = append(messages, llm.Message{
							Role:       llm.RoleTool,
							Name:       toolName,
							ToolCallID: tc.ID,
							Content:    rejectMsg,
						})
						continue
					}
					approvalStatus = "APPROVED"
				}
			}

			// Execute tool via Registry
			res, err := a.registry.Execute(ctx, toolName, rawArgs)
			exitCode := 0
			if !res.Success {
				exitCode = 1
			}

			a.recordAudit(task.ID, task.SessionID, toolName, string(rawArgs), res.Output, exitCode, res.DurationMs, approvalStatus)

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

func (a *Agent) finishTask(task *Task, status TaskStatus, result string, errMsg string) {
	a.mu.Lock()
	task.Status = status
	task.Result = result
	task.Error = errMsg
	task.UpdatedAt = time.Now()
	a.mu.Unlock()

	a.emitEvent(task, Event{
		Type:      EventTaskCompleted,
		TaskID:    task.ID,
		SessionID: task.SessionID,
		Status:    status,
		Message:   result,
		Error:     errMsg,
	})
}

func (a *Agent) recordAudit(taskID, sessionID, tool string, input any, result any, exitCode int, durationMs int64, approval string) {
	if a.auditLogger != nil {
		_ = a.auditLogger.Log(audit.Entry{
			TaskID:         taskID,
			SessionID:      sessionID,
			Tool:           tool,
			Input:          input,
			Result:         result,
			ExitCode:       exitCode,
			DurationMs:     durationMs,
			ApprovalStatus: approval,
		})
	}
}

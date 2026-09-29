# Bob Architecture

Bob is a personal AI computer agent designed to run locally on an Apple Silicon Mac, reasoning through tasks using a local LLM and controlling the machine via secure tools (terminal, filesystem, and display capture).

```
                    REMOTE USER
                        │
                        ▼
                ┌───────────────┐
                │ Remote Web UI  │
                │ / Chat Client │
                └───────┬───────┘
                        │
                  Tailscale network
                        │
                        ▼
              ┌─────────────────────┐
              │    Bob API Server   │
              │                     │
              │ Authentication      │
              │ Sessions            │
              │ Streaming (WS/SSE)  │
              │ Task management     │
              └──────────┬──────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │    Bob Agent Core   │
              │                     │
              │ Planner Loop        │
              │ Tool Dispatcher     │
              │ Context / History   │
              │ Security Sandbox    │
              └──────────┬──────────┘
                         │
                         ▼
              ┌─────────────────────┐
              │    Local LLM        │
              │                     │
              │ Ollama (qwen2.5)    │
              │ OpenAI Compatible   │
              └──────────┬──────────┘
                         │
                         ▼
                 ┌───────────────┐
                 │ Tool Registry │
                 └───────┬───────┘
                         │
        ┌────────────────┼─────────────────┐
        ▼                ▼                 ▼
  Terminal Tool    Filesystem Tool   Screenshot Tool
  (policy/exec)    (workspace check)  (screencapture)
        │                │                 │
        └────────────────┼─────────────────┘
                         │
                         ▼
                    macOS Host
```

---

## Component Deep Dive

### 1. Agent Planner & Execution Loop (`internal/agent`)
The agent loop implements a controlled cycle:
1. **Context Assembly**: Concatenates Bob's system persona, current session conversation history, and the user's task prompt.
2. **LLM Invocation**: Emits `agent.thinking` event and passes available tool definitions to the local model.
3. **Tool Call Parsing**: If the LLM generates one or more tool calls:
   - Validates command against `internal/security/policy.go`.
   - If `APPROVAL_REQUIRED`, pauses task state and sends `tool.approval_required` event over WebSocket/SSE to human operator.
   - If `SAFE` or `APPROVED`, dispatches execution through `internal/tools/registry`.
4. **Audit Logging**: Structured JSON entry written with tool, parameters, duration, exit code, and approval state.
5. **Tool Feedback**: Results returned as `tool` messages to the LLM.
6. **Cycle Repeats**: Loop terminates when LLM produces a final assistant message or hits `max_steps` (default 20).

### 2. Local LLM Layer (`internal/llm`)
Abstracted behind:
```go
type LLM interface {
    Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
    Stream(ctx context.Context, request ChatRequest) (<-chan StreamEvent, error)
    Name() string
}
```
- `OllamaLLM`: Interacts directly with Ollama's local HTTP API (`http://127.0.0.1:11434/api/chat`).
- `MockLLM`: Used for deterministic, hermetic unit tests.

### 3. Tool System & Registry (`internal/tools`)
All capabilities must implement:
```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() any
    Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}
```
- `terminal_exec`: Shell execution with working directory sandboxing, timeout limits, output truncation, and command policy verification.
- `read_file`, `write_file`, `list_directory`, `search_files`: Filesystem operations restricted to approved directories (`~/Projects`, `~/Documents`, etc.) with symlink escape prevention.
- `take_screenshot`: macOS `/usr/sbin/screencapture` wrapper.

### 4. Computer Control Abstraction (`internal/computer`)
```go
type Computer interface {
    Screenshot(ctx context.Context) (ScreenshotResult, error)
    MoveMouse(ctx context.Context, x, y int) error
    Click(ctx context.Context, x, y int) error
    Type(ctx context.Context, text string) error
    KeyPress(ctx context.Context, key string) error
    Scroll(ctx context.Context, x, y int) error
}
```
Currently `Screenshot` is implemented; mouse/keyboard automation methods return `ErrNotImplemented` to define future GUI automation boundaries cleanly.

### 5. Coding Agent Abstraction (`internal/coding`)
```go
type CodingAgent interface {
    Name() string
    Available() (bool, string)
    Execute(ctx context.Context, request CodingTask) (CodingResult, error)
}
```
Defines delegation boundaries for software development subagents like AntiGravity.

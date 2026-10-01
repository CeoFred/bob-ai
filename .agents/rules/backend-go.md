# Bob-AI Backend (Go) Development Rules

- **Idiomatic Go & Structure**:
  - Follow standard Go conventions (gofmt, golint). Keep packages in `internal/` with clear responsibility boundaries.
  - No global mutable state. Inject dependencies (Loggers, Configs, LLM Clients, Tool Registries) explicitly via constructors (e.g. `NewAgent()`, `NewServer()`).

- **Concurrency & State Safety**:
  - Guard concurrent state modifications in `Agent`, `SessionManager`, `Registry`, and `Server` with `sync.RWMutex` or `sync.Mutex`.
  - Always respect `context.Context` cancellation across long-running tasks, tool executions, and HTTP/WebSocket connections.

- **Tool Implementation Contract**:
  - Every tool in `internal/tools/` must implement `registry.Tool`:
    - `Name() string`
    - `Description() string`
    - `InputSchema() json.RawMessage`
    - `Execute(ctx context.Context, input json.RawMessage) (registry.ToolResult, error)`
  - Schema must be valid JSON Schema Draft-07/2020-12 so LLMs parse tool definitions accurately.

- **Hermetic Unit & Integration Testing**:
  - Unit tests must never depend on external network services or running Ollama instances.
  - Use `internal/llm/mock.go` (`NewMockLLM()`) to test agent reasoning loops, step limit enforcement, and tool extraction.
  - Run `go test -v ./...` to verify all test suites before concluding backend changes.

# 🤖 Bob — Local AI Computer Agent

**Bob** is a secure, personal AI computer agent that runs locally on an Apple Silicon Mac. It connects to a local LLM (such as Ollama with Qwen 2.5 Coder), exposes a clean remote web interface accessible over Tailscale, and executes controlled terminal commands, filesystem operations, and display captures with human-in-the-loop security policies.

---

## Key Features

- **100% Local Inference**: Zero dependency on cloud LLMs; powered by local Ollama (`qwen2.5-coder:7b` / `qwen2.5-coder:14b` / `llama3.1:8b`).
- **Secure Remote Access**: Connect from iPhone or remote laptop via Tailscale mesh VPN without public ports.
- **Controlled Tool Execution**:
  - `terminal_exec` — Safe zsh shell command runner with duration and exit code capture.
  - `read_file`, `write_file`, `list_directory`, `search_files` — Sandboxed workspace file operations.
  - `take_screenshot` — macOS display capture.
- **Security & Safety Policy Layer**:
  - Command classification (`SAFE`, `APPROVAL_REQUIRED`, `BLOCKED`).
  - Path canonicalization preventing directory traversal (`..`) and symlink escapes.
  - Interactive web UI approval prompts for sensitive commands.
  - Automatic secret & token redaction in audit logs.
- **Real-Time Streaming**: Live WebSocket and Server-Sent Events (SSE) streaming of agent reasoning, tool starts, tool outputs, and completions.
- **macOS Daemon & launchd Service**: Runs in the background without needing an open terminal window.
- **Future-Ready Extensibility**:
  - `Computer` GUI control interface ready for mouse/keyboard automation.
  - `CodingAgent` interface ready for AntiGravity delegation.

---

## Quick Start

### 1. Hardware Detection & Recommendation
```bash
make sysinfo
```

### 2. Pull Recommended Local Model
```bash
ollama serve
ollama pull qwen2.5-coder:7b
```

### 3. Build & Run
```bash
make build
make run
```
Access Bob at `http://localhost:8787` (or your Tailscale IP `http://100.x.y.z:8787`).

---

## CLI Management

```bash
./scripts/bob start      # Start background daemon
./scripts/bob status     # Check health & status
./scripts/bob logs       # Stream daemon logs
./scripts/bob stop       # Stop background daemon
./scripts/bob sysinfo    # Inspect hardware & recommended models
```

---

## Project Structure

```
bob/
├── cmd/
│   └── bob/
│       └── main.go              # Main server & agent entrypoint
│
├── internal/
│   ├── agent/                   # Planner loop, max steps, event streaming
│   ├── llm/                     # LLM interface, Ollama client, Mock LLM
│   ├── tools/
│   │   ├── registry/            # Tool registry & schema builder
│   │   ├── terminal/            # Terminal execution tool
│   │   ├── filesystem/          # Sandboxed file read/write/list/search
│   │   └── screenshot/          # macOS screen capture tool
│   ├── computer/                # Computer control interface abstraction
│   ├── coding/                  # AntiGravity coding subagent interface
│   ├── security/                # Policy rules, path sandbox, secret redactor
│   ├── sessions/                # Session manager & history persistence
│   ├── audit/                   # Structured JSON audit logging
│   ├── server/                  # HTTP REST API, SSE, WebSockets, Auth
│   └── config/                  # Configuration & hardware auto-detection
│
├── web/                         # React + TypeScript + Vite remote web UI
│   ├── src/
│   │   ├── components/          # Chat, ToolActivity, Approval, Audit, Modals
│   │   ├── services/            # API client, WebSocket stream client
│   │   └── App.tsx
│   └── dist/                    # Compiled production UI bundle
│
├── scripts/
│   ├── bob                      # CLI daemon manager
│   └── install-service.sh       # Native macOS launchd service installer
│
├── service/
│   └── com.bob.agent.plist      # launchd service definition
│
├── docs/
│   ├── architecture.md          # System architecture & component design
│   ├── setup.md                 # Setup & quickstart guide
│   ├── security.md              # Security policies & defense in depth
│   ├── tailscale.md             # Tailscale remote networking guide
│   └── antigravity.md           # AntiGravity integration & findings
│
├── .env.example
├── config.example.yaml
├── Makefile
└── README.md
```

---

## Testing

Run all unit and integration tests:
```bash
make test
```
All tests use hermetic mocks for the LLM and pass without requiring external services.

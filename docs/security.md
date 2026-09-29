# Bob Security Architecture & Philosophy

## Security Philosophy

> **The LLM is untrusted.** The LLM decides what action it wants to take, but the host security policy layer decides whether that action is permissible.

Bob enforces multi-layer defense-in-depth:

```
LLM tool request
      ↓
[Tool Registry]
      ↓
[Command / Path Policy Layer]
      ├── BLOCKED ───────────→ Immediate Rejection & Audit Log
      ├── APPROVAL_REQUIRED ─→ Pause Task → Human Approval in UI
      └── SAFE ──────────────→ Sandbox Execution → Audit Log
```

---

## 1. Command Execution Policy

Commands submitted to `terminal_exec` are evaluated by `internal/security/policy.go`:

| Classification | Behavior | Examples |
|---|---|---|
| **SAFE** | Auto-executed | `pwd`, `ls`, `git status`, `git diff`, `git log`, `go test`, `echo`, `uname` |
| **APPROVAL_REQUIRED** | Pauses task; requires user confirmation in UI | `rm`, `mv`, `chmod`, `sudo`, `launchctl`, `diskutil`, `brew install`, `npm install`, arbitrary scripts |
| **BLOCKED** | Immediately denied | `rm -rf /`, `mkfs`, fork bombs (`:(){ :|:& };:`), raw disk write (`dd if=...`) |

### Chained Command Protection
Commands containing chaining operators (`;`, `&&`, `||`, backticks, `$()`) are automatically classified as `APPROVAL_REQUIRED` or `BLOCKED` to prevent hidden subshell injections.

---

## 2. Filesystem Sandboxing & Workspace Restriction

Filesystem operations (`read_file`, `write_file`, `list_directory`, `search_files`) and working directories for terminal commands are validated by `internal/security/sandbox.go`:

- Paths must resolve strictly within `allowed_paths` (e.g. `~/Projects`, `~/Documents`, `~/BobWorkspace`).
- Prevents `..` path traversal attacks.
- Resolves all symlinks via `filepath.EvalSymlinks` to prevent symlink traversal escapes outside the workspace.

---

## 3. Redaction & Audit Trail

Every tool execution generates an immutable JSON audit log in `~/.bob/audit.jsonl`:
- Includes timestamp, task ID, session ID, tool name, input parameters, result summary, exit code, execution duration, and approval status.
- **Redactor (`internal/security/redaction.go`)**: Automatically strips API tokens, `Bearer` authorization headers, and password parameters before logging or exposing in status responses.

---

## 4. Mac State & Locked Sessions

Bob runs as a daemon and does not depend on the display being awake for:
- Terminal commands
- Filesystem tasks
- Task queuing
- API serving

**Note on Locked Mac vs Asleep Mac:**
- **Mac Locked + Awake**: Background daemon continues processing; screenshots will capture the lock screen or active window buffer.
- **Mac Asleep**: Hardware sleep halts background execution unless macOS `caffeinate` or power settings keep network and CPU active.

# Bob-AI Security & Sandboxing Rules

- **Untrusted Model Assumption**:
  Always assume local and remote LLM outputs are untrusted. Never allow direct unvalidated execution of shell commands, unrestricted file access, or unapproved destructive actions.

- **Command Policy Compliance**:
  - Commands categorized as `BLOCKED` (e.g. `rm -rf /`, raw disk formatting, fork bombs, destructive kernel parameters) must never be permitted or bypassed under any circumstances.
  - Commands categorized as `APPROVAL_REQUIRED` (e.g. `git push`, system modifications, package installations, service alterations) must explicitly go through Bob's approval flow (`PendingApproval` event state) before execution.
  - Safe inspection commands (`SAFE`, e.g. `ls`, `git status`, `sw_vers`, `pwd`) execute directly without blocking.
  - Disallow chained command injections (`&&`, `;`, `||`, `|`, `` ` ``) when passing raw parameters into shell templates.

- **Filesystem Sandbox Boundaries**:
  - All file read/write/list operations must be constrained within configured allowed workspace roots (`config.filesystem.allowed_paths`).
  - Always canonicalize paths using `filepath.Clean` and `filepath.EvalSymlinks` before boundary checks to prevent `../` directory traversal or symlink escapes.

- **Secret & PII Redaction**:
  - API keys, bearer tokens, OAuth secrets, private keys, and passwords must be scrubbed by `internal/security/redaction.go` before logging to audit logs or streaming over WebSocket/SSE.
  - Audit logs in `.bob_data/audit.jsonl` must remain immutable and append-only.

- **Network Exposure**:
  - Bob API and Web servers must only bind to `127.0.0.1` (localhost) or designated Tailscale WireGuard interface addresses.
  - Never bind unsecured HTTP endpoints to `0.0.0.0` on public networks without an authentication token configured.

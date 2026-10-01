---
name: bob-build-and-test
description: >-
  Use this skill when building Bob's Vite frontend, compiling the Go backend binary,
  running the Go unit and integration test suite, or executing full project verification.
---

# Bob Build & Test Skill

Runbooks and procedures for compiling and validating the Bob AI agent repository.

## Quick Commands

- **Full Project Build**: `make build` (compiles web frontend + Go binary to `bin/bob`)
- **Run All Tests**: `make test` (`go test -v ./...`)
- **Dev Mode**: `make dev` (runs Go backend directly)
- **Inspect Hardware**: `make sysinfo`

## Step-by-Step Build & Verification Procedure

1. **Frontend Compilation**:
   ```bash
   cd web && npm run build
   ```
   Ensures TypeScript types pass and bundle is emitted to `web/dist/`.

2. **Backend Compilation**:
   ```bash
   mkdir -p bin
   go build -o bin/bob cmd/bob/main.go
   ```
   Ensures Go compiler builds the binary cleanly.

3. **Run Hermetic Unit & Integration Tests**:
   ```bash
   go test -v ./...
   ```
   Verifies:
   - Agent planner loop & step limiting (`internal/agent/`)
   - Tool execution & sandboxed filesystem (`internal/tools/`)
   - Security policies, chained command blocking, and path traversal guards (`internal/security/`)
   - Audit logging & secret redaction (`internal/audit/`, `internal/security/`)
   - HTTP/WebSocket API routing & auth middleware (`internal/server/`)

4. **Verify Clean Exit**:
   Ensure all tests report `PASS` with zero failures.

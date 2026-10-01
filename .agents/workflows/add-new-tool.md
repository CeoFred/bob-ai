# Workflow: Add a New Tool to Bob

Follow this multi-step workflow when introducing a new tool to the Bob agent ecosystem.

## Steps

1. **Define the Tool Contract**:
   Create a new package under `internal/tools/<name>/`.
   Implement `registry.Tool` (`Name`, `Description`, `InputSchema`, `Execute`).

2. **Configure Security Classification**:
   Edit `internal/security/policy.go` to assign the appropriate command risk level (`SAFE`, `APPROVAL_REQUIRED`, or `BLOCKED`).

3. **Register Tool in Engine**:
   In `cmd/bob/main.go`, instantiate and register the tool in `toolRegistry.Register(...)`.

4. **Add Unit Tests**:
   Create unit tests covering valid executions, edge cases, and invalid schemas.
   Run:
   ```bash
   go test -v ./internal/tools/...
   ```

5. **Update Knowledge Graph**:
   Update graphify to register new structural nodes and call relationships:
   ```bash
   $(cat graphify-out/.graphify_python) -m graphify --update
   ```

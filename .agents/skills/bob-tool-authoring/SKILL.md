---
name: bob-tool-authoring
description: >-
  Use this skill when adding a new tool to Bob's agent registry, defining JSON input schemas,
  configuring security policy classifications, or writing tool unit tests.
---

# Bob Tool Authoring Skill

Step-by-step guidelines for implementing, securing, and registering new tools in Bob.

## Tool Interface Definition

All tools implement the `Tool` interface from `internal/tools/registry/registry.go`:

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() json.RawMessage
    Execute(ctx context.Context, input json.RawMessage) (ToolResult, error)
}
```

## Step 1: Create Tool Package

Create a package under `internal/tools/<tool_name>/`:
- Define the input struct and serialize its JSON schema in `InputSchema()`.
- Implement `Execute(ctx context.Context, input json.RawMessage) (registry.ToolResult, error)`.
- Use `registry.ToolResult{Output: "...", Metadata: map[string]interface{}{...}}`.

## Step 2: Define Security Classification

Add the tool or its command patterns to `internal/security/policy.go`:
- `SAFE`: Non-destructive read/inspection commands (e.g. read files, list dirs, view status).
- `APPROVAL_REQUIRED`: Destructive, network-pushing, or persistent changes requiring user approval via UI.
- `BLOCKED`: Dangerous system commands that are unconditionally prohibited.

## Step 3: Register Tool in Main Entrypoint

Register the new tool instance in `cmd/bob/main.go`:
```go
toolRegistry.Register(mytool.New(...))
```

## Step 4: Write Unit Tests

Add tests in `internal/tools/<tool_name>/<tool_name>_test.go` or `internal/tools/tools_test.go`:
- Test valid input parameters.
- Test malformed JSON inputs.
- Test error conditions and sandbox boundaries.
- Run `go test -v ./internal/tools/...` to verify.

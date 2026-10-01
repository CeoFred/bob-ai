---
name: bob-antigravity-adapter
description: >-
  Use this skill when extending the AntiGravity CodingAgent adapter, integrating computer automation
  interfaces, or reviewing the AntiGravity delegation architecture in Bob.
---

# Bob AntiGravity Adapter Skill

Architecture and guidelines for Bob's integration with Google AntiGravity and autonomous coding agents.

## Architectural Context

Bob provides an extensible interface abstraction in `internal/coding/coding.go`:
```go
type CodingAgent interface {
    Name() string
    Available() bool
    Execute(ctx context.Context, task CodingTask) (CodingResult, error)
}
```

And `AntiGravityCodingAgent` in `internal/coding/coding.go`:
- Adapts AntiGravity capabilities into Bob's agent planning loop.
- Encapsulates complex refactoring, multi-file code editing, and external coding workflows.

## AntiGravity Boundary Rules (`docs/antigravity.md`)

- **Host Inspection**: Bob inspects available local tools, git repositories, and dependencies before delegating tasks.
- **Explicit Interfaces**: Only invoke officially documented APIs, CLI interfaces, and workspace hooks.
- **Safety Policy**: Delegated subagents operate under Bob's sandbox bounds; any code modifications or tool runs remain subject to audit logging in `.bob_data/audit.jsonl`.

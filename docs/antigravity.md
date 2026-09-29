# AntiGravity Integration Findings & Architecture Boundary

## Executive Summary

Bob includes a dedicated abstraction layer (`internal/coding`) designed to delegate software engineering, repository refactoring, and code debugging tasks to AntiGravity when an official CLI or automation API is exposed.

---

## Local Inspection Findings

Inspection of the host system (`macOS 26.6 / Apple Silicon M4 Pro`) performed during initialization:

| Inspection Check | Result | Detail |
|---|---|---|
| `which agy` | Not found | No `agy` binary in default `$PATH` |
| `which antigravity` | Not found | No `antigravity` headless CLI in `$PATH` |
| `/Applications/Antigravity.app` | Present | Desktop GUI bundle installed |
| `~/Library/Application Support/Antigravity` | Present | Application storage and configurations exist |

### Policy on Private/Undocumented APIs
Per architecture guidelines:
1. Bob **does not** reverse engineer internal database files, scrape private IPC channels, or hook unauthorized background processes.
2. Bob establishes a clean `CodingAgent` interface (`internal/coding/coding.go`) ready for official CLI or local API binding.

---

## Architecture Integration Boundary

Bob provides the `CodingAgent` interface:

```go
type CodingTask struct {
    Prompt      string   `json:"prompt"`
    RepoPath    string   `json:"repo_path"`
    TargetFiles []string `json:"target_files,omitempty"`
    TimeoutSec  int      `json:"timeout_seconds,omitempty"`
}

type CodingResult struct {
    Success       bool     `json:"success"`
    Summary       string   `json:"summary"`
    ModifiedFiles []string `json:"modified_files,omitempty"`
    Logs          string   `json:"logs,omitempty"`
    DurationMs    int64    `json:"duration_ms"`
}

type CodingAgent interface {
    Name() string
    Available() (bool, string)
    Execute(ctx context.Context, request CodingTask) (CodingResult, error)
}
```

---

## How to Enable AntiGravity Delegation Later

When AntiGravity CLI (`agy`) or a local agent API endpoint becomes available in your environment:

1. Ensure the binary is linked in your path:
   ```bash
   ln -s /Applications/Antigravity.app/Contents/Resources/bin/agy /usr/local/bin/agy
   ```
2. Enable AntiGravity integration in `config.yaml`:
   ```yaml
   coding_agent:
     provider: "antigravity"
     enabled: true
   ```
3. Bob will automatically detect `agy` at startup and route coding requests (e.g. *"Fix the failing unit tests in repo X"*) directly to the AntiGravity subagent engine.

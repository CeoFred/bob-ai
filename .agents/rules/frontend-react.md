# Bob-AI Frontend (React/TypeScript) Rules

- **Stack & Architecture**:
  - React 19 + TypeScript + Vite + Tailwind CSS v4 in `web/`.
  - Component hierarchy: `web/src/components/` for modular UI elements, `web/src/services/` for WebSocket and HTTP API client logic.

- **Streaming & Real-time State**:
  - Maintain clean WebSocket event lifecycle handling (`CONNECTING`, `OPEN`, `CLOSED`, `RECONNECTING`).
  - Handle all Bob Agent event types (`step_start`, `thought`, `tool_call_start`, `tool_call_result`, `approval_required`, `task_complete`, `error`).
  - Automatically scroll and update streaming chat views with minimal unnecessary re-renders.

- **Human-in-the-Loop Approval UI**:
  - Prompt user with `ApprovalBanner.tsx` when `approval_required` events arrive.
  - Present explicit command text, security risk level, and reasons for approval. Include clear Approve and Deny action buttons.

- **Build Validation**:
  - Always verify TypeScript types and build artifacts in `web/` with:
    ```bash
    cd web && npm run build
    ```

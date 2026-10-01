# Workflow: Deploy Local Daemon

Runbook for preparing, building, and running Bob as a background service on macOS.

## Steps

1. **Inspect Host Hardware & Verify Ollama**:
   ```bash
   make sysinfo
   ollama list
   ```
   Ensure the recommended model (e.g. `qwen2.5-coder:7b`) is pulled.

2. **Build Full Project**:
   ```bash
   make build
   ```

3. **Verify Configuration**:
   Check `config.yaml` or `.env` for host, port, LLM model name, and security sandbox settings.

4. **Launch Background Service**:
   Option A (via CLI script):
   ```bash
   ./scripts/bob start
   ./scripts/bob status
   ```

   Option B (via launchd agent):
   ```bash
   ./scripts/install-service.sh install
   ```

5. **Verify Web Console Access**:
   Open `http://localhost:8787` in your browser and confirm WebSocket connection status.

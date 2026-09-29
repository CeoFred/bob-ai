# Bob Setup & Quickstart Guide

This guide walks you through setting up and running Bob on an Apple Silicon Mac.

---

## Prerequisites

1. **macOS**: macOS 13+ (Ventura, Sonoma, Sequoia or newer) on Apple Silicon (M1/M2/M3/M4).
2. **Go Toolchain**: Go 1.22+ (`brew install go`)
3. **Node.js**: Node 18+ (`brew install node`)
4. **Ollama**: Local LLM runner (`brew install ollama`)
5. **Tailscale**: Secure mesh networking (`brew install --cask tailscale` or App Store)

---

## Step 1: Detect Hardware & Model Recommendation

Run the built-in sysinfo tool to inspect your Mac and get recommended local models:

```bash
make sysinfo
```

*Example output on an M4 Pro (24 GB RAM):*
```
OS:             macOS 26.6 (arm64)
Apple Silicon:  Apple M4 Pro
Memory (RAM):   24.0 GB
Disk Available: 41Gi
Ollama Service: true

Recommended Local Models for this hardware:
 - qwen2.5-coder:7b     (Fast, precise tool calling, ~5.5 GB RAM)
 - qwen2.5-coder:14b    (Stronger reasoning, ~9 GB RAM)
 - llama3.1:8b          (General reasoning, ~6 GB RAM)
```

---

## Step 2: Start Ollama and Pull Recommended Model

Start the Ollama daemon:
```bash
ollama serve
```

In a separate terminal, pull your preferred model:
```bash
ollama pull qwen2.5-coder:7b
```

---

## Step 3: Build Bob

Build both the React web UI and the Go server binary:
```bash
make build
```

This compiles:
- `web/dist/` — React frontend assets
- `bin/bob` — Compiled native Go binary

---

## Step 4: Configure Bob

Copy the example configuration:
```bash
cp config.example.yaml config.yaml
```

Optionally set an API token in `config.yaml` or `.env`:
```bash
export BOB_API_TOKEN="my-secret-token"
```

---

## Step 5: Run Bob

### Foreground Development Mode:
```bash
make dev
# or
./bin/bob
```

### Background Daemon via CLI:
```bash
./scripts/bob start
./scripts/bob status
./scripts/bob logs
./scripts/bob stop
```

### Background Service via macOS launchd:
```bash
make service-install
# To uninstall later:
make service-uninstall
```

---

## Step 6: macOS Permissions

Bob requires specific macOS system permissions for screen capture:

1. Open **System Settings** > **Privacy & Security** > **Screen & System Audio Recording**.
2. If running Bob in terminal (e.g. iTerm or Terminal), grant **Screen Recording** access to your terminal app.
3. If running as a launchd service, grant **Screen Recording** access to `/usr/sbin/screencapture` or Bob's binary.

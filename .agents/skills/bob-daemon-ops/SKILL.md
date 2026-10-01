---
name: bob-daemon-ops
description: >-
  Use this skill when managing Bob background daemon processes, configuring macOS launchd services,
  troubleshooting logs, or setting up Tailscale remote mesh networking.
---

# Bob Daemon Operations Skill

Operational guide for managing Bob as a macOS daemon and background launchd service.

## Daemon CLI Management

The helper script `./scripts/bob` wraps daemon lifecycle management:

```bash
./scripts/bob start      # Start daemon in background (PID recorded in .bob_data/bob.pid)
./scripts/bob status     # Check process health and port binding
./scripts/bob logs       # Tail logs from .bob_data/bob.log
./scripts/bob stop       # Send SIGTERM and wait for clean shutdown
./scripts/bob sysinfo    # Hardware inspection and recommended LLM parameters
```

## Launchd Service Installation

To keep Bob running across user logins and restarts:

1. **Build the binary**:
   ```bash
   make build
   ```

2. **Install Service**:
   ```bash
   ./scripts/install-service.sh install
   ```
   Copies `service/com.bob.agent.plist` to `~/Library/LaunchAgents/` and loads it via `launchctl`.

3. **Check Service Status**:
   ```bash
   launchctl list | grep com.bob.agent
   ```

4. **Uninstall Service**:
   ```bash
   ./scripts/install-service.sh uninstall
   ```

## Tailscale Remote Access

1. Ensure Tailscale is running on the macOS host:
   ```bash
   tailscale ip -4
   ```
2. Configure `config.yaml` with host binding:
   ```yaml
   server:
     host: "0.0.0.0" # Or specific 100.x.y.z Tailscale IP
     port: 8787
   ```
3. Connect securely from mobile/laptop at `http://<tailscale-ip>:8787`.

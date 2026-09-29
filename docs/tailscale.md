# Tailscale Remote Access Guide

Bob uses **Tailscale** to enable secure remote access from your phone or secondary computer without exposing ports to the public internet.

```
 Remote Device (Phone / Laptop)
             │
             ▼
      Tailscale Mesh (WireGuard VPN)
             │
             ▼
        Mac Host (Bob)
   http://100.x.y.z:8787
```

---

## 1. Install & Setup Tailscale

1. **On your Mac**:
   - Install Tailscale via `brew install --cask tailscale` or download from [Tailscale.com](https://tailscale.com/download).
   - Sign in to your Tailscale account.

2. **On your Phone / Remote Device**:
   - Install the Tailscale app (iOS App Store or Google Play Store).
   - Sign in to the **same** Tailscale account (tailnet).

---

## 2. Locate your Mac's Tailscale IP

Run the following command on your Mac:
```bash
tailscale ip -4
# or
./scripts/bob sysinfo
```
*Example IP:* `100.115.92.45`

Or check your Tailscale hostname:
```bash
tailscale status
```
*Example Hostname:* `bobs-macbook.tailnet-xyz.ts.net`

---

## 3. Connect to Bob Remotely

1. Start Bob on your Mac:
   ```bash
   ./scripts/bob start
   ```
2. Open your mobile browser or laptop browser on the same Tailscale network:
   ```
   http://<mac-tailscale-ip>:8787
   ```
   *(e.g., `http://100.115.92.45:8787`)*

3. If an API token is configured, append `?token=<your_token>` or authenticate when prompted.

---

## 4. Enabling Tailscale HTTPS (MagicDNS + TLS)

If you prefer full TLS encryption on Tailscale:
```bash
tailscale serve --bg 8787
```
Tailscale will automatically provision a Let's Encrypt TLS certificate for your Tailscale machine name, allowing you to browse at:
```
https://bobs-macbook.tailnet-xyz.ts.net
```

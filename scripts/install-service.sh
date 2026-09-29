#!/usr/bin/env bash
# Install Bob as a native macOS launchd user agent
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PLIST_NAME="com.bob.agent.plist"
SOURCE_PLIST="$ROOT_DIR/service/$PLIST_NAME"
TARGET_DIR="$HOME/Library/LaunchAgents"
TARGET_PLIST="$TARGET_DIR/$PLIST_NAME"

ACTION="${1:-install}"

mkdir -p "$TARGET_DIR"
mkdir -p "$HOME/.bob"

case "$ACTION" in
  install)
    echo "Installing Bob launchd user agent..."
    # Copy binary to ~/.bob/bin/bob or /usr/local/bin/bob
    mkdir -p "$HOME/.bob/bin"
    cp "$ROOT_DIR/bin/bob" "$HOME/.bob/bin/bob" 2>/dev/null || (cd "$ROOT_DIR" && go build -o "$HOME/.bob/bin/bob" cmd/bob/main.go)

    # Generate custom plist with current user and paths
    cat <<EOF > "$TARGET_PLIST"
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.bob.agent</string>
    <key>ProgramArguments</key>
    <array>
        <string>$HOME/.bob/bin/bob</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$HOME/.bob/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>$HOME/.bob/stderr.log</string>
    <key>WorkingDirectory</key>
    <string>$ROOT_DIR</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
        <key>LANG</key>
        <string>en_US.UTF-8</string>
    </dict>
</dict>
</plist>
EOF

    echo "Loading launchd service..."
    launchctl unload "$TARGET_PLIST" 2>/dev/null || true
    launchctl load "$TARGET_PLIST"
    echo "✓ Bob service successfully installed and started!"
    echo "• Status: launchctl list | grep com.bob.agent"
    echo "• Logs:   tail -f ~/.bob/stdout.log"
    ;;

  uninstall)
    echo "Uninstalling Bob launchd service..."
    launchctl unload "$TARGET_PLIST" 2>/dev/null || true
    rm -f "$TARGET_PLIST"
    echo "✓ Bob launchd service uninstalled."
    ;;

  *)
    echo "Usage: ./install-service.sh {install|uninstall}"
    exit 1
    ;;
esac

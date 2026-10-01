#!/usr/bin/env bash
# Bob Development Runner - Live reload for Backend (Air) & Frontend (Vite)
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

# Check for air binary
AIR_BIN=""
if command -v air >/dev/null 2>&1; then
  AIR_BIN="air"
elif [ -f "$(go env GOPATH)/bin/air" ]; then
  AIR_BIN="$(go env GOPATH)/bin/air"
fi

# Ensure web dependencies are installed
if [ ! -d "$ROOT_DIR/web/node_modules" ]; then
  echo "📦 Installing frontend dependencies..."
  (cd "$ROOT_DIR/web" && npm install)
fi

echo "====================================================="
echo "   🚀 Starting Bob in Full-Stack Auto-Reload Dev Mode"
echo "====================================================="
echo " • Frontend (Vite HMR):  http://localhost:5173"
echo " • Backend API & WS:     http://localhost:8787"
if [ -n "$AIR_BIN" ]; then
  echo " • Backend Auto-Reload:  Active (via Air)"
else
  echo " • Backend Auto-Reload:  Inactive (Air not found, using 'go run')"
  echo "   (Install Air for Go hot-reload: go install github.com/air-verse/air@latest)"
fi
echo "====================================================="
echo "Press Ctrl+C to stop both servers."
echo ""

# Cleanup on exit
cleanup() {
  echo ""
  echo "🛑 Stopping dev servers..."
  kill $(jobs -p) 2>/dev/null || true
  wait 2>/dev/null || true
  echo "✓ Dev servers stopped."
}
trap cleanup EXIT INT TERM

# Start Backend
if [ -n "$AIR_BIN" ]; then
  $AIR_BIN &
  BACKEND_PID=$!
else
  go run cmd/bob/main.go &
  BACKEND_PID=$!
fi

# Start Frontend
(cd "$ROOT_DIR/web" && npm run dev) &
FRONTEND_PID=$!

# Wait for both processes
wait

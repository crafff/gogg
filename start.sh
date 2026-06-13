#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

PID_FILE=".pids"
LOG_DIR="logs"
mkdir -p "$LOG_DIR"

if [[ -f "$PID_FILE" ]]; then
  echo "Already running? Found $PID_FILE — run ./stop.sh first."
  exit 1
fi

# Build backend
echo "Building backend..."
go build -o gogg .

# Start backend
echo "Starting backend (port 8080)..."
./gogg serve >> "$LOG_DIR/server.log" 2>&1 &
BACKEND_PID=$!

# Wait briefly and check it didn't crash immediately
sleep 0.5
if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
  echo "Backend failed to start. Check $LOG_DIR/server.log"
  exit 1
fi

# Start frontend dev server
echo "Starting frontend dev server (port 5173)..."
cd web
npm run dev >> "../$LOG_DIR/web-dev.log" 2>&1 &
FRONTEND_PID=$!
cd ..

sleep 0.5
if ! kill -0 "$FRONTEND_PID" 2>/dev/null; then
  echo "Frontend failed to start. Check $LOG_DIR/web-dev.log"
  kill "$BACKEND_PID" 2>/dev/null || true
  exit 1
fi

echo "$BACKEND_PID $FRONTEND_PID" > "$PID_FILE"

echo ""
echo "Started:"
echo "  Backend:  http://localhost:8080  (PID $BACKEND_PID, log: $LOG_DIR/server.log)"
echo "  Frontend: http://localhost:5173  (PID $FRONTEND_PID, log: $LOG_DIR/web-dev.log)"
echo ""
echo "Run ./stop.sh to stop."

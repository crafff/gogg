#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

PID_FILE=".pids"

if [[ ! -f "$PID_FILE" ]]; then
  echo "No $PID_FILE found — nothing to stop."
  exit 0
fi

read -r BACKEND_PID FRONTEND_PID < "$PID_FILE"

stop_pid() {
  local pid=$1
  local name=$2
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid"
    echo "Stopped $name (PID $pid)"
  else
    echo "$name (PID $pid) was already stopped"
  fi
}

stop_pid "$BACKEND_PID"  "backend"
stop_pid "$FRONTEND_PID" "frontend"

rm -f "$PID_FILE"
echo "Done."

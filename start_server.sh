#!/usr/bin/env sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
RUNTIME_DIR="$ROOT_DIR/.runtime"
SERVER_BIN="$RUNTIME_DIR/micro-uac-server"
PID_FILE="$RUNTIME_DIR/micro-uac-server.pid"
STDOUT_LOG="$RUNTIME_DIR/stdout.log"

cd "$ROOT_DIR"

OLD_PID=""
if [ -f "$PID_FILE" ]; then
    OLD_PID=$(tr -d '[:space:]' <"$PID_FILE")
fi

if [ -n "$OLD_PID" ]; then
    kill "$OLD_PID" 2>/dev/null || true
fi

go build ./...
rm -rf "$RUNTIME_DIR"
mkdir -p "$RUNTIME_DIR"
go build -o "$SERVER_BIN" ./cmd/server

nohup "$SERVER_BIN" -config "$ROOT_DIR/config.json" >"$STDOUT_LOG" 2>&1 &
echo $! >"$PID_FILE"
printf 'started pid=%s\n' "$(cat "$PID_FILE")"

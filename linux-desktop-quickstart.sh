#!/usr/bin/env bash
set -euo pipefail

# quickstart helper that mirrors the Windows script but produces a Linux-native binary.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_DIR="$SCRIPT_DIR/desktop-agent"
CONFIG_DIR="$AGENT_DIR/config"
CONFIG_FILE="$CONFIG_DIR/config.json"
EXAMPLE_CONFIG="$CONFIG_DIR/config.example.json"
BIN_DIR="$AGENT_DIR/bin"
AGENT_BINARY="$BIN_DIR/queue-up-agent"

# ensure config directory and file exist
if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "Config missing at $CONFIG_FILE; copying from example..."
  cp "$EXAMPLE_CONFIG" "$CONFIG_FILE"
fi

mkdir -p "$BIN_DIR"

cd "$AGENT_DIR"

# building the Fyne desktop UI needs gcc/cgo on Linux
if command -v gcc >/dev/null 2>&1; then
  echo "gcc detected; building the desktop agent with UI support..."
else
  echo "Warning: gcc not found in PATH. Install build-essential (e.g. apt install build-essential) before running again to enable Fyne UI builds." >&2
fi

CGO_ENABLED=1 go build -o "$AGENT_BINARY" ./cmd/queue-up-agent

echo "Starting queue-up agent from $AGENT_BINARY"
"$AGENT_BINARY" -config "$CONFIG_FILE"

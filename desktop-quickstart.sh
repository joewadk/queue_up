#!/bin/bash
#startup the desktop agent on windows :)

set -euo pipefail #exit on error
#assign global vars for paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_DIR="$SCRIPT_DIR/desktop-agent"
CONFIG_DIR="$AGENT_DIR/config"
CONFIG_FILE="$CONFIG_DIR/config.json"
EXAMPLE_CONFIG="$CONFIG_DIR/config.example.json"

cd "$AGENT_DIR"

# ensure config directory exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Config file not found at $CONFIG_FILE. Creating from example..."
    cp "$EXAMPLE_CONFIG" "$CONFIG_FILE"
fi

#build go binary for windows.
go build -o bin/queue-up-agent.exe ./cmd/queue-up-agent

#run with explicit config path.
bin/queue-up-agent.exe -config "$CONFIG_FILE"

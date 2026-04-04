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

# load repository-level environment overrides (if any)
ENV_FILE="$SCRIPT_DIR/.env"
if [ -f "$ENV_FILE" ]; then
    echo "Loading environment from $ENV_FILE"
    set -o allexport
    # shellcheck disable=SC1090
    source "$ENV_FILE"
    set +o allexport
fi

# ensure config directory exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Config file not found at $CONFIG_FILE. Creating from example..."
    cp "$EXAMPLE_CONFIG" "$CONFIG_FILE"
fi

# build go binary in desktop-agent root 
if command -v gcc >/dev/null 2>&1; then
    echo "gcc detected; building desktop UI-enabled agent..."
    go build -o queue-up-agent.exe ./cmd/queue-up-agent
else #otherwise, build no_gl version of agent to avoid UI dependency issues.
    echo "gcc not found in PATH; building no_gl agent (desktop UI disabled)."
    echo "Install gcc (MinGW-w64/MSYS2) and re-run to enable dashboard UI."
    go build -tags="no_gl" -o queue-up-agent.exe ./cmd/queue-up-agent
fi

#run with explicit config path.
./queue-up-agent.exe -config "$CONFIG_FILE"

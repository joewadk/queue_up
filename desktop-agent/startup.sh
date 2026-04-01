#!/usr/bin/env bash
set -euo pipefail
# thsi script adds the agent to startup apps on windows. 
STARTUP_NAME="QueueUpDesktopAgent"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_EXE="${SCRIPT_DIR}/queue-up-agent.exe"
CONFIG_PATH="${SCRIPT_DIR}/config/config.json"

if [[ ! -f "${AGENT_EXE}" ]]; then
  if [[ -f "${SCRIPT_DIR}/bin/queue-up-agent.exe" ]]; then
    AGENT_EXE="${SCRIPT_DIR}/bin/queue-up-agent.exe"
  else
    echo "Error: agent executable not found at ${AGENT_EXE}" >&2
    exit 1
  fi
fi

if [[ ! -f "${CONFIG_PATH}" ]]; then
  echo "Error: config file not found at ${CONFIG_PATH}" >&2
  exit 1
fi

echo "Enabling startup entry: ${STARTUP_NAME}"
"${AGENT_EXE}" -install-startup -startup-name "${STARTUP_NAME}" -config "${CONFIG_PATH}"

echo
echo "Checking startup status: ${STARTUP_NAME}"
"${AGENT_EXE}" -startup-status -startup-name "${STARTUP_NAME}"

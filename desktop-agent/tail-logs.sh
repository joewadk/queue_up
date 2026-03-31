#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="${SCRIPT_DIR}/logs/enforcement.jsonl"
TAIL_LINES="${TAIL_LINES:-50}"

mkdir -p "$(dirname "${LOG_FILE}")"
touch "${LOG_FILE}"

if command -v tail >/dev/null 2>&1; then
  tail -n "${TAIL_LINES}" -F "${LOG_FILE}"
  exit 0
fi

# Fallback for environments without GNU tail (for example some Windows setups).
if command -v powershell.exe >/dev/null 2>&1; then
  powershell.exe -NoProfile -Command "Get-Content -Path '${LOG_FILE}' -Tail ${TAIL_LINES} -Wait"
  exit 0
fi

echo "No supported log tail command found (need 'tail' or 'powershell.exe')." >&2
exit 1

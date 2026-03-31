#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="${SCRIPT_DIR}/logs/enforcement.jsonl"

mkdir -p "$(dirname "${LOG_FILE}")"
touch "${LOG_FILE}"

tail -f "${LOG_FILE}"

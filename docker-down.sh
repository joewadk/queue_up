#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

#stop and remove containers, networks, and volumes. clean up any dangling volumes too.
docker compose --project-name queue_up --env-file .env -f infra/docker/docker-compose.yml down -v

echo "Queue Up services stopped and volumes removed."

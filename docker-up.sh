#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

ENV_FILE="$SCRIPT_DIR/.env"

#build and start the containers
docker compose --project-name queue_up --env-file .env -f infra/docker/docker-compose.yml up -d --build


echo "Queue Up services started."
echo "Backend health: http://localhost:8080/health"


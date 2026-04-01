#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

ENV_FILE="$SCRIPT_DIR/.env"

#build and start the containers
docker compose --project-name queue_up --env-file .env -f infra/docker/docker-compose.yml up -d --build


echo "Queue Up services started."

#check backend health
echo "Backend health:\n"
curl -i http://localhost:8080/health

#check submission sanitizer health
echo "Submission sanitizer webhook:\n"
curl -i http://localhost:8090/v1/submissions/health

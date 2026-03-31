#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

ENV_FILE="$SCRIPT_DIR/.env"

if [ ! -s "$ENV_FILE" ]; then #if .env doesn't exist or is empty, create it with a random POSTGRES_PASSWORD
  umask 077
  if command -v openssl >/dev/null 2>&1; then
    DB_PASSWORD="$(openssl rand -base64 32 | tr -d '\r\n')"
  else
    DB_PASSWORD="$(head -c 32 /dev/urandom | base64 | tr -d '\r\n')"
  fi
  {
    echo "POSTGRES_PASSWORD=$DB_PASSWORD"
  } > "$ENV_FILE"
  echo "Created local .env at $ENV_FILE"
fi

#build and start the containers
docker compose --project-name queue_up --env-file .env -f infra/docker/docker-compose.yml up -d --build

echo "Queue Up services started."
echo "Backend health: http://localhost:8080/health"

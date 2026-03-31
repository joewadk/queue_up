#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENV_FILE="$SCRIPT_DIR/.env"
ENV_EXAMPLE="$SCRIPT_DIR/.env.example"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.ec2.yml"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required. install Docker Engine first."
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose plugin is required."
  exit 1
fi

if [ ! -f "$ENV_FILE" ]; then
  cp "$ENV_EXAMPLE" "$ENV_FILE"
  echo "Created $ENV_FILE from template."
  echo "Edit POSTGRES_PASSWORD in $ENV_FILE and rerun."
  exit 1
fi

if grep -q "replace-with-long-random-password" "$ENV_FILE"; then
  echo "Please set a real POSTGRES_PASSWORD in $ENV_FILE before deploy."
  exit 1
fi

cd "$REPO_DIR"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --build

echo "Queue Up EC2 stack is up."
echo "Backend is available through nginx on port 80."

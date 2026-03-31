#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

ENV_FILE="$SCRIPT_DIR/.env"

generate_password() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 32 | tr -d '\r\n'
    return
  fi
  head -c 32 /dev/urandom | base64 | tr -d '\r\n'
}

if [ ! -s "$ENV_FILE" ]; then #if .env doesn't exist or is empty, create it with a random POSTGRES_PASSWORD
  umask 077
  DB_PASSWORD="$(generate_password)"
  {
    echo "POSTGRES_PASSWORD=$DB_PASSWORD"
  } > "$ENV_FILE"
  echo "Created local .env at $ENV_FILE"
fi

CURRENT_PASSWORD="$(sed -n 's/^POSTGRES_PASSWORD=//p' "$ENV_FILE" | head -n 1)"
if [ -z "$CURRENT_PASSWORD" ] || [ "${#CURRENT_PASSWORD}" -lt 16 ] || [ "$CURRENT_PASSWORD" = "charlie_charlie_kirkie" ]; then
  umask 077
  DB_PASSWORD="$(generate_password)"
  {
    echo "POSTGRES_PASSWORD=$DB_PASSWORD"
  } > "$ENV_FILE"
  echo "Rotated weak POSTGRES_PASSWORD in $ENV_FILE"
fi

#build and start the containers
docker compose --project-name queue_up --env-file .env -f infra/docker/docker-compose.yml up -d --build

echo "Queue Up services started."
echo "Backend health: http://localhost:8080/health"

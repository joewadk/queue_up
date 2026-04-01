# Queue Up Backend (Postgres Foundation)

This is the initial backend scaffold with local Postgres as the source of truth.

## Run Postgres

From repo root:

```bash
docker compose -f infra/docker/docker-compose.yml up -d
```

The first startup automatically applies SQL files in `backend/migrations`.

If you changed/added migrations after first boot, reset local DB:

```bash
docker compose -f infra/docker/docker-compose.yml down -v
docker compose -f infra/docker/docker-compose.yml up -d
```

## Run Backend

Run the provided bash script to start the backend:

```bash
./docker-up.sh
```

This script will spin up the Postgres database and the Go backend server using Docker Compose. Ensure Docker is installed and running on your system.

## Run API (after Go is installed)

```bash
cd backend
go run ./cmd/api-server
```

Health check:

```bash
curl http://localhost:8080/health
```

## Current Endpoints

- `GET /health`
- `GET /v1/recommendation/today?user_id=<uuid>`
- `POST /v1/completions`
- `GET /v1/daily-queue?user_id=<uuid>&date=YYYY-MM-DD` (`date` optional)
- `GET /v1/concepts`
- `GET /v1/users/by-leetcode?username=<leetcode_username>`
- `POST /v1/users/bootstrap` (creates first-time local user after LeetCode username verification)
- `PUT /v1/users/{user_id}/concepts` (set preferred categories)
- `POST /v1/users/{user_id}/queue/refresh` (refresh today's queue)
- `GET /v1/users/{user_id}/history?limit=50` (recent completed problems)

Example:

```bash
curl "http://localhost:8080/v1/recommendation/today?user_id=00000000-0000-0000-0000-000000000001"
```

Notes:

- Returns existing assignments for today if they already exist.
- Otherwise creates up to 3 assignments (`Easy` + `NEETCODE_150` seed).
- Prioritizes the user's selected concept preferences when available.

Completion example:

```bash
curl -X POST "http://localhost:8080/v1/completions" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"00000000-0000-0000-0000-000000000001\",\"problem_id\":1,\"timestamp\":\"2026-03-31T15:00:00Z\",\"source\":\"desktop\",\"verification\":\"manual\"}"
```

Daily queue with completion checkbox state:

```bash
curl "http://localhost:8080/v1/daily-queue?user_id=00000000-0000-0000-0000-000000000001"
```

## End-to-End Local Test Commands

1. Start/reset DB:
   - `docker compose -f infra/docker/docker-compose.yml down -v`
   - `docker compose -f infra/docker/docker-compose.yml up -d`
2. Run backend:
   - `cd backend`
   - `go mod tidy`
   - `go run ./cmd/api-server`
3. Check health:
   - `curl http://localhost:8080/health`
4. Generate today recommendation:
   - `curl "http://localhost:8080/v1/recommendation/today?user_id=00000000-0000-0000-0000-000000000001"`
5. Mark one as complete:
   - `curl -X POST "http://localhost:8080/v1/completions" -H "Content-Type: application/json" -d "{\"user_id\":\"00000000-0000-0000-0000-000000000001\",\"problem_id\":1,\"source\":\"desktop\",\"verification\":\"manual\"}"`
6. Verify mobile checkbox source endpoint:
   - `curl "http://localhost:8080/v1/daily-queue?user_id=00000000-0000-0000-0000-000000000001"`

## Why Postgres First

- Reliable source of truth for assignment history, concept preferences, and mastery progression.
- Needed for spaced repetition correctness and analytics.
- Redis can be added next for real-time pub/sub and caching once core logic is stable.

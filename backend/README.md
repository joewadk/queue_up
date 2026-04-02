# Queue Up Backend (Postgres Foundation)

This backend scaffold now powers the desktop agent MVP (mobile-focused clients are still next up) with local Postgres as the source of truth.

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

Required env vars for submission sanitizer webhook (Java service):

```bash
export SUBMISSION_SANITIZER_WEBHOOK_URL="http://localhost:8090/v1/submissions/sanitize"
export SUBMISSION_SANITIZER_WEBHOOK_TIMEOUT_MS="3000"
```

`/v1/completions` now requires the sanitizer webhook to be configured and reachable.

Optional recommender fallback env vars (LeetCode API seed):

```bash
export LEETCODE_API_BASE_URL="https://leetcode-api-pied.vercel.app"
export LEETCODE_API_TIMEOUT_MS="4000"
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

Webhook contract (for Java sanitizer):

- Request JSON:
  - `user_id` (string)
  - `problem_id` (number)
  - `expected_slug` (string)
  - `submission_url` (string)
- Response JSON:
  - `valid` (boolean)
  - `sanitized_submission_url` (string)
  - `reason` (string, optional; used when `valid=false`)

Daily queue with completion checkbox state:

```bash
curl "http://localhost:8080/v1/daily-queue?user_id=00000000-0000-0000-0000-000000000001"
```

## Schema & Problem Catalog

- `users` record the Clerk identity and timezone that every assignment references.
- `concepts` stores both topic and technique buckets, so the backend can label Arrays, Graphs, Queue, DFS, prefix sums, and the rest.
- `user_concept_preferences` captures what concept clusters the user selected so scheduler decisions can favor those buckets.
- `problems` is the seeded catalog. Each row keeps the `slug`, difficulty, canonical `url`, `source_set` tag (`NEETCODE_150` plus the DSU/Queue extras), deterministic `queue_rank`, and raw LeetCode `tags`, making it easy to grow the catalog with specialized sets (prefix sums, cumulative sums, segment trees) over time.
- `problem_concepts` maps problems to one or more concepts/techniques so assignments can cross-link multiple learning goals.
- `daily_assignments` enforces the “three problems/day” cap, records `position`, and tracks status transitions (`ASSIGNED`, `COMPLETED`, `SKIPPED`) for spaced repetition analytics.

The catalog currently leans on NeetCode 150 plus manual additions and is being expanded with more specialized problems (prefix sums, cumulative sums, etc.) as the database is curated.

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

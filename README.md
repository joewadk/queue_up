# Queue Up

Queue Up pairs spaced-repetition coaching with enforced focus, but the current MVP is centered on the desktop agent (mobile support remains on the roadmap).

## High-Level Purpose

- Mobile app delivers daily LeetCode problems using spaced repetition by concept cluster (DFS, DP, Graphs, Queue, etc.).
- Desktop app detects distracting game usage and forces a LeetCode tab to open in the default browser.
- Backend coordinates onboarding, scheduling, event delivery, auth, and analytics.

## Product Constraints (Current MVP)

- Desktop enforcement is the shipping experience: the Go-based agent opens a LeetCode tab whenever a configured game is detected, and the bundled Fyne UI lets you bootstrap via LeetCode credentials, pick concepts, and mark problems done.
- LeetCode metadata source: [noworneverev/leetcode-api](https://github.com/noworneverev/leetcode-api?tab=readme-ov-file) (queried by backend adapter).
- First login flow: user selects a starting concept cluster (example: Graphs, DP, Queue).
- Initial recommendation policy: prefer Easy problems first.
- Daily cap: assign up to `3` problems per day.
- Seed problem set: NeetCode 150 curated list as baseline; expand later to a broader internal set.
- Mobile app behavior: push notifications for daily queue + pending completions.
- Desktop behavior: when game usage is detected, open/switch to a new LeetCode tab in the system default browser, pointing to the current daily problem.

## Schema Snapshot

- `users` store the Clerk-linked identity, timezone, and creation timestamp that anchor every assignment.
- `concepts` captures both topical buckets (Arrays, Graphs) and technique-focused entries (DFS, Sliding Window, Prefix Sums, etc.), and the `type` enum keeps topics and techniques distinct.
- `user_concept_preferences` tracks each concept the user explicitly chose so the scheduler can bias today's queue toward those buckets.
- `problems` is the seeded catalog. Each row holds the `slug`, difficulty, canonical `url`, `source_set` tag (currently `NEETCODE_150` plus the handful of extra DSU/Queue/technique entries we added), `queue_rank`, and raw LeetCode `tags`. These fields make it easy to extend the catalog with specialized sets like prefix sums or segment trees before a broader LeetCode sync.
- `problem_concepts` links problems to one or more concepts or techniques, enabling cross-topic assignments.
- `daily_assignments` guarantees each user gets up to three problems per day, enforces unique (user, date, position), and records status (`ASSIGNED`, `COMPLETED`, `SKIPPED`) for the spaced repetition loop.
- The problem catalog already leans on the NeetCode 150 list plus our manual additions, but upcoming seeds will add more specialized problems (e.g., prefix sums, cumulative sums, and other focused techniques) and grow the database over time.

## Core Architecture

```mermaid
flowchart LR
    U[User] --> M[Mobile App<br/>Swift]
    U --> D[Desktop Agent<br/>Go + Fyne UI]

    M <--> |WebSocket| B[Backend API + Scheduler<br/>Go]
    D <--> |REST/gRPC| B
    B <--> R[(Redis Pub/Sub)]
    B <--> P[(Postgres)]
    B <--> C[Clerk Auth]
    B <--> L[LeetCode API Adapter<br/>noworneverev/leetcode-api]

    R --> B
    B --> M
    B --> D

    classDef user fill:#FFF3C4,stroke:#B78103,color:#3D2A00,stroke-width:1.5px;
    classDef mobile fill:#D9EFFF,stroke:#1E6FA8,color:#0D2E45,stroke-width:1.5px;
    classDef desktop fill:#E8F5E9,stroke:#2E7D32,color:#163818,stroke-width:1.5px;
    classDef backend fill:#EDE7F6,stroke:#5E35B1,color:#2D1757,stroke-width:1.5px;
    classDef infra fill:#FFE5D0,stroke:#C25B00,color:#4A2000,stroke-width:1.5px;

    class U user;
    class M mobile;
    class D desktop;
    class B backend;
    class R,P,C,L infra;
```

## End-to-End Data Flow

```mermaid
%%{init: {"theme": "neutral"}}%%
sequenceDiagram
    autonumber
    participant Mobile as Mobile App
    participant Desktop as Desktop Agent
    participant Backend as Go Backend
    participant Redis as Redis
    participant PG as Postgres
    participant LC as LeetCode API
    participant Clerk as Clerk

    rect rgb(239, 246, 255)
        Note over Mobile,Backend: Auth + Profile Bootstrap
        Mobile->>Backend: Send sign-in token
        Backend->>Clerk: Validate session
        Clerk-->>Backend: User identity
        Backend->>PG: Upsert user profile
    end

    rect rgb(240, 253, 244)
        Note over Backend,LC: Daily Recommendation
        Backend->>LC: Fetch problem metadata
        LC-->>Backend: Problems + tags + difficulty
        Backend->>PG: Persist catalog and mappings
        Backend->>PG: Compute today's assignments
        Backend->>Redis: Publish problem.assigned
        Redis-->>Mobile: Push queue update
    end

    rect rgb(254, 242, 242)
        Note over Desktop,Mobile: Distraction Enforcement
        Desktop->>Desktop: Detect distracting process
        Desktop->>Backend: Send user.distracted
        Backend->>PG: Persist enforcement log
        Backend->>Redis: Publish user.distracted
        Desktop->>Desktop: Open today's LeetCode problem
        Redis-->>Mobile: Real-time nudge
    end

    rect rgb(245, 243, 255)
        Note over Mobile,Backend: Completion Loop
        Mobile->>Backend: Submit completion
        Backend->>PG: Record completion + update mastery
        Backend->>Redis: Publish problem.completed
        Redis-->>Mobile: Refresh progress/streaks
    end
```

## Feedback Loops

```mermaid
flowchart TD
    A[Spaced Repetition Loop]
    A1[Select due concepts/problems]
    A2[Push via WebSocket]
    A3[User solves problem]
    A4[Update mastery + interval in Postgres]
    A --> A1 --> A2 --> A3 --> A4 --> A1

    B[Behavioral Enforcement Loop]
    B1[Desktop detects distracting app]
    B2[Open current daily problem]
    B3[Log event in Postgres]
    B4[Notify mobile in real time]
    B --> B1 --> B2 --> B3 --> B4 --> B1

    C[Analytics Loop]
    C1[Aggregate progress, streaks, concepts]
    C2[Expose dashboard APIs]
    C3[Mobile renders heatmaps + trends]
    C --> C1 --> C2 --> C3 --> C1

    classDef spaced fill:#E8F1FF,stroke:#2F6FB6,color:#12314F,stroke-width:1.5px;
    classDef enforcement fill:#E9F7EC,stroke:#2E7D32,color:#17401B,stroke-width:1.5px;
    classDef analytics fill:#FFF1E5,stroke:#C25B00,color:#522300,stroke-width:1.5px;

    class A,A1,A2,A3,A4 spaced;
    class B,B1,B2,B3,B4 enforcement;
    class C,C1,C2,C3 analytics;
```

## MVP User Flow

```mermaid
%%{init: {
  "theme": "base",
  "themeCSS": ".nodeLabel,.edgeLabel,.label,text,tspan{paint-order:stroke;stroke:#111;stroke-width:0.5px;stroke-linejoin:round;text-shadow:0.5px 0.5px 1px rgba(0,0,0,0.18);}"
}}%%
flowchart TD
    A["User logs in"] --> B["Pick starting concept<br/>(Graphs / DP / Queue / ...)"]
    B --> C["Backend builds initial plan<br/>Easy-first from NeetCode 150 seed"]
    C --> D["Assign daily queue<br/>max 3 problems"]
    D --> E["Mobile receives push notification"]
    E --> F["User solves problem(s)"]
    F --> G["Backend updates mastery + next review"]

    H["Desktop detects game app in foreground"] --> I["Fetch current daily problem URL"]
    I --> J["Open/switch to LeetCode tab in default browser"]
    J --> K["Log enforcement event + notify mobile"]
    K --> F

    classDef start fill:#E3F2FD,stroke:#1565C0,stroke-width:1px,color:#0D47A1;
    classDef learning fill:#E8F5E9,stroke:#2E7D32,stroke-width:1px,color:#1B5E20;
    classDef delivery fill:#EDE7F6,stroke:#5E35B1,stroke-width:1px,color:#311B92;
    classDef enforcement fill:#FFF3E0,stroke:#EF6C00,stroke-width:1px,color:#E65100;

    class A start;
    class B,C,D,F,G learning;
    class E,K delivery;
    class H,I,J enforcement;
```

## Development Notes

- `submission-sanitizer-java/src/main/resources/application.properties` now ships in source control so the sanitizer service has an explicit default config instead of hiding it under ignore globs.
- `submission-sanitizer-java/target/` is ignored so Maven class artifacts stay out of git while the source config and code stay visible.
- The desktop agent keeps `config-example.json` versioned while each contributor can keep a local `desktop-agent/config.json` for machine-specific overrides.
- The Windows build script now runs `goversioninfo` (when installed) to bake the product metadata defined in `desktop-agent/cmd/queue-up-agent/versioninfo.json` into the executable; install it via `go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest` before running `desktop-agent/build-windows.ps1`.
- Infra side includes the `infra/aws/nginx` configs; we’ll drop a dedicated Nginx block once the SwiftUI client is in place.
- Swift UI mobile screens are coming next, so keep an eye on the `mobile/` branch layout once it gets merged.
- Problem catalog is already seeded with NeetCode 150 plus DSU/Queue extras and is being expanded with specialized topics (prefix sums, cumulative sums, etc.) so future assignments can drill into narrower skill slices.

## Remote backend endpoint

- The production API now lives at `https://queue-up-backend.duckdns.org`. Local desktop agents and any other clients should point their `backend_base_url`/API base to that domain instead of `localhost:8080` when you want to hit the deployed Postgres-backed dataset.
- Certbot-protected HTTPS is proxying to the Dockerized Go backend via the host-level Nginx proxy, so hitting `/health` or any `v1/...` endpoint on that hostname behaves the same as calling the local container ports.
- Rebuild or copy `desktop-agent/config.json` from `config.example.json` if you have a custom config; make sure `backend_base_url` uses the DuckDNS URL before starting the agent outside your Docker host.

## Docker Setup (Postgres + Backend)

### Prerequisites

- Docker Desktop running
- Bash shell (Git Bash/WSL/macOS/Linux)

### Start services

```bash
./docker-up.sh
```

This builds and runs:

- `queue-up-postgres` on `localhost:5432`
- `queue-up-go-backend` on `localhost:8080`

### Stop services (and clear DB volume)

```bash
./docker-down.sh
```

### Quick health check

```bash
curl http://localhost:8080/health
```

### API test flow

1. Generate today's recommendations:

```bash
curl "http://localhost:8080/v1/recommendation/today?user_id=00000000-0000-0000-0000-000000000001"
```

2. Mark problem complete from desktop source:

```bash
curl -X POST "http://localhost:8080/v1/completions" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"00000000-0000-0000-0000-000000000001\",\"problem_id\":1,\"source\":\"desktop\",\"verification\":\"manual\"}"
```

3. Query daily queue with checkbox state:

```bash
curl "http://localhost:8080/v1/daily-queue?user_id=00000000-0000-0000-0000-000000000001"
```

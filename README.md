# Queue Up

Queue Up is a two-pronged system to improve LeetCode learning retention and enforce productive behavior across mobile and desktop.

## High-Level Purpose

- Mobile app delivers daily LeetCode problems using spaced repetition by concept cluster (DFS, DP, Graphs, Queue, etc.).
- Desktop app detects distracting game usage and forces a LeetCode tab to open in the default browser.
- Backend coordinates onboarding, scheduling, event delivery, auth, and analytics.

## Product Constraints (Current MVP)

- LeetCode metadata source: [noworneverev/leetcode-api](https://github.com/noworneverev/leetcode-api?tab=readme-ov-file) (queried by backend adapter).
- First login flow: user selects a starting concept cluster (example: Graphs, DP, Queue).
- Initial recommendation policy: prefer Easy problems first.
- Daily cap: assign up to `3` problems per day.
- Seed problem set: NeetCode 150 curated list as baseline; expand later to a broader internal set.
- Mobile app behavior: push notifications for daily queue + pending completions.
- Desktop behavior: when game usage is detected, open/switch to a new LeetCode tab in the system default browser, pointing to the current daily problem.

## Core Architecture

```mermaid
flowchart LR
    U[User] --> M[Mobile App<br/>Swift]
    U --> D[Desktop Agent<br/>Go or Java]

    M <--> |WebSocket| B[Backend API + Scheduler<br/>Go or Java]
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
%%{init: {
  "theme": "base",
  "themeCSS": ".messageText,.noteText,.labelBox,.labelText,text,tspan{paint-order:stroke;stroke:#111;stroke-width:0.5px;stroke-linejoin:round;text-shadow:0.6px 0.6px 1px rgba(0,0,0,0.22);}"
}}%%
sequenceDiagram
    box rgb(227, 242, 253) Clients
    participant Mobile as Mobile App (Swift)
    participant Desktop as Desktop Agent
    end

    box rgb(237, 231, 246) Core Backend
    participant Backend as Backend (Go/Java)
    end

    box rgb(255, 243, 224) Realtime + Storage
    participant Redis as Redis Pub/Sub
    participant PG as Postgres
    end

    box rgb(232, 245, 233) External Providers
    participant LC as LeetCode API
    participant Clerk as Clerk
    end

    Note over Mobile,Backend: Auth bootstrap
    Mobile->>Backend: Sign in token
    Backend->>Clerk: Validate user/session
    Clerk-->>Backend: User identity
    Backend->>PG: Upsert user profile

    Note over Backend,LC: Problem metadata sync
    Backend->>LC: Poll problems + tags + difficulty
    LC-->>Backend: Problem metadata
    Backend->>PG: Store catalog + concept mapping

    Note over Backend,Mobile: Spaced repetition assignment loop
    Backend->>PG: Compute daily spaced-repetition recommendations
    Backend->>Redis: Publish daily_problem_assigned
    Redis-->>Mobile: Notification event
    Mobile->>Backend: Problem completed
    Backend->>PG: Update mastery + next review date

    Note over Desktop,Mobile: Behavioral enforcement loop
    Desktop->>Desktop: Detect distracting app usage
    Desktop->>Backend: enforcement_triggered
    Backend->>PG: Persist enforcement log
    Backend->>Redis: Publish enforcement event
    Desktop->>Desktop: Open daily LeetCode problem in browser
    Redis-->>Mobile: Real-time nudge notification
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

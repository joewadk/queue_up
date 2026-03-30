# Queue Up

Queue Up is a two-pronged system to improve LeetCode learning retention and enforce productive behavior across mobile and desktop.

## High-Level Purpose

- Mobile app delivers daily LeetCode problems using spaced repetition by concept cluster (DFS, DP, Graphs, etc.).
- Desktop app detects distracting app usage and forces the daily recommended LeetCode problem to open.
- Backend coordinates scheduling, event delivery, auth, and analytics.

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
    B <--> L[LeetCode Metadata Adapter]

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
    Desktop->>Desktop: Open recommended LeetCode URL in browser
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
    B2[Open daily recommended problem]
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

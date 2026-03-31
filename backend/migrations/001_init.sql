-- Queue Up initial schema (MVP)

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    clerk_user_id TEXT UNIQUE NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'America/New_York',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS concepts (
    id BIGSERIAL PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,          -- e.g. GRAPH, DP, QUEUE
    display_name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_concept_preferences (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    concept_id BIGINT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    selected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, concept_id)
);

CREATE TABLE IF NOT EXISTS problems (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,          -- leetcode slug
    title TEXT NOT NULL,
    difficulty TEXT NOT NULL CHECK (difficulty IN ('Easy', 'Medium', 'Hard')),
    url TEXT NOT NULL,
    source_set TEXT NOT NULL DEFAULT 'NEETCODE_150', -- seed now, expand later
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS problem_concepts (
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    concept_id BIGINT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    PRIMARY KEY (problem_id, concept_id)
);

CREATE TABLE IF NOT EXISTS daily_assignments (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    assignment_date DATE NOT NULL,
    source TEXT NOT NULL DEFAULT 'SPACED_REPETITION',
    position SMALLINT NOT NULL CHECK (position >= 1 AND position <= 3),
    status TEXT NOT NULL DEFAULT 'ASSIGNED' CHECK (status IN ('ASSIGNED', 'COMPLETED', 'SKIPPED')),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, assignment_date, position),
    UNIQUE (user_id, assignment_date, problem_id)
);

CREATE TABLE IF NOT EXISTS problem_attempts (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    assignment_date DATE,
    quality SMALLINT NOT NULL CHECK (quality >= 0 AND quality <= 5),
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_problem_state (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    ease NUMERIC(4,2) NOT NULL DEFAULT 2.50,
    repetitions INTEGER NOT NULL DEFAULT 0,
    interval_days INTEGER NOT NULL DEFAULT 1,
    next_review_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_quality SMALLINT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, problem_id)
);

CREATE TABLE IF NOT EXISTS enforcement_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID,
    executable TEXT NOT NULL,
    action TEXT NOT NULL,
    problem_url TEXT NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_daily_assignments_user_date ON daily_assignments(user_id, assignment_date);
CREATE INDEX IF NOT EXISTS idx_problem_attempts_user_time ON problem_attempts(user_id, attempted_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_problem_state_next_review ON user_problem_state(user_id, next_review_at);

-- Seed concept dictionary.
INSERT INTO concepts (code, display_name) VALUES
('GRAPH', 'Graphs'),
('DP', 'Dynamic Programming'),
('QUEUE', 'Queue'),
('DFS', 'Depth-First Search'),
('BFS', 'Breadth-First Search')
ON CONFLICT (code) DO NOTHING;

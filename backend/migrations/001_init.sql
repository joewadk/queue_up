-- USERS
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    clerk_user_id TEXT UNIQUE NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'America/New_York',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- CONCEPTS (topics + techniques)
CREATE TABLE IF NOT EXISTS concepts (
    id BIGSERIAL PRIMARY KEY,
    code TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('TOPIC', 'TECHNIQUE')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Ensure `type` column exists in `concepts` table
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'concepts' AND column_name = 'type'
    ) THEN
        ALTER TABLE concepts ADD COLUMN type TEXT NOT NULL CHECK (type IN ('TOPIC', 'TECHNIQUE'));
    END IF;
END $$;

-- USER PREFERENCES
CREATE TABLE IF NOT EXISTS user_concept_preferences (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    concept_id BIGINT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    selected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, concept_id)
);

-- PROBLEMS
CREATE TABLE IF NOT EXISTS problems (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    difficulty TEXT NOT NULL CHECK (difficulty IN ('Easy', 'Medium', 'Hard')),
    url TEXT NOT NULL,
    source_set TEXT NOT NULL DEFAULT 'NEETCODE_150',
    queue_rank INTEGER NOT NULL DEFAULT 1000,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    tags TEXT[] DEFAULT '{}', -- raw tags from leetcode (for future use)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- PROBLEM ↔ CONCEPT (CRITICAL)
CREATE TABLE IF NOT EXISTS problem_concepts (
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    concept_id BIGINT NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    PRIMARY KEY (problem_id, concept_id)
);

-- DAILY ASSIGNMENTS
CREATE TABLE IF NOT EXISTS daily_assignments (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    assignment_date DATE NOT NULL,
    source TEXT NOT NULL DEFAULT 'SPACED_REPETITION',
    position SMALLINT NOT NULL CHECK (position >= 1 AND position <= 3),
    status TEXT NOT NULL DEFAULT 'ASSIGNED'
        CHECK (status IN ('ASSIGNED', 'COMPLETED', 'SKIPPED')),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (user_id, assignment_date, position),
    UNIQUE (user_id, assignment_date, problem_id)
);

CREATE INDEX IF NOT EXISTS idx_daily_assignments_user_date
ON daily_assignments(user_id, assignment_date);

-- =========================
-- SEED CONCEPTS (NC150 + EXTRA)
-- =========================

INSERT INTO concepts (code, display_name, type) VALUES

-- CORE TOPICS (NC150 STRUCTURE)
('ARRAY_HASHING', 'Arrays & Hashing', 'TOPIC'),
('TWO_POINTERS', 'Two Pointers', 'TECHNIQUE'),
('STACK', 'Stack', 'TECHNIQUE'),
('BINARY_SEARCH', 'Binary Search', 'TECHNIQUE'),
('SLIDING_WINDOW', 'Sliding Window', 'TECHNIQUE'),
('LINKED_LIST', 'Linked List', 'TOPIC'),
('TREE', 'Trees', 'TOPIC'),
('TRIE', 'Tries', 'TOPIC'),
('BACKTRACKING', 'Backtracking', 'TECHNIQUE'),
('HEAP', 'Heap / Priority Queue', 'TECHNIQUE'),
('GRAPH', 'Graphs', 'TOPIC'),
('DP_1D', '1-D Dynamic Programming', 'TOPIC'),
('INTERVALS', 'Intervals', 'TOPIC'),
('GREEDY', 'Greedy', 'TECHNIQUE'),
('ADVANCED_GRAPH', 'Advanced Graphs', 'TOPIC'),
('DP_2D', '2-D Dynamic Programming', 'TOPIC'),
('BIT_MANIPULATION', 'Bit Manipulation', 'TECHNIQUE'),
('MATH_GEOMETRY', 'Math & Geometry', 'TOPIC'),

-- YOUR ADDITIONS
('DSU', 'Disjoint Set Union (Union Find)', 'TECHNIQUE'),
('QUEUE', 'Queue', 'TECHNIQUE'),

-- SUPPORTING (helps mapping later)
('DFS', 'Depth-First Search', 'TECHNIQUE'),
('BFS', 'Breadth-First Search', 'TECHNIQUE')

ON CONFLICT (code) DO NOTHING;
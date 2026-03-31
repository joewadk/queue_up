CREATE TABLE IF NOT EXISTS problem_completions (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id BIGINT NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    completed_at TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('desktop', 'mobile', 'discord')),
    verification TEXT NOT NULL CHECK (verification IN ('api', 'manual')),
    submission_url TEXT,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_problem_completions_user_time
    ON problem_completions(user_id, completed_at DESC);

CREATE INDEX IF NOT EXISTS idx_problem_completions_user_problem
    ON problem_completions(user_id, problem_id);

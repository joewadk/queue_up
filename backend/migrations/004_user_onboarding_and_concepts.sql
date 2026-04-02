ALTER TABLE users
    ADD COLUMN IF NOT EXISTS leetcode_username TEXT UNIQUE,
    ADD COLUMN IF NOT EXISTS onboarding_completed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_users_leetcode_username
    ON users(leetcode_username);

UPDATE users
SET leetcode_username = clerk_user_id
WHERE leetcode_username IS NULL
  AND clerk_user_id IS NOT NULL;

INSERT INTO concepts (code, display_name, type) VALUES
('ARRAY', 'Array', 'TECHNIQUE'),
('TWO_POINTER', 'Two Pointers', 'TECHNIQUE'),
('STACK', 'Stack', 'TECHNIQUE'),
('BINARY_SEARCH', 'Binary Search', 'TECHNIQUE'),
('SLIDING_WINDOW', 'Sliding Window', 'TECHNIQUE'),
('LINKED_LIST', 'Linked List', 'TECHNIQUE'),
('TREE', 'Tree', 'TECHNIQUE'),
('HEAP_PRIORITY_QUEUE', 'Heap / Priority Queue', 'TECHNIQUE'),
('BACKTRACKING', 'Backtracking', 'TECHNIQUE'),
('TRIE', 'Trie', 'TECHNIQUE'),
('INTERVAL', 'Intervals', 'TECHNIQUE'),
('GREEDY', 'Greedy', 'TECHNIQUE'),
('BIT_MANIPULATION', 'Bit Manipulation', 'TECHNIQUE'),
('PREFIX_SUM', 'Prefix Sum', 'TECHNIQUE')
ON CONFLICT (code) DO NOTHING;

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

INSERT INTO concepts (code, display_name) VALUES
('ARRAY', 'Array'),
('TWO_POINTER', 'Two Pointers'),
('STACK', 'Stack'),
('BINARY_SEARCH', 'Binary Search'),
('SLIDING_WINDOW', 'Sliding Window'),
('LINKED_LIST', 'Linked List'),
('TREE', 'Tree'),
('HEAP_PRIORITY_QUEUE', 'Heap / Priority Queue'),
('BACKTRACKING', 'Backtracking'),
('TRIE', 'Trie'),
('INTERVAL', 'Intervals'),
('GREEDY', 'Greedy'),
('BIT_MANIPULATION', 'Bit Manipulation'),
('PREFIX_SUM', 'Prefix Sum')
ON CONFLICT (code) DO NOTHING;

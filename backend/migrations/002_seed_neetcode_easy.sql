-- Seed starter Easy-first problem pool from NeetCode 150 (initial subset).
-- This is intentionally small to unblock MVP recommender integration.

INSERT INTO problems (slug, title, difficulty, url, source_set) VALUES
('two-sum', 'Two Sum', 'Easy', 'https://leetcode.com/problems/two-sum/', 'NEETCODE_150'),
('valid-parentheses', 'Valid Parentheses', 'Easy', 'https://leetcode.com/problems/valid-parentheses/', 'NEETCODE_150'),
('merge-two-sorted-lists', 'Merge Two Sorted Lists', 'Easy', 'https://leetcode.com/problems/merge-two-sorted-lists/', 'NEETCODE_150'),
('best-time-to-buy-and-sell-stock', 'Best Time to Buy and Sell Stock', 'Easy', 'https://leetcode.com/problems/best-time-to-buy-and-sell-stock/', 'NEETCODE_150'),
('valid-anagram', 'Valid Anagram', 'Easy', 'https://leetcode.com/problems/valid-anagram/', 'NEETCODE_150'),
('binary-search', 'Binary Search', 'Easy', 'https://leetcode.com/problems/binary-search/', 'NEETCODE_150'),
('climbing-stairs', 'Climbing Stairs', 'Easy', 'https://leetcode.com/problems/climbing-stairs/', 'NEETCODE_150'),
('min-cost-climbing-stairs', 'Min Cost Climbing Stairs', 'Easy', 'https://leetcode.com/problems/min-cost-climbing-stairs/', 'NEETCODE_150'),
('flood-fill', 'Flood Fill', 'Easy', 'https://leetcode.com/problems/flood-fill/', 'NEETCODE_150'),
('maximum-depth-of-binary-tree', 'Maximum Depth of Binary Tree', 'Easy', 'https://leetcode.com/problems/maximum-depth-of-binary-tree/', 'NEETCODE_150'),
('find-if-path-exists-in-graph', 'Find if Path Exists in Graph', 'Easy', 'https://leetcode.com/problems/find-if-path-exists-in-graph/', 'NEETCODE_150'),
('find-the-town-judge', 'Find the Town Judge', 'Easy', 'https://leetcode.com/problems/find-the-town-judge/', 'NEETCODE_150'),
('number-of-recent-calls', 'Number of Recent Calls', 'Easy', 'https://leetcode.com/problems/number-of-recent-calls/', 'NEETCODE_150'),
('implement-stack-using-queues', 'Implement Stack using Queues', 'Easy', 'https://leetcode.com/problems/implement-stack-using-queues/', 'NEETCODE_150')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'DP'
WHERE p.slug IN ('climbing-stairs', 'min-cost-climbing-stairs')
ON CONFLICT DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'GRAPH'
WHERE p.slug IN ('find-if-path-exists-in-graph', 'find-the-town-judge')
ON CONFLICT DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'QUEUE'
WHERE p.slug IN ('number-of-recent-calls', 'implement-stack-using-queues')
ON CONFLICT DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'DFS'
WHERE p.slug IN ('flood-fill', 'maximum-depth-of-binary-tree')
ON CONFLICT DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'BFS'
WHERE p.slug IN ('flood-fill', 'find-if-path-exists-in-graph')
ON CONFLICT DO NOTHING;

-- Generic baseline concepts for starter array/hash/list practice.
INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'QUEUE'
WHERE p.slug IN ('valid-parentheses', 'merge-two-sorted-lists')
ON CONFLICT DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'DP'
WHERE p.slug IN ('best-time-to-buy-and-sell-stock')
ON CONFLICT DO NOTHING;

-- Expand concept mappings for initial seeded NeetCode starter problems.
-- This makes category switches produce visibly different queues.

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'ARRAY'
WHERE p.slug IN ('two-sum', 'valid-anagram', 'best-time-to-buy-and-sell-stock')
ON CONFLICT DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'STACK'
WHERE p.slug IN ('valid-parentheses', 'implement-stack-using-queues')
ON CONFLICT DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'LINKED_LIST'
WHERE p.slug IN ('merge-two-sorted-lists')
ON CONFLICT DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'BINARY_SEARCH'
WHERE p.slug IN ('binary-search')
ON CONFLICT DO NOTHING;

INSERT INTO problem_concepts (problem_id, concept_id)
SELECT p.id, c.id
FROM problems p
JOIN concepts c ON c.code = 'TREE'
WHERE p.slug IN ('maximum-depth-of-binary-tree')
ON CONFLICT DO NOTHING;

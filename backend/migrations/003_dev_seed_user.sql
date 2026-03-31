-- Dev-only convenience seed for local testing.

INSERT INTO users (id, clerk_user_id, timezone) VALUES
('00000000-0000-0000-0000-000000000001', 'dev_clerk_user_1', 'America/New_York')
ON CONFLICT (id) DO NOTHING;

INSERT INTO user_concept_preferences (user_id, concept_id)
SELECT '00000000-0000-0000-0000-000000000001', c.id
FROM concepts c
WHERE c.code IN ('GRAPH', 'DP', 'QUEUE')
ON CONFLICT DO NOTHING;

# Submission Sanitizer Java Webhook

Spring Boot webhook for LeetCode submission URL sanitization/verification.

## Run

```bash
cd submission-sanitizer-java
mvn spring-boot:run
```

Runs on `http://localhost:8090` by default.

## Endpoint

`POST /v1/submissions/sanitize`

Health:

`GET /v1/submissions/health`

Request:

```json
{
  "user_id": "00000000-0000-0000-0000-000000000001",
  "problem_id": 42,
  "expected_slug": "valid-parentheses",
  "submission_url": "https://leetcode.com/problems/valid-parentheses/submissions/1891518729"
}
```

Response:

```json
{
  "valid": true,
  "sanitized_submission_url": "https://leetcode.com/problems/valid-parentheses/submissions/1891518729/",
  "reason": ""
}
```

Invalid responses return `valid=false` with `reason`.

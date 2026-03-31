# Queue Up Desktop Agent (Base)

Windows desktop agent that detects configured game executables and opens a LeetCode URL in the default browser.

## What This Base Version Does

- Polls running processes via `tasklist` at a configurable interval.
- Matches against a configurable list of executable names (case-insensitive).
- Starts with:
  - `Marvel-Win64-Shipping.exe` (Marvel Rivals)
  - `masterduel.exe` (Yu-Gi-Oh Master Duel)
- Enforces a cooldown to avoid repeated tab spam (set lower while testing).
- Logs enforcement events to JSONL for future Postgres/backend integration.
- Optional poll heartbeat logs to verify it is continuously scanning.

## Quick Start

1. Copy example config:
   - `copy config\\config.example.json config\\config.json`
2. Edit `config\\config.json`:
   - Set `leetcode_problem_url` to the current problem URL you want to enforce.
   - Keep `dry_run: true` for safe test mode first.
3. Run:
   - `go run .\\cmd\\queue-up-agent -config config\\config.json`

## Config Fields

- `poll_interval_seconds`: process scan frequency.
- `cooldown_seconds`: minimum time before re-enforcing for the same exe (`0` = every poll).
- `leetcode_problem_url`: URL opened in default browser on enforcement.
- `watched_executables`: list of process names to detect.
- `log_file_path`: JSONL log output path.
- `dry_run`: when `true`, logs actions without opening browser.
- `log_polls`: when `true`, writes a heartbeat log each polling cycle.

## Next Steps

- Replace static `leetcode_problem_url` with backend endpoint for “problem of the day.”
- Send enforcement events to backend API and persist to Postgres.
- Upgrade process detection from “running process” to “foreground active window” if needed.
- Add executable policy hot-reload and user-editable allow/block lists.

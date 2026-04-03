# Queue Up Desktop Agent

Windows desktop agent that detects configured game executables and opens a LeetCode URL in the default browser.

The agent is implemented in Go and the native UI (tray + windows) is powered by the Fyne toolkit so the enforcement + onboarding experience stays in a single binary.

## What This Base Version Does

- Polls running processes via `tasklist` at a configurable interval.
- Matches against a configurable list of executable names (case-insensitive).
- Starts with:
  - `Marvel-Win64-Shipping.exe` (Marvel Rivals)
  - `masterduel.exe` (Yu-Gi-Oh Master Duel)
- Enforces a cooldown to avoid repeated tab spam (set lower while testing).
- Logs enforcement events to JSONL for future Postgres/backend integration.
- Optional poll heartbeat logs to verify it is continuously scanning.
- Registers/unregisters itself in Windows Startup Apps via `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`.
- Optional system tray mode with actions:
  - `Open Today's Problem`
  - `Mark as Done` (posts completion to backend)
  - `Open Dashboard` (opens native desktop UI)
- Native desktop UI (Fyne) for:
  - First-time login/bootstrap by LeetCode username (with backend verification)
  - Choosing NeetCode categories
  - Refreshing personal problem-of-the-day queue
  - Viewing previously completed problems
  - Marking selected queue problem complete (optional submission URL)

## Quick Start

1. Run the quickstart script:
   ```bash
   ./desktop-quickstart.sh
   ```

   This script will:
   - Build the `queue-up-agent` binary.
   - Ensure the `config.json` file exists (copies from `config.example.json` if missing).
   - Run the agent with the appropriate configuration.

    The Windows build invoked by the quickstart script now passes `-ldflags "-H=windowsgui"` so the generated `queue-up-agent.exe` runs as a GUI subsystem process. That means double-clicking it or launching it from startup only shows the tray/desktop windows and closing any originating `cmd` tab no longer terminates the agent.

2. Edit `config/config.json` if needed:
   - Set `leetcode_problem_url` to the current problem URL you want to enforce.
   - Adjust other settings as required.

3. For native desktop UI support, ensure Fyne prerequisites are installed on Windows (`gcc` via MinGW-w64 is typically required for cgo builds).

## Config Fields

- `poll_interval_seconds`: process scan frequency.
- `cooldown_seconds`: minimum time before re-enforcing for the same exe (`0` = every poll).
- `backend_base_url`: Queue Up backend base URL (for recommendation fetch).
- `user_id`: user id passed to `/v1/recommendation/today`.
- `request_timeout_seconds`: HTTP timeout for backend recommendation fetch.
- `leetcode_problem_url`: URL opened in default browser on enforcement.
- `watched_executables`: list of process names to detect.
- `log_file_path`: JSONL log output path.
- `dry_run`: when `true`, logs actions without opening browser.
- `log_polls`: when `true`, writes a heartbeat log each polling cycle.
- `enable_tray`: when `true`, starts tray UI mode automatically.
- `open_gui_on_start`: when `true`, launches native desktop UI on tray startup.

Run native UI directly (no tray):

```bash
bin/queue-up-agent.exe -desktop-ui -config config/config.json
```

Tray `Mark as Done` behavior:

- Requires `backend_base_url` and `user_id`.
- Selects first incomplete problem from `/v1/daily-queue`.
- Falls back to `/v1/recommendation/today` if queue lookup fails.
- Posts completion to `/v1/completions` with:
  - `source: "desktop"`
  - `verification: "manual_tray"`

Recommendation behavior:

- If `backend_base_url` + `user_id` are set and backend responds, agent opens recommended problem URL.
- If backend is unavailable or returns no recommendation, agent falls back to `leetcode_problem_url`.

## Startup Reliability Notes

- Startup registration now launches with `-tray` so the system tray is always shown on login.
- `log_file_path` is resolved relative to the config file directory, so startup from `HKCU\Run` no longer depends on current working directory.
- `startup.sh` now looks for `bin/queue-up-agent.exe` first to match quickstart builds.

## Icon Consistency (Tray + EXE)

- Tray icon is sourced from `internal/tray/assets/queue_up.ico`.
- To embed the same icon into the Windows executable, run:

```powershell
./build-windows.ps1
```

If `rsrc` is installed, this script generates `cmd/queue-up-agent/queue_up.syso` from the same `.ico` before build.

## Next Steps

- Replace static `leetcode_problem_url` with backend endpoint for “problem of the day.”
- Send enforcement events to backend API and persist to Postgres.
- Upgrade process detection from “running process” to “foreground active window” if needed.
- Add executable policy hot-reload and user-editable allow/block lists.
- Align the tray/GUI flows with the expanded problem schema (NeetCode 150 + the future specialized sets such as prefix sums, cumulative sums, etc.) so new catalog items surface in the desktop experience.

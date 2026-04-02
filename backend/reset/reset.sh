#!/bin/bash
set -e
# drops all tables and resets the database to an empty state.
# WARNING: this will delete all data in the database, especially user data. Use with caution.


psql -f 005_reset_tables.sql "postgresql://queue_up:00gundam@localhost:55432/queue_up"
psql -f migrations/001_init.sql "postgresql://queue_up:00gundam@localhost:55432/queue_up"
psql -f migrations/002_dev_seed_user.sql "postgresql://queue_up:00gundam@localhost:55432/queue_up"
psql -f migrations/003_problem_completions.sql "postgresql://queue_up:00gundam@localhost:55432/queue_up"
psql -f migrations/004_user_onboarding_and_concepts.sql "postgresql://queue_up:00gundam@localhost:55432/queue_up"

#repopulate the database with the NC150 problems
cd ../desktop-agent
go run populate_db.go


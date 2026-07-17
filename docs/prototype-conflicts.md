# Current Prototype Conflict Fix List

After Alpha 0, stop adding unplanned prototype features and proceed into Alpha 1/2/3 in the following order.

Status update: Alpha 1 conflicts for migrations, localhost, CredentialStore, Repository/DTO, and API endpoints were fixed on 2026-07-16. Alpha 2/3 items in the table remain as data-correctness acceptance checks.

| Priority | Current conflict | Must be fixed to | Target phase |
| --- | --- | --- | --- |
| P0 | `internal/app/app.go` embeds 8 simplified tables and has no migration version/checksum | Use `migrations/000001_initial.up.sql`, `schema_migrations`, and tests for empty database/repeated startup | Alpha 1 |
| P0 | `-host` can be set to `0.0.0.0` | Reject non-`127.0.0.1` startup arguments; Launcher reads the real handshake address | Alpha 1 |
| P0 | Global `auth.token`; handlers/syncer read the token directly | Inject `MemoryCredentialStore`; business logic depends only on `CredentialStore` | Alpha 1 |
| P0 | Handlers execute SQL directly and return arbitrary maps/bare arrays/bare errors | Repository + DTO; list envelope and unified error structure | Alpha 1 |
| P0 | Endpoints still use prototype names such as `/api/activity`, `/api/jobs`, `/api/reports/generate`, `/api/repositories/sync` | Align with frozen endpoints such as `/api/activities`, `/api/sync-jobs`, `POST /api/reports` | Alpha 1 |
| P0 | Commits only have the Activity projection; reviews/workflow runs have no fact tables; PR files are not synced | Upsert separate tables idempotently by stable ID/SHA | Alpha 2 |
| P0 | GitHub lists are fixed to `per_page=100` and do not follow Link pagination | GitHub Client consistently parses Link headers and returns PageResult | Alpha 2 |
| P0 | PR CI queries the repository's latest 20 runs and does not compare against PR `head_sha` | Store PR head SHA and workflow runs; only aggregate matching repository + head_sha | Alpha 2 |
| P0 | PR detail, review, and CI request errors are ignored while the job still completes | Write repository/resource-level errors to the job; final status is partial/failed | Alpha 2 |
| P0 | `members.commits/pull_requests/reviews` increments by `+1` on every sync | Remove cumulative facts; aggregate member metrics from commit/PR/review tables; three syncs of the same fixture produce identical results | Alpha 2 |
| P1 | Risk scan deletes all open risks and rebuilds them, so it cannot auto-resolve or preserve lifecycle | Upsert by stable `signal_key`; signals that no longer trigger are updated to Resolved | Alpha 3 |
| P1 | Waiting Review uses PR creation time and does not exclude Draft/Approval/Changes Requested | Compute from the frozen rule, latest effective review, and waiting start | Alpha 3 |
| P1 | Stale PR directly uses PR `updated_at` without defining latest effective activity | Sync and compute `last_activity_at` | Alpha 3 |
| P1 | CI Risk uses repository-level `ci_state` cached on the PR | Compute from current-head workflow facts at query time | Alpha 3 |
| P1 | Frontend risk thresholds are still stored in LocalStorage; the backend has a separate inconsistent settings JSON | Unify through `GET/PATCH /api/settings`, save transactionally, and trigger recalculation | Alpha 3 |
| P1 | Reports count by Activity type; sections, intervals, and fact links do not match the fixed specification | Query from PR/review/workflow/risk/file facts with fixed seven sections and GitHub links | Alpha 3 |
| P1 | Report detail returns a single-element array, and the browser generates the download file itself | `GET /api/reports/{id}` returns an object and provides `/download` | Alpha 3 |
| P1 | Repository selection only sends `owner/name`, with no server-side default 5/hard limit 20 constraint | Persist Repository first, then select by local ID and validate the count | Alpha 2 |
| P2 | The plan references `teampulse-local-first-architecture-option-a.md`, which is missing from the current repository | Restore the architecture document or correct the plan link to avoid a broken design reference | Alpha 1 |

Acceptance order: land migrations and contracts first; then fix data sync idempotency and PR/CI association; finally fix risks and weekly reports. Keep `make run` usable after each item is completed.

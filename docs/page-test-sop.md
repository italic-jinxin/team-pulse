# TeamPulse Page Manual Test SOP

| Item | Current value |
| --- | --- |
| SOP version | 1.4.1 |
| Updated | 2026-07-17 |
| Coverage baseline | Alpha 2 automated data pipeline (Schema Version 1) |
| Supported platforms | Apple Silicon, macOS 13 and later, desktop browsers |

This document is the sole baseline for TeamPulse page-level manual regression testing. Whenever a feature, page, API contract, or phase is completed, update the corresponding test cases, expected results, known limitations, and changelog at the end of this document.

## 1. Test Record

Copy and fill in this section before execution:

| Item | Record |
| --- | --- |
| Test date |  |
| Tester |  |
| Git commit |  |
| macOS / browser version |  |
| Server URL |  |
| Data directory |  |
| GitHub test account | Record only the login; never record the token |
| Test repositories |  |
| Overall result | PASS / FAIL / BLOCKED |

Result definitions:

- `PASS`: actual result matches the expectation.
- `FAIL`: the case is executable but the result is wrong; record the page, steps, time, screenshot, and relevant GitHub links.
- `BLOCKED`: blocked by permissions, rate limits, or insufficient test data; record the blocking reason.
- `N/A`: explicitly not applicable in this run; the reason must be stated.

## 2. Prerequisites

### 2.1 Test Data

Prepare a fine-grained PAT with all repository permissions set to read-only:

- Metadata: Read.
- Contents: Read.
- Pull requests: Read.
- Actions: Read.

Select 1 to 5 known repositories and record the following GitHub facts in advance:

- At least one commit in the last 30 days.
- At least one open PR, ideally including both a draft and a non-draft PR.
- At least one submitted review.
- At least one workflow run for the current PR head SHA.
- At least one merged PR in the previous natural week, to verify weekly reports.
- Optional: a PR waiting for review, inactive for more than 5 days, or with current-head CI failure.

Do not write the PAT into this document, screenshots, issues, logs, or terminal command history.

### 2.2 Start with an Isolated Data Directory

Full regression uses an isolated test database by default to avoid touching production local data:

```bash
./scripts/run-test.sh
```

Open the Vite frontend URL printed by the terminal, `http://127.0.0.1:5174`. The script always uses `./tmp/manual-test`, fixes the backend at `http://127.0.0.1:19421`, and will not read or write the production directory `~/Library/Application Support/TeamPulse`. Vite proxies `/api` to that backend and provides frontend HMR.

For a fresh first-start state:

```bash
./scripts/run-test.sh --fresh
```

`--fresh` only deletes the fixed `./tmp/manual-test` test directory. Before deletion, it checks the server PID recorded in that directory; any running test service must be stopped first. If `19421` or `5174` is already occupied, the script fails before cleanup or test-data creation.

Note: the PAT exists only in server process memory. After the server restarts, GitHub must be connected again. Frontend source changes only trigger Vite HMR and do not restart the server, so they do not clear the PAT.

## 3. First Start and Empty State

### TP-SMOKE-001 Startup and Home Page

- [ ] Run the startup command. The terminal prints `server_ready` once, and the URL uses `127.0.0.1`.
- [ ] The terminal also prints Backend URL `http://127.0.0.1:19421` and Frontend URL `http://127.0.0.1:5174`.
- [ ] Open the Frontend URL. The page title and sidebar show TeamPulse, and Local Server is Running.
- [ ] Overview shows `Finish setup` with initial progress `0/5 complete`.
- [ ] Active Members, Open PRs, CI Failed, and Delivery Risks are all 0.
- [ ] Activity, Risk Signals, and Current Focus show empty states with no blank screen or raw error JSON.

Expected: the page loads, `Last sync` is `never`, Connect GitHub is available, and Generate report is unavailable.

### TP-SMOKE-002 Full Page Navigation

Click the sidebar items in order:

- [ ] Overview: title `Engineering Overview`.
- [ ] Activity: title `Activity Timeline`, with a search box and four filter controls.
- [ ] Pull Requests: title `Pull Requests`, with metrics, Action Filter, Queue, and Selected PR.
- [ ] Team: title `Team Activity`, with time-range controls.
- [ ] Repositories: title `Repositories`, with four repository metrics and the repository table area.
- [ ] Risks: title `Risk Signals`, with the three rule configurations.
- [ ] Reports: title `Reports`, with Weekly Report, Preview, and History.
- [ ] Settings: shows GitHub Connection, Repository Sync, Local Server, Notifications, and Data & Privacy.

Expected: exactly one navigation item is Active at a time. Page switches do not refresh, blank-screen, or log console errors.

### TP-SMOKE-003 Test Data Isolation

- [ ] Startup output shows `TeamPulse test data: <project directory>/tmp/manual-test`.
- [ ] Startup output shows the production directory as `Production data is untouched`.
- [ ] After connecting, syncing, or generating a report, confirm data is written to `./tmp/manual-test/teampulse.db`.

Expected: tests do not modify any database, report, or backup under `~/Library/Application Support/TeamPulse`.

### TP-SMOKE-004 Test Database Must Not Enter Git

- [ ] Start the isolated test service once and confirm `./tmp/manual-test/teampulse.db` was generated.
- [ ] Run `git check-ignore -v tmp/manual-test/teampulse.db`.
- [ ] Run `make check-repository`.
- [ ] Run `git status --short --untracked-files=all` and confirm it does not show `tmp/` or SQLite data files.

Expected: the database is excluded by both directory and extension rules in `.gitignore`; the repository check prints `Repository check passed`. Even if someone uses `git add -f` to force-add the database, `make test` and CI fail.

### TP-SMOKE-005 Frontend Hot Reload and Process Cleanup

- [ ] Keep the test script running, then modify and save a visible page text or style under `web/src/`.
- [ ] Confirm the browser updates automatically, the terminal does not rerun the Go build, and the backend PID stays the same.
- [ ] Revert the test change and confirm the browser updates automatically again.
- [ ] Press `Ctrl-C` to stop the script, then run `lsof -nP -iTCP:19421 -sTCP:LISTEN` and `lsof -nP -iTCP:5174 -sTCP:LISTEN`.

Expected: Vite HMR updates the page without restarting the backend. After exit, no TeamPulse test process listens on either port.

## 4. GitHub Connection

### TP-AUTH-001 Invalid PAT

- [ ] Go to Settings, enter a syntactically valid but invalid test token in the Token input.
- [ ] Click Connect GitHub.

Expected: the page shows the GitHub token rejection error. Connection status remains Not connected. The token does not appear anywhere else on the page.

### TP-AUTH-002 Valid PAT

- [ ] Enter a valid fine-grained PAT and click Connect GitHub.
- [ ] Wait for connection success and repository list loading.

Expected:

- GitHub Connection shows Connected, with the correct GitHub login and `memory` source.
- The token input is cleared, and neither the page nor API responses return the token.
- Repository Sync shows PAT-accessible repositories; name, description, and Private marker match GitHub.
- The Reload button is available.

### TP-AUTH-003 In-Process Credentials

- [ ] Record the current connection status, then stop the server normally.
- [ ] Restart with the same test data directory and refresh the page.

Expected: historical GitHub facts remain, but connection status returns to Not connected. After re-entering the PAT, the app can continue.

## 5. Repository Selection and Sync

### TP-REPO-001 Select Repositories

- [ ] Verify that first load selects at most 5 repositories by default.
- [ ] Use Search repositories to search by owner or repository name.
- [ ] Select and clear a single repository.
- [ ] Click Clear, then click Select visible.
- [ ] After clicking Clear, refresh the repository list and confirm repositories are not selected by default again.
- [ ] Finally select 1 to 5 test repositories.

Expected: first load defaults to at most 5 repositories selected by the server. Selection count updates in real time. The same repository is not duplicated. The server rejects more than 20 selected repositories. After the user explicitly clicks Clear, refresh does not restore the default selection.

### TP-SYNC-001 First Sync

- [ ] Click `Sync N repositories`.
- [ ] Observe Sync Details: job ID, status, progress, current stage, current repository, and repository index.
- [ ] Wait until status becomes Completed, Partial, or Failed.

Expected:

- Completed: progress is 100%, and Dashboard refreshes automatically.
- Partial: the UI clearly says some repositories failed, and data from successful repositories remains viewable.
- Failed: shows a redacted error, without Authorization headers, tokens, or raw GitHub response bodies.
- Recent sync jobs shows this job and its final status.
- Resource failures are written under the corresponding `repository`, `commits`, `pull_requests`, `pull_request_files`, `reviews`, or `workflow_runs`; the job must not show Completed.

### TP-SYNC-002 Repeated Sync Idempotency

- [ ] Record the member count and PR count on Overview, plus one member's Commit/PR/Review counts on Team.
- [ ] Sync the same repositories two more times, for three total runs.
- [ ] Record the same metrics after each run completes.

Expected: if GitHub has no new facts, all three results are the same. Member statistics do not increase unconditionally on every sync. If they differ, mark the result as FAIL and save the three metric sets.

## 6. Overview

### TP-OVERVIEW-001 Post-Sync Overview

- [ ] Check that the connection, repository, and sync steps in Finish setup are complete.
- [ ] Cross-check Active Members, Open PRs, CI Failed, and Delivery Risks counts against the corresponding pages.
- [ ] Check that Recent Engineering Activity shows the latest events, up to the display limit.
- [ ] Click `Review PR queue`; it should navigate to Pull Requests.
- [ ] Return to Overview and click `Open risks`; it should navigate to Risks.

Expected: metrics match the corresponding lists. Activity includes repository, actor, type, and relative time. No old Large PR risk prompt appears.

## 7. Activity

### TP-ACTIVITY-001 Timeline and Filters

- [ ] Search for a known commit title, PR title, or member login.
- [ ] Switch Repository, Actor, Activity Type, and Time Range one by one.
- [ ] Select a combination that should have no results, then restore All.

Expected: timeline is sorted by descending `occurred_at`; filtered results, event counts, and empty states are correct. Breakdown commit/PR/review counts are reasonably consistent with the current synced data.

## 8. Pull Requests

### TP-PR-001 PR Queue and Details

- [ ] Sample one PR and compare its number, title, author, repository, state, and time against GitHub.
- [ ] Switch Action Filter: Fix CI, Review needed, Ready to merge, Reduce scope, Monitor.
- [ ] Click a PR row and inspect Selected PR.
- [ ] Compare Review State, Checks, Draft, added/deleted lines, and updated time against GitHub.
- [ ] Find the sampled PR's local `id` in the `GET /api/pull-requests` response, call `GET /api/pull-requests/{id}`, confirm `pull_request.files_complete` is true, and confirm the number of `files` equals `pull_request.changed_files`.
- [ ] In the detail API's `files`, verify `filename`, `previous_filename`, `status`, `additions`, `deletions`, and `changes`; confirm full file pagination was persisted.
- [ ] Sample and verify the `is_test`, `is_documentation`, `is_configuration`, `is_dependency`, and `is_migration` classifications. Confirm the detail API's `files` objects do not include the `patch`, `blob_url`, `raw_url`, or `contents_url` fields from GitHub's raw PR Files response.
- [ ] Click Open pull request and confirm it opens the correct GitHub PR.

Expected: the page is used to inspect the PR queue and Selected PR. The current UI does not show file facts; file pagination, classification, and sensitive-field checks are based on the detail API. After full file pagination succeeds, `files_complete` is true; if any file page fails, it remains false. Draft PRs must not be treated as Waiting Review risks. PR CI only comes from the current head SHA. Alpha 2 has not yet completed latest-attempt filtering for the same workflow; when rerun scenarios are encountered, record the GitHub facts and mark them as a known limitation.

## 9. Team

### TP-TEAM-001 Member Fact Aggregation

- [ ] Switch Today, 7 days, 30 days, and All time.
- [ ] Select one member and check Commit, PR, Review, Active days, and top repositories.
- [ ] Compare Recent signals against the Activity page.
- [ ] Verify with the three TP-SYNC-002 runs that statistics are not duplicated.

Expected: members come from synced facts, and no personal performance score or ranking is shown. Time-range switching correctly affects visible activity.

## 10. Repositories

### TP-REPOSITORY-001 Repository Health Overview

- [ ] Check that Tracked Repositories matches the current repository list count.
- [ ] Check each repository's Activity, Open PRs, Failed CI, Health, and Last Activity.
- [ ] For a repository with failed PR CI, verify CI Degraded and Needs attention.

Expected: repository health comes from PR CI state for that repository and does not use workflow state from other repositories.

## 11. Risks

### TP-RISK-001 Three Risk Rules

- [ ] Confirm the page only shows Waiting for review, Stale pull request, and CI failure threshold.
- [ ] Temporarily lower the Waiting Review and Stale thresholds, then click Save risk rules.
- [ ] Confirm Version increases and the thresholds persist after page refresh.
- [ ] Run one sync and check whether qualifying non-draft PRs produce risks.
- [ ] Compare against GitHub for Approval, Changes Requested, Draft, and current-head CI.
- [ ] After the test, restore 48 hours, 5 days, and 1 failure.

Expected: only open risks are shown. Exclusion conditions such as Draft, effective Approval, and Changes Requested are correct. Failures from old head SHAs do not produce current PR CI risks.

### TP-RISK-002 Resolve Lifecycle

- [ ] Click Resolve group for a risk group.
- [ ] Confirm the risk disappears from the Open list and the Overview risk count decreases accordingly.
- [ ] If the underlying condition still exists, run sync again.

Expected: when the condition still exists, the risk can reopen. When the condition disappears, it remains Resolved and lifecycle history is preserved.

## 12. Reports

### TP-REPORT-001 Generate Fixed Weekly Report

- [ ] Confirm a PAT is connected and at least one repository is selected and synced.
- [ ] Go to Reports and confirm it shows the current IANA timezone, selected repositories, and previous natural week.
- [ ] Click Generate Weekly Report.

Expected: Preview contains exactly these sections in this fixed order:

1. Summary
2. Completed
3. In Progress
4. Reviews
5. CI Health
6. Risks
7. Repository Activity

- [ ] Sample-check merged/open PR, review, CI, risk counts, and GitHub links.
- [ ] Click Copy Markdown, paste into a local text editor, and check content completeness.
- [ ] Click Download .md, confirm the download succeeds and content matches Preview.
- [ ] Click the report in Report History and confirm the same Markdown can be reloaded.

Expected: the report is based only on selected repositories and the previous natural week. Nothing is uploaded to the cloud. The same fact snapshot generates identical content.

## 13. Settings and Notifications

### TP-SETTINGS-001 Local State and Privacy

- [ ] Check that Host is `127.0.0.1`, and that the Database path and Local-only prompt are correct.
- [ ] Confirm Repository cloning is API-only and Cloud AI analysis is Disabled.
- [ ] Use Refresh/Server Status on the page and confirm Dashboard can refresh.

Expected: the page does not display PATs, Authorization headers, raw patches, or complete GitHub responses.

### TP-NOTIFY-001 Notification Settings

- [ ] Click Enable, and let the tester decide whether to grant browser notification permission.
- [ ] Toggle High risk, Sync failed, Sync success, and Weekly report reminder one by one.
- [ ] Change the Reminder Day and Time, then refresh the page.

Expected: switches and reminder time persist in the current browser. If permission is denied, the page does not interrupt other functionality.

## 14. Failure and Recovery

### TP-ERROR-001 Insufficient Token Permissions

- [ ] Sync with a test token that lacks Actions permission (optional).

Current expected result: related repositories may enter Partial/Failed, while other already-synced facts are retained. Fine-grained degradation without Actions permission is not implemented yet and is a current known limitation to be handled in a later reliability phase; completed Alpha 2 work no longer promises this item.

### TP-ERROR-002 Server Restart

- [ ] After sync completes, stop and restart the server.
- [ ] Refresh the page and visit every page.

Expected: database facts, risk configuration, and report history are retained. PAT connection state is cleared. There are no duplicate migrations or schema errors.

## 15. Current Known Limitations

- Sampling against real public and private GitHub repository data has not yet been executed; passing automated fixtures cannot replace this manual acceptance check.
- The current head SHA is now correctly constrained, but aggregation for the latest attempt of the same workflow is not complete.
- Fine-grained degradation without Actions permission can still cause repository sync to enter Partial/Failed.
- The sidebar and Settings currently show default port 19421. When the server is started directly and falls back to another port, use `server_ready.url` as the source of truth. The isolated test script requires fixed port 19421 and will not start when the port conflicts.
- Alpha reports are fixed to weekly Markdown for the previous natural week and do not include daily/risk reports or multiple templates.

Known limitations must not be marked directly as PASS. During testing, record them as `N/A (known limitation)` or as independent defects, then update this section and the test steps after the corresponding phase implements them.

## 16. Regression Result Summary

| Test ID | Result | Defect / evidence |
| --- | --- | --- |
| TP-SMOKE-001 |  |  |
| TP-SMOKE-002 |  |  |
| TP-SMOKE-003 |  |  |
| TP-SMOKE-004 |  |  |
| TP-SMOKE-005 |  |  |
| TP-AUTH-001 |  |  |
| TP-AUTH-002 |  |  |
| TP-AUTH-003 |  |  |
| TP-REPO-001 |  |  |
| TP-SYNC-001 |  |  |
| TP-SYNC-002 |  |  |
| TP-OVERVIEW-001 |  |  |
| TP-ACTIVITY-001 |  |  |
| TP-PR-001 |  |  |
| TP-TEAM-001 |  |  |
| TP-REPOSITORY-001 |  |  |
| TP-RISK-001 |  |  |
| TP-RISK-002 |  |  |
| TP-REPORT-001 |  |  |
| TP-SETTINGS-001 |  |  |
| TP-NOTIFY-001 |  |  |
| TP-ERROR-001 |  |  |
| TP-ERROR-002 |  |  |

## 17. SOP Maintenance Rules

Whenever a development task is completed:

1. Update the version, date, and coverage baseline at the top of this document.
2. Add test IDs for new features; update steps and expected results when behavior changes.
3. Remove fixed limitations from "Current Known Limitations" and add the corresponding regression steps.
4. Update the changelog below.
5. Execute affected tests; execute the full SOP when a phase is completed.
6. Features or phases whose SOP updates are missing must not be marked complete in the development plan.

## 18. Changelog

| Date | SOP version | Development baseline | Change |
| --- | --- | --- | --- |
| 2026-07-16 | 1.0 | Alpha 1 / Schema 1 | Established the regression baseline for first connection, sync, eight pages, risks, weekly reports, notifications, error recovery, and idempotency |
| 2026-07-16 | 1.1 | Alpha 1 / Schema 1 | Switched to the fixed isolated test startup script and added test data isolation checks |
| 2026-07-16 | 1.2 | Alpha 1 / Schema 1 | Added Git ignore and repository checks for the test database to prevent local databases from being committed by force |
| 2026-07-16 | 1.3 | Alpha 1 / Schema 1 | Changed the isolated test launcher to a two-process backend plus Vite HMR mode; added fixed-port, hot-reload, and exit-cleanup regression steps |
| 2026-07-17 | 1.4 | Alpha 2 automated data pipeline / Schema 1 | Added regression coverage for default repositories, paginated PR files, resource-level errors, head SHA, and three-run idempotency; kept real public/private repository sampling as a manual item |
| 2026-07-17 | 1.4.1 | Alpha 2 automated data pipeline / Schema 1 | Moved PR file pagination, classification, and sensitive-field validation to detail API steps, and clarified fine-grained degradation without Actions permission as later reliability-phase work |

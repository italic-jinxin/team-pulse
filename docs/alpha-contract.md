# TeamPulse Alpha Minimum Contract

Scope freeze: manual read-only PAT, manual sync for at most 20 repositories, three risk rules, fixed Markdown weekly reports, and a basic Launcher. APIs always use `/api`; timestamps are UTC RFC3339; list responses consistently return `{"items":[],"next_cursor":null}`. Errors consistently use `{"error":{"code":"INVALID_ARGUMENT","message":"...","details":{},"request_id":"..."}}`.

## Core APIs

| Method | Path | Request / response summary |
| --- | --- | --- |
| GET | `/api/health` | `{status, version, schema_version}` |
| GET | `/api/app/status` | `{data_dir, authenticated, selected_repositories, last_sync_at}` |
| POST | `/api/system/shutdown` | `202 {status}`; Launcher control domain only |
| GET | `/api/github/auth/status` | `{authenticated, source, login}`; never returns a token |
| POST | `/api/github/auth/token` | `{token}` -> `{authenticated, login}` |
| DELETE | `/api/github/auth` | `204`; clears in-memory credentials without deleting facts |
| GET | `/api/repositories` | Repository list, including `id/github_id/full_name/selected/visibility_status/last_sync_*` |
| PATCH | `/api/repositories/selection` | `{repository_ids:[id...]}`; recommended default is 5, hard server limit is 20 |
| GET | `/api/members` | Fact-aggregated `{id,login,avatar_url,commit_count,pull_request_count,review_count,last_active_at}` |
| POST | `/api/sync-jobs` | `{repository_ids:[id...],since?}` -> `202 {job_id,status}` |
| GET | `/api/sync-jobs/{id}` | `{id,kind,status,current_stage,progress_percent,message,errors:[]}` |
| GET | `/api/activities` | Event list filtered by `repository_id/member_id/type/from/to/limit` |
| GET | `/api/pull-requests` | PR summaries filtered by `repository_id/state/review_state/ci_state/from/to/limit` |
| GET | `/api/pull-requests/{id}` | PR, files, latest effective reviews, and CI summary for the current head SHA only |
| GET | `/api/risks` | Risk list filtered by `status/severity/rule_type/repository_id/limit` |
| PATCH | `/api/risks/{id}` | `{status:"open|resolved"}` -> Risk; automatic recalculation may change the status again |
| POST | `/api/reports` | `{kind:"weekly",period_start?,period_end?,repository_ids?}` -> `201` Report |
| GET | `/api/reports` | Report history list |
| GET | `/api/reports/{id}` | Report metadata and Markdown |
| GET | `/api/reports/{id}/download` | `text/markdown` attachment |
| GET | `/api/settings` | Risk thresholds and non-sensitive application settings |
| PATCH | `/api/settings` | `{version,changes}`; transactionally updates settings and returns the new version |

Stable error codes: `INVALID_ARGUMENT`, `NOT_FOUND`, `CONFLICT`, `CREDENTIAL_MISSING`, `GITHUB_UNAUTHORIZED`, `GITHUB_FORBIDDEN`, `GITHUB_RATE_LIMITED`, `GITHUB_UNAVAILABLE`, `SYNC_PARTIAL`, `DATABASE_ERROR`, `INTERNAL_ERROR`. HTTP statuses map one-to-one with error codes, and handlers must not return raw SQL or GitHub responses.

HTTP mapping: 400 `INVALID_ARGUMENT`; 401 `CREDENTIAL_MISSING/GITHUB_UNAUTHORIZED`; 403 `GITHUB_FORBIDDEN`; 404 `NOT_FOUND`; 409 `CONFLICT`; 429 `GITHUB_RATE_LIMITED`; 502/503 `GITHUB_UNAVAILABLE`; 500 `DATABASE_ERROR/INTERNAL_ERROR`. Core DTOs are fixed as: `Repository{id,github_id,full_name,description,private,selected,visibility_status,last_sync_at,last_sync_status,last_sync_error}`, `Activity{id,event_type,repository,actor,title,url,occurred_at}`, `PullRequest{id,github_id,repository,number,title,author,url,state,draft,review_state,ci_state,head_sha,additions,deletions,changed_files,files_complete,created_at,updated_at,merged_at,last_activity_at}`, `Risk{id,rule_type,severity,status,repository,pull_request,reason,suggested_action,detected_at,last_evaluated_at,resolved_at}`, `Report{id,source_account,kind,title,period_start,period_end,timezone,facts_cutoff_at,repository_scope,repositories,template_version,markdown,created_at}`.

## Module Interfaces

```go
type CredentialStore interface {
    Get(ctx context.Context) (Credential, error)
    Set(ctx context.Context, credential Credential) error
    Delete(ctx context.Context) error
}

type GitHubClient interface {
    CurrentUser(ctx context.Context) (User, error)
    ListRepositories(ctx context.Context, page Page) (PageResult[GitHubRepository], error)
    ListCommits(ctx context.Context, repo RepoRef, since time.Time, page Page) (PageResult[Commit], error)
    ListPullRequests(ctx context.Context, repo RepoRef, state string, page Page) (PageResult[PullRequest], error)
    ListPullRequestFiles(ctx context.Context, repo RepoRef, number int, page Page) (PageResult[PRFile], error)
    ListReviews(ctx context.Context, repo RepoRef, number int, page Page) (PageResult[Review], error)
    ListWorkflowRuns(ctx context.Context, repo RepoRef, since time.Time, page Page) (PageResult[WorkflowRun], error)
}

type Repository interface {
    UpsertSyncPage(ctx context.Context, page SyncPage) error
    GetSyncTargets(ctx context.Context, ids []int64) ([]RepoRef, error)
    GetRiskSnapshot(ctx context.Context, at time.Time) (RiskSnapshot, error)
    GetReportFacts(ctx context.Context, query ReportQuery) (ReportFacts, error)
    ListActivities(ctx context.Context, query ActivityQuery) (PageResult[Activity], error)
    ListPullRequests(ctx context.Context, query PullRequestQuery) (PageResult[PullRequestView], error)
}

type SyncService interface { Start(context.Context, SyncRequest) (SyncJob, error); Get(context.Context, string) (SyncJob, error) }
type RiskService interface { Recalculate(context.Context, []int64) error; List(context.Context, RiskQuery) (PageResult[Risk], error); SetStatus(context.Context, string, string) (Risk, error) }
type ReportService interface { Generate(context.Context, ReportQuery) (Report, error); List(context.Context, ReportQuery) (PageResult[Report], error); Get(context.Context, string) (Report, error) }
```

The dependency direction is fixed as `API/Jobs -> Services -> Repository/GitHubClient/CredentialStore`. Handlers must not read tokens or build SQL. Risk and report logic only consume structured snapshots/facts returned by Repository. Alpha uses `MemoryCredentialStore` and polling jobs; Device Flow, Keychain, and SSE can be added later as adapters or job implementations.

PRAGMA foreign_keys = ON;

-- Table: database migration records, used to track applied versions and verify migration files are immutable.
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY, -- Migration version, incremented as an integer
    name TEXT NOT NULL, -- Migration name
    checksum TEXT NOT NULL, -- Checksum of the migration file contents
    applied_at TEXT NOT NULL -- Migration execution time, UTC RFC3339
);

-- Table: connected GitHub account identities; credentials and tokens are not stored.
CREATE TABLE github_accounts (
    id INTEGER PRIMARY KEY, -- Local primary key
    github_id INTEGER NOT NULL UNIQUE, -- Immutable numeric GitHub user ID
    login TEXT NOT NULL, -- GitHub login, which can be changed by the user
    avatar_url TEXT, -- GitHub avatar URL
    profile_url TEXT, -- GitHub profile URL
    auth_status TEXT NOT NULL DEFAULT 'connected'
        CHECK (auth_status IN ('connected', 'reauth_required', 'disconnected')), -- Account connection status
    github_created_at TEXT, -- GitHub account creation time, UTC RFC3339
    github_updated_at TEXT, -- GitHub account update time, UTC RFC3339
    synced_at TEXT NOT NULL, -- Latest local sync time, UTC RFC3339
    deleted_at TEXT -- Soft deletion time; NULL means active
);

CREATE INDEX idx_github_accounts_login ON github_accounts(login);

-- Table: GitHub repository facts, storing selection and sync status by immutable GitHub ID.
CREATE TABLE repositories (
    id INTEGER PRIMARY KEY, -- Local primary key
    account_id INTEGER NOT NULL REFERENCES github_accounts(id) ON DELETE RESTRICT, -- Owning local GitHub account ID
    github_id INTEGER NOT NULL UNIQUE, -- Immutable numeric GitHub repository ID
    node_id TEXT, -- GitHub GraphQL Node ID
    owner_login TEXT NOT NULL, -- Repository owner login snapshot
    name TEXT NOT NULL, -- Repository name without owner
    full_name TEXT NOT NULL, -- Full owner/name repository name; can be updated after rename
    description TEXT, -- Repository description
    private INTEGER NOT NULL DEFAULT 0 CHECK (private IN (0, 1)), -- Whether the repository is private; 0 no, 1 yes
    default_branch TEXT, -- Default branch name
    html_url TEXT NOT NULL, -- GitHub repository page URL
    visibility_status TEXT NOT NULL DEFAULT 'visible'
        CHECK (visibility_status IN ('visible', 'inaccessible', 'deleted')), -- Local visibility status
    selected INTEGER NOT NULL DEFAULT 0 CHECK (selected IN (0, 1)), -- Whether the user selected the repository for sync; 0 no, 1 yes
    github_created_at TEXT, -- GitHub repository creation time, UTC RFC3339
    github_updated_at TEXT, -- GitHub repository update time, UTC RFC3339
    pushed_at TEXT, -- Latest GitHub push time, UTC RFC3339
    synced_at TEXT NOT NULL, -- Latest repository metadata sync time, UTC RFC3339
    selection_updated_at TEXT, -- Latest time the user changed selection state, UTC RFC3339
    last_sync_at TEXT, -- End time of the latest data sync, UTC RFC3339
    last_sync_status TEXT CHECK (last_sync_status IS NULL OR last_sync_status IN ('completed', 'partial', 'failed')), -- Latest sync result
    last_sync_error_code TEXT, -- Stable error code from the latest sync
    last_sync_error_message TEXT, -- Redacted error summary from the latest sync
    deleted_at TEXT, -- Soft deletion time; NULL means active
    UNIQUE (account_id, full_name) -- Full repository name is unique within the same account
);

CREATE INDEX idx_repositories_selected ON repositories(selected, full_name);
CREATE INDEX idx_repositories_visibility ON repositories(visibility_status, deleted_at);

-- Table: GitHub members found in synced data, including commit, PR, and review participants.
CREATE TABLE team_members (
    id INTEGER PRIMARY KEY, -- Local primary key
    github_id INTEGER UNIQUE, -- Immutable numeric GitHub user ID; nullable when unmapped
    login TEXT NOT NULL, -- GitHub login or display identity when unmapped
    display_name TEXT, -- GitHub display name
    avatar_url TEXT, -- GitHub avatar URL
    profile_url TEXT, -- GitHub profile URL
    user_type TEXT NOT NULL DEFAULT 'User', -- GitHub user type, such as User or Bot
    visibility_status TEXT NOT NULL DEFAULT 'visible'
        CHECK (visibility_status IN ('visible', 'inaccessible', 'deleted')), -- Local visibility status
    github_created_at TEXT, -- GitHub user creation time, UTC RFC3339
    github_updated_at TEXT, -- GitHub user update time, UTC RFC3339
    synced_at TEXT NOT NULL, -- Latest local sync time, UTC RFC3339
    deleted_at TEXT -- Soft deletion time; NULL means active
);

CREATE INDEX idx_team_members_login ON team_members(login);
CREATE INDEX idx_team_members_visibility ON team_members(visibility_status, deleted_at);

-- Table: commit facts; repository ID and SHA form the stable upstream identity.
CREATE TABLE commits (
    id INTEGER PRIMARY KEY, -- Local primary key
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE, -- Owning local repository ID
    sha TEXT NOT NULL, -- Git Commit SHA
    node_id TEXT, -- GitHub GraphQL Node ID
    author_member_id INTEGER REFERENCES team_members(id) ON DELETE SET NULL, -- Mapped commit author member ID
    author_login_snapshot TEXT, -- Author login snapshot at sync time
    author_name TEXT, -- Author name recorded in the Git commit
    committer_member_id INTEGER REFERENCES team_members(id) ON DELETE SET NULL, -- Mapped committer member ID
    message TEXT NOT NULL, -- Full commit message
    html_url TEXT NOT NULL, -- GitHub commit page URL
    authored_at TEXT NOT NULL, -- Git author time, UTC RFC3339
    committed_at TEXT, -- Git commit time, UTC RFC3339
    synced_at TEXT NOT NULL, -- Latest local sync time, UTC RFC3339
    deleted_at TEXT, -- Soft deletion time; NULL means active
    UNIQUE (repository_id, sha) -- Commit SHA is unique within the same repository
);

CREATE INDEX idx_commits_repository_authored ON commits(repository_id, authored_at DESC);
CREATE INDEX idx_commits_author_authored ON commits(author_member_id, authored_at DESC);

-- Table: pull request facts, including current head SHA for precise CI association.
CREATE TABLE pull_requests (
    id INTEGER PRIMARY KEY, -- Local primary key
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE, -- Owning local repository ID
    github_id INTEGER NOT NULL UNIQUE, -- Immutable numeric GitHub PR ID
    node_id TEXT, -- GitHub GraphQL Node ID
    number INTEGER NOT NULL, -- PR number within the repository
    author_member_id INTEGER REFERENCES team_members(id) ON DELETE SET NULL, -- PR author member ID
    author_login_snapshot TEXT NOT NULL, -- Author login snapshot at sync time
    title TEXT NOT NULL, -- PR title
    body TEXT, -- PR description body
    html_url TEXT NOT NULL, -- GitHub PR page URL
    state TEXT NOT NULL CHECK (state IN ('open', 'closed')), -- PR state: open or closed
    draft INTEGER NOT NULL DEFAULT 0 CHECK (draft IN (0, 1)), -- Whether this is a draft; 0 no, 1 yes
    head_ref TEXT, -- Source branch name
    head_sha TEXT NOT NULL, -- Current source branch head SHA, used to associate workflow runs
    base_ref TEXT, -- Target branch name
    base_sha TEXT, -- Target branch base SHA
    additions INTEGER NOT NULL DEFAULT 0 CHECK (additions >= 0), -- Added lines
    deletions INTEGER NOT NULL DEFAULT 0 CHECK (deletions >= 0), -- Deleted lines
    changed_files INTEGER NOT NULL DEFAULT 0 CHECK (changed_files >= 0), -- Changed file count reported by GitHub
    files_complete INTEGER NOT NULL DEFAULT 1 CHECK (files_complete IN (0, 1)), -- Whether the PR file list is complete; 0 no, 1 yes
    review_requested_at TEXT, -- Latest review request time or first observed review request time, UTC RFC3339
    last_author_activity_at TEXT, -- Latest effective author activity time, UTC RFC3339
    last_activity_at TEXT NOT NULL, -- Latest effective PR activity time, UTC RFC3339
    github_created_at TEXT NOT NULL, -- GitHub PR creation time, UTC RFC3339
    github_updated_at TEXT NOT NULL, -- GitHub PR update time, UTC RFC3339
    closed_at TEXT, -- PR close time, UTC RFC3339
    merged_at TEXT, -- PR merge time, UTC RFC3339
    synced_at TEXT NOT NULL, -- Latest local sync time, UTC RFC3339
    deleted_at TEXT, -- Soft deletion time; NULL means active
    UNIQUE (repository_id, number) -- PR number is unique within the same repository
);

CREATE INDEX idx_pull_requests_repository_state_updated
    ON pull_requests(repository_id, state, github_updated_at DESC);
CREATE INDEX idx_pull_requests_head_sha ON pull_requests(repository_id, head_sha);
CREATE INDEX idx_pull_requests_last_activity ON pull_requests(state, draft, last_activity_at);
CREATE INDEX idx_pull_requests_merged_at ON pull_requests(merged_at DESC);

-- Table: PR changed-file metadata and classification; raw patches are not stored.
CREATE TABLE pull_request_files (
    id INTEGER PRIMARY KEY, -- Local primary key
    pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE, -- Owning local PR ID
    filename TEXT NOT NULL, -- Current file path
    previous_filename TEXT, -- File path before rename
    change_status TEXT NOT NULL, -- GitHub file change status, such as added, modified, removed, or renamed
    additions INTEGER NOT NULL DEFAULT 0 CHECK (additions >= 0), -- Added lines in the file
    deletions INTEGER NOT NULL DEFAULT 0 CHECK (deletions >= 0), -- Deleted lines in the file
    changes INTEGER NOT NULL DEFAULT 0 CHECK (changes >= 0), -- Total changed lines in the file
    language TEXT, -- Programming language inferred from extension
    module_name TEXT, -- Module name inferred from directory
    is_test INTEGER NOT NULL DEFAULT 0 CHECK (is_test IN (0, 1)), -- Whether this is a test file; 0 no, 1 yes
    is_documentation INTEGER NOT NULL DEFAULT 0 CHECK (is_documentation IN (0, 1)), -- Whether this is a documentation file; 0 no, 1 yes
    is_configuration INTEGER NOT NULL DEFAULT 0 CHECK (is_configuration IN (0, 1)), -- Whether this is a configuration file; 0 no, 1 yes
    is_dependency INTEGER NOT NULL DEFAULT 0 CHECK (is_dependency IN (0, 1)), -- Whether this is a dependency manifest or lockfile; 0 no, 1 yes
    is_migration INTEGER NOT NULL DEFAULT 0 CHECK (is_migration IN (0, 1)), -- Whether this is a database migration file; 0 no, 1 yes
    synced_at TEXT NOT NULL, -- Latest local sync time, UTC RFC3339
    deleted_at TEXT, -- Soft deletion time; NULL means active
    UNIQUE (pull_request_id, filename) -- Current file path is unique within the same PR
);

CREATE INDEX idx_pull_request_files_pr_filename
    ON pull_request_files(pull_request_id, filename);

-- Table: PR review facts, recording the commit SHA targeted by each review.
CREATE TABLE pull_request_reviews (
    id INTEGER PRIMARY KEY, -- Local primary key
    pull_request_id INTEGER NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE, -- Owning local PR ID
    github_id INTEGER NOT NULL UNIQUE, -- Immutable numeric GitHub review ID
    node_id TEXT, -- GitHub GraphQL Node ID
    reviewer_member_id INTEGER REFERENCES team_members(id) ON DELETE SET NULL, -- Reviewer member ID
    reviewer_login_snapshot TEXT NOT NULL, -- Reviewer login snapshot at sync time
    state TEXT NOT NULL
        CHECK (state IN ('APPROVED', 'CHANGES_REQUESTED', 'COMMENTED', 'DISMISSED', 'PENDING')), -- Review state
    commit_sha TEXT, -- PR commit SHA targeted when the review was submitted
    html_url TEXT, -- GitHub review page URL
    submitted_at TEXT, -- Review submission time, UTC RFC3339
    dismissed_at TEXT, -- Review dismissal time, UTC RFC3339
    synced_at TEXT NOT NULL, -- Latest local sync time, UTC RFC3339
    deleted_at TEXT -- Soft deletion time; NULL means active
);

CREATE INDEX idx_pull_request_reviews_pr_submitted
    ON pull_request_reviews(pull_request_id, submitted_at DESC);
CREATE INDEX idx_pull_request_reviews_reviewer_submitted
    ON pull_request_reviews(reviewer_member_id, submitted_at DESC);

-- Table: GitHub Actions workflow runs, using repository and head SHA to associate PR CI precisely.
CREATE TABLE workflow_runs (
    id INTEGER PRIMARY KEY, -- Local primary key
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE, -- Owning local repository ID
    github_id INTEGER NOT NULL UNIQUE, -- Immutable numeric GitHub workflow run ID
    node_id TEXT, -- GitHub GraphQL Node ID
    workflow_id INTEGER NOT NULL, -- Numeric GitHub workflow ID
    workflow_name TEXT NOT NULL, -- Workflow display name
    run_number INTEGER NOT NULL, -- Run sequence number within the workflow
    run_attempt INTEGER NOT NULL DEFAULT 1, -- Retry attempt count for the same run
    event TEXT, -- Trigger event, such as pull_request or push
    head_branch TEXT, -- Head branch for the run
    head_sha TEXT NOT NULL, -- Head SHA for the run
    status TEXT NOT NULL, -- Run status, such as queued, in_progress, or completed
    conclusion TEXT, -- Completion conclusion, such as success, failure, or timed_out
    html_url TEXT NOT NULL, -- GitHub workflow run page URL
    github_created_at TEXT NOT NULL, -- GitHub run creation time, UTC RFC3339
    github_updated_at TEXT NOT NULL, -- GitHub run update time, UTC RFC3339
    run_started_at TEXT, -- Run start time, UTC RFC3339
    completed_at TEXT, -- Run completion time, UTC RFC3339
    synced_at TEXT NOT NULL, -- Latest local sync time, UTC RFC3339
    deleted_at TEXT -- Soft deletion time; NULL means active
);

CREATE INDEX idx_workflow_runs_repository_head_created
    ON workflow_runs(repository_id, head_sha, github_created_at DESC);
CREATE INDEX idx_workflow_runs_workflow_head_attempt
    ON workflow_runs(repository_id, workflow_id, head_sha, run_number DESC, run_attempt DESC);

-- Table: idempotent activity timeline projection derived from fact tables, supporting page filters and navigation.
CREATE TABLE activity_events (
    id INTEGER PRIMARY KEY, -- Local primary key
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE, -- Owning local repository ID
    actor_member_id INTEGER REFERENCES team_members(id) ON DELETE SET NULL, -- Acting member ID
    actor_login_snapshot TEXT, -- Actor login snapshot at sync time
    event_type TEXT NOT NULL, -- Activity type, such as commit.pushed, pr.merged, or pr.reviewed
    source_type TEXT NOT NULL, -- Source fact type, such as commit, pull_request, review, or workflow_run
    source_id TEXT NOT NULL, -- Stable GitHub ID or SHA of the source fact
    pull_request_id INTEGER REFERENCES pull_requests(id) ON DELETE CASCADE, -- Related local PR ID
    title TEXT NOT NULL, -- Timeline display title
    html_url TEXT, -- GitHub source fact page URL
    occurred_at TEXT NOT NULL, -- Activity occurrence time, UTC RFC3339
    metadata_json TEXT NOT NULL DEFAULT '{}', -- Non-critical extended display data JSON; not a core fact source
    synced_at TEXT NOT NULL, -- Latest local sync time, UTC RFC3339
    deleted_at TEXT, -- Soft deletion time; NULL means active
    UNIQUE (event_type, source_type, source_id, occurred_at) -- Idempotency unique key for activity events
);

CREATE INDEX idx_activity_events_repository_occurred
    ON activity_events(repository_id, occurred_at DESC);
CREATE INDEX idx_activity_events_actor_occurred
    ON activity_events(actor_member_id, occurred_at DESC);
CREATE INDEX idx_activity_events_type_occurred
    ON activity_events(event_type, occurred_at DESC);

-- Table: persisted summary status for manual sync jobs and future scheduled sync jobs.
CREATE TABLE sync_jobs (
    id TEXT PRIMARY KEY, -- Sync job UUID
    kind TEXT NOT NULL CHECK (kind IN ('initial', 'manual', 'incremental', 'compensation')), -- Job type
    status TEXT NOT NULL
        CHECK (status IN ('pending', 'running', 'completed', 'partial', 'failed', 'cancelled')), -- Job status
    requested_at TEXT NOT NULL, -- Job request time, UTC RFC3339
    started_at TEXT, -- Job start time, UTC RFC3339
    finished_at TEXT, -- Job finish time, UTC RFC3339
    current_stage TEXT, -- Current sync resource stage
    completed_items INTEGER NOT NULL DEFAULT 0 CHECK (completed_items >= 0), -- Completed item count
    total_items INTEGER NOT NULL DEFAULT 0 CHECK (total_items >= 0), -- Estimated total item count
    progress_percent INTEGER NOT NULL DEFAULT 0 CHECK (progress_percent BETWEEN 0 AND 100), -- Job progress percentage
    message TEXT, -- User-facing current progress message
    error_code TEXT, -- Stable job-level error code
    error_summary TEXT -- Redacted job-level error summary
);

CREATE INDEX idx_sync_jobs_status_requested ON sync_jobs(status, requested_at DESC);

-- Table: repositories included in a sync job and each repository's execution result.
CREATE TABLE sync_job_repositories (
    job_id TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE, -- Sync job UUID
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE, -- Local repository ID
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'completed', 'partial', 'failed')), -- Sync status for this repository
    started_at TEXT, -- Repository sync start time, UTC RFC3339
    finished_at TEXT, -- Repository sync finish time, UTC RFC3339
    PRIMARY KEY (job_id, repository_id) -- Repository is unique within the same job
);

-- Table: repository- and resource-level sync errors; raw GitHub responses are not stored.
CREATE TABLE sync_job_errors (
    id INTEGER PRIMARY KEY, -- Local primary key
    job_id TEXT NOT NULL REFERENCES sync_jobs(id) ON DELETE CASCADE, -- Owning sync job UUID
    repository_id INTEGER REFERENCES repositories(id) ON DELETE CASCADE, -- Related local repository ID; nullable for job-level errors
    resource_type TEXT NOT NULL, -- Failed resource type, such as commits, pull_requests, reviews, or workflow_runs
    error_code TEXT NOT NULL, -- Stable error code
    message TEXT NOT NULL, -- User-facing redacted error message
    occurred_at TEXT NOT NULL -- Error occurrence time, UTC RFC3339
);

CREATE INDEX idx_sync_job_errors_job ON sync_job_errors(job_id, occurred_at);

-- Table: risk rule toggles, severity, and deterministic threshold configuration.
CREATE TABLE risk_rules (
    rule_type TEXT PRIMARY KEY, -- Stable rule type, such as waiting_for_review, stale_pull_request, or ci_failure
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)), -- Whether the rule is enabled; 0 no, 1 yes
    severity TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high')), -- Default severity
    config_json TEXT NOT NULL DEFAULT '{}', -- Rule threshold configuration JSON
    updated_at TEXT NOT NULL -- Configuration update time, UTC RFC3339
);

-- Table: current lifecycle status and evidence snapshot for each stable risk signal.
CREATE TABLE risk_signals (
    id TEXT PRIMARY KEY, -- Risk signal UUID
    signal_key TEXT NOT NULL UNIQUE, -- Stable idempotency key composed from rule, repository, and subject
    rule_type TEXT NOT NULL REFERENCES risk_rules(rule_type) ON DELETE RESTRICT, -- Rule type that triggered this signal
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE, -- Owning local repository ID
    pull_request_id INTEGER REFERENCES pull_requests(id) ON DELETE CASCADE, -- Related local PR ID
    subject_type TEXT NOT NULL, -- Risk subject type, such as pull_request
    subject_id TEXT NOT NULL, -- Stable risk subject ID
    severity TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high')), -- Current severity
    status TEXT NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'resolved', 'acknowledged', 'dismissed')), -- Current lifecycle status
    reason_code TEXT NOT NULL, -- Stable trigger reason code
    reason TEXT NOT NULL, -- User-facing risk reason
    suggested_action TEXT NOT NULL, -- Suggested handling action
    evidence_json TEXT NOT NULL DEFAULT '{}', -- Structured evidence JSON used for trigger evaluation
    detected_at TEXT NOT NULL, -- First detection time, UTC RFC3339
    last_evaluated_at TEXT NOT NULL, -- Latest rule evaluation time, UTC RFC3339
    resolved_at TEXT, -- Latest automatic or manual resolution time, UTC RFC3339
    acknowledged_at TEXT, -- Risk acknowledgement time, UTC RFC3339
    dismissed_at TEXT, -- Risk dismissal time, UTC RFC3339
    dismiss_reason TEXT, -- Dismissal reason
    occurrence_count INTEGER NOT NULL DEFAULT 1 CHECK (occurrence_count >= 1), -- Number of times the risk has recurred
    UNIQUE (rule_type, repository_id, subject_type, subject_id) -- Keep only one current signal for the same rule and subject
);

CREATE INDEX idx_risk_signals_status_severity_detected
    ON risk_signals(status, severity, detected_at DESC);
CREATE INDEX idx_risk_signals_pr_status ON risk_signals(pull_request_id, status);

-- Table: risk status transition history, used to reconstruct risk status at any report cutoff time.
CREATE TABLE risk_signal_events (
    id INTEGER PRIMARY KEY, -- Local primary key
    risk_signal_id TEXT NOT NULL REFERENCES risk_signals(id) ON DELETE CASCADE, -- Owning risk signal UUID
    occurrence_number INTEGER NOT NULL CHECK (occurrence_number >= 1), -- Risk occurrence number
    event_type TEXT NOT NULL
        CHECK (event_type IN ('opened', 'resolved', 'acknowledged', 'dismissed')), -- Status transition type
    reason_code TEXT, -- Transition reason code
    evidence_json TEXT NOT NULL DEFAULT '{}', -- Structured evidence snapshot JSON at transition time
    occurred_at TEXT NOT NULL, -- Status transition time, UTC RFC3339
    UNIQUE (risk_signal_id, occurrence_number, event_type) -- Transition type is unique within the same occurrence
);

CREATE INDEX idx_risk_signal_events_signal_occurred
    ON risk_signal_events(risk_signal_id, occurred_at DESC);
CREATE INDEX idx_risk_signal_events_occurred
    ON risk_signal_events(occurred_at DESC, event_type);

-- Table: deterministic Markdown reports and the fact cutoff time used for generation.
CREATE TABLE generated_reports (
    id TEXT PRIMARY KEY, -- Report UUID
    source_account_id INTEGER NOT NULL REFERENCES github_accounts(id) ON DELETE RESTRICT, -- Local GitHub account ID that provides report data
    source_account_login_snapshot TEXT NOT NULL, -- Data source account login snapshot at report generation time
    kind TEXT NOT NULL CHECK (kind IN ('weekly', 'daily', 'risk')), -- Report type
    title TEXT NOT NULL, -- Report title
    period_start TEXT NOT NULL, -- Statistics interval start time, UTC RFC3339
    period_end TEXT NOT NULL, -- Statistics interval end time, UTC RFC3339
    timezone TEXT NOT NULL, -- IANA timezone used to calculate natural day and week boundaries
    facts_cutoff_at TEXT NOT NULL, -- Cutoff time for facts visible at report generation time, UTC RFC3339
    repository_scope TEXT NOT NULL DEFAULT 'selected'
        CHECK (repository_scope IN ('selected', 'explicit')), -- Repository scope source: currently selected repositories or explicit request
    template_version TEXT NOT NULL, -- Fixed Markdown template version, such as weekly-v1
    markdown TEXT NOT NULL, -- Full Markdown content
    content_sha256 TEXT NOT NULL, -- Markdown content SHA-256 for deterministic verification
    created_at TEXT NOT NULL -- Report generation time, UTC RFC3339
);

CREATE INDEX idx_generated_reports_created ON generated_reports(created_at DESC);
CREATE INDEX idx_generated_reports_period ON generated_reports(period_start, period_end);
CREATE INDEX idx_generated_reports_account_created
    ON generated_reports(source_account_id, created_at DESC);

-- Table: repository set actually included when a report is generated, avoiding hidden core scope in JSON.
CREATE TABLE generated_report_repositories (
    report_id TEXT NOT NULL REFERENCES generated_reports(id) ON DELETE CASCADE, -- Report UUID
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE RESTRICT, -- Local repository ID included in the report
    full_name_snapshot TEXT NOT NULL, -- owner/name repository snapshot at report generation time
    PRIMARY KEY (report_id, repository_id) -- Repository is unique within the same report
);

CREATE INDEX idx_generated_report_repositories_repository
    ON generated_report_repositories(repository_id, report_id);

-- Table: versioned non-sensitive application settings; credentials and tokens are forbidden.
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY, -- Stable setting key
    value_json TEXT NOT NULL, -- Setting value JSON
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1), -- Optimistic concurrency control version
    updated_at TEXT NOT NULL -- Setting update time, UTC RFC3339
);

INSERT INTO risk_rules(rule_type, enabled, severity, config_json, updated_at) VALUES
    ('waiting_for_review', 1, 'medium', '{"hours":48}', strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('stale_pull_request', 1, 'medium', '{"days":5}', strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ('ci_failure', 1, 'high', '{"failure_threshold":1}', strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

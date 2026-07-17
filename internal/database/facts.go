package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AccountFact struct {
	GitHubID        int64
	Login           string
	AvatarURL       string
	ProfileURL      string
	GitHubCreatedAt string
	GitHubUpdatedAt string
	SyncedAt        string
}

type AccountRecord struct {
	ID       int64
	GitHubID int64
	Login    string
}

type RepositoryFact struct {
	GitHubID        int64
	NodeID          string
	OwnerLogin      string
	Name            string
	FullName        string
	Description     string
	Private         bool
	DefaultBranch   string
	HTMLURL         string
	GitHubCreatedAt string
	GitHubUpdatedAt string
	PushedAt        string
	SyncedAt        string
}

type SyncTarget struct {
	ID       int64
	GitHubID int64
	FullName string
}

type MemberFact struct {
	GitHubID    int64
	Login       string
	DisplayName string
	AvatarURL   string
	ProfileURL  string
	UserType    string
	SyncedAt    string
}

type CommitFact struct {
	RepositoryID      int64
	SHA               string
	NodeID            string
	AuthorMemberID    *int64
	AuthorLogin       string
	AuthorName        string
	CommitterMemberID *int64
	Message           string
	HTMLURL           string
	AuthoredAt        string
	CommittedAt       string
	SyncedAt          string
}

type PullRequestFact struct {
	RepositoryID         int64
	GitHubID             int64
	NodeID               string
	Number               int
	AuthorMemberID       *int64
	AuthorLogin          string
	Title                string
	Body                 string
	HTMLURL              string
	State                string
	Draft                bool
	HeadRef              string
	HeadSHA              string
	BaseRef              string
	BaseSHA              string
	Additions            int
	Deletions            int
	ChangedFiles         int
	FilesComplete        bool
	ReviewRequestedAt    string
	LastAuthorActivityAt string
	LastActivityAt       string
	GitHubCreatedAt      string
	GitHubUpdatedAt      string
	ClosedAt             string
	MergedAt             string
	SyncedAt             string
}

type PullRequestFileFact struct {
	Filename         string
	PreviousFilename string
	Status           string
	Additions        int
	Deletions        int
	Changes          int
	Language         string
	ModuleName       string
	IsTest           bool
	IsDocumentation  bool
	IsConfiguration  bool
	IsDependency     bool
	IsMigration      bool
}

type ReviewFact struct {
	PullRequestID int64
	GitHubID      int64
	NodeID        string
	ReviewerID    *int64
	ReviewerLogin string
	State         string
	CommitSHA     string
	HTMLURL       string
	SubmittedAt   string
	SyncedAt      string
}

type WorkflowRunFact struct {
	RepositoryID    int64
	GitHubID        int64
	NodeID          string
	WorkflowID      int64
	WorkflowName    string
	RunNumber       int
	RunAttempt      int
	Event           string
	HeadBranch      string
	HeadSHA         string
	Status          string
	Conclusion      string
	HTMLURL         string
	GitHubCreatedAt string
	GitHubUpdatedAt string
	RunStartedAt    string
	CompletedAt     string
	SyncedAt        string
}

type ActivityFact struct {
	RepositoryID  int64
	ActorMemberID *int64
	ActorLogin    string
	EventType     string
	SourceType    string
	SourceID      string
	PullRequestID *int64
	Title         string
	HTMLURL       string
	OccurredAt    string
	MetadataJSON  string
	SyncedAt      string
}

func (r *SQLiteRepository) UpsertAccount(ctx context.Context, fact AccountFact) (AccountRecord, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO github_accounts(
			github_id, login, avatar_url, profile_url, auth_status,
			github_created_at, github_updated_at, synced_at, deleted_at
		) VALUES(?,?,?,?, 'connected', ?,?,?, NULL)
		ON CONFLICT(github_id) DO UPDATE SET
			login=excluded.login, avatar_url=excluded.avatar_url,
			profile_url=excluded.profile_url, auth_status='connected',
			github_created_at=excluded.github_created_at,
			github_updated_at=excluded.github_updated_at,
			synced_at=excluded.synced_at, deleted_at=NULL
	`, fact.GitHubID, fact.Login, nullableString(fact.AvatarURL), nullableString(fact.ProfileURL),
		nullableString(fact.GitHubCreatedAt), nullableString(fact.GitHubUpdatedAt), fact.SyncedAt)
	if err != nil {
		return AccountRecord{}, err
	}
	return r.AccountByGitHubID(ctx, fact.GitHubID)
}

func (r *SQLiteRepository) AccountByGitHubID(ctx context.Context, githubID int64) (AccountRecord, error) {
	var account AccountRecord
	err := r.db.QueryRowContext(ctx, `
		SELECT id, github_id, login FROM github_accounts
		WHERE github_id=? AND deleted_at IS NULL
	`, githubID).Scan(&account.ID, &account.GitHubID, &account.Login)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountRecord{}, ErrNotFound
	}
	return account, err
}

func (r *SQLiteRepository) AccountByLogin(ctx context.Context, login string) (AccountRecord, error) {
	var account AccountRecord
	err := r.db.QueryRowContext(ctx, `
		SELECT id, github_id, login FROM github_accounts
		WHERE login=? AND deleted_at IS NULL ORDER BY synced_at DESC LIMIT 1
	`, login).Scan(&account.ID, &account.GitHubID, &account.Login)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountRecord{}, ErrNotFound
	}
	return account, err
}

func (r *SQLiteRepository) UpsertRepository(ctx context.Context, accountID int64, fact RepositoryFact) (int64, error) {
	owner, name, ok := strings.Cut(fact.FullName, "/")
	if fact.OwnerLogin == "" {
		fact.OwnerLogin = owner
	}
	if fact.Name == "" && ok {
		fact.Name = name
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO repositories(
			account_id, github_id, node_id, owner_login, name, full_name,
			description, private, default_branch, html_url, visibility_status,
			github_created_at, github_updated_at, pushed_at, synced_at, deleted_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,'visible',?,?,?,?,NULL)
		ON CONFLICT(github_id) DO UPDATE SET
			account_id=excluded.account_id, node_id=excluded.node_id,
			owner_login=excluded.owner_login, name=excluded.name,
			full_name=excluded.full_name, description=excluded.description,
			private=excluded.private, default_branch=excluded.default_branch,
			html_url=excluded.html_url, visibility_status='visible',
			github_created_at=excluded.github_created_at,
			github_updated_at=excluded.github_updated_at,
			pushed_at=excluded.pushed_at, synced_at=excluded.synced_at,
			deleted_at=NULL
	`, accountID, fact.GitHubID, nullableString(fact.NodeID), fact.OwnerLogin, fact.Name,
		fact.FullName, nullableString(fact.Description), fact.Private,
		nullableString(fact.DefaultBranch), fact.HTMLURL,
		nullableString(fact.GitHubCreatedAt), nullableString(fact.GitHubUpdatedAt),
		nullableString(fact.PushedAt), fact.SyncedAt)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.db.QueryRowContext(ctx, "SELECT id FROM repositories WHERE github_id=?", fact.GitHubID).Scan(&id)
	return id, err
}

func (r *SQLiteRepository) SetRepositorySelection(ctx context.Context, ids []int64) error {
	if len(ids) > 20 {
		return fmt.Errorf("at most 20 repositories may be selected")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "UPDATE repositories SET selected=0, selection_updated_at=?", utcNow()); err != nil {
		return err
	}
	if len(ids) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		args := make([]any, 0, len(ids)+1)
		args = append(args, utcNow())
		for _, id := range ids {
			args = append(args, id)
		}
		result, err := tx.ExecContext(ctx,
			"UPDATE repositories SET selected=1, selection_updated_at=? WHERE id IN ("+placeholders+") AND deleted_at IS NULL",
			args...,
		)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != int64(len(ids)) {
			return fmt.Errorf("one or more repositories were not found")
		}
	}
	return tx.Commit()
}

func (r *SQLiteRepository) EnsureDefaultRepositorySelection(ctx context.Context, accountID int64, limit int) error {
	if limit <= 0 || limit > 5 {
		limit = 5
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var selectionInitialized, repositoryCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(selection_updated_at), COUNT(*)
		FROM repositories
		WHERE account_id=? AND deleted_at IS NULL
	`, accountID).Scan(&selectionInitialized, &repositoryCount); err != nil {
		return err
	}
	if selectionInitialized > 0 || repositoryCount == 0 {
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE repositories
		SET selected=1, selection_updated_at=?
		WHERE id IN (
			SELECT id FROM repositories
			WHERE account_id=? AND deleted_at IS NULL
			ORDER BY COALESCE(pushed_at, '') DESC, full_name
			LIMIT ?
		)
	`, utcNow(), accountID, limit); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SQLiteRepository) SyncTargets(ctx context.Context, ids []int64) ([]SyncTarget, error) {
	query := `SELECT id, github_id, full_name FROM repositories WHERE selected=1 AND deleted_at IS NULL ORDER BY full_name`
	args := []any{}
	if len(ids) > 0 {
		if len(ids) > 20 {
			return nil, fmt.Errorf("at most 20 repositories may be synchronized")
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		query = "SELECT id, github_id, full_name FROM repositories WHERE id IN (" + placeholders + ") AND deleted_at IS NULL ORDER BY full_name"
		for _, id := range ids {
			args = append(args, id)
		}
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SyncTarget{}
	for rows.Next() {
		var target SyncTarget
		if err := rows.Scan(&target.ID, &target.GitHubID, &target.FullName); err != nil {
			return nil, err
		}
		result = append(result, target)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no repositories selected")
	}
	if len(result) > 20 {
		return nil, fmt.Errorf("at most 20 repositories may be synchronized")
	}
	return result, nil
}

func (r *SQLiteRepository) CreateSyncJob(ctx context.Context, id string, targets []SyncTarget) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sync_jobs(id, kind, status, requested_at, total_items, message)
		VALUES(?, 'manual', 'pending', ?, ?, 'Queued')
	`, id, utcNow(), len(targets)); err != nil {
		return err
	}
	for _, target := range targets {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sync_job_repositories(job_id, repository_id) VALUES(?,?)
		`, id, target.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLiteRepository) UpdateJob(ctx context.Context, id, status, stage, message string, progress, completed int, errorCode, errorSummary string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE sync_jobs SET
			status=?, current_stage=?, message=?, progress_percent=?, completed_items=?,
			started_at=CASE WHEN ?='running' AND started_at IS NULL THEN ? ELSE started_at END,
			finished_at=CASE WHEN ? IN ('completed','partial','failed','cancelled') THEN ? ELSE finished_at END,
			error_code=?, error_summary=?
		WHERE id=?
	`, status, nullableString(stage), nullableString(message), progress, completed,
		status, utcNow(), status, utcNow(), nullableString(errorCode), nullableString(errorSummary), id)
	return err
}

func (r *SQLiteRepository) UpdateJobRepository(ctx context.Context, jobID string, repositoryID int64, status string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE sync_job_repositories SET
			status=?,
			started_at=CASE WHEN ?='running' AND started_at IS NULL THEN ? ELSE started_at END,
			finished_at=CASE WHEN ? IN ('completed','partial','failed') THEN ? ELSE finished_at END
		WHERE job_id=? AND repository_id=?
	`, status, status, utcNow(), status, utcNow(), jobID, repositoryID)
	return err
}

func (r *SQLiteRepository) AddJobError(ctx context.Context, jobID string, repositoryID *int64, resourceType, code, message string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sync_job_errors(job_id, repository_id, resource_type, error_code, message, occurred_at)
		VALUES(?,?,?,?,?,?)
	`, jobID, repositoryID, resourceType, code, message, utcNow())
	return err
}

func (r *SQLiteRepository) UpsertMember(ctx context.Context, fact MemberFact) (*int64, error) {
	if fact.GitHubID == 0 || fact.Login == "" {
		return nil, nil
	}
	userType := fact.UserType
	if userType == "" {
		userType = "User"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO team_members(github_id, login, display_name, avatar_url, profile_url, user_type, synced_at, deleted_at)
		VALUES(?,?,?,?,?,?,?,NULL)
		ON CONFLICT(github_id) DO UPDATE SET
			login=excluded.login, display_name=excluded.display_name,
			avatar_url=excluded.avatar_url, profile_url=excluded.profile_url,
			user_type=excluded.user_type, synced_at=excluded.synced_at, deleted_at=NULL
	`, fact.GitHubID, fact.Login, nullableString(fact.DisplayName), nullableString(fact.AvatarURL),
		nullableString(fact.ProfileURL), userType, fact.SyncedAt)
	if err != nil {
		return nil, err
	}
	var id int64
	if err := r.db.QueryRowContext(ctx, "SELECT id FROM team_members WHERE github_id=?", fact.GitHubID).Scan(&id); err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *SQLiteRepository) UpsertCommit(ctx context.Context, fact CommitFact) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO commits(
			repository_id, sha, node_id, author_member_id, author_login_snapshot,
			author_name, committer_member_id, message, html_url, authored_at,
			committed_at, synced_at, deleted_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,NULL)
		ON CONFLICT(repository_id, sha) DO UPDATE SET
			node_id=excluded.node_id, author_member_id=excluded.author_member_id,
			author_login_snapshot=excluded.author_login_snapshot, author_name=excluded.author_name,
			committer_member_id=excluded.committer_member_id, message=excluded.message,
			html_url=excluded.html_url, authored_at=excluded.authored_at,
			committed_at=excluded.committed_at, synced_at=excluded.synced_at, deleted_at=NULL
	`, fact.RepositoryID, fact.SHA, nullableString(fact.NodeID), fact.AuthorMemberID,
		nullableString(fact.AuthorLogin), nullableString(fact.AuthorName), fact.CommitterMemberID,
		fact.Message, fact.HTMLURL, fact.AuthoredAt, nullableString(fact.CommittedAt), fact.SyncedAt)
	return err
}

func (r *SQLiteRepository) UpsertPullRequest(ctx context.Context, fact PullRequestFact) (int64, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pull_requests(
			repository_id, github_id, node_id, number, author_member_id, author_login_snapshot,
			title, body, html_url, state, draft, head_ref, head_sha, base_ref, base_sha,
			additions, deletions, changed_files, files_complete, review_requested_at,
			last_author_activity_at, last_activity_at, github_created_at, github_updated_at,
			closed_at, merged_at, synced_at, deleted_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL)
		ON CONFLICT(github_id) DO UPDATE SET
			repository_id=excluded.repository_id, node_id=excluded.node_id, number=excluded.number,
			author_member_id=excluded.author_member_id, author_login_snapshot=excluded.author_login_snapshot,
			title=excluded.title, body=excluded.body, html_url=excluded.html_url,
			state=excluded.state, draft=excluded.draft, head_ref=excluded.head_ref,
			head_sha=excluded.head_sha, base_ref=excluded.base_ref, base_sha=excluded.base_sha,
			additions=excluded.additions, deletions=excluded.deletions,
			changed_files=excluded.changed_files, files_complete=excluded.files_complete,
			review_requested_at=COALESCE(excluded.review_requested_at, pull_requests.review_requested_at),
			last_author_activity_at=excluded.last_author_activity_at,
			last_activity_at=excluded.last_activity_at,
			github_updated_at=excluded.github_updated_at, closed_at=excluded.closed_at,
			merged_at=excluded.merged_at, synced_at=excluded.synced_at, deleted_at=NULL
	`, fact.RepositoryID, fact.GitHubID, nullableString(fact.NodeID), fact.Number,
		fact.AuthorMemberID, fact.AuthorLogin, fact.Title, nullableString(fact.Body), fact.HTMLURL,
		fact.State, fact.Draft, nullableString(fact.HeadRef), fact.HeadSHA,
		nullableString(fact.BaseRef), nullableString(fact.BaseSHA), fact.Additions, fact.Deletions,
		fact.ChangedFiles, fact.FilesComplete, nullableString(fact.ReviewRequestedAt),
		nullableString(fact.LastAuthorActivityAt), fact.LastActivityAt, fact.GitHubCreatedAt,
		fact.GitHubUpdatedAt, nullableString(fact.ClosedAt), nullableString(fact.MergedAt), fact.SyncedAt)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.db.QueryRowContext(ctx, "SELECT id FROM pull_requests WHERE github_id=?", fact.GitHubID).Scan(&id)
	return id, err
}

func (r *SQLiteRepository) ReplacePullRequestFiles(ctx context.Context, pullRequestID int64, files []PullRequestFileFact, filesComplete bool, syncedAt string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE pull_request_files SET deleted_at=? WHERE pull_request_id=? AND deleted_at IS NULL
	`, syncedAt, pullRequestID); err != nil {
		return err
	}
	for _, file := range files {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO pull_request_files(
				pull_request_id, filename, previous_filename, change_status,
				additions, deletions, changes, language, module_name,
				is_test, is_documentation, is_configuration, is_dependency,
				is_migration, synced_at, deleted_at
			) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL)
			ON CONFLICT(pull_request_id, filename) DO UPDATE SET
				previous_filename=excluded.previous_filename,
				change_status=excluded.change_status,
				additions=excluded.additions, deletions=excluded.deletions,
				changes=excluded.changes, language=excluded.language,
				module_name=excluded.module_name, is_test=excluded.is_test,
				is_documentation=excluded.is_documentation,
				is_configuration=excluded.is_configuration,
				is_dependency=excluded.is_dependency,
				is_migration=excluded.is_migration,
				synced_at=excluded.synced_at, deleted_at=NULL
		`, pullRequestID, file.Filename, nullableString(file.PreviousFilename), file.Status,
			file.Additions, file.Deletions, file.Changes, nullableString(file.Language),
			nullableString(file.ModuleName), file.IsTest, file.IsDocumentation,
			file.IsConfiguration, file.IsDependency, file.IsMigration, syncedAt); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, "UPDATE pull_requests SET files_complete=? WHERE id=?", filesComplete, pullRequestID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (r *SQLiteRepository) UpsertReview(ctx context.Context, fact ReviewFact) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pull_request_reviews(
			pull_request_id, github_id, node_id, reviewer_member_id,
			reviewer_login_snapshot, state, commit_sha, html_url, submitted_at, synced_at, deleted_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,NULL)
		ON CONFLICT(github_id) DO UPDATE SET
			pull_request_id=excluded.pull_request_id, node_id=excluded.node_id,
			reviewer_member_id=excluded.reviewer_member_id,
			reviewer_login_snapshot=excluded.reviewer_login_snapshot,
			state=excluded.state, commit_sha=excluded.commit_sha, html_url=excluded.html_url,
			submitted_at=excluded.submitted_at, synced_at=excluded.synced_at, deleted_at=NULL
	`, fact.PullRequestID, fact.GitHubID, nullableString(fact.NodeID), fact.ReviewerID,
		fact.ReviewerLogin, fact.State, nullableString(fact.CommitSHA), nullableString(fact.HTMLURL),
		nullableString(fact.SubmittedAt), fact.SyncedAt)
	return err
}

func (r *SQLiteRepository) UpsertWorkflowRun(ctx context.Context, fact WorkflowRunFact) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workflow_runs(
			repository_id, github_id, node_id, workflow_id, workflow_name, run_number,
			run_attempt, event, head_branch, head_sha, status, conclusion, html_url,
			github_created_at, github_updated_at, run_started_at, completed_at, synced_at, deleted_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL)
		ON CONFLICT(github_id) DO UPDATE SET
			repository_id=excluded.repository_id, node_id=excluded.node_id,
			workflow_id=excluded.workflow_id, workflow_name=excluded.workflow_name,
			run_number=excluded.run_number, run_attempt=excluded.run_attempt,
			event=excluded.event, head_branch=excluded.head_branch, head_sha=excluded.head_sha,
			status=excluded.status, conclusion=excluded.conclusion, html_url=excluded.html_url,
			github_created_at=excluded.github_created_at, github_updated_at=excluded.github_updated_at,
			run_started_at=excluded.run_started_at, completed_at=excluded.completed_at,
			synced_at=excluded.synced_at, deleted_at=NULL
	`, fact.RepositoryID, fact.GitHubID, nullableString(fact.NodeID), fact.WorkflowID,
		fact.WorkflowName, fact.RunNumber, fact.RunAttempt, nullableString(fact.Event),
		nullableString(fact.HeadBranch), fact.HeadSHA, fact.Status, nullableString(fact.Conclusion),
		fact.HTMLURL, fact.GitHubCreatedAt, fact.GitHubUpdatedAt, nullableString(fact.RunStartedAt),
		nullableString(fact.CompletedAt), fact.SyncedAt)
	return err
}

func (r *SQLiteRepository) UpsertActivity(ctx context.Context, fact ActivityFact) error {
	metadata := fact.MetadataJSON
	if metadata == "" {
		metadata = "{}"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO activity_events(
			repository_id, actor_member_id, actor_login_snapshot, event_type,
			source_type, source_id, pull_request_id, title, html_url,
			occurred_at, metadata_json, synced_at, deleted_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,NULL)
		ON CONFLICT(event_type, source_type, source_id, occurred_at) DO UPDATE SET
			repository_id=excluded.repository_id, actor_member_id=excluded.actor_member_id,
			actor_login_snapshot=excluded.actor_login_snapshot,
			pull_request_id=excluded.pull_request_id, title=excluded.title,
			html_url=excluded.html_url, metadata_json=excluded.metadata_json,
			synced_at=excluded.synced_at, deleted_at=NULL
	`, fact.RepositoryID, fact.ActorMemberID, nullableString(fact.ActorLogin), fact.EventType,
		fact.SourceType, fact.SourceID, fact.PullRequestID, fact.Title,
		nullableString(fact.HTMLURL), fact.OccurredAt, metadata, fact.SyncedAt)
	return err
}

func (r *SQLiteRepository) MarkRepositorySync(ctx context.Context, repositoryID int64, status, errorCode, errorMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE repositories SET last_sync_at=?, last_sync_status=?,
			last_sync_error_code=?, last_sync_error_message=?
		WHERE id=?
	`, utcNow(), status, nullableString(errorCode), nullableString(errorMessage), repositoryID)
	return err
}

func utcNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

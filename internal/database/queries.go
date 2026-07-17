package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("not found")

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

type AppStatus struct {
	Repositories     int     `json:"repositories"`
	OpenPullRequests int     `json:"open_pull_requests"`
	OpenRisks        int     `json:"open_risks"`
	Members          int     `json:"members"`
	LastSyncAt       *string `json:"last_sync_at"`
}

type RepositoryRecord struct {
	ID                   int64   `json:"id"`
	GitHubID             int64   `json:"github_id"`
	FullName             string  `json:"full_name"`
	Description          string  `json:"description"`
	Private              bool    `json:"private"`
	DefaultBranch        string  `json:"default_branch"`
	HTMLURL              string  `json:"html_url"`
	Selected             bool    `json:"selected"`
	VisibilityStatus     string  `json:"visibility_status"`
	GitHubUpdatedAt      *string `json:"updated_at"`
	LastSyncAt           *string `json:"last_sync_at"`
	LastSyncStatus       *string `json:"last_sync_status"`
	LastSyncErrorCode    *string `json:"last_sync_error_code"`
	LastSyncErrorMessage *string `json:"last_sync_error_message"`
}

type ActivityRecord struct {
	ID         int64   `json:"id"`
	Repository string  `json:"repository"`
	Actor      string  `json:"actor"`
	Type       string  `json:"type"`
	Title      string  `json:"title"`
	URL        *string `json:"url"`
	OccurredAt string  `json:"occurred_at"`
}

type MemberRecord struct {
	ID           int64   `json:"id"`
	Login        string  `json:"login"`
	AvatarURL    string  `json:"avatar_url"`
	Commits      int     `json:"commits"`
	PullRequests int     `json:"pull_requests"`
	Reviews      int     `json:"reviews"`
	LastActiveAt *string `json:"last_active_at"`
}

type PullRequestRecord struct {
	ID             int64   `json:"id"`
	GitHubID       int64   `json:"github_id"`
	Repository     string  `json:"repository"`
	Number         int     `json:"number"`
	Title          string  `json:"title"`
	Author         string  `json:"author"`
	URL            string  `json:"url"`
	State          string  `json:"state"`
	Draft          bool    `json:"draft"`
	ReviewState    string  `json:"review_state"`
	CIState        string  `json:"ci_state"`
	HeadSHA        string  `json:"head_sha"`
	Additions      int     `json:"additions"`
	Deletions      int     `json:"deletions"`
	ChangedFiles   int     `json:"changed_files"`
	FilesComplete  bool    `json:"files_complete"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	MergedAt       *string `json:"merged_at"`
	LastActivityAt string  `json:"last_activity_at"`
}

type PullRequestFileRecord struct {
	Filename         string  `json:"filename"`
	PreviousFilename *string `json:"previous_filename"`
	Status           string  `json:"status"`
	Additions        int     `json:"additions"`
	Deletions        int     `json:"deletions"`
	Changes          int     `json:"changes"`
	Language         *string `json:"language"`
	ModuleName       *string `json:"module_name"`
	IsTest           bool    `json:"is_test"`
	IsDocumentation  bool    `json:"is_documentation"`
	IsConfiguration  bool    `json:"is_configuration"`
	IsDependency     bool    `json:"is_dependency"`
	IsMigration      bool    `json:"is_migration"`
}

type PullRequestReviewRecord struct {
	GitHubID    int64   `json:"github_id"`
	Reviewer    string  `json:"reviewer"`
	State       string  `json:"state"`
	CommitSHA   *string `json:"commit_sha"`
	URL         *string `json:"url"`
	SubmittedAt *string `json:"submitted_at"`
}

type WorkflowRunRecord struct {
	GitHubID   int64   `json:"github_id"`
	WorkflowID int64   `json:"workflow_id"`
	Name       string  `json:"name"`
	RunNumber  int     `json:"run_number"`
	RunAttempt int     `json:"run_attempt"`
	HeadSHA    string  `json:"head_sha"`
	Status     string  `json:"status"`
	Conclusion *string `json:"conclusion"`
	URL        string  `json:"url"`
	CreatedAt  string  `json:"created_at"`
}

type PullRequestDetail struct {
	PullRequest  PullRequestRecord         `json:"pull_request"`
	Files        []PullRequestFileRecord   `json:"files"`
	Reviews      []PullRequestReviewRecord `json:"reviews"`
	WorkflowRuns []WorkflowRunRecord       `json:"workflow_runs"`
}

type RiskRecord struct {
	ID              string  `json:"id"`
	Type            string  `json:"type"`
	Severity        string  `json:"severity"`
	Repository      string  `json:"repository"`
	PRNumber        *int    `json:"pr_number"`
	Owner           *string `json:"owner"`
	Reason          string  `json:"reason"`
	SuggestedAction string  `json:"suggested_action"`
	Status          string  `json:"status"`
	DetectedAt      string  `json:"detected_at"`
	LastEvaluatedAt string  `json:"last_evaluated_at"`
	ResolvedAt      *string `json:"resolved_at"`
}

type JobRecord struct {
	ID             string  `json:"id"`
	Type           string  `json:"type"`
	Status         string  `json:"status"`
	Progress       int     `json:"progress"`
	Message        *string `json:"message"`
	CreatedAt      string  `json:"created_at"`
	StartedAt      *string `json:"started_at"`
	EndedAt        *string `json:"ended_at"`
	Error          *string `json:"error"`
	CurrentStage   *string `json:"current_stage"`
	CompletedItems int     `json:"completed_items"`
	TotalItems     int     `json:"total_items"`
}

type ReportRecord struct {
	ID                         string   `json:"id"`
	SourceAccountID            int64    `json:"source_account_id"`
	SourceAccountLoginSnapshot string   `json:"source_account_login_snapshot"`
	Kind                       string   `json:"kind"`
	Title                      string   `json:"title"`
	PeriodStart                string   `json:"period_start"`
	PeriodEnd                  string   `json:"period_end"`
	Timezone                   string   `json:"timezone"`
	FactsCutoffAt              string   `json:"facts_cutoff_at"`
	RepositoryScope            string   `json:"repository_scope"`
	TemplateVersion            string   `json:"template_version"`
	Markdown                   *string  `json:"markdown,omitempty"`
	CreatedAt                  string   `json:"created_at"`
	Repositories               []string `json:"repositories"`
}

func (r *SQLiteRepository) Status(ctx context.Context) (AppStatus, error) {
	var result AppStatus
	err := r.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM repositories WHERE deleted_at IS NULL),
			(SELECT COUNT(*) FROM pull_requests WHERE state='open' AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM risk_signals WHERE status='open'),
			(SELECT COUNT(*) FROM team_members WHERE deleted_at IS NULL),
			(SELECT MAX(finished_at) FROM sync_jobs WHERE status IN ('completed','partial'))
	`).Scan(&result.Repositories, &result.OpenPullRequests, &result.OpenRisks, &result.Members, &result.LastSyncAt)
	return result, err
}

func (r *SQLiteRepository) ListRepositories(ctx context.Context) ([]RepositoryRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, github_id, full_name, COALESCE(description,''), private,
		       COALESCE(default_branch,''), html_url, selected, visibility_status,
		       github_updated_at, last_sync_at, last_sync_status,
		       last_sync_error_code, last_sync_error_message
		FROM repositories
		WHERE deleted_at IS NULL
		ORDER BY full_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []RepositoryRecord{}
	for rows.Next() {
		var item RepositoryRecord
		if err := rows.Scan(&item.ID, &item.GitHubID, &item.FullName, &item.Description, &item.Private,
			&item.DefaultBranch, &item.HTMLURL, &item.Selected, &item.VisibilityStatus,
			&item.GitHubUpdatedAt, &item.LastSyncAt, &item.LastSyncStatus,
			&item.LastSyncErrorCode, &item.LastSyncErrorMessage); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) ListActivities(ctx context.Context, limit int) ([]ActivityRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT ae.id, repo.full_name, COALESCE(ae.actor_login_snapshot, member.login, ''),
		       ae.event_type, ae.title, ae.html_url, ae.occurred_at
		FROM activity_events ae
		JOIN repositories repo ON repo.id=ae.repository_id
		LEFT JOIN team_members member ON member.id=ae.actor_member_id
		WHERE ae.deleted_at IS NULL
		ORDER BY ae.occurred_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ActivityRecord{}
	for rows.Next() {
		var item ActivityRecord
		if err := rows.Scan(&item.ID, &item.Repository, &item.Actor, &item.Type, &item.Title, &item.URL, &item.OccurredAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) ListMembers(ctx context.Context) ([]MemberRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT member.id, member.login, COALESCE(member.avatar_url,''),
		       (SELECT COUNT(*) FROM commits c WHERE c.author_member_id=member.id AND c.deleted_at IS NULL),
		       (SELECT COUNT(*) FROM pull_requests pr WHERE pr.author_member_id=member.id AND pr.deleted_at IS NULL),
		       (SELECT COUNT(*) FROM pull_request_reviews review WHERE review.reviewer_member_id=member.id AND review.deleted_at IS NULL),
		       (SELECT MAX(occurred_at) FROM activity_events ae WHERE ae.actor_member_id=member.id AND ae.deleted_at IS NULL)
		FROM team_members member
		WHERE member.deleted_at IS NULL
		ORDER BY 7 DESC, member.login
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []MemberRecord{}
	for rows.Next() {
		var item MemberRecord
		if err := rows.Scan(&item.ID, &item.Login, &item.AvatarURL, &item.Commits, &item.PullRequests, &item.Reviews, &item.LastActiveAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) ListPullRequests(ctx context.Context, limit int) ([]PullRequestRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT pr.id, pr.github_id, repo.full_name, pr.number, pr.title,
		       pr.author_login_snapshot, pr.html_url, pr.state, pr.draft,
		       CASE
		         WHEN EXISTS (
		           SELECT 1 FROM pull_request_reviews review
		           WHERE review.pull_request_id=pr.id AND review.deleted_at IS NULL
		             AND review.state='CHANGES_REQUESTED'
		         ) THEN 'changes_requested'
		         WHEN EXISTS (
		           SELECT 1 FROM pull_request_reviews review
		           WHERE review.pull_request_id=pr.id AND review.deleted_at IS NULL
		             AND review.state='APPROVED' AND (review.commit_sha IS NULL OR review.commit_sha=pr.head_sha)
		         ) THEN 'approved'
		         ELSE 'waiting'
		       END AS review_state,
		       CASE
		         WHEN EXISTS (
		           SELECT 1 FROM workflow_runs run
		           WHERE run.repository_id=pr.repository_id AND run.head_sha=pr.head_sha
		             AND run.deleted_at IS NULL AND run.conclusion IN ('failure','timed_out')
		         ) THEN 'failed'
		         WHEN EXISTS (
		           SELECT 1 FROM workflow_runs run
		           WHERE run.repository_id=pr.repository_id AND run.head_sha=pr.head_sha
		             AND run.deleted_at IS NULL AND run.conclusion='success'
		         ) THEN 'passed'
		         ELSE 'unknown'
		       END AS ci_state,
		       pr.head_sha, pr.additions, pr.deletions, pr.changed_files, pr.files_complete,
		       pr.github_created_at, pr.github_updated_at, pr.merged_at, pr.last_activity_at
		FROM pull_requests pr
		JOIN repositories repo ON repo.id=pr.repository_id
		WHERE pr.deleted_at IS NULL
		ORDER BY pr.github_updated_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PullRequestRecord{}
	for rows.Next() {
		var item PullRequestRecord
		if err := rows.Scan(
			&item.ID, &item.GitHubID, &item.Repository, &item.Number, &item.Title,
			&item.Author, &item.URL, &item.State, &item.Draft, &item.ReviewState,
			&item.CIState, &item.HeadSHA, &item.Additions, &item.Deletions,
			&item.ChangedFiles, &item.FilesComplete, &item.CreatedAt, &item.UpdatedAt,
			&item.MergedAt, &item.LastActivityAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) GetPullRequest(ctx context.Context, id int64) (PullRequestDetail, error) {
	items, err := r.ListPullRequests(ctx, 200)
	if err != nil {
		return PullRequestDetail{}, err
	}
	var pullRequest PullRequestRecord
	found := false
	for _, item := range items {
		if item.ID == id {
			pullRequest = item
			found = true
			break
		}
	}
	if !found {
		return PullRequestDetail{}, ErrNotFound
	}
	detail := PullRequestDetail{
		PullRequest: pullRequest,
		Files:       []PullRequestFileRecord{}, Reviews: []PullRequestReviewRecord{},
		WorkflowRuns: []WorkflowRunRecord{},
	}
	fileRows, err := r.db.QueryContext(ctx, `
		SELECT filename, previous_filename, change_status, additions, deletions, changes,
		       language, module_name, is_test, is_documentation, is_configuration,
		       is_dependency, is_migration
		FROM pull_request_files WHERE pull_request_id=? AND deleted_at IS NULL ORDER BY filename
	`, id)
	if err != nil {
		return PullRequestDetail{}, err
	}
	for fileRows.Next() {
		var file PullRequestFileRecord
		if err := fileRows.Scan(&file.Filename, &file.PreviousFilename, &file.Status,
			&file.Additions, &file.Deletions, &file.Changes, &file.Language, &file.ModuleName,
			&file.IsTest, &file.IsDocumentation, &file.IsConfiguration,
			&file.IsDependency, &file.IsMigration); err != nil {
			fileRows.Close()
			return PullRequestDetail{}, err
		}
		detail.Files = append(detail.Files, file)
	}
	if err := fileRows.Close(); err != nil {
		return PullRequestDetail{}, err
	}
	reviewRows, err := r.db.QueryContext(ctx, `
		SELECT github_id, reviewer_login_snapshot, state, commit_sha, html_url, submitted_at
		FROM pull_request_reviews WHERE pull_request_id=? AND deleted_at IS NULL
		ORDER BY submitted_at DESC
	`, id)
	if err != nil {
		return PullRequestDetail{}, err
	}
	for reviewRows.Next() {
		var review PullRequestReviewRecord
		if err := reviewRows.Scan(&review.GitHubID, &review.Reviewer, &review.State,
			&review.CommitSHA, &review.URL, &review.SubmittedAt); err != nil {
			reviewRows.Close()
			return PullRequestDetail{}, err
		}
		detail.Reviews = append(detail.Reviews, review)
	}
	if err := reviewRows.Close(); err != nil {
		return PullRequestDetail{}, err
	}
	runRows, err := r.db.QueryContext(ctx, `
		SELECT github_id, workflow_id, workflow_name, run_number, run_attempt,
		       head_sha, status, conclusion, html_url, github_created_at
		FROM workflow_runs
		WHERE repository_id=(SELECT repository_id FROM pull_requests WHERE id=?)
		  AND head_sha=(SELECT head_sha FROM pull_requests WHERE id=?)
		  AND deleted_at IS NULL
		ORDER BY github_created_at DESC, run_attempt DESC
	`, id, id)
	if err != nil {
		return PullRequestDetail{}, err
	}
	for runRows.Next() {
		var run WorkflowRunRecord
		if err := runRows.Scan(&run.GitHubID, &run.WorkflowID, &run.Name,
			&run.RunNumber, &run.RunAttempt, &run.HeadSHA, &run.Status,
			&run.Conclusion, &run.URL, &run.CreatedAt); err != nil {
			runRows.Close()
			return PullRequestDetail{}, err
		}
		detail.WorkflowRuns = append(detail.WorkflowRuns, run)
	}
	if err := runRows.Close(); err != nil {
		return PullRequestDetail{}, err
	}
	return detail, nil
}

func (r *SQLiteRepository) ListRisks(ctx context.Context) ([]RiskRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT signal.id, signal.rule_type, signal.severity, repo.full_name,
		       pr.number, pr.author_login_snapshot, signal.reason, signal.suggested_action,
		       signal.status, signal.detected_at, signal.last_evaluated_at, signal.resolved_at
		FROM risk_signals signal
		JOIN repositories repo ON repo.id=signal.repository_id
		LEFT JOIN pull_requests pr ON pr.id=signal.pull_request_id
		WHERE signal.status='open'
		ORDER BY CASE signal.severity WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
		         signal.detected_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []RiskRecord{}
	for rows.Next() {
		var item RiskRecord
		if err := rows.Scan(&item.ID, &item.Type, &item.Severity, &item.Repository,
			&item.PRNumber, &item.Owner, &item.Reason, &item.SuggestedAction,
			&item.Status, &item.DetectedAt, &item.LastEvaluatedAt, &item.ResolvedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) SetRiskStatus(ctx context.Context, id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var occurrence int
	if err := tx.QueryRowContext(ctx, "SELECT occurrence_count FROM risk_signals WHERE id=?", id).Scan(&occurrence); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE risk_signals
		SET status=?, resolved_at=CASE WHEN ?='resolved' THEN ? ELSE NULL END
		WHERE id=?
	`, status, status, now, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	if status == "resolved" {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO risk_signal_events(
				risk_signal_id, occurrence_number, event_type, occurred_at
			) VALUES(?,?,'resolved',?)
		`, id, occurrence, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLiteRepository) ListJobs(ctx context.Context, limit int) ([]JobRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, kind, status, progress_percent, message, requested_at,
		       started_at, finished_at, error_summary, current_stage,
		       completed_items, total_items
		FROM sync_jobs ORDER BY requested_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []JobRecord{}
	for rows.Next() {
		var item JobRecord
		if err := rows.Scan(&item.ID, &item.Type, &item.Status, &item.Progress,
			&item.Message, &item.CreatedAt, &item.StartedAt, &item.EndedAt,
			&item.Error, &item.CurrentStage, &item.CompletedItems, &item.TotalItems); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) GetJob(ctx context.Context, id string) (JobRecord, error) {
	var item JobRecord
	err := r.db.QueryRowContext(ctx, `
		SELECT id, kind, status, progress_percent, message, requested_at,
		       started_at, finished_at, error_summary, current_stage,
		       completed_items, total_items
		FROM sync_jobs WHERE id=?
	`, id).Scan(&item.ID, &item.Type, &item.Status, &item.Progress,
		&item.Message, &item.CreatedAt, &item.StartedAt, &item.EndedAt,
		&item.Error, &item.CurrentStage, &item.CompletedItems, &item.TotalItems)
	if errors.Is(err, sql.ErrNoRows) {
		return JobRecord{}, ErrNotFound
	}
	return item, err
}

func (r *SQLiteRepository) ListReports(ctx context.Context) ([]ReportRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source_account_id, source_account_login_snapshot, kind, title,
		       period_start, period_end, timezone, facts_cutoff_at, repository_scope,
		       template_version, created_at
		FROM generated_reports ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ReportRecord{}
	for rows.Next() {
		var item ReportRecord
		if err := rows.Scan(&item.ID, &item.SourceAccountID, &item.SourceAccountLoginSnapshot,
			&item.Kind, &item.Title, &item.PeriodStart, &item.PeriodEnd, &item.Timezone,
			&item.FactsCutoffAt, &item.RepositoryScope, &item.TemplateVersion, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Repositories, err = r.reportRepositories(ctx, result[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *SQLiteRepository) GetReport(ctx context.Context, id string) (ReportRecord, error) {
	var item ReportRecord
	err := r.db.QueryRowContext(ctx, `
		SELECT id, source_account_id, source_account_login_snapshot, kind, title,
		       period_start, period_end, timezone, facts_cutoff_at, repository_scope,
		       template_version, markdown, created_at
		FROM generated_reports WHERE id=?
	`, id).Scan(&item.ID, &item.SourceAccountID, &item.SourceAccountLoginSnapshot,
		&item.Kind, &item.Title, &item.PeriodStart, &item.PeriodEnd, &item.Timezone,
		&item.FactsCutoffAt, &item.RepositoryScope, &item.TemplateVersion, &item.Markdown,
		&item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ReportRecord{}, ErrNotFound
	}
	if err != nil {
		return ReportRecord{}, err
	}
	item.Repositories, err = r.reportRepositories(ctx, id)
	return item, err
}

func (r *SQLiteRepository) reportRepositories(ctx context.Context, reportID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT full_name_snapshot FROM generated_report_repositories
		WHERE report_id=? ORDER BY full_name_snapshot
	`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

func WrapQueryError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

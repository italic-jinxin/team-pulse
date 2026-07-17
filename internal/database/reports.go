package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type ReportPullRequest struct {
	Repository string
	Number     int
	Title      string
	URL        string
	Author     string
	Additions  int
	Deletions  int
}

type ReportRisk struct {
	Severity   string
	Repository string
	Number     int
	Reason     string
	Action     string
	URL        string
}

type ReportRepositoryActivity struct {
	Repository string
	Commits    int
	Reviews    int
}

type ReportFacts struct {
	Completed          []ReportPullRequest
	InProgress         []ReportPullRequest
	ReviewCount        int
	CISuccessCount     int
	CIFailureCount     int
	Risks              []ReportRisk
	RepositoryActivity []ReportRepositoryActivity
}

type NewReport struct {
	ID                 string
	SourceAccountID    int64
	SourceAccountLogin string
	Kind               string
	Title              string
	PeriodStart        string
	PeriodEnd          string
	Timezone           string
	FactsCutoffAt      string
	RepositoryScope    string
	TemplateVersion    string
	Markdown           string
	ContentSHA256      string
	RepositoryIDs      []int64
	CreatedAt          string
}

func (r *SQLiteRepository) ReportFacts(ctx context.Context, repositoryIDs []int64, periodStart, periodEnd string) (ReportFacts, error) {
	if len(repositoryIDs) == 0 {
		return ReportFacts{}, fmt.Errorf("report requires at least one repository")
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(repositoryIDs)), ",")
	baseArgs := make([]any, 0, len(repositoryIDs)+2)
	for _, id := range repositoryIDs {
		baseArgs = append(baseArgs, id)
	}

	completedArgs := append(append([]any{}, baseArgs...), periodStart, periodEnd)
	completed, err := queryReportPullRequests(ctx, r.db, `
		SELECT repo.full_name, pr.number, pr.title, pr.html_url,
		       pr.author_login_snapshot, pr.additions, pr.deletions
		FROM pull_requests pr JOIN repositories repo ON repo.id=pr.repository_id
		WHERE pr.repository_id IN (`+placeholders+`)
		  AND pr.merged_at>=? AND pr.merged_at<? AND pr.deleted_at IS NULL
		ORDER BY pr.merged_at, repo.full_name, pr.number
	`, completedArgs...)
	if err != nil {
		return ReportFacts{}, err
	}

	inProgressArgs := append(append([]any{}, baseArgs...), periodEnd, periodStart)
	inProgress, err := queryReportPullRequests(ctx, r.db, `
		SELECT repo.full_name, pr.number, pr.title, pr.html_url,
		       pr.author_login_snapshot, pr.additions, pr.deletions
		FROM pull_requests pr JOIN repositories repo ON repo.id=pr.repository_id
		WHERE pr.repository_id IN (`+placeholders+`)
		  AND pr.state='open' AND pr.github_created_at<? AND pr.last_activity_at>=?
		  AND pr.deleted_at IS NULL
		ORDER BY repo.full_name, pr.number
	`, inProgressArgs...)
	if err != nil {
		return ReportFacts{}, err
	}

	facts := ReportFacts{Completed: completed, InProgress: inProgress}
	reviewArgs := append(append([]any{}, baseArgs...), periodStart, periodEnd)
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pull_request_reviews review
		JOIN pull_requests pr ON pr.id=review.pull_request_id
		WHERE pr.repository_id IN (`+placeholders+`)
		  AND review.state IN ('APPROVED','CHANGES_REQUESTED')
		  AND review.submitted_at>=? AND review.submitted_at<?
		  AND review.deleted_at IS NULL
	`, reviewArgs...).Scan(&facts.ReviewCount); err != nil {
		return ReportFacts{}, err
	}

	ciArgs := append(append([]any{}, baseArgs...), periodStart, periodEnd)
	if err := r.db.QueryRowContext(ctx, `
		SELECT
		  COALESCE(SUM(CASE WHEN conclusion='success' THEN 1 ELSE 0 END),0),
		  COALESCE(SUM(CASE WHEN conclusion IN ('failure','timed_out') THEN 1 ELSE 0 END),0)
		FROM workflow_runs
		WHERE repository_id IN (`+placeholders+`)
		  AND github_created_at>=? AND github_created_at<? AND deleted_at IS NULL
	`, ciArgs...).Scan(&facts.CISuccessCount, &facts.CIFailureCount); err != nil {
		return ReportFacts{}, err
	}

	riskArgs := append(append([]any{}, baseArgs...), periodEnd, periodEnd)
	riskRows, err := r.db.QueryContext(ctx, `
		SELECT signal.severity, repo.full_name, COALESCE(pr.number,0),
		       signal.reason, signal.suggested_action, COALESCE(pr.html_url,'')
		FROM risk_signals signal
		JOIN repositories repo ON repo.id=signal.repository_id
		LEFT JOIN pull_requests pr ON pr.id=signal.pull_request_id
		WHERE signal.repository_id IN (`+placeholders+`)
		  AND signal.detected_at<? AND (signal.resolved_at IS NULL OR signal.resolved_at>=?)
		ORDER BY CASE signal.severity WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END,
		         repo.full_name, pr.number
	`, riskArgs...)
	if err != nil {
		return ReportFacts{}, err
	}
	for riskRows.Next() {
		var risk ReportRisk
		if err := riskRows.Scan(&risk.Severity, &risk.Repository, &risk.Number, &risk.Reason, &risk.Action, &risk.URL); err != nil {
			riskRows.Close()
			return ReportFacts{}, err
		}
		facts.Risks = append(facts.Risks, risk)
	}
	if err := riskRows.Close(); err != nil {
		return ReportFacts{}, err
	}

	activityArgs := append(append([]any{}, baseArgs...), periodStart, periodEnd, periodStart, periodEnd)
	activityRows, err := r.db.QueryContext(ctx, `
		SELECT repo.full_name,
		       (SELECT COUNT(*) FROM commits c WHERE c.repository_id=repo.id AND c.authored_at>=? AND c.authored_at<? AND c.deleted_at IS NULL),
		       (SELECT COUNT(*) FROM pull_request_reviews review
		          JOIN pull_requests pr ON pr.id=review.pull_request_id
		          WHERE pr.repository_id=repo.id AND review.submitted_at>=? AND review.submitted_at<?
		            AND review.state IN ('APPROVED','CHANGES_REQUESTED') AND review.deleted_at IS NULL)
		FROM repositories repo
		WHERE repo.id IN (`+placeholders+`) AND repo.deleted_at IS NULL
		ORDER BY repo.full_name
	`, reorderActivityArgs(baseArgs, activityArgs[len(baseArgs):])...)
	if err != nil {
		return ReportFacts{}, err
	}
	for activityRows.Next() {
		var activity ReportRepositoryActivity
		if err := activityRows.Scan(&activity.Repository, &activity.Commits, &activity.Reviews); err != nil {
			activityRows.Close()
			return ReportFacts{}, err
		}
		facts.RepositoryActivity = append(facts.RepositoryActivity, activity)
	}
	if err := activityRows.Close(); err != nil {
		return ReportFacts{}, err
	}
	return facts, nil
}

func queryReportPullRequests(ctx context.Context, db *sql.DB, query string, args ...any) ([]ReportPullRequest, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ReportPullRequest{}
	for rows.Next() {
		var pullRequest ReportPullRequest
		if err := rows.Scan(&pullRequest.Repository, &pullRequest.Number, &pullRequest.Title,
			&pullRequest.URL, &pullRequest.Author, &pullRequest.Additions, &pullRequest.Deletions); err != nil {
			return nil, err
		}
		result = append(result, pullRequest)
	}
	return result, rows.Err()
}

func reorderActivityArgs(repositoryIDs, timeArgs []any) []any {
	result := append([]any{}, timeArgs...)
	return append(result, repositoryIDs...)
}

func (r *SQLiteRepository) SaveReport(ctx context.Context, report NewReport) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO generated_reports(
			id, source_account_id, source_account_login_snapshot, kind, title,
			period_start, period_end, timezone, facts_cutoff_at, repository_scope,
			template_version, markdown, content_sha256, created_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, report.ID, report.SourceAccountID, report.SourceAccountLogin, report.Kind, report.Title,
		report.PeriodStart, report.PeriodEnd, report.Timezone, report.FactsCutoffAt,
		report.RepositoryScope, report.TemplateVersion, report.Markdown,
		report.ContentSHA256, report.CreatedAt); err != nil {
		return err
	}
	for _, repositoryID := range report.RepositoryIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO generated_report_repositories(report_id, repository_id, full_name_snapshot)
			SELECT ?, id, full_name FROM repositories WHERE id=? AND deleted_at IS NULL
		`, report.ID, repositoryID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

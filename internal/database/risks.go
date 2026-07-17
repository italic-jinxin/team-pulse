package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type RiskSettings struct {
	WaitingReviewHours int `json:"waiting_review_hours"`
	StalePRDays        int `json:"stale_pr_days"`
	CIFailureThreshold int `json:"ci_failure_threshold"`
}

type SettingsRecord struct {
	Version   int          `json:"version"`
	RiskRules RiskSettings `json:"risk_rules"`
}

type RiskCandidate struct {
	ID                   int64
	GitHubID             int64
	RepositoryID         int64
	Repository           string
	Number               int
	Owner                string
	State                string
	Draft                bool
	ReviewState          string
	CIState              string
	ReviewRequestedAt    string
	LastAuthorActivityAt string
	LastActivityAt       string
}

type RiskDecision struct {
	RuleType        string
	RepositoryID    int64
	PullRequestID   int64
	SubjectID       string
	Severity        string
	ReasonCode      string
	Reason          string
	SuggestedAction string
	EvidenceJSON    string
}

func DefaultRiskSettings() RiskSettings {
	return RiskSettings{WaitingReviewHours: 48, StalePRDays: 5, CIFailureThreshold: 1}
}

func (r *SQLiteRepository) GetSettings(ctx context.Context) (SettingsRecord, error) {
	settings := SettingsRecord{Version: 1, RiskRules: DefaultRiskSettings()}
	rows, err := r.db.QueryContext(ctx, "SELECT rule_type, config_json FROM risk_rules")
	if err != nil {
		return SettingsRecord{}, err
	}
	for rows.Next() {
		var ruleType, raw string
		if err := rows.Scan(&ruleType, &raw); err != nil {
			rows.Close()
			return SettingsRecord{}, err
		}
		var config map[string]int
		if err := json.Unmarshal([]byte(raw), &config); err != nil {
			rows.Close()
			return SettingsRecord{}, fmt.Errorf("decode %s config: %w", ruleType, err)
		}
		switch ruleType {
		case "waiting_for_review":
			settings.RiskRules.WaitingReviewHours = config["hours"]
		case "stale_pull_request":
			settings.RiskRules.StalePRDays = config["days"]
		case "ci_failure":
			settings.RiskRules.CIFailureThreshold = config["failure_threshold"]
		}
	}
	if err := rows.Close(); err != nil {
		return SettingsRecord{}, err
	}
	var versionJSON string
	err = r.db.QueryRowContext(ctx, "SELECT value_json FROM app_settings WHERE key='settings_version'").Scan(&versionJSON)
	if err == nil {
		if version, parseErr := strconv.Atoi(versionJSON); parseErr == nil && version > 0 {
			settings.Version = version
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SettingsRecord{}, err
	}
	return settings, nil
}

func (r *SQLiteRepository) UpdateSettings(ctx context.Context, expectedVersion int, settings RiskSettings) (SettingsRecord, error) {
	if settings.WaitingReviewHours < 1 || settings.WaitingReviewHours > 720 ||
		settings.StalePRDays < 1 || settings.StalePRDays > 365 ||
		settings.CIFailureThreshold < 1 || settings.CIFailureThreshold > 20 {
		return SettingsRecord{}, fmt.Errorf("risk rule values are outside the allowed range")
	}
	current, err := r.GetSettings(ctx)
	if err != nil {
		return SettingsRecord{}, err
	}
	if expectedVersion != current.Version {
		return SettingsRecord{}, fmt.Errorf("settings version conflict")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SettingsRecord{}, err
	}
	defer tx.Rollback()
	now := utcNow()
	configs := map[string]map[string]int{
		"waiting_for_review": {"hours": settings.WaitingReviewHours},
		"stale_pull_request": {"days": settings.StalePRDays},
		"ci_failure":         {"failure_threshold": settings.CIFailureThreshold},
	}
	for ruleType, config := range configs {
		raw, _ := json.Marshal(config)
		if _, err := tx.ExecContext(ctx, "UPDATE risk_rules SET config_json=?, updated_at=? WHERE rule_type=?", string(raw), now, ruleType); err != nil {
			return SettingsRecord{}, err
		}
	}
	nextVersion := current.Version + 1
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO app_settings(key, value_json, version, updated_at)
		VALUES('settings_version', ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,
			version=excluded.version, updated_at=excluded.updated_at
	`, strconv.Itoa(nextVersion), nextVersion, now); err != nil {
		return SettingsRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return SettingsRecord{}, err
	}
	return SettingsRecord{Version: nextVersion, RiskRules: settings}, nil
}

func (r *SQLiteRepository) RiskCandidates(ctx context.Context) ([]RiskCandidate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pr.id, pr.github_id, pr.repository_id, repo.full_name, pr.number,
		       pr.author_login_snapshot, pr.state, pr.draft,
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
		       END,
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
		       END,
		       COALESCE(pr.review_requested_at,''), COALESCE(pr.last_author_activity_at,''),
		       pr.last_activity_at
		FROM pull_requests pr
		JOIN repositories repo ON repo.id=pr.repository_id
		WHERE pr.deleted_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []RiskCandidate{}
	for rows.Next() {
		var candidate RiskCandidate
		if err := rows.Scan(&candidate.ID, &candidate.GitHubID, &candidate.RepositoryID,
			&candidate.Repository, &candidate.Number, &candidate.Owner, &candidate.State,
			&candidate.Draft, &candidate.ReviewState, &candidate.CIState,
			&candidate.ReviewRequestedAt, &candidate.LastAuthorActivityAt,
			&candidate.LastActivityAt); err != nil {
			return nil, err
		}
		result = append(result, candidate)
	}
	return result, rows.Err()
}

func (r *SQLiteRepository) ReconcileRisks(ctx context.Context, decisions []RiskDecision) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	seen := make(map[string]struct{}, len(decisions))
	for _, decision := range decisions {
		signalKey := fmt.Sprintf("%s:%d:pull_request:%s", decision.RuleType, decision.RepositoryID, decision.SubjectID)
		seen[signalKey] = struct{}{}
		var id, status string
		var occurrence int
		err := tx.QueryRowContext(ctx, `
			SELECT id, status, occurrence_count FROM risk_signals WHERE signal_key=?
		`, signalKey).Scan(&id, &status, &occurrence)
		if errors.Is(err, sql.ErrNoRows) {
			id = riskID(signalKey)
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO risk_signals(
					id, signal_key, rule_type, repository_id, pull_request_id,
					subject_type, subject_id, severity, status, reason_code,
					reason, suggested_action, evidence_json, detected_at, last_evaluated_at
				) VALUES(?,?,?,?,?,'pull_request',?,?, 'open', ?,?,?,?,?,?)
			`, id, signalKey, decision.RuleType, decision.RepositoryID, decision.PullRequestID,
				decision.SubjectID, decision.Severity, decision.ReasonCode, decision.Reason,
				decision.SuggestedAction, decision.EvidenceJSON, now, now); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO risk_signal_events(risk_signal_id, occurrence_number, event_type, reason_code, evidence_json, occurred_at)
				VALUES(?,1,'opened',?,?,?)
			`, id, decision.ReasonCode, decision.EvidenceJSON, now); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if status == "resolved" || status == "dismissed" {
			occurrence++
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO risk_signal_events(risk_signal_id, occurrence_number, event_type, reason_code, evidence_json, occurred_at)
				VALUES(?,?,'opened',?,?,?)
			`, id, occurrence, decision.ReasonCode, decision.EvidenceJSON, now); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE risk_signals SET severity=?, status='open', reason_code=?, reason=?,
				suggested_action=?, evidence_json=?, last_evaluated_at=?, resolved_at=NULL,
				dismissed_at=NULL, dismiss_reason=NULL, occurrence_count=?
			WHERE id=?
		`, decision.Severity, decision.ReasonCode, decision.Reason, decision.SuggestedAction,
			decision.EvidenceJSON, now, occurrence, id); err != nil {
			return err
		}
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, signal_key, occurrence_count FROM risk_signals
		WHERE status IN ('open','acknowledged')
	`)
	if err != nil {
		return err
	}
	type openSignal struct {
		id, key    string
		occurrence int
	}
	openSignals := []openSignal{}
	for rows.Next() {
		var signal openSignal
		if err := rows.Scan(&signal.id, &signal.key, &signal.occurrence); err != nil {
			rows.Close()
			return err
		}
		openSignals = append(openSignals, signal)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, signal := range openSignals {
		if _, ok := seen[signal.key]; ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, "UPDATE risk_signals SET status='resolved', resolved_at=?, last_evaluated_at=? WHERE id=?", now, now, signal.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO risk_signal_events(risk_signal_id, occurrence_number, event_type, occurred_at)
			VALUES(?,?,'resolved',?)
		`, signal.id, signal.occurrence, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func riskID(signalKey string) string {
	sum := sha256.Sum256([]byte(signalKey))
	return "risk_" + hex.EncodeToString(sum[:12])
}

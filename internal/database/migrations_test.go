package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	rootmigrations "github.com/italic-jinxin/team-pulse/migrations"
	_ "modernc.org/sqlite"
)

func TestMigrationRunnerAppliesInitialSchemaAndIsIdempotent(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "teampulse.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	runner := NewMigrationRunner(rootmigrations.Files)
	ctx := context.Background()
	if err := runner.Apply(ctx, db); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := runner.Apply(ctx, db); err != nil {
		t.Fatalf("second apply: %v", err)
	}

	var versionCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if versionCount != 1 {
		t.Fatalf("migration count = %d, want 1", versionCount)
	}

	for _, table := range []string{
		"github_accounts", "repositories", "team_members", "commits",
		"pull_requests", "pull_request_files", "pull_request_reviews", "workflow_runs",
		"activity_events", "sync_jobs", "risk_signals", "generated_reports",
	} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %s count = %d, want 1", table, count)
		}
	}

	var foreignKeyErrors int
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		foreignKeyErrors++
	}
	if foreignKeyErrors != 0 {
		t.Fatalf("foreign key errors = %d", foreignKeyErrors)
	}
}

package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenBacksUpAndRebuildsLegacySchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "teampulse.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`
		CREATE TABLE repositories (
			id INTEGER PRIMARY KEY,
			full_name TEXT UNIQUE NOT NULL
		);
		INSERT INTO repositories(id, full_name) VALUES(42, 'owner/legacy');
	`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, backupPath, err := Open(context.Background(), path, filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if backupPath == "" {
		t.Fatal("expected legacy backup path")
	}
	if info, err := os.Stat(backupPath); err != nil || info.Size() == 0 {
		t.Fatalf("legacy backup invalid: info=%v err=%v", info, err)
	}

	var githubIDColumn int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('repositories') WHERE name='github_id'
	`).Scan(&githubIDColumn); err != nil {
		t.Fatal(err)
	}
	if githubIDColumn != 1 {
		t.Fatalf("new repository github_id columns = %d, want 1", githubIDColumn)
	}

	backup, err := sql.Open("sqlite", backupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var fullName string
	if err := backup.QueryRow("SELECT full_name FROM repositories WHERE id=42").Scan(&fullName); err != nil {
		t.Fatal(err)
	}
	if fullName != "owner/legacy" {
		t.Fatalf("backup repository = %q", fullName)
	}
}

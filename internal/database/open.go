package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	rootmigrations "github.com/italic-jinxin/team-pulse/migrations"
	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, path, backupDir string) (*sql.DB, string, error) {
	db, err := openSQLite(path)
	if err != nil {
		return nil, "", err
	}

	legacy, err := isLegacySchema(ctx, db)
	if err != nil {
		db.Close()
		return nil, "", err
	}

	var backupPath string
	if legacy {
		if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			db.Close()
			return nil, "", fmt.Errorf("checkpoint legacy database: %w", err)
		}
		if err := db.Close(); err != nil {
			return nil, "", fmt.Errorf("close legacy database: %w", err)
		}
		backupPath, err = backupLegacyDatabase(path, backupDir)
		if err != nil {
			return nil, "", err
		}
		for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
			if err := os.Remove(candidate); err != nil && !os.IsNotExist(err) {
				return nil, backupPath, fmt.Errorf("remove legacy database file %q: %w", candidate, err)
			}
		}
		db, err = openSQLite(path)
		if err != nil {
			return nil, backupPath, err
		}
	}

	if err := NewMigrationRunner(rootmigrations.Files).Apply(ctx, db); err != nil {
		db.Close()
		return nil, backupPath, err
	}
	return db, backupPath, nil
}

func openSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=10000;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure database: %w", err)
	}
	return db, nil
}

func isLegacySchema(ctx context.Context, db *sql.DB) (bool, error) {
	var migrationsTable int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'schema_migrations'
	`).Scan(&migrationsTable); err != nil {
		return false, fmt.Errorf("inspect schema migrations: %w", err)
	}
	if migrationsTable != 0 {
		return false, nil
	}

	rows, err := db.QueryContext(ctx, "PRAGMA table_info(repositories)")
	if err != nil {
		return false, fmt.Errorf("inspect repository schema: %w", err)
	}
	defer rows.Close()

	var hasFullName, hasGitHubID bool
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scan repository schema: %w", err)
		}
		switch name {
		case "full_name":
			hasFullName = true
		case "github_id":
			hasGitHubID = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate repository schema: %w", err)
	}
	return hasFullName && !hasGitHubID, nil
}

func backupLegacyDatabase(path, backupDir string) (string, error) {
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return "", fmt.Errorf("create backup directory: %w", err)
	}
	name := "teampulse-legacy-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".db"
	destination := filepath.Join(backupDir, name)

	sourceFile, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open legacy database for backup: %w", err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("create legacy database backup: %w", err)
	}
	complete := false
	defer func() {
		destinationFile.Close()
		if !complete {
			_ = os.Remove(destination)
		}
	}()

	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return "", fmt.Errorf("copy legacy database backup: %w", err)
	}
	if err := destinationFile.Sync(); err != nil {
		return "", fmt.Errorf("sync legacy database backup: %w", err)
	}
	if err := destinationFile.Close(); err != nil {
		return "", fmt.Errorf("close legacy database backup: %w", err)
	}
	complete = true
	return destination, nil
}

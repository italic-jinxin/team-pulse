package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MigrationRunner struct {
	files fs.FS
}

type migration struct {
	version  int64
	name     string
	checksum string
	sql      string
}

func NewMigrationRunner(files fs.FS) *MigrationRunner {
	return &MigrationRunner{files: files}
}

func (r *MigrationRunner) Apply(ctx context.Context, db *sql.DB) error {
	migrations, err := r.load()
	if err != nil {
		return err
	}

	applied, err := appliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if checksum, ok := applied[m.version]; ok {
			if checksum != m.checksum {
				return fmt.Errorf("migration %06d checksum mismatch", m.version)
			}
			continue
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return err
		}
	}
	return nil
}

func (r *MigrationRunner) load() ([]migration, error) {
	names, err := fs.Glob(r.files, "*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(names)

	result := make([]migration, 0, len(names))
	var previous int64
	for _, path := range names {
		base := filepath.Base(path)
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid migration filename %q", base)
		}
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || version <= previous {
			return nil, fmt.Errorf("invalid migration version in %q", base)
		}
		body, err := fs.ReadFile(r.files, path)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", base, err)
		}
		sum := sha256.Sum256(body)
		result = append(result, migration{
			version:  version,
			name:     strings.TrimSuffix(parts[1], ".up.sql"),
			checksum: hex.EncodeToString(sum[:]),
			sql:      string(body),
		})
		previous = version
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no database migrations found")
	}
	return result, nil
}

func appliedMigrations(ctx context.Context, db *sql.DB) (map[int64]string, error) {
	var exists int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master
		WHERE type = 'table' AND name = 'schema_migrations'
	`).Scan(&exists); err != nil {
		return nil, fmt.Errorf("inspect migration table: %w", err)
	}
	if exists == 0 {
		return map[int64]string{}, nil
	}

	rows, err := db.QueryContext(ctx, "SELECT version, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read applied migrations: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]string)
	for rows.Next() {
		var version int64
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		result[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return result, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %06d: %w", m.version, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("apply migration %06d: %w", m.version, err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations(version, name, checksum, applied_at) VALUES(?, ?, ?, ?)",
		m.version, m.name, m.checksum, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("record migration %06d: %w", m.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %06d: %w", m.version, err)
	}
	return nil
}

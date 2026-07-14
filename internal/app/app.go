package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

type App struct {
	DB       *sql.DB
	DataDir  string
	shutdown chan struct{}
}

func New(dataDir string) (*App, error) {
	if dataDir == "" {
		dataDir = defaultDataDir()
	}
	for _, d := range []string{"", "logs", "reports", "run", "cache", "backups"} {
		if err := os.MkdirAll(filepath.Join(dataDir, d), 0700); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "teampulse.db")+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// TeamPulse is a single-process local app. A single connection prevents
	// competing goroutines from surfacing SQLITE_BUSY while WAL keeps reads fast.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=10000;"); err != nil {
		db.Close()
		return nil, err
	}
	a := &App{DB: db, DataDir: dataDir, shutdown: make(chan struct{})}
	if err := a.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return a, nil
}

func defaultDataDir() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "TeamPulse")
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "TeamPulse")
	}
	return filepath.Join(home, ".teampulse")
}

func (a *App) migrate() error                     { _, err := a.DB.Exec(schema); return err }
func (a *App) Close() error                       { return a.DB.Close() }
func (a *App) ShutdownRequested() <-chan struct{} { return a.shutdown }
func (a *App) RemoveRuntimeState() {
	_ = os.Remove(filepath.Join(a.DataDir, "run", "server.pid"))
	_ = os.Remove(filepath.Join(a.DataDir, "run", "server.json"))
}
func (a *App) WriteRuntimeState(pid int, host string, port int, url string) error {
	run := filepath.Join(a.DataDir, "run")
	if err := os.WriteFile(filepath.Join(run, "server.pid"), []byte(fmt.Sprint(pid)), 0600); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(map[string]any{"pid": pid, "host": host, "port": port, "url": url, "started_at": time.Now(), "version": "0.1.0"}, "", "  ")
	return os.WriteFile(filepath.Join(run, "server.json"), b, 0600)
}

const schema = `
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS repositories (id INTEGER PRIMARY KEY, full_name TEXT UNIQUE NOT NULL, description TEXT, private INTEGER NOT NULL DEFAULT 0, default_branch TEXT, selected INTEGER NOT NULL DEFAULT 1, updated_at TEXT);
CREATE TABLE IF NOT EXISTS members (login TEXT PRIMARY KEY, avatar_url TEXT, commits INTEGER DEFAULT 0, pull_requests INTEGER DEFAULT 0, reviews INTEGER DEFAULT 0, last_active_at TEXT);
CREATE TABLE IF NOT EXISTS activities (id TEXT PRIMARY KEY, repository TEXT NOT NULL, actor TEXT, type TEXT NOT NULL, title TEXT, url TEXT, occurred_at TEXT NOT NULL, metadata_json TEXT);
CREATE INDEX IF NOT EXISTS idx_activities_date ON activities(occurred_at DESC);
CREATE TABLE IF NOT EXISTS pull_requests (id TEXT PRIMARY KEY, repository TEXT NOT NULL, number INTEGER NOT NULL, title TEXT NOT NULL, author TEXT, url TEXT, state TEXT, draft INTEGER DEFAULT 0, review_state TEXT, ci_state TEXT, additions INTEGER DEFAULT 0, deletions INTEGER DEFAULT 0, created_at TEXT, updated_at TEXT, merged_at TEXT, UNIQUE(repository,number));
CREATE TABLE IF NOT EXISTS risks (id TEXT PRIMARY KEY, type TEXT NOT NULL, severity TEXT NOT NULL, repository TEXT, pr_number INTEGER, owner TEXT, reason TEXT NOT NULL, suggested_action TEXT, status TEXT NOT NULL DEFAULT 'open', detected_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS reports (id TEXT PRIMARY KEY, type TEXT NOT NULL, title TEXT NOT NULL, markdown TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS jobs (id TEXT PRIMARY KEY, type TEXT NOT NULL, status TEXT NOT NULL, progress INTEGER DEFAULT 0, message TEXT, created_at TEXT NOT NULL, started_at TEXT, ended_at TEXT, error TEXT);
`

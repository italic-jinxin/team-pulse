package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/italic-jinxin/team-pulse/internal/credentials"
	appdb "github.com/italic-jinxin/team-pulse/internal/database"
	githubclient "github.com/italic-jinxin/team-pulse/internal/github"
)

type App struct {
	DB            *sql.DB
	DataDir       string
	LegacyBackup  string
	Credentials   credentials.Store
	Repository    *appdb.SQLiteRepository
	GitHubBaseURL string
	shutdown      chan struct{}
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
	db, legacyBackup, err := appdb.Open(
		context.Background(),
		filepath.Join(dataDir, "teampulse.db"),
		filepath.Join(dataDir, "backups"),
	)
	if err != nil {
		return nil, err
	}
	return &App{
		DB:            db,
		DataDir:       dataDir,
		LegacyBackup:  legacyBackup,
		Credentials:   credentials.NewMemoryStore(),
		Repository:    appdb.NewSQLiteRepository(db),
		GitHubBaseURL: githubclient.DefaultAPIBaseURL,
		shutdown:      make(chan struct{}),
	}, nil
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

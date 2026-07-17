package app

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	apihttp "github.com/italic-jinxin/team-pulse/internal/api"
)

//go:embed webdist/*
var webAssets embed.FS

func (a *App) Router(origin string) http.Handler {
	return apihttp.Router(origin, apihttp.Handlers{
		Health: a.health, AppStatus: a.status, Shutdown: a.requestShutdown,
		AuthStatus: a.authStatus, SetToken: a.setToken, ClearToken: a.clearToken,
		ListRepositories: a.listRepositories, UpdateRepositorySelection: a.updateRepositorySelection,
		StartSync: a.startSync, ListSyncJobs: a.listJobs, GetSyncJob: a.getJob,
		ListActivities: a.listActivity, ListMembers: a.listMembers,
		ListPullRequests: a.listPullRequests, GetPullRequest: a.getPullRequest,
		ListRisks: a.listRisks, UpdateRisk: a.updateRisk,
		GenerateReport: a.generateReport, ListReports: a.listReports,
		GetReport: a.getReport, DownloadReport: a.downloadReport,
		GetSettings: a.getSettings, UpdateSettings: a.updateSettings,
		SPA: a.serveSPA,
	})
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	var schemaVersion int
	_ = a.DB.QueryRowContext(r.Context(), "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&schemaVersion)
	respond(w, http.StatusOK, map[string]any{"status": "ok", "version": "0.1.0", "schema_version": schemaVersion})
}

func (a *App) requestShutdown(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusAccepted, map[string]string{"status": "shutting_down"})
	select {
	case <-a.shutdown:
	default:
		close(a.shutdown)
	}
}

func (a *App) serveSPA(w http.ResponseWriter, r *http.Request) {
	dist, _ := fs.Sub(webAssets, "webdist")
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path != "" {
		if _, err := fs.Stat(dist, path); err == nil {
			http.FileServer(http.FS(dist)).ServeHTTP(w, r)
			return
		}
	}
	data, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		http.Error(w, "web assets not built", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func respond(w http.ResponseWriter, status int, value any) {
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func respondList[T any](w http.ResponseWriter, items []T) {
	respond(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nil})
}

func respondAPIError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	respond(w, status, map[string]any{"error": map[string]any{
		"code": code, "message": message, "details": details,
		"request_id": middleware.GetReqID(r.Context()),
	}})
}

func decode(r *http.Request, value any) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(value)
}

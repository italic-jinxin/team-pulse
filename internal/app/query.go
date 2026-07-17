package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/italic-jinxin/team-pulse/internal/credentials"
	appdb "github.com/italic-jinxin/team-pulse/internal/database"
	githubclient "github.com/italic-jinxin/team-pulse/internal/github"
)

func (a *App) status(w http.ResponseWriter, r *http.Request) {
	status, err := a.Repository.Status(r.Context())
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to read application status", nil)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"repositories":       status.Repositories,
		"open_pull_requests": status.OpenPullRequests,
		"open_risks":         status.OpenRisks,
		"members":            status.Members,
		"last_sync_at":       status.LastSyncAt,
		"data_dir":           a.DataDir,
		"legacy_backup":      nullable(a.LegacyBackup),
	})
}

func (a *App) authStatus(w http.ResponseWriter, r *http.Request) {
	credential, err := a.Credentials.Get(r.Context())
	if errors.Is(err, credentials.ErrNotFound) {
		respond(w, http.StatusOK, map[string]any{"authenticated": false, "source": "", "login": ""})
		return
	}
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to read credential state", nil)
		return
	}
	respond(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"source":        credential.Source,
		"login":         credential.Login,
	})
}

func (a *App) setToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Token string `json:"token"`
	}
	if decode(r, &input) != nil || (!strings.HasPrefix(input.Token, "github_pat_") && !strings.HasPrefix(input.Token, "ghp_")) {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid GitHub token", nil)
		return
	}
	client, err := githubclient.NewClientWithBaseURL(input.Token, 15*time.Second, a.GitHubBaseURL)
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to configure GitHub client", nil)
		return
	}
	user, err := client.CurrentUser(r.Context())
	if err != nil {
		respondAPIError(w, r, http.StatusUnauthorized, "GITHUB_UNAUTHORIZED", "GitHub rejected this token", nil)
		return
	}
	syncedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := a.Repository.UpsertAccount(r.Context(), appdb.AccountFact{
		GitHubID:        user.ID,
		Login:           user.Login,
		AvatarURL:       user.AvatarURL,
		ProfileURL:      user.HTMLURL,
		GitHubCreatedAt: user.CreatedAt,
		GitHubUpdatedAt: user.UpdatedAt,
		SyncedAt:        syncedAt,
	}); err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to save GitHub account", nil)
		return
	}
	if err := a.Credentials.Set(r.Context(), credentials.Credential{
		Token: input.Token, Source: "memory", Login: user.Login,
	}); err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to store GitHub credential", nil)
		return
	}
	respond(w, http.StatusOK, map[string]any{"authenticated": true, "login": user.Login})
}

func (a *App) clearToken(w http.ResponseWriter, r *http.Request) {
	if err := a.Credentials.Delete(r.Context()); err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to clear GitHub credential", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) listRepositories(w http.ResponseWriter, r *http.Request) {
	if _, err := a.Credentials.Get(r.Context()); err == nil {
		if err := a.refreshRepositoryCatalog(r.Context()); err != nil {
			respondAPIError(w, r, http.StatusBadGateway, "GITHUB_UNAVAILABLE", "Unable to refresh GitHub repositories", nil)
			return
		}
	}
	items, err := a.Repository.ListRepositories(r.Context())
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to list repositories", nil)
		return
	}
	respondList(w, items)
}

func (a *App) updateRepositorySelection(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RepositoryIDs []int64 `json:"repository_ids"`
	}
	if decode(r, &input) != nil {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid repository selection", nil)
		return
	}
	if err := a.Repository.SetRepositorySelection(r.Context(), input.RepositoryIDs); err != nil {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), nil)
		return
	}
	items, err := a.Repository.ListRepositories(r.Context())
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to read repository selection", nil)
		return
	}
	respondList(w, items)
}

func (a *App) listActivity(w http.ResponseWriter, r *http.Request) {
	items, err := a.Repository.ListActivities(r.Context(), 200)
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to list activities", nil)
		return
	}
	respondList(w, items)
}

func (a *App) listMembers(w http.ResponseWriter, r *http.Request) {
	items, err := a.Repository.ListMembers(r.Context())
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to list members", nil)
		return
	}
	respondList(w, items)
}

func (a *App) listPullRequests(w http.ResponseWriter, r *http.Request) {
	items, err := a.Repository.ListPullRequests(r.Context(), 200)
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to list pull requests", nil)
		return
	}
	respondList(w, items)
}

func (a *App) getPullRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid pull request ID", nil)
		return
	}
	detail, err := a.Repository.GetPullRequest(r.Context(), id)
	if errors.Is(err, appdb.ErrNotFound) {
		respondAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Pull request not found", nil)
		return
	}
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to read pull request", nil)
		return
	}
	respond(w, http.StatusOK, detail)
}

func (a *App) listRisks(w http.ResponseWriter, r *http.Request) {
	items, err := a.Repository.ListRisks(r.Context())
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to list risks", nil)
		return
	}
	respondList(w, items)
}

func (a *App) updateRisk(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
	}
	if decode(r, &input) != nil || (input.Status != "open" && input.Status != "resolved") {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid risk status", nil)
		return
	}
	err := a.Repository.SetRiskStatus(r.Context(), chi.URLParam(r, "id"), input.Status)
	if errors.Is(err, appdb.ErrNotFound) {
		respondAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Risk not found", nil)
		return
	}
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to update risk", nil)
		return
	}
	respond(w, http.StatusOK, map[string]string{"status": input.Status})
}

func (a *App) listJobs(w http.ResponseWriter, r *http.Request) {
	items, err := a.Repository.ListJobs(r.Context(), 20)
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to list sync jobs", nil)
		return
	}
	respondList(w, items)
}

func (a *App) getJob(w http.ResponseWriter, r *http.Request) {
	item, err := a.Repository.GetJob(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, appdb.ErrNotFound) {
		respondAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Sync job not found", nil)
		return
	}
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to read sync job", nil)
		return
	}
	respond(w, http.StatusOK, item)
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nowID(prefix string) string {
	return prefix + "_" + time.Now().UTC().Format("20060102T150405.000000000")
}

func marshal(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

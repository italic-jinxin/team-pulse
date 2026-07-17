package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	appdb "github.com/italic-jinxin/team-pulse/internal/database"
	reportdomain "github.com/italic-jinxin/team-pulse/internal/reports"
)

func (a *App) generateReport(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Kind          string  `json:"kind"`
		PeriodStart   string  `json:"period_start"`
		PeriodEnd     string  `json:"period_end"`
		Timezone      string  `json:"timezone"`
		RepositoryIDs []int64 `json:"repository_ids"`
	}
	if err := decode(r, &input); err != nil {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid report request", nil)
		return
	}
	if input.Kind == "" {
		input.Kind = "weekly"
	}
	if input.Kind != "weekly" {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Alpha reports only support kind=weekly", nil)
		return
	}
	if input.Timezone == "" {
		input.Timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(input.Timezone)
	if err != nil {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid IANA timezone", nil)
		return
	}
	periodStart, periodEnd, err := reportdomain.Period(input.PeriodStart, input.PeriodEnd, location, time.Now())
	if err != nil {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), nil)
		return
	}
	targets, err := a.Repository.SyncTargets(r.Context(), input.RepositoryIDs)
	if err != nil {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), nil)
		return
	}
	repositoryIDs := make([]int64, 0, len(targets))
	for _, target := range targets {
		repositoryIDs = append(repositoryIDs, target.ID)
	}
	facts, err := a.Repository.ReportFacts(
		r.Context(), repositoryIDs,
		periodStart.UTC().Format(time.RFC3339Nano),
		periodEnd.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to query report facts", nil)
		return
	}
	credential, err := a.Credentials.Get(r.Context())
	if err != nil {
		respondAPIError(w, r, http.StatusUnauthorized, "CREDENTIAL_MISSING", "Connect GitHub before generating a report", nil)
		return
	}
	account, err := a.Repository.AccountByLogin(r.Context(), credential.Login)
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to identify report data source", nil)
		return
	}
	markdown := reportdomain.RenderWeekly(periodStart, periodEnd, input.Timezone, facts)
	sum := sha256.Sum256([]byte(markdown))
	now := time.Now().UTC()
	id := nowID("report")
	scope := "selected"
	if len(input.RepositoryIDs) > 0 {
		scope = "explicit"
	}
	report := appdb.NewReport{
		ID: id, SourceAccountID: account.ID, SourceAccountLogin: account.Login,
		Kind: "weekly", Title: "Engineering Weekly Summary",
		PeriodStart: periodStart.UTC().Format(time.RFC3339Nano),
		PeriodEnd:   periodEnd.UTC().Format(time.RFC3339Nano),
		Timezone:    input.Timezone, FactsCutoffAt: now.Format(time.RFC3339Nano),
		RepositoryScope: scope, TemplateVersion: reportdomain.TemplateVersion,
		Markdown: markdown, ContentSHA256: hex.EncodeToString(sum[:]),
		RepositoryIDs: repositoryIDs, CreatedAt: now.Format(time.RFC3339Nano),
	}
	if err := a.Repository.SaveReport(r.Context(), report); err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to save report", nil)
		return
	}
	saved, err := a.Repository.GetReport(r.Context(), id)
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to read generated report", nil)
		return
	}
	respond(w, http.StatusCreated, saved)
}

func (a *App) listReports(w http.ResponseWriter, r *http.Request) {
	items, err := a.Repository.ListReports(r.Context())
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to list reports", nil)
		return
	}
	respondList(w, items)
}

func (a *App) getReport(w http.ResponseWriter, r *http.Request) {
	report, err := a.Repository.GetReport(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, appdb.ErrNotFound) {
		respondAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Report not found", nil)
		return
	}
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to read report", nil)
		return
	}
	respond(w, http.StatusOK, report)
}

func (a *App) downloadReport(w http.ResponseWriter, r *http.Request) {
	report, err := a.Repository.GetReport(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, appdb.ErrNotFound) {
		respondAPIError(w, r, http.StatusNotFound, "NOT_FOUND", "Report not found", nil)
		return
	}
	if err != nil || report.Markdown == nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to download report", nil)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="teampulse-weekly-report.md"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(*report.Markdown))
}

package app

import (
	"context"
	"net/http"
	"time"

	appdb "github.com/italic-jinxin/team-pulse/internal/database"
	riskengine "github.com/italic-jinxin/team-pulse/internal/risks"
)

func (a *App) scanRisks(ctx context.Context) error {
	settings, err := a.Repository.GetSettings(ctx)
	if err != nil {
		return err
	}
	candidates, err := a.Repository.RiskCandidates(ctx)
	if err != nil {
		return err
	}
	return a.Repository.ReconcileRisks(
		ctx,
		riskengine.Evaluate(settings.RiskRules, candidates, time.Now().UTC()),
	)
}

func (a *App) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := a.Repository.GetSettings(r.Context())
	if err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to read settings", nil)
		return
	}
	respond(w, http.StatusOK, settings)
}

func (a *App) updateSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Version int `json:"version"`
		Changes struct {
			RiskRules appdb.RiskSettings `json:"risk_rules"`
		} `json:"changes"`
	}
	if decode(r, &input) != nil || input.Version < 1 {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid settings update", nil)
		return
	}
	settings, err := a.Repository.UpdateSettings(r.Context(), input.Version, input.Changes.RiskRules)
	if err != nil {
		status := http.StatusBadRequest
		code := "INVALID_ARGUMENT"
		if err.Error() == "settings version conflict" {
			status = http.StatusConflict
			code = "CONFLICT"
		}
		respondAPIError(w, r, status, code, err.Error(), nil)
		return
	}
	if err := a.scanRisks(r.Context()); err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Settings saved but risk recalculation failed", nil)
		return
	}
	respond(w, http.StatusOK, settings)
}

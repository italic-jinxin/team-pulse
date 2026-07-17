package risks

import (
	"testing"
	"time"

	appdb "github.com/italic-jinxin/team-pulse/internal/database"
)

func TestEvaluateExcludesDraftAndUsesCurrentFacts(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	candidate := appdb.RiskCandidate{
		ID: 1, GitHubID: 10, RepositoryID: 2, Number: 7, State: "open",
		ReviewState: "waiting", CIState: "failed",
		LastAuthorActivityAt: "2026-07-12T00:00:00Z",
		LastActivityAt:       "2026-07-01T00:00:00Z",
	}
	if decisions := Evaluate(appdb.DefaultRiskSettings(), []appdb.RiskCandidate{candidate}, now); len(decisions) != 3 {
		t.Fatalf("decisions = %#v, want three rules", decisions)
	}
	candidate.Draft = true
	if decisions := Evaluate(appdb.DefaultRiskSettings(), []appdb.RiskCandidate{candidate}, now); len(decisions) != 0 {
		t.Fatalf("draft decisions = %#v", decisions)
	}
	candidate.Draft = false
	candidate.ReviewState = "approved"
	candidate.CIState = "passed"
	candidate.LastActivityAt = "2026-07-15T00:00:00Z"
	if decisions := Evaluate(appdb.DefaultRiskSettings(), []appdb.RiskCandidate{candidate}, now); len(decisions) != 0 {
		t.Fatalf("healthy decisions = %#v", decisions)
	}
}

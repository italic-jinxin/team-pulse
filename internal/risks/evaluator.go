package risks

import (
	"encoding/json"
	"fmt"
	"time"

	appdb "github.com/italic-jinxin/team-pulse/internal/database"
)

func Evaluate(settings appdb.RiskSettings, candidates []appdb.RiskCandidate, now time.Time) []appdb.RiskDecision {
	decisions := []appdb.RiskDecision{}
	for _, candidate := range candidates {
		if candidate.State != "open" || candidate.Draft {
			continue
		}
		subjectID := fmt.Sprint(candidate.GitHubID)
		lastActivity, _ := time.Parse(time.RFC3339, candidate.LastActivityAt)
		if !lastActivity.IsZero() && now.Sub(lastActivity) > time.Duration(settings.StalePRDays)*24*time.Hour {
			decisions = append(decisions, appdb.RiskDecision{
				RuleType: "stale_pull_request", RepositoryID: candidate.RepositoryID,
				PullRequestID: candidate.ID, SubjectID: subjectID, Severity: "medium",
				ReasonCode:      "PR_STALE",
				Reason:          fmt.Sprintf("#%d has had no activity for %.0f days", candidate.Number, now.Sub(lastActivity).Hours()/24),
				SuggestedAction: "Check whether the pull request is still needed",
				EvidenceJSON:    jsonValue(map[string]any{"last_activity_at": candidate.LastActivityAt}),
			})
		}
		if candidate.ReviewState == "waiting" {
			waitStarted := latestTime(candidate.ReviewRequestedAt, candidate.LastAuthorActivityAt)
			if !waitStarted.IsZero() && now.Sub(waitStarted) > time.Duration(settings.WaitingReviewHours)*time.Hour {
				decisions = append(decisions, appdb.RiskDecision{
					RuleType: "waiting_for_review", RepositoryID: candidate.RepositoryID,
					PullRequestID: candidate.ID, SubjectID: subjectID, Severity: "medium",
					ReasonCode:      "REVIEW_WAIT_EXCEEDED",
					Reason:          fmt.Sprintf("#%d has waited %.0f hours for review", candidate.Number, now.Sub(waitStarted).Hours()),
					SuggestedAction: "Assign or remind a reviewer",
					EvidenceJSON:    jsonValue(map[string]any{"wait_started_at": waitStarted.Format(time.RFC3339)}),
				})
			}
		}
		if candidate.CIState == "failed" {
			decisions = append(decisions, appdb.RiskDecision{
				RuleType: "ci_failure", RepositoryID: candidate.RepositoryID,
				PullRequestID: candidate.ID, SubjectID: subjectID, Severity: "high",
				ReasonCode:      "CURRENT_HEAD_CI_FAILED",
				Reason:          fmt.Sprintf("CI is failing on #%d", candidate.Number),
				SuggestedAction: "Inspect the failed workflow and rerun CI",
				EvidenceJSON:    jsonValue(map[string]any{"ci_state": candidate.CIState}),
			})
		}
	}
	return decisions
}

func latestTime(values ...string) time.Time {
	var latest time.Time
	for _, value := range values {
		parsed, err := time.Parse(time.RFC3339, value)
		if err == nil && parsed.After(latest) {
			latest = parsed
		}
	}
	return latest
}

func jsonValue(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

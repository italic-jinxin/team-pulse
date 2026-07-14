package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type riskRules struct {
	WaitingReviewHours int `json:"waitingReviewHours"`
	StalePRDays        int `json:"stalePrDays"`
	LargePRLines       int `json:"largePrLines"`
	CIFailureThreshold int `json:"ciFailureThreshold"`
}

func (a *App) scanRisks() error {
	_, err := a.DB.Exec("DELETE FROM risks WHERE status='open'")
	if err != nil {
		return err
	}
	rows, err := a.DB.Query("SELECT repository,number,title,author,review_state,ci_state,additions+deletions,created_at,updated_at FROM pull_requests WHERE state='open'")
	if err != nil {
		return err
	}
	type pullRequest struct {
		repo, title, owner, review, ci, created, updated string
		num, lines                                       int
	}
	var pullRequests []pullRequest
	for rows.Next() {
		var pr pullRequest
		if err := rows.Scan(&pr.repo, &pr.num, &pr.title, &pr.owner, &pr.review, &pr.ci, &pr.lines, &pr.created, &pr.updated); err != nil {
			rows.Close()
			return err
		}
		pullRequests = append(pullRequests, pr)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	now := time.Now().UTC()
	rules := a.riskRules()
	for _, pr := range pullRequests {
		ut, _ := time.Parse(time.RFC3339, pr.updated)
		ct, _ := time.Parse(time.RFC3339, pr.created)
		if now.Sub(ut) > time.Duration(rules.StalePRDays)*24*time.Hour {
			a.addRisk("stale_pull_request", "medium", pr.repo, pr.num, pr.owner, fmt.Sprintf("#%d has had no activity for %.0f days", pr.num, now.Sub(ut).Hours()/24), "Check whether the pull request is still needed")
		}
		if pr.review == "waiting" && now.Sub(ct) > time.Duration(rules.WaitingReviewHours)*time.Hour {
			a.addRisk("waiting_for_review", "medium", pr.repo, pr.num, pr.owner, fmt.Sprintf("#%d has waited %.0f hours for review", pr.num, now.Sub(ct).Hours()), "Assign or remind a reviewer")
		}
		if pr.ci == "failed" {
			a.addRisk("ci_failure", "high", pr.repo, pr.num, pr.owner, fmt.Sprintf("CI is failing on #%d", pr.num), "Inspect the failed workflow and rerun CI")
		}
		if pr.lines > rules.LargePRLines {
			a.addRisk("large_pull_request", "low", pr.repo, pr.num, pr.owner, fmt.Sprintf("#%d changes %d lines", pr.num, pr.lines), "Consider splitting the change")
		}
	}
	return nil
}
func (a *App) addRisk(kind, severity, repo string, num int, owner, reason, action string) {
	id := fmt.Sprintf("%s:%s:%d", kind, repo, num)
	_, _ = a.DB.Exec("INSERT OR REPLACE INTO risks(id,type,severity,repository,pr_number,owner,reason,suggested_action,status,detected_at) VALUES(?,?,?,?,?,?,?,?, 'open',?)", id, kind, severity, repo, num, owner, reason, action, time.Now().UTC().Format(time.RFC3339))
}

func defaultRiskRules() riskRules {
	return riskRules{WaitingReviewHours: 48, StalePRDays: 5, LargePRLines: 800, CIFailureThreshold: 1}
}

func (a *App) riskRules() riskRules {
	rules := defaultRiskRules()
	var raw string
	if err := a.DB.QueryRow("SELECT value FROM settings WHERE key='risk_rules'").Scan(&raw); err == nil {
		_ = json.Unmarshal([]byte(raw), &rules)
	}
	if rules.WaitingReviewHours <= 0 {
		rules.WaitingReviewHours = 48
	}
	if rules.StalePRDays <= 0 {
		rules.StalePRDays = 5
	}
	if rules.LargePRLines <= 0 {
		rules.LargePRLines = 800
	}
	if rules.CIFailureThreshold <= 0 {
		rules.CIFailureThreshold = 1
	}
	return rules
}

func (a *App) getRiskRules(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, a.riskRules())
}

func (a *App) setRiskRules(w http.ResponseWriter, r *http.Request) {
	rules := defaultRiskRules()
	if decode(r, &rules) != nil {
		respond(w, 400, map[string]string{"error": "invalid risk rules"})
		return
	}
	b, _ := json.Marshal(rules)
	_, err := a.DB.Exec("INSERT INTO settings(key,value) VALUES('risk_rules',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(b))
	if err != nil {
		respond(w, 500, map[string]string{"error": err.Error()})
		return
	}
	respond(w, 200, rules)
}

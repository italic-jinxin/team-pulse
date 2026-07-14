package app

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strings"
	"time"
)

func (a *App) generateReport(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Type     string `json:"type"`
		Scope    string `json:"scope"`
		Length   string `json:"length"`
		Template string `json:"template"`
	}
	_ = decode(r, &in)
	if in.Type == "" {
		in.Type = "weekly"
	}
	if in.Length == "" {
		in.Length = "standard"
	}
	if in.Template == "" {
		in.Template = "executive"
	}
	days := -7
	if in.Type == "daily" {
		days = -1
	}
	since := time.Now().AddDate(0, 0, days).UTC().Format(time.RFC3339)
	var commits, prs, reviews, merged, risks int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM activities WHERE type='commit.pushed' AND occurred_at>=?", since).Scan(&commits)
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM activities WHERE type IN ('pr.updated','pr.opened') AND occurred_at>=?", since).Scan(&prs)
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM activities WHERE type='pr.reviewed' AND occurred_at>=?", since).Scan(&reviews)
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM activities WHERE type='pr.merged' AND occurred_at>=?", since).Scan(&merged)
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM risks WHERE status='open'").Scan(&risks)
	var b strings.Builder
	titleKind := reportTitle(in.Type)
	fmt.Fprintf(&b, "# TeamPulse %s\n\n_Generated %s_\n\n", titleKind, time.Now().Format("2 Jan 2006"))
	writeReportIntro(&b, in.Template, commits, prs, reviews, merged, risks)
	fmt.Fprintf(&b, "## Priority Risks\n\n")
	limit := 5
	if in.Length == "brief" {
		limit = 3
	}
	if in.Length == "detailed" {
		limit = 20
	}
	rows, _ := a.DB.Query("SELECT severity,repository,pr_number,reason,suggested_action FROM risks WHERE status='open' ORDER BY CASE severity WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, detected_at DESC LIMIT ?", limit)
	listed := 0
	if rows != nil {
		for rows.Next() {
			var severity, repo, reason, action string
			var n int
			_ = rows.Scan(&severity, &repo, &n, &reason, &action)
			fmt.Fprintf(&b, "### [%s] %s #%d\n\n%s. **Next:** %s.\n\n", strings.ToUpper(severity), repo, n, reason, action)
			listed++
		}
		_ = rows.Close()
	}
	if listed == 0 {
		fmt.Fprintf(&b, "No open priority risks detected.\n\n")
	} else if risks > listed {
		fmt.Fprintf(&b, "_%d additional open risks omitted. Use Detailed length for a longer export._\n\n", risks-listed)
	}
	id := nowID("report")
	title := titleKind + " — " + time.Now().Format("2 Jan 2006")
	created := time.Now().UTC().Format(time.RFC3339)
	_, err := a.DB.Exec("INSERT INTO reports(id,type,title,markdown,created_at) VALUES(?,?,?,?,?)", id, in.Type, title, b.String(), created)
	if err != nil {
		respond(w, 500, map[string]string{"error": err.Error()})
		return
	}
	respond(w, 201, map[string]any{"id": id, "type": in.Type, "title": title, "markdown": b.String(), "created_at": created})
}

func writeReportIntro(b *strings.Builder, template string, commits, prs, reviews, merged, risks int) {
	switch template {
	case "standup":
		fmt.Fprintf(b, "## Standup\n\n- **Moved:** %d commits, %d merged pull requests\n- **In review:** %d active pull requests, %d reviews\n- **Blocked / watch:** %d open risks\n\n", commits, merged, prs, reviews, risks)
	case "engineering":
		fmt.Fprintf(b, "## Engineering Detail\n\n- Commits: %d\n- Active pull requests: %d\n- Reviews: %d\n- Merged pull requests: %d\n- Open risks: %d\n\n## Engineering Notes\n\n- Review PR queue for owner, review, CI, and size signals.\n- Use repository view to identify concentration of failed checks.\n\n", commits, prs, reviews, merged, risks)
	case "risk":
		fmt.Fprintf(b, "## Risk-Focused Summary\n\n- Open risks: %d\n- Active pull requests: %d\n- Reviews completed: %d\n\n", risks, prs, reviews)
	default:
		fmt.Fprintf(b, "## Executive Summary\n\n- %d commits\n- %d active pull requests\n- %d reviews\n- %d merged pull requests\n- %d open risks\n\n", commits, prs, reviews, merged, risks)
	}
}

func reportTitle(kind string) string {
	switch kind {
	case "daily":
		return "Daily Report"
	case "risk":
		return "Risk Summary"
	default:
		return "Weekly Report"
	}
}
func (a *App) listReports(w http.ResponseWriter, r *http.Request) {
	queryJSON(w, a.DB, "SELECT id,type,title,created_at FROM reports ORDER BY created_at DESC")
}
func (a *App) getReport(w http.ResponseWriter, r *http.Request) {
	queryJSON(w, a.DB, "SELECT id,type,title,markdown,created_at FROM reports WHERE id=?", chi.URLParam(r, "id"))
}

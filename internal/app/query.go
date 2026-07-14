package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (a *App) status(w http.ResponseWriter, r *http.Request) {
	var repos, prs, risks, members int
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM repositories").Scan(&repos)
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM pull_requests WHERE state='open'").Scan(&prs)
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM risks WHERE status='open'").Scan(&risks)
	_ = a.DB.QueryRow("SELECT COUNT(*) FROM members").Scan(&members)
	respond(w, 200, map[string]any{"repositories": repos, "open_pull_requests": prs, "open_risks": risks, "members": members, "data_dir": a.DataDir})
}
func (a *App) authStatus(w http.ResponseWriter, r *http.Request) {
	auth.RLock()
	set := auth.token != ""
	source := auth.source
	auth.RUnlock()
	if !set {
		if cmd := exec.Command("gh", "auth", "token"); cmd.Run() == nil {
			set = true
			source = "github_cli"
		}
	}
	respond(w, 200, map[string]any{"authenticated": set, "source": source})
}
func (a *App) setToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if decode(r, &in) != nil || !strings.HasPrefix(in.Token, "github_pat_") && !strings.HasPrefix(in.Token, "ghp_") {
		respond(w, 400, map[string]string{"error": "invalid GitHub token"})
		return
	}
	c := &ghClient{token: in.Token, http: &http.Client{Timeout: 15 * time.Second}}
	var user struct {
		Login string `json:"login"`
	}
	if err := c.get(r.Context(), "/user", &user); err != nil {
		respond(w, 401, map[string]string{"error": "GitHub rejected this token"})
		return
	}
	auth.Lock()
	auth.token = in.Token
	auth.source = "memory"
	auth.Unlock()
	respond(w, 200, map[string]string{"login": user.Login})
}
func (a *App) clearToken(w http.ResponseWriter, r *http.Request) {
	auth.Lock()
	auth.token = ""
	auth.source = ""
	auth.Unlock()
	respond(w, 204, nil)
}

func rowsJSON(w http.ResponseWriter, rows *sql.Rows, err error) {
	if err != nil {
		respond(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if rows.Scan(ptrs...) != nil {
			continue
		}
		m := map[string]any{}
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				m[c] = string(b)
			} else {
				m[c] = vals[i]
			}
		}
		out = append(out, m)
	}
	respond(w, 200, out)
}
func queryJSON(w http.ResponseWriter, db *sql.DB, query string, args ...any) {
	rows, err := db.Query(query, args...)
	rowsJSON(w, rows, err)
}
func (a *App) listRepositories(w http.ResponseWriter, r *http.Request) {
	queryJSON(w, a.DB, "SELECT id,full_name,description,private,default_branch,updated_at FROM repositories ORDER BY full_name")
}
func (a *App) listActivity(w http.ResponseWriter, r *http.Request) {
	queryJSON(w, a.DB, "SELECT id,repository,actor,type,title,url,occurred_at FROM activities ORDER BY occurred_at DESC LIMIT 200")
}
func (a *App) listMembers(w http.ResponseWriter, r *http.Request) {
	queryJSON(w, a.DB, "SELECT login,avatar_url,commits,pull_requests,reviews,last_active_at FROM members ORDER BY last_active_at DESC")
}
func (a *App) listPullRequests(w http.ResponseWriter, r *http.Request) {
	queryJSON(w, a.DB, "SELECT id,repository,number,title,author,url,state,draft,review_state,ci_state,additions,deletions,created_at,updated_at,merged_at FROM pull_requests ORDER BY updated_at DESC LIMIT 200")
}
func (a *App) listRisks(w http.ResponseWriter, r *http.Request) {
	queryJSON(w, a.DB, "SELECT id,type,severity,repository,pr_number,owner,reason,suggested_action,status,detected_at FROM risks ORDER BY CASE severity WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, detected_at DESC")
}
func (a *App) updateRisk(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Status string `json:"status"`
	}
	if decode(r, &in) != nil || (in.Status != "open" && in.Status != "resolved" && in.Status != "ignored") {
		respond(w, 400, map[string]string{"error": "invalid status"})
		return
	}
	res, err := a.DB.Exec("UPDATE risks SET status=? WHERE id=?", in.Status, chi.URLParam(r, "id"))
	if err != nil {
		respond(w, 500, map[string]string{"error": err.Error()})
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respond(w, 404, map[string]string{"error": "not found"})
		return
	}
	respond(w, 200, map[string]string{"status": in.Status})
}
func (a *App) listJobs(w http.ResponseWriter, r *http.Request) {
	queryJSON(w, a.DB, "SELECT id,type,status,progress,message,created_at,started_at,ended_at,error FROM jobs ORDER BY created_at DESC LIMIT 20")
}
func (a *App) getJob(w http.ResponseWriter, r *http.Request) {
	queryJSON(w, a.DB, "SELECT id,type,status,progress,message,created_at,started_at,ended_at,error FROM jobs WHERE id=?", chi.URLParam(r, "id"))
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nowID(prefix string) string {
	return prefix + "_" + time.Now().UTC().Format("20060102T150405.000000000")
}
func marshal(v any) string { b, _ := json.Marshal(v); return string(b) }

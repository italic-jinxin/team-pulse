package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type ghClient struct {
	token string
	http  *http.Client
}

func (a *App) githubRepositories(w http.ResponseWriter, r *http.Request) {
	c, err := githubClient()
	if err != nil {
		respond(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}
	var repos []struct {
		ID          int64  `json:"id"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
		PushedAt    string `json:"pushed_at"`
		Owner       struct {
			AvatarURL string `json:"avatar_url"`
		} `json:"owner"`
	}
	if err := c.get(r.Context(), "/user/repos?affiliation=owner,collaborator,organization_member&sort=pushed&direction=desc&per_page=100", &repos); err != nil {
		respond(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	respond(w, http.StatusOK, repos)
}

func githubClient() (*ghClient, error) {
	auth.RLock()
	token := auth.token
	auth.RUnlock()
	if token == "" {
		out, err := exec.Command("gh", "auth", "token").Output()
		if err != nil {
			return nil, fmt.Errorf("connect GitHub with a PAT or run gh auth login")
		}
		token = strings.TrimSpace(string(out))
	}
	return &ghClient{token: token, http: &http.Client{Timeout: 30 * time.Second}}, nil
}
func (c *ghClient) get(ctx context.Context, path string, out any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com"+path, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("GitHub API %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (a *App) startSync(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Repositories []string `json:"repositories"`
	}
	if decode(r, &in) != nil || len(in.Repositories) == 0 {
		respond(w, 400, map[string]string{"error": "repositories must contain owner/name values"})
		return
	}
	for _, repo := range in.Repositories {
		if len(strings.Split(repo, "/")) != 2 {
			respond(w, 400, map[string]string{"error": "invalid repository: " + repo})
			return
		}
	}
	id := nowID("sync")
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.DB.Exec("INSERT INTO jobs(id,type,status,progress,message,created_at) VALUES(?, 'github_sync','pending',0,'Queued',?)", id, now)
	go a.sync(id, in.Repositories)
	respond(w, 202, map[string]string{"job_id": id})
}

func (a *App) sync(jobID string, repos []string) {
	started := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.DB.Exec("UPDATE jobs SET status='running',started_at=?,message='Connecting to GitHub' WHERE id=?", started, jobID)
	c, err := githubClient()
	if err != nil {
		a.failJob(jobID, err)
		return
	}
	since := time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339)
	for i, repo := range repos {
		base := i * 90 / len(repos)
		next := (i + 1) * 90 / len(repos)
		update := func(fraction int, message string) {
			progress := base + (next-base)*fraction/100
			_, _ = a.DB.Exec("UPDATE jobs SET progress=?,message=? WHERE id=?", progress, message, jobID)
		}
		update(3, "Starting "+repo)
		if err := a.syncRepo(context.Background(), c, repo, since, update); err != nil {
			a.failJob(jobID, fmt.Errorf("%s: %w", repo, err))
			return
		}
	}
	_, _ = a.DB.Exec("UPDATE jobs SET progress=92,message='Scanning risk signals' WHERE id=?", jobID)
	if err := a.scanRisks(); err != nil {
		a.failJob(jobID, err)
		return
	}
	ended := time.Now().UTC().Format(time.RFC3339)
	_, _ = a.DB.Exec("UPDATE jobs SET status='completed',progress=100,message='Sync complete',ended_at=? WHERE id=?", ended, jobID)
}
func (a *App) failJob(id string, err error) {
	_, _ = a.DB.Exec("UPDATE jobs SET status='failed',error=?,message='Sync failed',ended_at=? WHERE id=?", err.Error(), time.Now().UTC().Format(time.RFC3339), id)
}

func (a *App) syncRepo(ctx context.Context, c *ghClient, repo, since string, update func(fraction int, message string)) error {
	var meta struct {
		ID            int64  `json:"id"`
		FullName      string `json:"full_name"`
		Description   string `json:"description"`
		Private       bool   `json:"private"`
		DefaultBranch string `json:"default_branch"`
		UpdatedAt     string `json:"updated_at"`
	}
	update(8, "Reading repository metadata for "+repo)
	if err := c.get(ctx, "/repos/"+repo, &meta); err != nil {
		return err
	}
	_, _ = a.DB.Exec(`INSERT INTO repositories(id,full_name,description,private,default_branch,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET full_name=excluded.full_name,description=excluded.description,private=excluded.private,default_branch=excluded.default_branch,updated_at=excluded.updated_at`, meta.ID, meta.FullName, meta.Description, meta.Private, meta.DefaultBranch, meta.UpdatedAt)
	var commits []struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
		Commit  struct {
			Message string `json:"message"`
			Author  struct {
				Date string `json:"date"`
				Name string `json:"name"`
			} `json:"author"`
		} `json:"commit"`
		Author *struct {
			Login  string `json:"login"`
			Avatar string `json:"avatar_url"`
		} `json:"author"`
	}
	update(22, "Fetching commits for "+repo)
	if err := c.get(ctx, "/repos/"+repo+"/commits?since="+since+"&per_page=100", &commits); err != nil {
		return err
	}
	for _, v := range commits {
		actor := v.Commit.Author.Name
		avatar := ""
		if v.Author != nil {
			actor = v.Author.Login
			avatar = v.Author.Avatar
		}
		title := strings.Split(v.Commit.Message, "\n")[0]
		_, _ = a.DB.Exec("INSERT OR REPLACE INTO activities(id,repository,actor,type,title,url,occurred_at,metadata_json) VALUES(?,?,?,?,?,?,?,?)", "commit:"+v.SHA, repo, actor, "commit.pushed", title, v.HTMLURL, v.Commit.Author.Date, "{}")
		a.upsertMember(actor, avatar, "commits", v.Commit.Author.Date)
	}
	var prs []struct {
		ID        int64  `json:"id"`
		Number    int    `json:"number"`
		Title     string `json:"title"`
		HTMLURL   string `json:"html_url"`
		State     string `json:"state"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		MergedAt  string `json:"merged_at"`
		Draft     bool   `json:"draft"`
		User      struct {
			Login  string `json:"login"`
			Avatar string `json:"avatar_url"`
		} `json:"user"`
	}
	update(45, "Fetching pull requests for "+repo)
	if err := c.get(ctx, "/repos/"+repo+"/pulls?state=all&sort=updated&direction=desc&per_page=100", &prs); err != nil {
		return err
	}
	for i, p := range prs {
		if p.UpdatedAt < since {
			continue
		}
		if len(prs) > 0 {
			update(55+i*40/len(prs), fmt.Sprintf("Reading PR #%d in %s", p.Number, repo))
		}
		var detail struct {
			Additions int `json:"additions"`
			Deletions int `json:"deletions"`
		}
		_ = c.get(ctx, fmt.Sprintf("/repos/%s/pulls/%d", repo, p.Number), &detail)
		review, ci := a.prStates(ctx, c, repo, p.Number)
		id := fmt.Sprint(p.ID)
		_, _ = a.DB.Exec(`INSERT OR REPLACE INTO pull_requests(id,repository,number,title,author,url,state,draft,review_state,ci_state,additions,deletions,created_at,updated_at,merged_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, repo, p.Number, p.Title, p.User.Login, p.HTMLURL, p.State, p.Draft, review, ci, detail.Additions, detail.Deletions, p.CreatedAt, p.UpdatedAt, nullable(p.MergedAt))
		typ := "pr.updated"
		if p.MergedAt != "" {
			typ = "pr.merged"
		}
		_, _ = a.DB.Exec("INSERT OR REPLACE INTO activities(id,repository,actor,type,title,url,occurred_at,metadata_json) VALUES(?,?,?,?,?,?,?,?)", "pr:"+id, repo, p.User.Login, typ, p.Title, p.HTMLURL, p.UpdatedAt, "{}")
		a.upsertMember(p.User.Login, p.User.Avatar, "pull_requests", p.UpdatedAt)
	}
	update(100, "Finished "+repo)
	return nil
}
func (a *App) prStates(ctx context.Context, c *ghClient, repo string, n int) (string, string) {
	var reviews []struct {
		ID          int64  `json:"id"`
		State       string `json:"state"`
		SubmittedAt string `json:"submitted_at"`
		User        struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"user"`
	}
	_ = c.get(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews?per_page=100", repo, n), &reviews)
	state := "waiting"
	for _, rv := range reviews {
		switch rv.State {
		case "APPROVED":
			state = "approved"
		case "CHANGES_REQUESTED":
			state = "changes_requested"
		}
		a.upsertMember(rv.User.Login, rv.User.AvatarURL, "reviews", rv.SubmittedAt)
		_, _ = a.DB.Exec("INSERT OR REPLACE INTO activities(id,repository,actor,type,title,url,occurred_at,metadata_json) VALUES(?,?,?,?,?,?,?,?)", fmt.Sprintf("review:%d", rv.ID), repo, rv.User.Login, "pr.reviewed", fmt.Sprintf("Reviewed #%d", n), "", rv.SubmittedAt, marshal(map[string]string{"state": rv.State}))
	}
	var runs struct {
		WorkflowRuns []struct {
			Conclusion string `json:"conclusion"`
		} `json:"workflow_runs"`
	}
	ci := "unknown"
	if c.get(ctx, fmt.Sprintf("/repos/%s/actions/runs?event=pull_request&per_page=20", repo), &runs) == nil {
		for _, run := range runs.WorkflowRuns {
			if run.Conclusion == "failure" {
				ci = "failed"
				break
			}
			if run.Conclusion == "success" {
				ci = "passed"
			}
		}
	}
	return state, ci
}
func (a *App) upsertMember(login, avatar, kind, date string) {
	if login == "" {
		return
	}
	col := "commits"
	if kind == "pull_requests" {
		col = "pull_requests"
	}
	if kind == "reviews" {
		col = "reviews"
	}
	q := fmt.Sprintf(`INSERT INTO members(login,avatar_url,%s,last_active_at) VALUES(?,?,1,?) ON CONFLICT(login) DO UPDATE SET avatar_url=CASE WHEN excluded.avatar_url='' THEN members.avatar_url ELSE excluded.avatar_url END,%s=%s+1,last_active_at=MAX(last_active_at,excluded.last_active_at)`, col, col, col)
	_, _ = a.DB.Exec(q, login, avatar, date)
}

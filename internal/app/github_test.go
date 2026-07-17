package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/italic-jinxin/team-pulse/internal/credentials"
	appdb "github.com/italic-jinxin/team-pulse/internal/database"
	githubclient "github.com/italic-jinxin/team-pulse/internal/github"
)

type githubFixture struct {
	t                    *testing.T
	repositoryCount      int
	failPullRequestFiles bool
	changedFileCount     int
	expectedToken        string
}

func (fixture *githubFixture) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	expectedToken := fixture.expectedToken
	if expectedToken == "" {
		expectedToken = "token"
	}
	if request.Header.Get("Authorization") != "Bearer "+expectedToken {
		fixture.t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
	}
	response.Header().Set("Content-Type", "application/json")
	switch {
	case request.URL.Path == "/user":
		writeFixtureJSON(response, map[string]any{"id": 1, "login": "alice", "type": "User"})
	case request.URL.Path == "/user/repos":
		repositories := make([]map[string]any, fixture.repositoryCount)
		for index := range repositories {
			repositories[index] = map[string]any{
				"id": index + 100, "name": fmt.Sprintf("repo-%02d", index),
				"full_name": fmt.Sprintf("acme/repo-%02d", index),
				"html_url":  fmt.Sprintf("https://github.com/acme/repo-%02d", index),
				"pushed_at": fmt.Sprintf("2026-07-%02dT00:00:00Z", (index%28)+1),
				"owner":     map[string]any{"login": "acme"},
			}
		}
		writeFixtureJSON(response, repositories)
	case request.URL.Path == "/repos/acme/pulse":
		writeFixtureJSON(response, map[string]any{
			"id": 2, "node_id": "repo-node", "name": "pulse", "full_name": "acme/pulse",
			"html_url": "https://github.com/acme/pulse", "default_branch": "main",
			"owner": map[string]any{"id": 20, "login": "acme", "type": "Organization"},
		})
	case request.URL.Path == "/repos/acme/pulse/commits":
		writeFixtureJSON(response, []map[string]any{{
			"sha": "commit-sha", "node_id": "commit-node", "html_url": "https://github.com/acme/pulse/commit/commit-sha",
			"commit": map[string]any{
				"message": "Add deterministic sync", "author": map[string]any{"date": "2099-07-01T10:00:00Z", "name": "Alice"},
				"committer": map[string]any{"date": "2099-07-01T10:00:00Z"},
			},
			"author":    map[string]any{"id": 10, "login": "alice", "type": "User"},
			"committer": map[string]any{"id": 10, "login": "alice", "type": "User"},
		}})
	case request.URL.Path == "/repos/acme/pulse/pulls":
		writeFixtureJSON(response, []map[string]any{{"id": 30, "number": 7, "updated_at": "2099-07-02T10:00:00Z"}})
	case request.URL.Path == "/repos/acme/pulse/pulls/7":
		changedFileCount := fixture.changedFileCount
		if changedFileCount == 0 {
			changedFileCount = 2
		}
		writeFixtureJSON(response, map[string]any{
			"id": 30, "node_id": "pr-node", "number": 7, "title": "Complete Alpha 2",
			"body": "Facts", "html_url": "https://github.com/acme/pulse/pull/7", "state": "open",
			"created_at": "2099-07-01T09:00:00Z", "updated_at": "2099-07-02T10:00:00Z",
			"user":      map[string]any{"id": 10, "login": "alice", "type": "User"},
			"head":      map[string]any{"ref": "alpha-2", "sha": "head-sha"},
			"base":      map[string]any{"ref": "main", "sha": "base-sha"},
			"additions": 12, "deletions": 3, "changed_files": changedFileCount,
		})
	case request.URL.Path == "/repos/acme/pulse/pulls/7/files":
		if fixture.failPullRequestFiles {
			http.Error(response, `{"message":"fixture failure"}`, http.StatusInternalServerError)
			return
		}
		if request.URL.Query().Get("page") == "2" {
			writeFixtureJSON(response, []map[string]any{{
				"filename": "docs/alpha-2.md", "status": "added", "additions": 5, "deletions": 0, "changes": 5,
			}})
			return
		}
		response.Header().Set("Link", "</repos/acme/pulse/pulls/7/files?page=2>; rel=\"next\"")
		writeFixtureJSON(response, []map[string]any{{
			"filename": "internal/app/github_test.go", "previous_filename": "internal/app/sync_test.go",
			"status": "renamed", "additions": 7, "deletions": 3, "changes": 10,
		}})
	case request.URL.Path == "/repos/acme/pulse/pulls/7/reviews":
		writeFixtureJSON(response, []map[string]any{{
			"id": 40, "node_id": "review-node", "state": "APPROVED", "commit_id": "head-sha",
			"html_url":     "https://github.com/acme/pulse/pull/7#pullrequestreview-40",
			"submitted_at": "2099-07-02T11:00:00Z",
			"user":         map[string]any{"id": 11, "login": "bob", "type": "User"},
		}})
	case request.URL.Path == "/repos/acme/pulse/actions/runs":
		if request.URL.Query().Get("head_sha") != "head-sha" {
			fixture.t.Errorf("workflow head_sha = %q", request.URL.Query().Get("head_sha"))
		}
		writeFixtureJSON(response, map[string]any{"workflow_runs": []map[string]any{
			{
				"id": 50, "node_id": "run-node", "workflow_id": 60, "name": "CI", "run_number": 1, "run_attempt": 1,
				"event": "pull_request", "head_branch": "alpha-2", "head_sha": "head-sha", "status": "completed",
				"conclusion": "success", "html_url": "https://github.com/acme/pulse/actions/runs/50",
				"created_at": "2099-07-02T12:00:00Z", "updated_at": "2099-07-02T12:05:00Z",
			},
			{
				"id": 51, "workflow_id": 60, "name": "CI", "run_number": 0, "run_attempt": 1,
				"head_sha": "old-head", "status": "completed", "conclusion": "failure",
				"html_url":   "https://github.com/acme/pulse/actions/runs/51",
				"created_at": "2099-07-01T12:00:00Z", "updated_at": "2099-07-01T12:05:00Z",
			},
		}})
	default:
		http.NotFound(response, request)
	}
}

func TestSetTokenUsesInjectedGitHubBaseURL(t *testing.T) {
	fixture := &githubFixture{t: t, expectedToken: "github_pat_fixture"}
	server := httptest.NewServer(fixture)
	defer server.Close()
	application, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	application.GitHubBaseURL = server.URL
	response := performAPIRequest(t, application.Router("http://127.0.0.1:19421"), http.MethodPost,
		"/api/github/auth/token", `{"token":"github_pat_fixture"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("set token status=%d body=%s", response.Code, response.Body.String())
	}
	credential, err := application.Credentials.Get(context.Background())
	if err != nil || credential.Login != "alice" {
		t.Fatalf("credential=%#v err=%v", credential, err)
	}
}

func TestRepositoryCatalogDefaultsFiveOnceAndRejectsMoreThanTwenty(t *testing.T) {
	application, fixtureServer := newFixtureApplication(t, &githubFixture{t: t, repositoryCount: 21})
	defer application.Close()
	defer fixtureServer.Close()
	router := application.Router("http://127.0.0.1:19421")

	response := performAPIRequest(t, router, http.MethodGet, "/api/repositories", "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET repositories status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []appdb.RepositoryRecord `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	selected := 0
	ids := make([]int64, 0, len(payload.Items))
	for _, repository := range payload.Items {
		ids = append(ids, repository.ID)
		if repository.Selected {
			selected++
		}
	}
	if selected != 5 || len(ids) != 21 {
		t.Fatalf("repositories=%d selected=%d", len(ids), selected)
	}

	selectionBody, _ := json.Marshal(map[string]any{"repository_ids": ids})
	response = performAPIRequest(t, router, http.MethodPatch, "/api/repositories/selection", string(selectionBody))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("select 21 status=%d body=%s", response.Code, response.Body.String())
	}
	response = performAPIRequest(t, router, http.MethodPost, "/api/sync-jobs", string(selectionBody))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("sync 21 status=%d body=%s", response.Code, response.Body.String())
	}

	response = performAPIRequest(t, router, http.MethodPatch, "/api/repositories/selection", `{"repository_ids":[]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("clear selection status=%d body=%s", response.Code, response.Body.String())
	}
	response = performAPIRequest(t, router, http.MethodGet, "/api/repositories", "")
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, repository := range payload.Items {
		if repository.Selected {
			t.Fatalf("repository %s was reselected after intentional clear", repository.FullName)
		}
	}
}

func TestSyncRepoPersistsCompleteFactsIdempotently(t *testing.T) {
	application, fixtureServer := newFixtureApplication(t, &githubFixture{t: t})
	defer application.Close()
	defer fixtureServer.Close()
	target := seedSyncTarget(t, application)
	client, err := githubclient.NewClientWithBaseURL("token", 30*time.Second, fixtureServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	wantCounts := []int{1, 2, 1, 1, 2, 1, 1, 3}
	for run := 1; run <= 3; run++ {
		if err := application.syncRepo(context.Background(), client, target, "2026-07-01T00:00:00Z"); err != nil {
			t.Fatalf("sync run %d: %v", run, err)
		}
		if got := factCounts(t, application); fmt.Sprint(got) != fmt.Sprint(wantCounts) {
			t.Fatalf("sync run %d counts=%v want=%v", run, got, wantCounts)
		}
		members, err := application.Repository.ListMembers(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		aggregates := map[string][3]int{}
		for _, member := range members {
			aggregates[member.Login] = [3]int{member.Commits, member.PullRequests, member.Reviews}
		}
		if fmt.Sprint(aggregates) != fmt.Sprint(map[string][3]int{"alice": {1, 1, 0}, "bob": {0, 0, 1}}) {
			t.Fatalf("sync run %d member aggregates=%v", run, aggregates)
		}
	}
	var complete, testFile, documentationFile int
	var previousFilename string
	if err := application.DB.QueryRow("SELECT files_complete FROM pull_requests WHERE github_id=30").Scan(&complete); err != nil {
		t.Fatal(err)
	}
	if err := application.DB.QueryRow(`
		SELECT previous_filename, is_test FROM pull_request_files WHERE filename='internal/app/github_test.go'
	`).Scan(&previousFilename, &testFile); err != nil {
		t.Fatal(err)
	}
	if err := application.DB.QueryRow("SELECT is_documentation FROM pull_request_files WHERE filename='docs/alpha-2.md'").Scan(&documentationFile); err != nil {
		t.Fatal(err)
	}
	if complete != 1 || previousFilename != "internal/app/sync_test.go" || testFile != 1 || documentationFile != 1 {
		t.Fatalf("complete=%d previous=%q test=%d documentation=%d", complete, previousFilename, testFile, documentationFile)
	}
	var headSHA string
	if err := application.DB.QueryRow("SELECT head_sha FROM workflow_runs").Scan(&headSHA); err != nil || headSHA != "head-sha" {
		t.Fatalf("workflow head SHA=%q err=%v", headSHA, err)
	}
}

func TestSyncJobPersistsResourceFailureAsPartial(t *testing.T) {
	application, fixtureServer := newFixtureApplication(t, &githubFixture{t: t, failPullRequestFiles: true})
	defer application.Close()
	defer fixtureServer.Close()
	target := seedSyncTarget(t, application)
	if err := application.Repository.CreateSyncJob(context.Background(), "job-failure", []appdb.SyncTarget{target}); err != nil {
		t.Fatal(err)
	}
	client, err := githubclient.NewClientWithBaseURL("token", 30*time.Second, fixtureServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	application.syncWithClient(context.Background(), "job-failure", []appdb.SyncTarget{target}, client)
	var status, resource string
	if err := application.DB.QueryRow("SELECT status FROM sync_jobs WHERE id='job-failure'").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := application.DB.QueryRow("SELECT resource_type FROM sync_job_errors WHERE job_id='job-failure'").Scan(&resource); err != nil {
		t.Fatal(err)
	}
	var complete int
	if err := application.DB.QueryRow("SELECT files_complete FROM pull_requests WHERE github_id=30").Scan(&complete); err != nil {
		t.Fatal(err)
	}
	if status != "partial" || resource != "pull_request_files" || complete != 0 {
		t.Fatalf("status=%q resource=%q files_complete=%d", status, resource, complete)
	}
}

func TestSyncJobMarksIncompletePullRequestFileListPartial(t *testing.T) {
	application, fixtureServer := newFixtureApplication(t, &githubFixture{t: t, changedFileCount: 3})
	defer application.Close()
	defer fixtureServer.Close()
	target := seedSyncTarget(t, application)
	if err := application.Repository.CreateSyncJob(context.Background(), "job-incomplete-files", []appdb.SyncTarget{target}); err != nil {
		t.Fatal(err)
	}
	client, err := githubclient.NewClientWithBaseURL("token", 30*time.Second, fixtureServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	application.syncWithClient(context.Background(), "job-incomplete-files", []appdb.SyncTarget{target}, client)
	var status, resource string
	var complete, activeFiles int
	if err := application.DB.QueryRow("SELECT status FROM sync_jobs WHERE id='job-incomplete-files'").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := application.DB.QueryRow("SELECT resource_type FROM sync_job_errors WHERE job_id='job-incomplete-files'").Scan(&resource); err != nil {
		t.Fatal(err)
	}
	if err := application.DB.QueryRow("SELECT files_complete FROM pull_requests WHERE github_id=30").Scan(&complete); err != nil {
		t.Fatal(err)
	}
	if err := application.DB.QueryRow("SELECT COUNT(*) FROM pull_request_files WHERE deleted_at IS NULL").Scan(&activeFiles); err != nil {
		t.Fatal(err)
	}
	if status != "partial" || resource != "pull_request_files" || complete != 0 || activeFiles != 2 {
		t.Fatalf("status=%q resource=%q files_complete=%d active_files=%d", status, resource, complete, activeFiles)
	}
}

func TestPullRequestFileReplacementRollsBackAtomically(t *testing.T) {
	application, fixtureServer := newFixtureApplication(t, &githubFixture{t: t})
	defer application.Close()
	defer fixtureServer.Close()
	target := seedSyncTarget(t, application)
	client, err := githubclient.NewClientWithBaseURL("token", 30*time.Second, fixtureServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := application.syncRepo(context.Background(), client, target, "2026-07-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if _, err := application.DB.Exec(`
		CREATE TRIGGER fail_pull_request_file_insert
		BEFORE INSERT ON pull_request_files
		WHEN NEW.filename = 'docs/alpha-2.md'
		BEGIN
			SELECT RAISE(ABORT, 'forced pull request file failure');
		END
	`); err != nil {
		t.Fatal(err)
	}
	if err := application.Repository.CreateSyncJob(context.Background(), "job-atomic-files", []appdb.SyncTarget{target}); err != nil {
		t.Fatal(err)
	}
	application.syncWithClient(context.Background(), "job-atomic-files", []appdb.SyncTarget{target}, client)

	var status, resource string
	var complete, activeFiles int
	if err := application.DB.QueryRow("SELECT status FROM sync_jobs WHERE id='job-atomic-files'").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := application.DB.QueryRow("SELECT resource_type FROM sync_job_errors WHERE job_id='job-atomic-files'").Scan(&resource); err != nil {
		t.Fatal(err)
	}
	if err := application.DB.QueryRow("SELECT files_complete FROM pull_requests WHERE github_id=30").Scan(&complete); err != nil {
		t.Fatal(err)
	}
	if err := application.DB.QueryRow("SELECT COUNT(*) FROM pull_request_files WHERE deleted_at IS NULL").Scan(&activeFiles); err != nil {
		t.Fatal(err)
	}
	if status != "partial" || resource != "pull_request_files" || complete != 0 || activeFiles != 2 {
		t.Fatalf("status=%q resource=%q files_complete=%d active_files=%d", status, resource, complete, activeFiles)
	}
}

func TestClassifyPullRequestFile(t *testing.T) {
	tests := []struct {
		filename                                           string
		language, module                                   string
		test, documentation, config, dependency, migration bool
	}{
		{"internal/app/github_test.go", "Go", "internal", true, false, false, false, false},
		{"docs/alpha-2.md", "", "docs", false, true, false, false, false},
		{".github/workflows/ci.yml", "", ".github", false, false, true, false, false},
		{"package-lock.json", "", "", false, false, true, true, false},
		{"migrations/000002_add_fact.sql", "SQL", "migrations", false, false, false, false, true},
	}
	for _, test := range tests {
		t.Run(test.filename, func(t *testing.T) {
			fact := classifyPullRequestFile(test.filename)
			if fact.Language != test.language || fact.ModuleName != test.module || fact.IsTest != test.test ||
				fact.IsDocumentation != test.documentation || fact.IsConfiguration != test.config ||
				fact.IsDependency != test.dependency || fact.IsMigration != test.migration {
				t.Fatalf("classification = %#v", fact)
			}
		})
	}
}

func newFixtureApplication(t *testing.T, fixture *githubFixture) (*App, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(fixture)
	application, err := New(t.TempDir())
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	application.GitHubBaseURL = server.URL
	ctx := context.Background()
	if err := application.Credentials.Set(ctx, credentials.Credential{Token: "token", Source: "memory", Login: "alice"}); err != nil {
		application.Close()
		server.Close()
		t.Fatal(err)
	}
	if _, err := application.Repository.UpsertAccount(ctx, appdb.AccountFact{GitHubID: 1, Login: "alice", SyncedAt: "2026-07-17T00:00:00Z"}); err != nil {
		application.Close()
		server.Close()
		t.Fatal(err)
	}
	return application, server
}

func seedSyncTarget(t *testing.T, application *App) appdb.SyncTarget {
	t.Helper()
	ctx := context.Background()
	account, err := application.Repository.AccountByLogin(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	id, err := application.Repository.UpsertRepository(ctx, account.ID, appdb.RepositoryFact{
		GitHubID: 2, FullName: "acme/pulse", HTMLURL: "https://github.com/acme/pulse", SyncedAt: "2026-07-17T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	return appdb.SyncTarget{ID: id, GitHubID: 2, FullName: "acme/pulse"}
}

func factCounts(t *testing.T, application *App) []int {
	t.Helper()
	tables := []string{
		"repositories", "team_members", "commits", "pull_requests", "pull_request_files",
		"pull_request_reviews", "workflow_runs", "activity_events",
	}
	result := make([]int, len(tables))
	for index, table := range tables {
		if err := application.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&result[index]); err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func performAPIRequest(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Origin", "http://127.0.0.1:19421")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func writeFixtureJSON(response http.ResponseWriter, value any) {
	_ = json.NewEncoder(response).Encode(value)
}

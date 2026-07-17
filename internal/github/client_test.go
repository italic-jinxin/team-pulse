package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestListEndpointsFollowRelativeAndAbsoluteNextLinks(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		page := request.URL.Query().Get("page")
		if page == "" {
			query := request.URL.Query()
			query.Set("page", "2")
			next := request.URL.Path + "?" + query.Encode()
			if strings.Contains(request.URL.Path, "/commits") || strings.Contains(request.URL.Path, "/actions/runs") {
				next = server.URL + next
			}
			response.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"next\", <%s>; rel=\"last\"", next, next))
		}
		id := int64(1)
		if page == "2" {
			id = 2
		}
		switch {
		case request.URL.Path == "/user/repos":
			_ = json.NewEncoder(response).Encode([]Repository{{ID: id}})
		case strings.HasSuffix(request.URL.Path, "/commits"):
			_ = json.NewEncoder(response).Encode([]Commit{{SHA: fmt.Sprint(id)}})
		case strings.HasSuffix(request.URL.Path, "/pulls"):
			_ = json.NewEncoder(response).Encode([]PullRequest{{ID: id}})
		case strings.HasSuffix(request.URL.Path, "/files"):
			_ = json.NewEncoder(response).Encode([]PullRequestFile{{Filename: fmt.Sprintf("file-%d.go", id)}})
		case strings.HasSuffix(request.URL.Path, "/reviews"):
			_ = json.NewEncoder(response).Encode([]Review{{ID: id}})
		case strings.HasSuffix(request.URL.Path, "/actions/runs"):
			if request.URL.Query().Get("head_sha") != "head sha" {
				t.Errorf("head_sha = %q", request.URL.Query().Get("head_sha"))
			}
			_ = json.NewEncoder(response).Encode(map[string]any{"workflow_runs": []WorkflowRun{{ID: id}}})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	client, err := NewClientWithBaseURL("secret", 30*time.Second, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repositories, err := client.ListRepositories(ctx)
	if err != nil || len(repositories) != 2 {
		t.Fatalf("repositories = %#v, err=%v", repositories, err)
	}
	commits, err := client.ListCommits(ctx, "acme/pulse", "2026-07-01T00:00:00Z")
	if err != nil || len(commits) != 2 {
		t.Fatalf("commits = %#v, err=%v", commits, err)
	}
	pullRequests, err := client.ListPullRequests(ctx, "acme/pulse")
	if err != nil || len(pullRequests) != 2 {
		t.Fatalf("pull requests = %#v, err=%v", pullRequests, err)
	}
	files, err := client.ListPullRequestFiles(ctx, "acme/pulse", 7)
	if err != nil || len(files) != 2 {
		t.Fatalf("files = %#v, err=%v", files, err)
	}
	reviews, err := client.ListReviews(ctx, "acme/pulse", 7)
	if err != nil || len(reviews) != 2 {
		t.Fatalf("reviews = %#v, err=%v", reviews, err)
	}
	runs, err := client.ListWorkflowRuns(ctx, "acme/pulse", "head sha")
	if err != nil || len(runs) != 2 {
		t.Fatalf("workflow runs = %#v, err=%v", runs, err)
	}
	if client.http.Timeout != 30*time.Second {
		t.Fatalf("timeout = %s", client.http.Timeout)
	}
}

func TestListRepositoriesDoesNotTruncateAfterOneHundred(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		page := request.URL.Query().Get("page")
		count := 100
		if page == "2" {
			count = 1
		} else {
			response.Header().Set("Link", "</user/repos?page=2>; rel=\"next\"")
		}
		result := make([]Repository, count)
		for index := range result {
			result[index].ID = int64(index + 1)
		}
		_ = json.NewEncoder(response).Encode(result)
	}))
	defer server.Close()
	client, err := NewClientWithBaseURL("secret", time.Second, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.ListRepositories(context.Background())
	if err != nil || len(result) != 101 {
		t.Fatalf("repository count = %d, err=%v", len(result), err)
	}
}

func TestPaginationRejectsCrossOriginWithoutLeakingAuthorization(t *testing.T) {
	var requests atomic.Int32
	var authorization atomic.Value
	other := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		authorization.Store(request.Header.Get("Authorization"))
		_ = json.NewEncoder(response).Encode([]Repository{})
	}))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Link", fmt.Sprintf("<%s/user/repos?page=2>; rel=\"next\"", other.URL))
		_ = json.NewEncoder(response).Encode([]Repository{{ID: 1}})
	}))
	defer server.Close()
	client, err := NewClientWithBaseURL("secret", time.Second, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListRepositories(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cross-origin") {
		t.Fatalf("error = %v", err)
	}
	if requests.Load() != 0 {
		value, _ := authorization.Load().(string)
		t.Fatalf("cross-origin requests = %d, Authorization=%q", requests.Load(), value)
	}
}

func TestClientFollowsSameOriginRedirect(t *testing.T) {
	redirected := false
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/user/repos" {
			http.Redirect(response, request, "/redirected/repos", http.StatusTemporaryRedirect)
			return
		}
		if request.URL.Path != "/redirected/repos" {
			http.NotFound(response, request)
			return
		}
		redirected = true
		if got := request.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		_ = json.NewEncoder(response).Encode([]Repository{{ID: 1}})
	}))
	defer server.Close()
	client, err := NewClientWithBaseURL("secret", time.Second, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	repositories, err := client.ListRepositories(context.Background())
	if err != nil || len(repositories) != 1 || !redirected {
		t.Fatalf("repositories=%v redirected=%v err=%v", repositories, redirected, err)
	}
}

func TestClientRejectsCrossOriginRedirectWithoutLeakingAuthorization(t *testing.T) {
	var requests atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		t.Errorf("cross-origin Authorization = %q", request.Header.Get("Authorization"))
		_ = json.NewEncoder(response).Encode([]Repository{})
	}))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, other.URL+"/redirected/repos", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client, err := NewClientWithBaseURL("secret", time.Second, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListRepositories(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cross-origin") {
		t.Fatalf("error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("cross-origin requests = %d", requests.Load())
	}
}

func TestClientStopsRedirectLoop(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		http.Redirect(response, request, "/user/repos", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client, err := NewClientWithBaseURL("secret", 5*time.Second, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListRepositories(context.Background())
	if err == nil || !strings.Contains(err.Error(), "10 GitHub API redirects") {
		t.Fatalf("requests=%d error=%v", requests.Load(), err)
	}
	if requests.Load() != 10 {
		t.Fatalf("redirect requests=%d", requests.Load())
	}
}

func TestNextLinkResolvesAgainstCurrentPage(t *testing.T) {
	current, _ := url.Parse("https://api.github.test/repos/acme/pulse/pulls?page=1")
	next, err := nextLink(current, []string{"<?page=2>; rel=\"next\""})
	if err != nil || next != "https://api.github.test/repos/acme/pulse/pulls?page=2" {
		t.Fatalf("next = %q, err=%v", next, err)
	}
}

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiVersion        = "2022-11-28"
	DefaultAPIBaseURL = "https://api.github.com"
)

type Client struct {
	token   string
	http    *http.Client
	baseURL *url.URL
}

type APIError struct {
	StatusCode int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("GitHub API returned HTTP %d", e.StatusCode)
}

type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
	HTMLURL   string `json:"html_url"`
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Repository struct {
	ID            int64  `json:"id"`
	NodeID        string `json:"node_id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	PushedAt      string `json:"pushed_at"`
	Owner         User   `json:"owner"`
}

type Commit struct {
	SHA     string `json:"sha"`
	NodeID  string `json:"node_id"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Date string `json:"date"`
			Name string `json:"name"`
		} `json:"author"`
		Committer struct {
			Date string `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
	Author    *User `json:"author"`
	Committer *User `json:"committer"`
}

type PullRequest struct {
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ClosedAt  string `json:"closed_at"`
	MergedAt  string `json:"merged_at"`
	Draft     bool   `json:"draft"`
	User      User   `json:"user"`
	Head      struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	ChangedFiles int `json:"changed_files"`
}

type Review struct {
	ID          int64  `json:"id"`
	NodeID      string `json:"node_id"`
	State       string `json:"state"`
	CommitID    string `json:"commit_id"`
	HTMLURL     string `json:"html_url"`
	SubmittedAt string `json:"submitted_at"`
	User        User   `json:"user"`
}

type PullRequestFile struct {
	Filename         string `json:"filename"`
	PreviousFilename string `json:"previous_filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
}

type WorkflowRun struct {
	ID           int64  `json:"id"`
	NodeID       string `json:"node_id"`
	WorkflowID   int64  `json:"workflow_id"`
	Name         string `json:"name"`
	RunNumber    int    `json:"run_number"`
	RunAttempt   int    `json:"run_attempt"`
	Event        string `json:"event"`
	HeadBranch   string `json:"head_branch"`
	HeadSHA      string `json:"head_sha"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	HTMLURL      string `json:"html_url"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	RunStartedAt string `json:"run_started_at"`
}

func NewClient(token string, timeout time.Duration) *Client {
	client, err := NewClientWithBaseURL(token, timeout, DefaultAPIBaseURL)
	if err != nil {
		panic(err)
	}
	return client
}

func NewClientWithBaseURL(token string, timeout time.Duration, baseURL string) (*Client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return nil, fmt.Errorf("invalid GitHub API base URL")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/") + "/"
	return &Client{
		token: token,
		http: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if !sameOrigin(parsed, request.URL) {
					return fmt.Errorf("refusing cross-origin GitHub API redirect")
				}
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 GitHub API redirects")
				}
				return nil
			},
		},
		baseURL: parsed,
	}, nil
}

func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var result User
	err := c.get(ctx, "/user", &result)
	return result, err
}

func (c *Client) ListRepositories(ctx context.Context) ([]Repository, error) {
	return listAll[Repository](ctx, c, "/user/repos?affiliation=owner,collaborator,organization_member&sort=pushed&direction=desc&per_page=100", "")
}

func (c *Client) GetRepository(ctx context.Context, fullName string) (Repository, error) {
	var result Repository
	err := c.get(ctx, "/repos/"+fullName, &result)
	return result, err
}

func (c *Client) ListCommits(ctx context.Context, fullName, since string) ([]Commit, error) {
	return listAll[Commit](ctx, c, "/repos/"+fullName+"/commits?since="+url.QueryEscape(since)+"&per_page=100", "")
}

func (c *Client) ListPullRequests(ctx context.Context, fullName string) ([]PullRequest, error) {
	return listAll[PullRequest](ctx, c, "/repos/"+fullName+"/pulls?state=all&sort=updated&direction=desc&per_page=100", "")
}

func (c *Client) GetPullRequest(ctx context.Context, fullName string, number int) (PullRequest, error) {
	var result PullRequest
	err := c.get(ctx, fmt.Sprintf("/repos/%s/pulls/%d", fullName, number), &result)
	return result, err
}

func (c *Client) ListReviews(ctx context.Context, fullName string, number int) ([]Review, error) {
	return listAll[Review](ctx, c, fmt.Sprintf("/repos/%s/pulls/%d/reviews?per_page=100", fullName, number), "")
}

func (c *Client) ListPullRequestFiles(ctx context.Context, fullName string, number int) ([]PullRequestFile, error) {
	return listAll[PullRequestFile](ctx, c, fmt.Sprintf("/repos/%s/pulls/%d/files?per_page=100", fullName, number), "")
}

func (c *Client) ListWorkflowRuns(ctx context.Context, fullName, headSHA string) ([]WorkflowRun, error) {
	path := fmt.Sprintf("/repos/%s/actions/runs?head_sha=%s&per_page=100", fullName, url.QueryEscape(headSHA))
	return listAll[WorkflowRun](ctx, c, path, "workflow_runs")
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	response, err := c.doGet(ctx, path)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return json.NewDecoder(response.Body).Decode(result)
}

func (c *Client) doGet(ctx context.Context, path string) (*http.Response, error) {
	target, err := c.resolve(path)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	request.Header.Set("User-Agent", "TeamPulse/0.1.0")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode/100 != 2 {
		response.Body.Close()
		return nil, &APIError{StatusCode: response.StatusCode}
	}
	return response, nil
}

func (c *Client) resolve(path string) (*url.URL, error) {
	reference, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	target := c.baseURL.ResolveReference(reference)
	if !sameOrigin(c.baseURL, target) {
		return nil, fmt.Errorf("refusing cross-origin GitHub API URL")
	}
	return target, nil
}

func listAll[T any](ctx context.Context, client *Client, path, wrapperKey string) ([]T, error) {
	result := []T{}
	next := path
	for next != "" {
		response, err := client.doGet(ctx, next)
		if err != nil {
			return nil, err
		}
		page, err := decodeListPage[T](response.Body, wrapperKey)
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		result = append(result, page...)
		next, err = nextLink(response.Request.URL, response.Header.Values("Link"))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func decodeListPage[T any](reader io.Reader, wrapperKey string) ([]T, error) {
	if wrapperKey == "" {
		var result []T
		return result, json.NewDecoder(reader).Decode(&result)
	}
	var response map[string]json.RawMessage
	if err := json.NewDecoder(reader).Decode(&response); err != nil {
		return nil, err
	}
	var result []T
	if err := json.Unmarshal(response[wrapperKey], &result); err != nil {
		return nil, err
	}
	return result, nil
}

func nextLink(current *url.URL, headers []string) (string, error) {
	for _, header := range headers {
		for _, part := range strings.Split(header, ",") {
			sections := strings.Split(part, ";")
			if len(sections) < 2 {
				continue
			}
			target := strings.TrimSpace(sections[0])
			if !strings.HasPrefix(target, "<") || !strings.HasSuffix(target, ">") {
				continue
			}
			isNext := false
			for _, section := range sections[1:] {
				name, value, ok := strings.Cut(strings.TrimSpace(section), "=")
				if ok && strings.EqualFold(name, "rel") && strings.Trim(value, `"`) == "next" {
					isNext = true
					break
				}
			}
			if !isNext {
				continue
			}
			reference, err := url.Parse(strings.TrimSuffix(strings.TrimPrefix(target, "<"), ">"))
			if err != nil {
				return "", err
			}
			return current.ResolveReference(reference).String(), nil
		}
	}
	return "", nil
}

func sameOrigin(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}

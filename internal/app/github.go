package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/italic-jinxin/team-pulse/internal/credentials"
	appdb "github.com/italic-jinxin/team-pulse/internal/database"
	githubclient "github.com/italic-jinxin/team-pulse/internal/github"
)

type syncResourceError struct {
	Resource string
	Err      error
}

func (e *syncResourceError) Error() string { return e.Err.Error() }
func (e *syncResourceError) Unwrap() error { return e.Err }

func resourceError(resource string, err error) error {
	return &syncResourceError{Resource: resource, Err: err}
}

func (a *App) githubClient(ctx context.Context) (*githubclient.Client, error) {
	credential, err := a.Credentials.Get(ctx)
	if errors.Is(err, credentials.ErrNotFound) {
		return nil, fmt.Errorf("connect GitHub with a fine-grained PAT")
	}
	if err != nil {
		return nil, err
	}
	return githubclient.NewClientWithBaseURL(credential.Token, 30*time.Second, a.GitHubBaseURL)
}

func (a *App) refreshRepositoryCatalog(ctx context.Context) error {
	client, err := a.githubClient(ctx)
	if err != nil {
		return err
	}
	credential, err := a.Credentials.Get(ctx)
	if err != nil {
		return err
	}
	account, err := a.Repository.AccountByLogin(ctx, credential.Login)
	if err != nil {
		return err
	}
	repositories, err := client.ListRepositories(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, repository := range repositories {
		if _, err := a.Repository.UpsertRepository(ctx, account.ID, appdb.RepositoryFact{
			GitHubID: repository.ID, NodeID: repository.NodeID,
			OwnerLogin: repository.Owner.Login, Name: repository.Name, FullName: repository.FullName,
			Description: repository.Description, Private: repository.Private,
			DefaultBranch: repository.DefaultBranch, HTMLURL: repository.HTMLURL,
			GitHubCreatedAt: repository.CreatedAt, GitHubUpdatedAt: repository.UpdatedAt,
			PushedAt: repository.PushedAt, SyncedAt: now,
		}); err != nil {
			return err
		}
	}
	return a.Repository.EnsureDefaultRepositorySelection(ctx, account.ID, 5)
}

func (a *App) startSync(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RepositoryIDs []int64 `json:"repository_ids"`
	}
	if err := decode(r, &input); err != nil {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid sync request", nil)
		return
	}
	targets, err := a.Repository.SyncTargets(r.Context(), input.RepositoryIDs)
	if err != nil {
		respondAPIError(w, r, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error(), nil)
		return
	}
	id := nowID("sync")
	if err := a.Repository.CreateSyncJob(r.Context(), id, targets); err != nil {
		respondAPIError(w, r, http.StatusInternalServerError, "DATABASE_ERROR", "Unable to create sync job", nil)
		return
	}
	go a.sync(id, targets)
	respond(w, http.StatusAccepted, map[string]string{"job_id": id, "status": "pending"})
}

func (a *App) sync(jobID string, targets []appdb.SyncTarget) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	_ = a.Repository.UpdateJob(ctx, jobID, "running", "connect", "Connecting to GitHub", 0, 0, "", "")
	client, err := a.githubClient(ctx)
	if err != nil {
		a.failJob(ctx, jobID, err)
		return
	}
	a.syncWithClient(ctx, jobID, targets, client)
}

func (a *App) syncWithClient(ctx context.Context, jobID string, targets []appdb.SyncTarget, client *githubclient.Client) {
	since := time.Now().AddDate(0, 0, -30).UTC().Format(time.RFC3339)
	failures := 0
	for index, target := range targets {
		_ = a.Repository.UpdateJobRepository(ctx, jobID, target.ID, "running")
		progress := index * 90 / len(targets)
		_ = a.Repository.UpdateJob(ctx, jobID, "running", "repository", "Syncing "+target.FullName, progress, index, "", "")
		if err := a.syncRepo(ctx, client, target, since); err != nil {
			failures++
			resource := "repository"
			var typed *syncResourceError
			if errors.As(err, &typed) {
				resource = typed.Resource
			}
			_ = a.Repository.UpdateJobRepository(ctx, jobID, target.ID, "failed")
			_ = a.Repository.AddJobError(ctx, jobID, &target.ID, resource, "GITHUB_SYNC_FAILED", err.Error())
			_ = a.Repository.MarkRepositorySync(ctx, target.ID, "failed", "GITHUB_SYNC_FAILED", err.Error())
			continue
		}
		_ = a.Repository.UpdateJobRepository(ctx, jobID, target.ID, "completed")
		_ = a.Repository.MarkRepositorySync(ctx, target.ID, "completed", "", "")
	}
	_ = a.Repository.UpdateJob(ctx, jobID, "running", "risk_scan", "Scanning risk signals", 95, len(targets), "", "")
	if err := a.scanRisks(ctx); err != nil {
		a.failJob(ctx, jobID, err)
		return
	}
	if failures > 0 {
		_ = a.Repository.UpdateJob(ctx, jobID, "partial", "complete", "Sync completed with errors", 100, len(targets)-failures, "SYNC_PARTIAL", fmt.Sprintf("%d repositories failed", failures))
		return
	}
	_ = a.Repository.UpdateJob(ctx, jobID, "completed", "complete", "Sync complete", 100, len(targets), "", "")
}

func (a *App) failJob(ctx context.Context, id string, err error) {
	_ = a.Repository.UpdateJob(ctx, id, "failed", "failed", "Sync failed", 100, 0, "GITHUB_SYNC_FAILED", err.Error())
}

func (a *App) syncRepo(ctx context.Context, client *githubclient.Client, target appdb.SyncTarget, since string) error {
	credential, err := a.Credentials.Get(ctx)
	if err != nil {
		return resourceError("repository", err)
	}
	account, err := a.Repository.AccountByLogin(ctx, credential.Login)
	if err != nil {
		return resourceError("repository", err)
	}
	metadata, err := client.GetRepository(ctx, target.FullName)
	if err != nil {
		return resourceError("repository", fmt.Errorf("repository metadata: %w", err))
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	repositoryID, err := a.Repository.UpsertRepository(ctx, account.ID, appdb.RepositoryFact{
		GitHubID: metadata.ID, NodeID: metadata.NodeID, OwnerLogin: metadata.Owner.Login,
		Name: metadata.Name, FullName: metadata.FullName, Description: metadata.Description,
		Private: metadata.Private, DefaultBranch: metadata.DefaultBranch, HTMLURL: metadata.HTMLURL,
		GitHubCreatedAt: metadata.CreatedAt, GitHubUpdatedAt: metadata.UpdatedAt,
		PushedAt: metadata.PushedAt, SyncedAt: now,
	})
	if err != nil {
		return resourceError("repository", fmt.Errorf("save repository metadata: %w", err))
	}

	commits, err := client.ListCommits(ctx, target.FullName, since)
	if err != nil {
		return resourceError("commits", fmt.Errorf("commits: %w", err))
	}
	for _, commit := range commits {
		authorLogin := commit.Commit.Author.Name
		var authorID *int64
		if commit.Author != nil {
			authorLogin = commit.Author.Login
			authorID, err = a.Repository.UpsertMember(ctx, memberFact(*commit.Author, now))
			if err != nil {
				return resourceError("commits", fmt.Errorf("save commit author: %w", err))
			}
		}
		var committerID *int64
		if commit.Committer != nil {
			committerID, err = a.Repository.UpsertMember(ctx, memberFact(*commit.Committer, now))
			if err != nil {
				return resourceError("commits", fmt.Errorf("save commit committer: %w", err))
			}
		}
		if err := a.Repository.UpsertCommit(ctx, appdb.CommitFact{
			RepositoryID: repositoryID, SHA: commit.SHA, NodeID: commit.NodeID,
			AuthorMemberID: authorID, AuthorLogin: authorLogin, AuthorName: commit.Commit.Author.Name,
			CommitterMemberID: committerID, Message: commit.Commit.Message, HTMLURL: commit.HTMLURL,
			AuthoredAt: commit.Commit.Author.Date, CommittedAt: commit.Commit.Committer.Date, SyncedAt: now,
		}); err != nil {
			return resourceError("commits", fmt.Errorf("save commit: %w", err))
		}
		title := strings.Split(commit.Commit.Message, "\n")[0]
		if err := a.Repository.UpsertActivity(ctx, appdb.ActivityFact{
			RepositoryID: repositoryID, ActorMemberID: authorID, ActorLogin: authorLogin,
			EventType: "commit.pushed", SourceType: "commit", SourceID: commit.SHA,
			Title: title, HTMLURL: commit.HTMLURL, OccurredAt: commit.Commit.Author.Date, SyncedAt: now,
		}); err != nil {
			return resourceError("commits", fmt.Errorf("save commit activity: %w", err))
		}
	}

	pullRequests, err := client.ListPullRequests(ctx, target.FullName)
	if err != nil {
		return resourceError("pull_requests", fmt.Errorf("pull requests: %w", err))
	}
	for _, summary := range pullRequests {
		if summary.UpdatedAt < since {
			continue
		}
		detail, err := client.GetPullRequest(ctx, target.FullName, summary.Number)
		if err != nil {
			return resourceError("pull_requests", fmt.Errorf("pull request #%d: %w", summary.Number, err))
		}
		authorID, err := a.Repository.UpsertMember(ctx, memberFact(detail.User, now))
		if err != nil {
			return resourceError("pull_requests", fmt.Errorf("save pull request author: %w", err))
		}
		pullRequestID, err := a.Repository.UpsertPullRequest(ctx, appdb.PullRequestFact{
			RepositoryID: repositoryID, GitHubID: detail.ID, NodeID: detail.NodeID,
			Number: detail.Number, AuthorMemberID: authorID, AuthorLogin: detail.User.Login,
			Title: detail.Title, Body: detail.Body, HTMLURL: detail.HTMLURL,
			State: detail.State, Draft: detail.Draft, HeadRef: detail.Head.Ref, HeadSHA: detail.Head.SHA,
			BaseRef: detail.Base.Ref, BaseSHA: detail.Base.SHA, Additions: detail.Additions,
			Deletions: detail.Deletions, ChangedFiles: detail.ChangedFiles, FilesComplete: false,
			LastAuthorActivityAt: detail.UpdatedAt, LastActivityAt: detail.UpdatedAt,
			GitHubCreatedAt: detail.CreatedAt, GitHubUpdatedAt: detail.UpdatedAt,
			ClosedAt: detail.ClosedAt, MergedAt: detail.MergedAt, SyncedAt: now,
		})
		if err != nil {
			return resourceError("pull_requests", fmt.Errorf("save pull request: %w", err))
		}
		files, err := client.ListPullRequestFiles(ctx, target.FullName, detail.Number)
		if err != nil {
			return resourceError("pull_request_files", fmt.Errorf("pull request files for #%d: %w", detail.Number, err))
		}
		fileFacts := make([]appdb.PullRequestFileFact, 0, len(files))
		for _, file := range files {
			fact := classifyPullRequestFile(file.Filename)
			fact.PreviousFilename = file.PreviousFilename
			fact.Status = file.Status
			fact.Additions = file.Additions
			fact.Deletions = file.Deletions
			fact.Changes = file.Changes
			fileFacts = append(fileFacts, fact)
		}
		filesComplete := len(files) == detail.ChangedFiles
		if err := a.Repository.ReplacePullRequestFiles(ctx, pullRequestID, fileFacts, filesComplete, now); err != nil {
			return resourceError("pull_request_files", fmt.Errorf("save pull request files for #%d: %w", detail.Number, err))
		}
		if !filesComplete {
			return resourceError("pull_request_files", fmt.Errorf(
				"pull request files for #%d are incomplete: GitHub reported %d changed files but returned %d",
				detail.Number, detail.ChangedFiles, len(files),
			))
		}
		if err := a.syncReviews(ctx, client, target.FullName, repositoryID, pullRequestID, detail, now); err != nil {
			return resourceError("reviews", err)
		}
		if err := a.syncWorkflowRuns(ctx, client, target.FullName, repositoryID, detail.Head.SHA, now); err != nil {
			return resourceError("workflow_runs", err)
		}
		eventType := "pr.updated"
		occurredAt := detail.UpdatedAt
		if detail.MergedAt != "" {
			eventType = "pr.merged"
			occurredAt = detail.MergedAt
		}
		if err := a.Repository.UpsertActivity(ctx, appdb.ActivityFact{
			RepositoryID: repositoryID, ActorMemberID: authorID, ActorLogin: detail.User.Login,
			EventType: eventType, SourceType: "pull_request", SourceID: fmt.Sprint(detail.ID),
			PullRequestID: &pullRequestID, Title: detail.Title, HTMLURL: detail.HTMLURL,
			OccurredAt: occurredAt, SyncedAt: now,
		}); err != nil {
			return resourceError("pull_requests", fmt.Errorf("save pull request activity: %w", err))
		}
	}
	return nil
}

func (a *App) syncReviews(ctx context.Context, client *githubclient.Client, repository string, repositoryID, pullRequestID int64, pullRequest githubclient.PullRequest, syncedAt string) error {
	reviews, err := client.ListReviews(ctx, repository, pullRequest.Number)
	if err != nil {
		return fmt.Errorf("reviews for #%d: %w", pullRequest.Number, err)
	}
	for _, review := range reviews {
		reviewerID, err := a.Repository.UpsertMember(ctx, memberFact(review.User, syncedAt))
		if err != nil {
			return fmt.Errorf("save reviewer: %w", err)
		}
		if err := a.Repository.UpsertReview(ctx, appdb.ReviewFact{
			PullRequestID: pullRequestID, GitHubID: review.ID, NodeID: review.NodeID,
			ReviewerID: reviewerID, ReviewerLogin: review.User.Login, State: review.State,
			CommitSHA: review.CommitID, HTMLURL: review.HTMLURL,
			SubmittedAt: review.SubmittedAt, SyncedAt: syncedAt,
		}); err != nil {
			return fmt.Errorf("save review: %w", err)
		}
		if review.SubmittedAt != "" {
			if err := a.Repository.UpsertActivity(ctx, appdb.ActivityFact{
				RepositoryID: repositoryID, ActorMemberID: reviewerID, ActorLogin: review.User.Login,
				EventType: "pr.reviewed", SourceType: "review", SourceID: fmt.Sprint(review.ID),
				PullRequestID: &pullRequestID, Title: fmt.Sprintf("Reviewed #%d", pullRequest.Number),
				HTMLURL: review.HTMLURL, OccurredAt: review.SubmittedAt,
				MetadataJSON: marshal(map[string]string{"state": review.State}), SyncedAt: syncedAt,
			}); err != nil {
				return fmt.Errorf("save review activity: %w", err)
			}
		}
	}
	return nil
}

func (a *App) syncWorkflowRuns(ctx context.Context, client *githubclient.Client, repository string, repositoryID int64, headSHA, syncedAt string) error {
	runs, err := client.ListWorkflowRuns(ctx, repository, headSHA)
	if err != nil {
		return fmt.Errorf("workflow runs for head %s: %w", headSHA, err)
	}
	for _, run := range runs {
		if run.HeadSHA != headSHA {
			continue
		}
		if err := a.Repository.UpsertWorkflowRun(ctx, appdb.WorkflowRunFact{
			RepositoryID: repositoryID, GitHubID: run.ID, NodeID: run.NodeID,
			WorkflowID: run.WorkflowID, WorkflowName: run.Name, RunNumber: run.RunNumber,
			RunAttempt: run.RunAttempt, Event: run.Event, HeadBranch: run.HeadBranch,
			HeadSHA: run.HeadSHA, Status: run.Status, Conclusion: run.Conclusion,
			HTMLURL: run.HTMLURL, GitHubCreatedAt: run.CreatedAt, GitHubUpdatedAt: run.UpdatedAt,
			RunStartedAt: run.RunStartedAt, CompletedAt: completedAt(run), SyncedAt: syncedAt,
		}); err != nil {
			return fmt.Errorf("save workflow run: %w", err)
		}
	}
	return nil
}

func classifyPullRequestFile(filename string) appdb.PullRequestFileFact {
	normalized := strings.ToLower(strings.TrimSpace(filename))
	base := path.Base(normalized)
	extension := path.Ext(base)
	segments := strings.Split(normalized, "/")
	moduleName := ""
	if len(segments) > 1 {
		moduleName = segments[0]
	}
	languages := map[string]string{
		".c": "C", ".cc": "C++", ".cpp": "C++", ".css": "CSS", ".go": "Go",
		".html": "HTML", ".java": "Java", ".js": "JavaScript", ".jsx": "JavaScript",
		".kt": "Kotlin", ".py": "Python", ".rb": "Ruby", ".rs": "Rust", ".swift": "Swift",
		".sql": "SQL", ".ts": "TypeScript", ".tsx": "TypeScript",
	}
	dependencyFiles := map[string]bool{
		"cargo.lock": true, "cargo.toml": true, "go.mod": true, "go.sum": true,
		"package-lock.json": true, "package.json": true, "pnpm-lock.yaml": true,
		"poetry.lock": true, "requirements.txt": true, "yarn.lock": true,
	}
	configurationExtensions := map[string]bool{
		".ini": true, ".json": true, ".toml": true, ".yaml": true, ".yml": true,
	}
	isTest := strings.Contains(normalized, "/test/") || strings.Contains(normalized, "/tests/") ||
		strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") || strings.HasSuffix(base, "_test.go")
	isDocumentation := strings.HasPrefix(normalized, "docs/") || extension == ".md" || extension == ".mdx" || extension == ".rst"
	isMigration := strings.Contains(normalized, "/migrations/") || strings.HasPrefix(normalized, "migrations/") || strings.Contains(base, "migration")
	return appdb.PullRequestFileFact{
		Filename: filename, Language: languages[extension], ModuleName: moduleName,
		IsTest: isTest, IsDocumentation: isDocumentation,
		IsConfiguration: configurationExtensions[extension] || strings.HasPrefix(base, "."),
		IsDependency:    dependencyFiles[base], IsMigration: isMigration,
	}
}

func memberFact(user githubclient.User, syncedAt string) appdb.MemberFact {
	return appdb.MemberFact{
		GitHubID: user.ID, Login: user.Login, AvatarURL: user.AvatarURL,
		ProfileURL: user.HTMLURL, UserType: user.Type, SyncedAt: syncedAt,
	}
}

func completedAt(run githubclient.WorkflowRun) string {
	if run.Status == "completed" {
		return run.UpdatedAt
	}
	return ""
}

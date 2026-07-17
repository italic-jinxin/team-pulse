package database

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
)

func TestSQLiteRepositoryFactLifecycle(t *testing.T) {
	ctx := context.Background()
	db, _, err := Open(ctx, filepath.Join(t.TempDir(), "teampulse.db"), filepath.Join(t.TempDir(), "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewSQLiteRepository(db)

	account, err := repository.UpsertAccount(ctx, AccountFact{
		GitHubID: 1, Login: "alice", SyncedAt: "2026-07-16T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	repositoryID, err := repository.UpsertRepository(ctx, account.ID, RepositoryFact{
		GitHubID: 2, OwnerLogin: "acme", Name: "pulse", FullName: "acme/pulse",
		HTMLURL: "https://github.com/acme/pulse", SyncedAt: "2026-07-16T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.SetRepositorySelection(ctx, []int64{repositoryID}); err != nil {
		t.Fatal(err)
	}
	targets, err := repository.SyncTargets(ctx, nil)
	if err != nil || len(targets) != 1 {
		t.Fatalf("sync targets = %#v, err=%v", targets, err)
	}

	memberID, err := repository.UpsertMember(ctx, MemberFact{
		GitHubID: 3, Login: "alice", SyncedAt: "2026-07-16T00:00:00Z",
	})
	if err != nil || memberID == nil {
		t.Fatalf("member id = %v, err=%v", memberID, err)
	}
	if err := repository.UpsertCommit(ctx, CommitFact{
		RepositoryID: repositoryID, SHA: "abc", AuthorMemberID: memberID,
		AuthorLogin: "alice", Message: "Add sync", HTMLURL: "https://example/commit",
		AuthoredAt: "2026-07-10T10:00:00Z", SyncedAt: "2026-07-16T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	pullRequestID, err := repository.UpsertPullRequest(ctx, PullRequestFact{
		RepositoryID: repositoryID, GitHubID: 4, Number: 7, AuthorMemberID: memberID,
		AuthorLogin: "alice", Title: "Add facts", HTMLURL: "https://example/pr/7",
		State: "open", HeadSHA: "abc", Additions: 10, Deletions: 2,
		LastAuthorActivityAt: "2026-07-10T10:00:00Z", LastActivityAt: "2026-07-10T10:00:00Z",
		GitHubCreatedAt: "2026-07-09T10:00:00Z", GitHubUpdatedAt: "2026-07-10T10:00:00Z",
		SyncedAt: "2026-07-16T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertReview(ctx, ReviewFact{
		PullRequestID: pullRequestID, GitHubID: 5, ReviewerID: memberID,
		ReviewerLogin: "alice", State: "APPROVED", CommitSHA: "abc",
		SubmittedAt: "2026-07-11T10:00:00Z", SyncedAt: "2026-07-16T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertWorkflowRun(ctx, WorkflowRunFact{
		RepositoryID: repositoryID, GitHubID: 6, WorkflowID: 10, WorkflowName: "CI",
		RunNumber: 1, RunAttempt: 1, HeadSHA: "abc", Status: "completed",
		Conclusion: "success", HTMLURL: "https://example/actions/6",
		GitHubCreatedAt: "2026-07-11T11:00:00Z", GitHubUpdatedAt: "2026-07-11T11:05:00Z",
		SyncedAt: "2026-07-16T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpsertActivity(ctx, ActivityFact{
		RepositoryID: repositoryID, ActorMemberID: memberID, ActorLogin: "alice",
		EventType: "commit.pushed", SourceType: "commit", SourceID: "abc",
		Title: "Add sync", OccurredAt: "2026-07-10T10:00:00Z",
		SyncedAt: "2026-07-16T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	if err := repository.CreateSyncJob(ctx, "job-1", targets); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateJob(ctx, "job-1", "completed", "complete", "Done", 100, 1, "", ""); err != nil {
		t.Fatal(err)
	}

	pullRequests, err := repository.ListPullRequests(ctx, 10)
	if err != nil || len(pullRequests) != 1 || pullRequests[0].ReviewState != "approved" || pullRequests[0].CIState != "passed" {
		t.Fatalf("pull requests = %#v, err=%v", pullRequests, err)
	}
	members, err := repository.ListMembers(ctx)
	if err != nil || len(members) != 1 || members[0].Commits != 1 || members[0].Reviews != 1 {
		t.Fatalf("members = %#v, err=%v", members, err)
	}

	decision := RiskDecision{
		RuleType: "stale_pull_request", RepositoryID: repositoryID,
		PullRequestID: pullRequestID, SubjectID: "4", Severity: "medium",
		ReasonCode: "PR_STALE", Reason: "PR is stale", SuggestedAction: "Review it",
		EvidenceJSON: "{}",
	}
	if err := repository.ReconcileRisks(ctx, []RiskDecision{decision}); err != nil {
		t.Fatal(err)
	}
	risks, err := repository.ListRisks(ctx)
	if err != nil || len(risks) != 1 || risks[0].Status != "open" {
		t.Fatalf("risks = %#v, err=%v", risks, err)
	}
	if err := repository.ReconcileRisks(ctx, nil); err != nil {
		t.Fatal(err)
	}
	risks, err = repository.ListRisks(ctx)
	if err != nil || len(risks) != 0 {
		t.Fatalf("resolved risks = %#v, err=%v", risks, err)
	}
	var resolvedStatus string
	if err := db.QueryRow("SELECT status FROM risk_signals WHERE id=?", riskID("stale_pull_request:"+strconv.FormatInt(repositoryID, 10)+":pull_request:4")).Scan(&resolvedStatus); err != nil || resolvedStatus != "resolved" {
		t.Fatalf("resolved status = %q, err=%v", resolvedStatus, err)
	}

	facts, err := repository.ReportFacts(ctx, []int64{repositoryID}, "2026-07-06T00:00:00Z", "2026-07-13T00:00:00Z")
	if err != nil || len(facts.InProgress) != 1 || facts.ReviewCount != 1 || facts.CISuccessCount != 1 {
		t.Fatalf("report facts = %#v, err=%v", facts, err)
	}
	if err := repository.SaveReport(ctx, NewReport{
		ID: "report-1", SourceAccountID: account.ID, SourceAccountLogin: account.Login,
		Kind: "weekly", Title: "Weekly", PeriodStart: "2026-07-06T00:00:00Z",
		PeriodEnd: "2026-07-13T00:00:00Z", Timezone: "UTC",
		FactsCutoffAt: "2026-07-16T00:00:00Z", RepositoryScope: "selected",
		TemplateVersion: "weekly-v1", Markdown: "# Weekly\n", ContentSHA256: "hash",
		RepositoryIDs: []int64{repositoryID}, CreatedAt: "2026-07-16T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	report, err := repository.GetReport(ctx, "report-1")
	if err != nil || len(report.Repositories) != 1 || report.Repositories[0] != "acme/pulse" {
		t.Fatalf("report = %#v, err=%v", report, err)
	}
}

package reports

import (
	"fmt"
	"strings"
	"time"

	appdb "github.com/italic-jinxin/team-pulse/internal/database"
)

const TemplateVersion = "weekly-v1"

func Period(startRaw, endRaw string, location *time.Location, now time.Time) (time.Time, time.Time, error) {
	if startRaw == "" && endRaw == "" {
		localNow := now.In(location)
		weekday := (int(localNow.Weekday()) + 6) % 7
		thisMonday := time.Date(localNow.Year(), localNow.Month(), localNow.Day()-weekday, 0, 0, 0, 0, location)
		return thisMonday.AddDate(0, 0, -7), thisMonday, nil
	}
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("period_start must be RFC3339")
	}
	end, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("period_end must be RFC3339")
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("period_start must be before period_end")
	}
	return start, end, nil
}

func RenderWeekly(start, end time.Time, timezone string, facts appdb.ReportFacts) string {
	var builder strings.Builder
	fmt.Fprintln(&builder, "# Engineering Weekly Summary")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "_Period: %s to %s (%s)_\n\n", start.Format("2006-01-02"), end.Add(-time.Nanosecond).Format("2006-01-02"), timezone)
	fmt.Fprintln(&builder, "## Summary")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "- %d pull requests completed\n", len(facts.Completed))
	fmt.Fprintf(&builder, "- %d pull requests in progress\n", len(facts.InProgress))
	fmt.Fprintf(&builder, "- %d effective reviews submitted\n", facts.ReviewCount)
	fmt.Fprintf(&builder, "- %d open risk signals at period end\n\n", len(facts.Risks))

	writePullRequestSection(&builder, "Completed", facts.Completed, "No pull requests were merged in this period.")
	writePullRequestSection(&builder, "In Progress", facts.InProgress, "No active pull requests were found for this period.")
	fmt.Fprintln(&builder, "## Reviews")
	fmt.Fprintln(&builder)
	fmt.Fprintf(&builder, "%d approvals or changes-requested reviews were submitted.\n\n", facts.ReviewCount)
	fmt.Fprintln(&builder, "## CI Health")
	fmt.Fprintln(&builder)
	totalCI := facts.CISuccessCount + facts.CIFailureCount
	if totalCI == 0 {
		fmt.Fprintln(&builder, "No completed workflow runs were available for this period.")
	} else {
		fmt.Fprintf(&builder, "%d of %d completed workflow runs succeeded; %d failed or timed out.\n", facts.CISuccessCount, totalCI, facts.CIFailureCount)
	}
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "## Risks")
	fmt.Fprintln(&builder)
	if len(facts.Risks) == 0 {
		fmt.Fprintln(&builder, "No open risk signals were present at period end.")
	} else {
		for _, risk := range facts.Risks {
			label := fmt.Sprintf("%s #%d", risk.Repository, risk.Number)
			if risk.URL != "" {
				label = fmt.Sprintf("[%s](%s)", label, risk.URL)
			}
			fmt.Fprintf(&builder, "- **%s** %s: %s Next: %s.\n", strings.ToUpper(risk.Severity), label, risk.Reason, risk.Action)
		}
	}
	fmt.Fprintln(&builder)
	fmt.Fprintln(&builder, "## Repository Activity")
	fmt.Fprintln(&builder)
	if len(facts.RepositoryActivity) == 0 {
		fmt.Fprintln(&builder, "No repository activity was available.")
	} else {
		for _, activity := range facts.RepositoryActivity {
			fmt.Fprintf(&builder, "- **%s**: %d commits, %d effective reviews\n", activity.Repository, activity.Commits, activity.Reviews)
		}
	}
	return strings.TrimSpace(builder.String()) + "\n"
}

func writePullRequestSection(builder *strings.Builder, title string, pullRequests []appdb.ReportPullRequest, empty string) {
	fmt.Fprintf(builder, "## %s\n\n", title)
	if len(pullRequests) == 0 {
		fmt.Fprintln(builder, empty)
		fmt.Fprintln(builder)
		return
	}
	for _, pullRequest := range pullRequests {
		fmt.Fprintf(builder, "- [**%s #%d**: %s](%s) by %s (+%d/-%d)\n",
			pullRequest.Repository, pullRequest.Number, pullRequest.Title, pullRequest.URL,
			pullRequest.Author, pullRequest.Additions, pullRequest.Deletions)
	}
	fmt.Fprintln(builder)
}

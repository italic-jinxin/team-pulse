package reports

import (
	"strings"
	"testing"
	"time"

	appdb "github.com/italic-jinxin/team-pulse/internal/database"
)

func TestRenderWeeklyHasFixedSectionsAndLinks(t *testing.T) {
	start := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)
	facts := appdb.ReportFacts{Completed: []appdb.ReportPullRequest{{
		Repository: "acme/pulse", Number: 7, Title: "Add facts",
		URL: "https://github.com/acme/pulse/pull/7", Author: "alice",
	}}}
	markdown := RenderWeekly(start, end, "UTC", facts)
	for _, section := range []string{"Summary", "Completed", "In Progress", "Reviews", "CI Health", "Risks", "Repository Activity"} {
		if !strings.Contains(markdown, "## "+section) {
			t.Errorf("missing section %q", section)
		}
	}
	if !strings.Contains(markdown, "https://github.com/acme/pulse/pull/7") {
		t.Fatal("missing GitHub pull request link")
	}
	if again := RenderWeekly(start, end, "UTC", facts); again != markdown {
		t.Fatal("renderer is not deterministic")
	}
}

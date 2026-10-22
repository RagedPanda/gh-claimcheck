package check

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
)

var now = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func daysAgo(days int) time.Time { return now.Add(-time.Duration(days) * 24 * time.Hour) }

// healthy returns facts for an open, unclaimed issue in an active repository.
func healthy() facts.Facts {
	merged := make([]facts.MergedPR, 0, 12)
	for index := 0; index < 12; index++ {
		association := "MEMBER"
		if index%3 == 0 {
			association = "CONTRIBUTOR"
		}
		merged = append(merged, facts.MergedPR{Author: fmt.Sprintf("user%d", index), Association: association, MergedAt: daysAgo(index + 2)})
	}
	return facts.Facts{
		Viewer: "me",
		Now:    now,
		Issue: facts.Issue{
			Repo: "acme/lib", Number: 7, Title: "parse() drops the last line", State: "OPEN",
			CreatedAt: daysAgo(3), Labels: []string{"bug"},
		},
		Repo: facts.Repo{NameWithOwner: "acme/lib", MergedPRs: merged},
	}
}

func TestHealthyIssueIsGo(t *testing.T) {
	report := Run(healthy())
	if report.Verdict != Go {
		t.Fatalf("verdict = %s, want GO; findings: %+v", report.Verdict, report.Findings)
	}
	assertHasTitle(t, report, "4 of the last 12 merged PRs came from 4 outside contributors")
}

func TestBlockers(t *testing.T) {
	cases := []struct {
		name   string
		change func(*facts.Facts)
		title  string
	}{
		{"closed", func(f *facts.Facts) { f.Issue.State, f.Issue.StateReason = "CLOSED", "NOT_PLANNED" }, "Issue is closed as not planned"},
		{"locked", func(f *facts.Facts) { f.Issue.Locked = true }, "Issue is locked"},
		{"archived", func(f *facts.Facts) { f.Repo.Archived = true }, "Repository is archived"},
		{"assigned", func(f *facts.Facts) { f.Issue.Assignees = []string{"alice"} }, "Assigned to @alice"},
		{"linked open PR", func(f *facts.Facts) {
			f.PullRequests = []facts.PullRequest{{Repo: "acme/lib", Number: 9, State: "OPEN", Author: "bob", Title: "fix parse", Source: facts.SourceLinked}}
		}, "An open PR already references it"},
		{"linked PR in the new home", func(f *facts.Facts) {
			f.PullRequests = []facts.PullRequest{{Repo: "acme/monorepo", Number: 4279, State: "OPEN", IsDraft: true, Author: "bob", Title: "fix parse", Source: facts.SourceLinked}}
		}, "An open draft PR already references it"},
		{"linked merged PR", func(f *facts.Facts) {
			f.PullRequests = []facts.PullRequest{{Repo: "acme/lib", Number: 9, State: "MERGED", Author: "bob", Source: facts.SourceLinked}}
		}, "A merged PR already references it"},
		{"fresh claim", func(f *facts.Facts) {
			f.Issue.Comments = []facts.Comment{{Author: "carol", Association: "NONE", Body: "Can I work on this?", CreatedAt: daysAgo(2)}}
		}, "Claimed by @carol"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issueFacts := healthy()
			tc.change(&issueFacts)
			report := Run(issueFacts)
			if report.Verdict != Skip {
				t.Fatalf("verdict = %s, want SKIP; findings: %+v", report.Verdict, report.Findings)
			}
			if report.Summary != tc.title {
				t.Errorf("summary = %q, want %q", report.Summary, tc.title)
			}
		})
	}
}

func TestCautions(t *testing.T) {
	cases := []struct {
		name   string
		change func(*facts.Facts)
		title  string
	}{
		{"old claim", func(f *facts.Facts) {
			f.Issue.Comments = []facts.Comment{{Author: "carol", Body: "I'll take this", CreatedAt: daysAgo(90)}}
		}, "Older claim by @carol"},
		{"title-matched open PR", func(f *facts.Facts) {
			f.PullRequests = []facts.PullRequest{{Repo: "acme/lib", Number: 11, State: "OPEN", Author: "dan", Title: "parse: keep last line", Source: facts.SourceTitleMatch}}
		}, "An open PR looks like the same fix"},
		{"maintainer pushback", func(f *facts.Facts) {
			f.Issue.Comments = []facts.Comment{{Author: "owner", Association: "OWNER", Body: "This is working as intended.", CreatedAt: daysAgo(1)}}
		}, "A maintainer pushed back"},
		{"blocking label", func(f *facts.Facts) { f.Issue.Labels = []string{"needs repro"} }, "Labelled needs repro"},
		{"moved", func(f *facts.Facts) { f.Repo.MovedTo, f.Repo.MovedNotice = "acme/monorepo", "Moved to the monorepo" }, "Project has moved to acme/monorepo"},
		{"insiders only", func(f *facts.Facts) {
			for index := range f.Repo.MergedPRs {
				f.Repo.MergedPRs[index].Association = "OWNER"
			}
		}, "None of the last 12 merged PRs came from outside contributors"},
		{"slow reviews", func(f *facts.Facts) {
			for index := range f.Repo.MergedPRs {
				f.Repo.MergedPRs[index].MergedAt = daysAgo(400 + index)
			}
		}, "The last outside PR was merged 13 months ago"},
		{"AI restricted", func(f *facts.Facts) {
			f.Repo.AIPolicyPath, f.Repo.AIPolicyText = "CONTRIBUTING.md", "We do not accept AI-generated pull requests."
		}, "AI contribution policy restricts AI-generated work (CONTRIBUTING.md)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issueFacts := healthy()
			tc.change(&issueFacts)
			report := Run(issueFacts)
			if report.Verdict != Caution {
				t.Fatalf("verdict = %s, want CAUTION; findings: %+v", report.Verdict, report.Findings)
			}
			if report.Summary != tc.title {
				t.Errorf("summary = %q, want %q", report.Summary, tc.title)
			}
		})
	}
}

func TestViewerOwnWorkDoesNotBlock(t *testing.T) {
	issueFacts := healthy()
	issueFacts.Issue.Assignees = []string{"Me"}
	issueFacts.Issue.Comments = []facts.Comment{{Author: "me", Body: "I'd like to work on this", CreatedAt: daysAgo(1)}}
	issueFacts.PullRequests = []facts.PullRequest{{Repo: "acme/lib", Number: 12, State: "OPEN", Author: "me", Source: facts.SourceLinked}}
	report := Run(issueFacts)
	if report.Verdict != Go {
		t.Fatalf("verdict = %s, want GO; findings: %+v", report.Verdict, report.Findings)
	}
	assertHasTitle(t, report, "Assigned to you")
	assertHasTitle(t, report, "You have claimed it")
	if !report.Yours || report.Summary != "You are already on it" {
		t.Errorf("Yours = %v, summary = %q; want true, %q", report.Yours, report.Summary, "You are already on it")
	}
}

func TestLooseTitleMatchesAreGroupedAsContext(t *testing.T) {
	issueFacts := healthy()
	for number := 20; number < 25; number++ {
		issueFacts.PullRequests = append(issueFacts.PullRequests, facts.PullRequest{
			Repo: "acme/lib", Number: number, State: "OPEN", Author: "dan", Title: "parse: support streams", Source: facts.SourceTitleMatch,
		})
	}
	report := Run(issueFacts)
	if report.Verdict != Go || report.Yours {
		t.Fatalf("verdict = %s, yours = %v; want GO, false; findings: %+v", report.Verdict, report.Yours, report.Findings)
	}
	assertHasTitle(t, report, "5 PRs share keywords with the title")
}

func TestOldUnmergedTitleMatchIsIgnored(t *testing.T) {
	issueFacts := healthy()
	issueFacts.PullRequests = []facts.PullRequest{{Repo: "acme/lib", Number: 3, State: "MERGED", CreatedAt: daysAgo(400), Source: facts.SourceTitleMatch}}
	if report := Run(issueFacts); report.Verdict != Go {
		t.Fatalf("verdict = %s, want GO; findings: %+v", report.Verdict, report.Findings)
	}
}

func TestClosedLinkedPRIsContext(t *testing.T) {
	issueFacts := healthy()
	issueFacts.PullRequests = []facts.PullRequest{{Repo: "acme/lib", Number: 5, State: "CLOSED", Author: "erin", Source: facts.SourceLinked}}
	report := Run(issueFacts)
	if report.Verdict != Go {
		t.Fatalf("verdict = %s, want GO", report.Verdict)
	}
	assertHasTitle(t, report, "An earlier PR was closed without merging")
}

func TestFindingsAreOrderedBySeverity(t *testing.T) {
	issueFacts := healthy()
	issueFacts.Issue.Labels = []string{"needs repro"}
	issueFacts.Issue.Assignees = []string{"alice"}
	report := Run(issueFacts)
	for index := 1; index < len(report.Findings); index++ {
		if levelRank[report.Findings[index-1].Level] > levelRank[report.Findings[index].Level] {
			t.Fatalf("findings out of order: %+v", report.Findings)
		}
	}
}

func assertHasTitle(t *testing.T, report Report, title string) {
	t.Helper()
	for _, finding := range report.Findings {
		if strings.HasPrefix(finding.Title, title) {
			return
		}
	}
	t.Errorf("no finding titled %q in %+v", title, report.Findings)
}

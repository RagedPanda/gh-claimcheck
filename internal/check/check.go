// Package check turns the facts about an issue into findings and a verdict.
// Every function is pure: the same facts always give the same report.
package check

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
	"github.com/RagedPanda/gh-claimcheck/internal/text"
)

// Levels, from most to least serious.
const (
	Block = "block" // someone else already has this, or it cannot be worked on
	Warn  = "warn"  // check before claiming
	Pass  = "pass"  // a good sign
	Info  = "info"  // useful to know
)

// Verdicts.
const (
	Skip    = "SKIP"
	Caution = "CAUTION"
	Go      = "GO"
)

// Finding is one thing worth telling the contributor.
type Finding struct {
	Level  string `json:"level"`
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	URL    string `json:"url,omitempty"`
}

// Report is the result of checking one issue.
type Report struct {
	Verdict  string    `json:"verdict"`
	Summary  string    `json:"summary"`
	Yours    bool      `json:"yours"` // you are assigned, have claimed it, or have an open PR for it
	Findings []Finding `json:"findings"`
}

const (
	staleClaimAge      = 30 * 24 * time.Hour
	staleMergeAge      = 180 * 24 * time.Hour
	oldIssueAge        = 2 * 365 * 24 * time.Hour
	minOutsideMergeSet = 10
)

// Run checks the facts about an issue and returns the report.
func Run(issueFacts facts.Facts) Report {
	findings := concat(
		// Most specific first: within a level, the first finding becomes the summary.
		issueState(issueFacts.Issue),
		pullRequests(issueFacts),
		assignees(issueFacts.Issue, issueFacts.Viewer),
		comments(issueFacts),
		reporterOffer(issueFacts),
		repoState(issueFacts.Repo),
		labels(issueFacts.Issue),
		outsideMerges(issueFacts.Repo.MergedPRs, issueFacts.Now),
		aiPolicy(issueFacts.Repo),
		contributing(issueFacts.Repo),
		issueAge(issueFacts.Issue, issueFacts.Now),
	)
	sortByLevel(findings)
	verdict, yours := verdictOf(findings), viewerInvolved(issueFacts)
	summary := summaryOf(verdict, findings)
	if yours && verdict == Go {
		summary = "You are already on it"
	}
	return Report{Verdict: verdict, Summary: summary, Yours: yours, Findings: findings}
}

// viewerInvolved reports whether the viewer is assigned, has claimed the issue, or has an open PR for it.
func viewerInvolved(issueFacts facts.Facts) bool {
	viewer := issueFacts.Viewer
	if viewer == "" {
		return false
	}
	for _, assignee := range issueFacts.Issue.Assignees {
		if strings.EqualFold(assignee, viewer) {
			return true
		}
	}
	for _, pr := range issueFacts.PullRequests {
		if pr.State == "OPEN" && strings.EqualFold(pr.Author, viewer) {
			return true
		}
	}
	for _, comment := range issueFacts.Issue.Comments {
		if strings.EqualFold(comment.Author, viewer) && text.IsClaim(comment.Body) {
			return true
		}
	}
	return strings.EqualFold(issueFacts.Issue.Author, viewer) && text.BodyClaim(issueFacts.Issue.Body) != ""
}

func issueState(issue facts.Issue) []Finding {
	if issue.State == "CLOSED" {
		reason := strings.ToLower(strings.ReplaceAll(issue.StateReason, "_", " "))
		return []Finding{{Level: Block, Title: "Issue is closed" + suffix(reason, " as ")}}
	}
	if issue.Locked {
		return []Finding{{Level: Block, Title: "Issue is locked", Detail: "Only collaborators can comment on it."}}
	}
	return []Finding{{Level: Pass, Title: "Issue is open"}}
}

func repoState(repo facts.Repo) []Finding {
	var findings []Finding
	if repo.Archived {
		findings = append(findings, Finding{Level: Block, Title: "Repository is archived", Detail: "It is read-only and accepts no pull requests."})
	}
	if repo.MovedNotice != "" {
		title := "README says the project has moved"
		if repo.MovedTo != "" {
			title = "Project has moved to " + repo.MovedTo
		}
		findings = append(findings, Finding{Level: Warn, Title: title, Detail: "Pull requests probably belong in the new home. Its pull requests were searched too. README: " + clip(repo.MovedNotice, 140)})
	}
	return findings
}

func assignees(issue facts.Issue, viewer string) []Finding {
	others := without(issue.Assignees, viewer)
	switch {
	case len(others) > 0:
		return []Finding{{Level: Block, Title: "Assigned to " + mentions(others)}}
	case len(issue.Assignees) > 0:
		return []Finding{{Level: Pass, Title: "Assigned to you"}}
	default:
		return []Finding{{Level: Pass, Title: "Nobody is assigned"}}
	}
}

// closeMatch is the title similarity above which a keyword match is treated as likely the same fix.
const closeMatch = 0.75

func pullRequests(issueFacts facts.Facts) []Finding {
	var findings []Finding
	var loose []facts.PullRequest
	for _, pr := range rankedBySimilarity(issueFacts.PullRequests, issueFacts.Issue.Title) {
		if pr.Source == facts.SourceTitleMatch && !strings.EqualFold(pr.Author, issueFacts.Viewer) && relevantMatch(pr, issueFacts.Issue) &&
			text.Similarity(issueFacts.Issue.Title, pr.Title) < closeMatch {
			loose = append(loose, pr)
			continue
		}
		if finding, ok := pullRequestFinding(pr, issueFacts.Issue, issueFacts.Viewer); ok {
			findings = append(findings, finding)
		}
	}
	if len(loose) > 0 {
		findings = append(findings, looseMatches(loose, issueFacts.Issue))
	}
	if len(findings) == 0 {
		return []Finding{{Level: Pass, Title: "No pull requests found for it", Detail: "Searched linked pull requests and the title's keywords."}}
	}
	return findings
}

// relevantMatch keeps title matches that could still matter: open, or merged after the issue was opened.
func relevantMatch(pr facts.PullRequest, issue facts.Issue) bool {
	return pr.State == "OPEN" || (pr.State == "MERGED" && pr.CreatedAt.After(issue.CreatedAt))
}

func rankedBySimilarity(prs []facts.PullRequest, issueTitle string) []facts.PullRequest {
	ranked := append([]facts.PullRequest{}, prs...)
	sort.SliceStable(ranked, func(left, right int) bool {
		return text.Similarity(issueTitle, ranked[left].Title) > text.Similarity(issueTitle, ranked[right].Title)
	})
	return ranked
}

func looseMatches(prs []facts.PullRequest, issue facts.Issue) Finding {
	refs := make([]string, 0, 3)
	for _, pr := range prs[:min(len(prs), 3)] {
		refs = append(refs, fmt.Sprintf("%s (%s)", prRef(pr, issue.Repo), strings.ToLower(pr.State)))
	}
	more := ""
	if len(prs) > 3 {
		more = fmt.Sprintf(" and %d more", len(prs)-3)
	}
	return Finding{
		Level:  Info,
		Title:  fmt.Sprintf("%d PR%s share%s keywords with the title", len(prs), plural(len(prs)), verbS(len(prs))),
		Detail: strings.Join(refs, ", ") + more + ". Probably different fixes, but skim them.",
	}
}

func pullRequestFinding(pr facts.PullRequest, issue facts.Issue, viewer string) (Finding, bool) {
	label := fmt.Sprintf("%s by @%s: %s", prRef(pr, issue.Repo), pr.Author, clip(pr.Title, 70))
	linked := pr.Source == facts.SourceLinked
	yours := strings.EqualFold(pr.Author, viewer)
	matched := fmt.Sprintf(" (its title matches %.0f%% of the issue title's key words)", 100*text.Similarity(issue.Title, pr.Title))
	switch {
	case pr.State == "OPEN" && yours:
		return Finding{Level: Info, Title: "Your open PR " + label, URL: pr.URL}, true
	case pr.State == "OPEN" && linked:
		return Finding{Level: Block, Title: "An " + openKind(pr) + " already references it", Detail: label, URL: pr.URL}, true
	case pr.State == "MERGED" && linked:
		return Finding{Level: Block, Title: "A merged PR already references it", Detail: label + ". It may already be fixed.", URL: pr.URL}, true
	case pr.State == "OPEN":
		return Finding{Level: Warn, Title: "An " + openKind(pr) + " looks like the same fix", Detail: label + matched, URL: pr.URL}, true
	case pr.State == "MERGED" && pr.CreatedAt.After(issue.CreatedAt):
		return Finding{Level: Warn, Title: "A PR that looks like the same fix was merged since the issue was opened", Detail: label + matched, URL: pr.URL}, true
	case pr.State == "CLOSED" && linked:
		return Finding{Level: Info, Title: "An earlier PR was closed without merging", Detail: label + ". Read its review before you start.", URL: pr.URL}, true
	}
	return Finding{}, false
}

func verbS(count int) string {
	if count == 1 {
		return "s"
	}
	return ""
}

func openKind(pr facts.PullRequest) string {
	if pr.IsDraft {
		return "open draft PR"
	}
	return "open PR"
}

func comments(issueFacts facts.Facts) []Finding {
	var findings []Finding
	claimed := map[string]bool{}
	for _, comment := range issueFacts.Issue.Comments {
		author := comment.Author
		switch {
		case strings.EqualFold(author, issueFacts.Viewer):
			if text.IsClaim(comment.Body) && !claimed[author] {
				claimed[author] = true
				findings = append(findings, Finding{Level: Info, Title: "You have claimed it", Detail: ago(comment.CreatedAt, issueFacts.Now)})
			}
		case isMaintainer(comment.Association) && text.IsDecline(comment.Body):
			findings = append(findings, Finding{Level: Warn, Title: "A maintainer pushed back", Detail: quote(author, comment, issueFacts.Now)})
		case text.IsClaim(comment.Body) && !claimed[author]:
			claimed[author] = true
			findings = append(findings, claimFinding(comment, issueFacts.Now))
		}
	}
	return findings
}

// reporterOffer reports an offer to fix the issue written into the issue itself,
// such as a "yes" to "Are you willing to submit a PR?". A claim comment by the same
// author is already reported by comments, so it takes precedence.
func reporterOffer(issueFacts facts.Facts) []Finding {
	issue := issueFacts.Issue
	line := text.BodyClaim(issue.Body)
	if line == "" || issue.Author == "" {
		return nil
	}
	if strings.EqualFold(issue.Author, issueFacts.Viewer) {
		return []Finding{{Level: Info, Title: "You offered to fix it in the issue", Detail: ago(issue.CreatedAt, issueFacts.Now)}}
	}
	for _, comment := range issue.Comments {
		if strings.EqualFold(comment.Author, issue.Author) && text.IsClaim(comment.Body) {
			return nil
		}
	}
	offer := facts.Comment{Author: issue.Author, Body: line, CreatedAt: issue.CreatedAt}
	return []Finding{{Level: Warn, Title: "The reporter offered to fix it", Detail: quote(issue.Author, offer, issueFacts.Now) + ". Ask whether they still plan to before you start."}}
}

func claimFinding(comment facts.Comment, now time.Time) Finding {
	if now.Sub(comment.CreatedAt) > staleClaimAge {
		return Finding{Level: Warn, Title: "Older claim by @" + comment.Author, Detail: quote(comment.Author, comment, now) + ". Ask whether they are still on it before you start."}
	}
	return Finding{Level: Block, Title: "Claimed by @" + comment.Author, Detail: quote(comment.Author, comment, now)}
}

func labels(issue facts.Issue) []Finding {
	var findings []Finding
	if blocking := text.BlockingLabels(issue.Labels); len(blocking) > 0 {
		findings = append(findings, Finding{Level: Warn, Title: "Labelled " + strings.Join(blocking, ", "), Detail: "Maintainers may not want a PR yet."})
	}
	if welcoming := text.WelcomingLabels(issue.Labels); len(welcoming) > 0 {
		findings = append(findings, Finding{Level: Pass, Title: "Labelled " + strings.Join(welcoming, ", ")})
	}
	return findings
}

func outsideMerges(merged []facts.MergedPR, now time.Time) []Finding {
	humans, outside := splitOutside(merged)
	switch {
	case len(humans) == 0:
		return []Finding{{Level: Warn, Title: "No merged pull requests found", Detail: "The project may not take pull requests."}}
	case len(outside) == 0 && len(humans) >= minOutsideMergeSet:
		return []Finding{{Level: Warn, Title: fmt.Sprintf("None of the last %d merged PRs came from outside contributors", len(humans)), Detail: "Maintainers may merge only their own work."}}
	case len(outside) == 0:
		return []Finding{{Level: Info, Title: fmt.Sprintf("None of the last %d merged PRs came from outside contributors", len(humans))}}
	}
	latest := outside[0].MergedAt
	authors := uniqueAuthors(outside)
	title := fmt.Sprintf("%d of the last %d merged PRs came from %d outside contributor%s", len(outside), len(humans), len(authors), plural(len(authors)))
	if now.Sub(latest) > staleMergeAge {
		return []Finding{{Level: Warn, Title: "The last outside PR was merged " + ago(latest, now), Detail: title + ". Reviews may be slow, or the project may be quiet."}}
	}
	return []Finding{{Level: Pass, Title: title, Detail: "The latest was " + ago(latest, now) + "."}}
}

// splitOutside drops bots and returns all human merges and the outside ones, newest first.
func splitOutside(merged []facts.MergedPR) ([]facts.MergedPR, []facts.MergedPR) {
	var humans, outside []facts.MergedPR
	for _, pr := range merged {
		if pr.IsBot || strings.HasSuffix(pr.Author, "[bot]") {
			continue
		}
		humans = append(humans, pr)
		if !isMaintainer(pr.Association) {
			outside = append(outside, pr)
		}
	}
	sort.SliceStable(outside, func(left, right int) bool { return outside[left].MergedAt.After(outside[right].MergedAt) })
	return humans, outside
}

func aiPolicy(repo facts.Repo) []Finding {
	policy := text.ReadAIPolicy(repo.AIPolicyText)
	where := repo.AIPolicyPath
	switch policy.Stance {
	case text.AIRestricted:
		return []Finding{{Level: Warn, Title: "AI contribution policy restricts AI-generated work (" + where + ")", Detail: policy.Excerpt}}
	case text.AIDisclose:
		return []Finding{{Level: Info, Title: "AI contribution policy asks for disclosure or your own words (" + where + ")", Detail: policy.Excerpt}}
	case text.AIMentioned:
		return []Finding{{Level: Info, Title: "Contributing docs mention AI (" + where + ")", Detail: policy.Excerpt}}
	}
	return nil
}

func contributing(repo facts.Repo) []Finding {
	var findings []Finding
	if repo.ContributingPath != "" {
		detail := "Read it before you open a PR."
		if mentionsCLA(repo.ContributingText) {
			detail = "It mentions a CLA, which you may need to sign."
		}
		findings = append(findings, Finding{Level: Info, Title: "Contributing guide: " + repo.ContributingPath, Detail: detail})
	}
	if repo.PRTemplatePath != "" {
		findings = append(findings, Finding{Level: Info, Title: "PR template: " + repo.PRTemplatePath, Detail: "Fill in every section."})
	}
	return findings
}

func issueAge(issue facts.Issue, now time.Time) []Finding {
	if issue.CreatedAt.IsZero() || now.Sub(issue.CreatedAt) < oldIssueAge {
		return nil
	}
	return []Finding{{Level: Info, Title: "Opened " + ago(issue.CreatedAt, now), Detail: "Check that it still happens on the latest version."}}
}

func verdictOf(findings []Finding) string {
	verdict := Go
	for _, finding := range findings {
		switch finding.Level {
		case Block:
			return Skip
		case Warn:
			verdict = Caution
		}
	}
	return verdict
}

func summaryOf(verdict string, findings []Finding) string {
	switch verdict {
	case Skip:
		return firstTitle(findings, Block)
	case Caution:
		return firstTitle(findings, Warn)
	}
	return "Nobody seems to be on it"
}

var levelRank = map[string]int{Block: 0, Warn: 1, Pass: 2, Info: 3}

func sortByLevel(findings []Finding) {
	sort.SliceStable(findings, func(left, right int) bool {
		return levelRank[findings[left].Level] < levelRank[findings[right].Level]
	})
}

func firstTitle(findings []Finding, level string) string {
	for _, finding := range findings {
		if finding.Level == level {
			return finding.Title
		}
	}
	return ""
}

func isMaintainer(association string) bool {
	return association == "OWNER" || association == "MEMBER" || association == "COLLABORATOR"
}

func mentionsCLA(guide string) bool {
	lower := strings.ToLower(guide)
	return strings.Contains(lower, "contributor license agreement") || strings.Contains(guide, "CLA")
}

func quote(author string, comment facts.Comment, now time.Time) string {
	return fmt.Sprintf("@%s, %s: %q", author, ago(comment.CreatedAt, now), clip(firstLine(comment.Body), 90))
}

func prRef(pr facts.PullRequest, issueRepo string) string {
	if strings.EqualFold(pr.Repo, issueRepo) {
		return fmt.Sprintf("#%d", pr.Number)
	}
	return fmt.Sprintf("%s#%d", pr.Repo, pr.Number)
}

func ago(then, now time.Time) string {
	days := int(now.Sub(then).Hours() / 24)
	switch {
	case days < 1:
		return "today"
	case days == 1:
		return "yesterday"
	case days < 60:
		return fmt.Sprintf("%d days ago", days)
	case days < 730:
		return fmt.Sprintf("%d months ago", days/30)
	}
	return fmt.Sprintf("%d years ago", days/365)
}

func uniqueAuthors(prs []facts.MergedPR) map[string]bool {
	authors := map[string]bool{}
	for _, pr := range prs {
		authors[strings.ToLower(pr.Author)] = true
	}
	return authors
}

func without(logins []string, viewer string) []string {
	var others []string
	for _, login := range logins {
		if !strings.EqualFold(login, viewer) {
			others = append(others, login)
		}
	}
	return others
}

func mentions(logins []string) string {
	prefixed := make([]string, len(logins))
	for index, login := range logins {
		prefixed[index] = "@" + login
	}
	return strings.Join(prefixed, ", ")
}

func firstLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, ">") {
			return trimmed
		}
	}
	return ""
}

func clip(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit-3]) + "..."
}

func suffix(value, separator string) string {
	if value == "" {
		return ""
	}
	return separator + value
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func concat(groups ...[]Finding) []Finding {
	var all []Finding
	for _, group := range groups {
		all = append(all, group...)
	}
	return all
}

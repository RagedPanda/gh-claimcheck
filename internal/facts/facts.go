// Package facts holds the plain data gathered about an issue and its repository.
// It has no behaviour, so the checks that read it stay pure and easy to test.
package facts

import "time"

// Facts is everything claimcheck knows about one issue.
type Facts struct {
	Viewer       string
	Issue        Issue
	Repo         Repo
	PullRequests []PullRequest
	Now          time.Time
}

// Issue is the issue being checked.
type Issue struct {
	Repo          string
	Number        int
	Title         string
	URL           string
	State         string // OPEN or CLOSED
	StateReason   string // COMPLETED, NOT_PLANNED, REOPENED or empty
	Author        string
	CreatedAt     time.Time
	Locked        bool
	Assignees     []string
	Labels        []string
	Comments      []Comment
	TotalComments int
}

// Comment is one comment on the issue.
type Comment struct {
	Author      string
	Association string // OWNER, MEMBER, COLLABORATOR, CONTRIBUTOR, NONE, ...
	Body        string
	CreatedAt   time.Time
}

// Repo describes the repository the issue lives in.
type Repo struct {
	NameWithOwner    string
	Archived         bool
	MovedTo          string // owner/name the README says the project moved to
	MovedNotice      string // the README line that says so
	ContributingPath string
	ContributingText string
	AIPolicyPath     string
	AIPolicyText     string
	PRTemplatePath   string
	MergedPRs        []MergedPR
}

// MergedPR is a recently merged pull request, used to judge whether outside PRs get merged.
type MergedPR struct {
	Author      string
	Association string
	IsBot       bool
	MergedAt    time.Time
}

// How a pull request was found.
const (
	SourceLinked     = "linked"      // linked to the issue or referencing it
	SourceTitleMatch = "title match" // found by searching the issue title's keywords
)

// PullRequest is a pull request that may already address the issue.
type PullRequest struct {
	Repo      string
	Number    int
	Title     string
	URL       string
	State     string // OPEN, CLOSED or MERGED
	IsDraft   bool
	Author    string
	CreatedAt time.Time
	Source    string
}

// Candidate is an issue found by a search, before it is checked.
type Candidate struct {
	Repo      string
	Number    int
	Title     string
	URL       string
	Comments  int
	Stars     int
	Labels    []string
	CreatedAt time.Time
}

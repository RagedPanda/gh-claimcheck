// Package search builds GitHub issue-search queries for finding work to pick up.
package search

import (
	"fmt"
	"strings"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
)

// Options narrows the search. Zero values mean "no filter".
type Options struct {
	Repos          []string
	Orgs           []string
	Languages      []string
	Labels         []string
	GoodFirstIssue bool
	HelpWanted     bool
	Days           int
	MaxComments    int
	Terms          string
}

// Query turns options into a GitHub search query for open, unclaimed issues.
func Query(options Options, now time.Time) string {
	parts := []string{"is:issue", "is:open", "no:assignee", "-linked:pr", "archived:false"}
	parts = append(parts, prefixed("repo:", options.Repos)...)
	parts = append(parts, prefixed("org:", options.Orgs)...)
	parts = append(parts, prefixed("language:", options.Languages)...)
	parts = append(parts, prefixed("label:", labels(options))...)
	if options.MaxComments >= 0 {
		parts = append(parts, fmt.Sprintf("comments:0..%d", options.MaxComments))
	}
	if options.Days > 0 {
		parts = append(parts, "created:>="+now.AddDate(0, 0, -options.Days).Format("2006-01-02"))
	}
	if terms := strings.TrimSpace(options.Terms); terms != "" {
		parts = append(parts, terms)
	}
	return strings.Join(parts, " ")
}

func labels(options Options) []string {
	all := append([]string{}, options.Labels...)
	if options.GoodFirstIssue {
		all = append(all, "good first issue")
	}
	if options.HelpWanted {
		all = append(all, "help wanted")
	}
	return all
}

// prefixed qualifies each value, quoting values that contain spaces.
func prefixed(prefix string, values []string) []string {
	var qualified []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.ContainsAny(value, " \t") {
			value = `"` + strings.ReplaceAll(value, `"`, "") + `"`
		}
		qualified = append(qualified, prefix+value)
	}
	return qualified
}

// HasStars reports whether a candidate's repository has at least minimum stars.
// Issue search cannot filter by stars, so this runs on the results.
func HasStars(minimum int) func(facts.Candidate) bool {
	return func(candidate facts.Candidate) bool { return candidate.Stars >= minimum }
}

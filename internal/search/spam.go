package search

import (
	"strings"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
)

// LooksLikeSpam reports whether an issue title is junk rather than a report: empty,
// or a single word such as "thanks", a bare link or a pasted token. A real report
// describes its problem in more than one word.
func LooksLikeSpam(title string) bool {
	return len(strings.Fields(title)) < 2
}

// Worthwhile reports whether a candidate is worth showing: its repository has at
// least minStars stars and its title does not look like spam.
func Worthwhile(minStars int) func(facts.Candidate) bool {
	hasStars := HasStars(minStars)
	return func(candidate facts.Candidate) bool {
		return hasStars(candidate) && !LooksLikeSpam(candidate.Title)
	}
}

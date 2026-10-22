// Package ref parses the ways a user can name an issue on the command line.
package ref

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Issue names one issue.
type Issue struct {
	Owner  string
	Name   string
	Number int
}

// Repo returns "owner/name".
func (issue Issue) Repo() string { return issue.Owner + "/" + issue.Name }

func (issue Issue) String() string { return fmt.Sprintf("%s#%d", issue.Repo(), issue.Number) }

var (
	issueURL   = regexp.MustCompile(`^(?:https?://)?(?:www\.)?github\.com/([\w.-]+)/([\w.-]+)/(issues|pull|pulls)/(\d+)(?:[/?#].*)?$`)
	fullRef    = regexp.MustCompile(`^([\w.-]+)/([\w.-]+)(?:#|/issues/)(\d+)$`)
	bareNumber = regexp.MustCompile(`^#?(\d+)$`)
	repoName   = regexp.MustCompile(`^([\w.-]+)/([\w.-]+)$`)
)

// Parse reads an issue reference: owner/repo#123, an issue URL, or #123 / 123
// in defaultRepo ("owner/name", may be empty when there is none).
func Parse(input, defaultRepo string) (Issue, error) {
	input = strings.TrimSpace(input)
	if match := issueURL.FindStringSubmatch(input); match != nil {
		if match[3] != "issues" {
			return Issue{}, fmt.Errorf("%s is a pull request; pass an issue instead", input)
		}
		return build(match[1], match[2], match[4])
	}
	if match := fullRef.FindStringSubmatch(input); match != nil {
		return build(match[1], match[2], match[3])
	}
	if match := bareNumber.FindStringSubmatch(input); match != nil {
		return inDefaultRepo(match[1], defaultRepo, input)
	}
	return Issue{}, fmt.Errorf("cannot read %q as an issue; use owner/repo#123 or an issue URL", input)
}

// ParseRepo reads "owner/name".
func ParseRepo(input string) (string, string, error) {
	match := repoName.FindStringSubmatch(strings.TrimSpace(input))
	if match == nil {
		return "", "", fmt.Errorf("cannot read %q as a repository; use owner/name", input)
	}
	return match[1], match[2], nil
}

func inDefaultRepo(number, defaultRepo, input string) (Issue, error) {
	if defaultRepo == "" {
		return Issue{}, fmt.Errorf("%s has no repository; use owner/repo%s, pass --repo, or run inside a clone", input, "#"+number)
	}
	owner, name, err := ParseRepo(defaultRepo)
	if err != nil {
		return Issue{}, err
	}
	return build(owner, name, number)
}

func build(owner, name, number string) (Issue, error) {
	parsed, err := strconv.Atoi(number)
	if err != nil || parsed <= 0 {
		return Issue{}, fmt.Errorf("invalid issue number %q", number)
	}
	return Issue{Owner: owner, Name: strings.TrimSuffix(name, ".git"), Number: parsed}, nil
}

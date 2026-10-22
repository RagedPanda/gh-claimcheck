// Package ghrepo recognises GitHub repositories in the many ways registries and
// manifests write them.
package ghrepo

import (
	"regexp"
	"strings"
)

var gitHubRepo = regexp.MustCompile(`(?i)(?:^|[/@:.])github\.com[/:]([A-Za-z0-9-]+)/([A-Za-z0-9._-]+)`)

// notRepos are GitHub paths that look like owner/repo but are not repositories.
var notRepos = map[string]bool{"sponsors": true, "orgs": true, "users": true, "features": true, "marketplace": true, "apps": true}

// FromURL extracts "owner/repo" from any form of GitHub URL: https, git+https, ssh,
// git@, tree/blob links and "github:owner/repo". It returns "" for anything else.
func FromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if rest, ok := strings.CutPrefix(raw, "github:"); ok {
		raw = "github.com/" + rest
	}
	match := gitHubRepo.FindStringSubmatch(raw)
	if match == nil || notRepos[strings.ToLower(match[1])] {
		return ""
	}
	name := strings.TrimSuffix(match[2], ".git")
	if name == "" || name == "." || name == ".." {
		return ""
	}
	return match[1] + "/" + name
}

// First returns the first candidate that is a GitHub repository.
func First(candidates ...string) string {
	for _, candidate := range candidates {
		if repo := FromURL(candidate); repo != "" {
			return repo
		}
	}
	return ""
}

// Package deps turns a project's dependencies into the repositories to search,
// and ranks the issues found there. It is pure: no files, no network.
package deps

import (
	"fmt"
	"sort"
	"strings"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
	"github.com/RagedPanda/gh-claimcheck/internal/manifest"
)

// Target is a repository to search, and the dependency that brought it in.
type Target struct {
	Repo   string // owner/name
	Via    string // the dependency's name, e.g. "@babel/core"
	Direct bool
}

// Filter says which dependencies to keep.
type Filter struct {
	DirectOnly bool
	NoDev      bool
}

// Keep applies the filter.
func Keep(dependencies []manifest.Dependency, filter Filter) []manifest.Dependency {
	var kept []manifest.Dependency
	for _, dependency := range dependencies {
		if (filter.DirectOnly && !dependency.Direct) || (filter.NoDev && dependency.Dev) {
			continue
		}
		kept = append(kept, dependency)
	}
	return kept
}

// Targets lists each repository once. Many packages can share a repository (a
// monorepo like babel); the label comes from a direct dependency when there is one.
// The order is direct first, then by repository.
func Targets(dependencies []manifest.Dependency, repoByKey map[string]string) []Target {
	byRepo := map[string]Target{}
	for _, dependency := range dependencies {
		repo := repoByKey[dependency.Key()]
		if repo == "" {
			continue
		}
		key := strings.ToLower(repo)
		existing, seen := byRepo[key]
		if seen && (existing.Direct || !dependency.Direct) {
			continue
		}
		byRepo[key] = Target{Repo: repo, Via: dependency.Name, Direct: dependency.Direct}
	}
	targets := make([]Target, 0, len(byRepo))
	for _, target := range byRepo {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(left, right int) bool {
		if targets[left].Direct != targets[right].Direct {
			return targets[left].Direct
		}
		return strings.ToLower(targets[left].Repo) < strings.ToLower(targets[right].Repo)
	})
	return targets
}

// SearchGroups splits the targets into groups of at most size repositories, one
// search each. Direct and transitive dependencies never share a group, so the
// many libraries-of-libraries cannot crowd out issues in the ones you chose.
func SearchGroups(targets []Target, size int) [][]string {
	var direct, transitive []string
	for _, target := range targets {
		if target.Direct {
			direct = append(direct, target.Repo)
		} else {
			transitive = append(transitive, target.Repo)
		}
	}
	return append(Chunk(direct, size), Chunk(transitive, size)...)
}

// Chunk splits values into groups of at most size, so one search query never
// carries too many repo: qualifiers.
func Chunk(values []string, size int) [][]string {
	var chunks [][]string
	for start := 0; start < len(values); start += size {
		chunks = append(chunks, values[start:min(start+size, len(values))])
	}
	return chunks
}

// Found is an issue found in a dependency's repository.
type Found struct {
	Issue  facts.Candidate
	Target Target
}

// Rank merges search results: each issue once, issues in direct dependencies
// first, then newest first, cut to limit. Issues from repositories that are not
// targets are dropped.
func Rank(results [][]facts.Candidate, targets []Target, limit int) []Found {
	targetByRepo := map[string]Target{}
	for _, target := range targets {
		targetByRepo[strings.ToLower(target.Repo)] = target
	}
	seen := map[string]bool{}
	var found []Found
	for _, batch := range results {
		for _, issue := range batch {
			target, known := targetByRepo[strings.ToLower(issue.Repo)]
			id := fmt.Sprintf("%s#%d", strings.ToLower(issue.Repo), issue.Number)
			if !known || seen[id] {
				continue
			}
			seen[id] = true
			found = append(found, Found{Issue: issue, Target: target})
		}
	}
	sort.SliceStable(found, func(left, right int) bool {
		if found[left].Target.Direct != found[right].Target.Direct {
			return found[left].Target.Direct
		}
		return found[left].Issue.CreatedAt.After(found[right].Issue.CreatedAt)
	})
	return found[:min(limit, len(found))]
}

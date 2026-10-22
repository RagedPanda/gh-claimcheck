package manifest

import "github.com/RagedPanda/gh-claimcheck/internal/ghrepo"

// gitDependency is a dependency fetched straight from a git repository. Only
// GitHub repositories can be searched, so other hosts report false.
func gitDependency(url string, direct, dev bool) (Dependency, bool) {
	slug := ghrepo.FromURL(url)
	return Dependency{Name: slug, Ecosystem: GitHub, Direct: direct, Dev: dev}, slug != ""
}

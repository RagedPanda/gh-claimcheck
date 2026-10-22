package registry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/manifest"
	"github.com/RagedPanda/gh-claimcheck/internal/parallel"
)

// concurrency bounds parallel registry requests; registries are public and rate limited.
const concurrency = 8

// maxBody caps registry responses; PyPI documents for old packages can be several megabytes.
const maxBody = 16 << 20

const userAgent = "gh-claimcheck (+https://github.com/RagedPanda/gh-claimcheck)"

// lookup says where an ecosystem's package metadata lives and how to read it.
type lookup struct {
	url        func(name string) string
	candidates func(body []byte) []string
}

var lookups = map[string]lookup{
	manifest.NPM:       {func(name string) string { return "https://registry.npmjs.org/" + escapeSegments(name) + "/latest" }, npmCandidates},
	manifest.PyPI:      {func(name string) string { return "https://pypi.org/pypi/" + url.PathEscape(name) + "/json" }, pypiCandidates},
	manifest.Cargo:     {func(name string) string { return "https://crates.io/api/v1/crates/" + url.PathEscape(name) }, cratesCandidates},
	manifest.RubyGems:  {func(name string) string { return "https://rubygems.org/api/v1/gems/" + url.PathEscape(name) + ".json" }, rubygemsCandidates},
	manifest.Packagist: {func(name string) string { return "https://repo.packagist.org/p2/" + escapeSegments(name) + ".json" }, packagistCandidates},
	manifest.Pub:       {func(name string) string { return "https://pub.dev/api/packages/" + url.PathEscape(name) }, pubCandidates},
	manifest.CocoaPods: {func(name string) string {
		return "https://trunk.cocoapods.org/api/v1/pods/" + url.PathEscape(name) + "/specs/latest"
	}, cocoapodsCandidates},
	manifest.Go: {func(name string) string { return "https://" + name + "?go-get=1" }, goImportCandidates},
}

// depsDevSystems are ecosystems resolved through deps.dev, whose registries do not expose source links simply.
var depsDevSystems = map[string]string{manifest.Maven: "maven", manifest.NuGet: "nuget"}

// errNotFound means the registry answered, but has no such package.
var errNotFound = errors.New("not found")

// Resolver looks packages up in their registries.
type Resolver struct {
	client  *http.Client
	limited *sync.Map // hosts that answered 429; they are not asked again this run
}

// New returns a resolver with a per-request timeout.
func New() Resolver {
	return Resolver{client: &http.Client{Timeout: 15 * time.Second}, limited: &sync.Map{}}
}

// Resolution is the outcome of looking dependencies up.
type Resolution struct {
	Repos       map[string]string // dependency Key() to owner/repo, for those on GitHub
	Looked      map[string]string // answers fetched in this run, to add to the cache ("" = not on GitHub)
	Failed      map[string]bool   // dependency Key()s whose lookup failed, so they were skipped
	RateLimited []string          // registries that asked us to slow down
}

// Resolve maps dependencies to their GitHub repositories, using cached answers
// where they are fresh and asking the registries for the rest.
func (resolver Resolver) Resolve(ctx context.Context, dependencies []manifest.Dependency, cache Cache, now time.Time) Resolution {
	answers := make([]string, len(dependencies))
	errs := make([]error, len(dependencies))
	fetched := make([]bool, len(dependencies))
	parallel.ForEach(len(dependencies), concurrency, func(index int) {
		if repo, ok := cache.Lookup(dependencies[index].Key(), now); ok {
			answers[index] = repo
			return
		}
		fetched[index] = true
		answers[index], errs[index] = resolver.repoFor(ctx, dependencies[index])
	})
	resolution := Resolution{Repos: map[string]string{}, Looked: map[string]string{}, Failed: map[string]bool{}}
	for index, dependency := range dependencies {
		switch {
		case errs[index] != nil:
			resolution.Failed[dependency.Key()] = true
		case fetched[index]:
			resolution.Looked[dependency.Key()] = answers[index]
		}
		if answers[index] != "" {
			resolution.Repos[dependency.Key()] = answers[index]
		}
	}
	resolver.limited.Range(func(host, _ any) bool {
		resolution.RateLimited = append(resolution.RateLimited, host.(string))
		return true
	})
	sort.Strings(resolution.RateLimited)
	return resolution
}

// repoFor returns the dependency's GitHub repository, "" when it has none, or an
// error when the registry could not be asked.
func (resolver Resolver) repoFor(ctx context.Context, dependency manifest.Dependency) (string, error) {
	switch {
	case dependency.Ecosystem == manifest.Swift:
		return GitHubRepo(dependency.Name), nil
	case dependency.Ecosystem == manifest.Go && GoModuleRepo(dependency.Name) != "":
		return GoModuleRepo(dependency.Name), nil
	case depsDevSystems[dependency.Ecosystem] != "":
		return resolver.viaDepsDev(ctx, depsDevSystems[dependency.Ecosystem], dependency.Name)
	}
	source, ok := lookups[dependency.Ecosystem]
	if !ok {
		return "", nil
	}
	body, err := resolver.get(ctx, source.url(dependency.Name))
	if err != nil {
		return "", notFoundIsEmpty(err)
	}
	return FirstGitHubRepo(source.candidates(body)...), nil
}

func (resolver Resolver) viaDepsDev(ctx context.Context, system, name string) (string, error) {
	base := "https://api.deps.dev/v3/systems/" + system + "/packages/" + url.PathEscape(name)
	body, err := resolver.get(ctx, base)
	if err != nil {
		return "", notFoundIsEmpty(err)
	}
	version := depsDevDefaultVersion(body)
	if version == "" {
		return "", nil
	}
	body, err = resolver.get(ctx, base+"/versions/"+url.PathEscape(version))
	if err != nil {
		return "", notFoundIsEmpty(err)
	}
	return FirstGitHubRepo(depsDevCandidates(body)...), nil
}

// notFoundIsEmpty treats a missing package (private, or a typo) as an answer, not a failure.
func notFoundIsEmpty(err error) error {
	if errors.Is(err, errNotFound) {
		return nil
	}
	return err
}

func (resolver Resolver) get(ctx context.Context, address string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	host := request.URL.Host
	if _, limited := resolver.limited.Load(host); limited {
		return nil, fmt.Errorf("%s: rate limited", host)
	}
	request.Header.Set("User-Agent", userAgent) // crates.io rejects requests without one
	request.Header.Set("Accept", "application/json")
	response, err := resolver.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	switch {
	case response.StatusCode == http.StatusTooManyRequests:
		resolver.limited.Store(host, true)
		return nil, fmt.Errorf("%s: rate limited", host)
	case response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusGone:
		return nil, errNotFound
	case response.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%s: %s", address, response.Status)
	}
	return io.ReadAll(io.LimitReader(response.Body, maxBody))
}

// escapeSegments escapes each path segment but keeps "/" for scoped names like @babel/core.
func escapeSegments(name string) string {
	segments := strings.Split(name, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

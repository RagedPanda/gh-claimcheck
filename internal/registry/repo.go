// Package registry maps packages to the GitHub repositories they are developed in,
// using each ecosystem's public registry. Extraction from registry responses is pure;
// fetching lives in resolver.go.
package registry

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// npmCandidates reads a registry.npmjs.org/<name>/latest document.
// "repository" may be a string ("owner/repo", "github:owner/repo", a URL) or an object with "url".
func npmCandidates(body []byte) []string {
	var document struct {
		Repository json.RawMessage
		Homepage   string
		Bugs       json.RawMessage
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	repository := stringOrURL(document.Repository)
	if strings.Count(repository, "/") == 1 && !strings.Contains(repository, ":") {
		repository = "github.com/" + repository // npm shorthand "owner/repo" means GitHub
	}
	return []string{repository, stringOrURL(document.Bugs), document.Homepage}
}

func stringOrURL(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var object struct{ URL string }
	if json.Unmarshal(raw, &object) == nil {
		return object.URL
	}
	return ""
}

var sourceLabel = regexp.MustCompile(`(?i)source|repo|code|github|issue|tracker|bug`)

// pypiCandidates reads a pypi.org/pypi/<name>/json document, preferring source links.
func pypiCandidates(body []byte) []string {
	var document struct {
		Info struct {
			ProjectURLs map[string]string `json:"project_urls"`
			HomePage    string            `json:"home_page"`
		}
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	preferred, others := sourceFirst(document.Info.ProjectURLs)
	return append(append(preferred, document.Info.HomePage), others...)
}

// sourceFirst splits labelled links into those that look like source or issue
// links and the rest, each sorted by label so the choice is stable.
func sourceFirst(links map[string]string) (preferred, others []string) {
	labels := make([]string, 0, len(links))
	for label := range links {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		if sourceLabel.MatchString(label) {
			preferred = append(preferred, links[label])
		} else {
			others = append(others, links[label])
		}
	}
	return preferred, others
}

// cratesCandidates reads a crates.io/api/v1/crates/<name> document.
func cratesCandidates(body []byte) []string {
	var document struct {
		Crate struct{ Repository, Homepage string }
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	return []string{document.Crate.Repository, document.Crate.Homepage}
}

// rubygemsCandidates reads a rubygems.org/api/v1/gems/<name>.json document.
func rubygemsCandidates(body []byte) []string {
	var document struct {
		SourceCodeURI string `json:"source_code_uri"`
		BugTrackerURI string `json:"bug_tracker_uri"`
		HomepageURI   string `json:"homepage_uri"`
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	return []string{document.SourceCodeURI, document.BugTrackerURI, document.HomepageURI}
}

// packagistCandidates reads a repo.packagist.org/p2/<vendor>/<name>.json document.
func packagistCandidates(body []byte) []string {
	var document struct {
		Packages map[string][]struct {
			Source   struct{ URL string }
			Homepage string
			Support  struct{ Source, Issues string }
		}
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	var candidates []string
	for _, versions := range document.Packages {
		if len(versions) > 0 {
			latest := versions[0]
			candidates = append(candidates, latest.Source.URL, latest.Support.Source, latest.Support.Issues, latest.Homepage)
		}
	}
	return candidates
}

// pubCandidates reads a pub.dev/api/packages/<name> document.
func pubCandidates(body []byte) []string {
	var document struct {
		Latest struct {
			Pubspec struct {
				Repository   string
				IssueTracker string `json:"issue_tracker"`
				Homepage     string
			}
		}
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	spec := document.Latest.Pubspec
	return []string{spec.Repository, spec.IssueTracker, spec.Homepage}
}

// cocoapodsCandidates reads a trunk.cocoapods.org/api/v1/pods/<name>/specs/latest podspec.
func cocoapodsCandidates(body []byte) []string {
	var document struct {
		Source   struct{ Git string }
		Homepage string
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	return []string{document.Source.Git, document.Homepage}
}

// jsrCandidates reads a jsr.io/api/scopes/<scope>/packages/<name> document.
func jsrCandidates(body []byte) []string {
	var document struct {
		GitHubRepository *struct{ Owner, Name string } `json:"githubRepository"`
	}
	if json.Unmarshal(body, &document) != nil || document.GitHubRepository == nil {
		return nil
	}
	return []string{"github.com/" + document.GitHubRepository.Owner + "/" + document.GitHubRepository.Name}
}

// condaCandidates reads an api.anaconda.org/package/<channel>/<name> document.
func condaCandidates(body []byte) []string {
	var document struct {
		DevURL       string `json:"dev_url"`
		SourceGitURL string `json:"source_git_url"`
		Home         string
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	return []string{document.DevURL, document.SourceGitURL, document.Home}
}

// hexCandidates reads a hex.pm/api/packages/<name> document's labelled links.
func hexCandidates(body []byte) []string {
	var document struct {
		Meta struct{ Links map[string]string }
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	preferred, others := sourceFirst(document.Meta.Links)
	return append(preferred, others...)
}

// homepageCandidates reads documents whose only source link is "homepage", such
// as a vcpkg port's vcpkg.json.
func homepageCandidates(body []byte) []string {
	var document struct{ Homepage string }
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	return []string{document.Homepage}
}

// clojarsCandidates reads a clojars.org/api/artifacts/<group>/<artifact> document.
func clojarsCandidates(body []byte) []string {
	var document struct {
		SCM      struct{ URL string }
		Homepage string
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	return []string{document.SCM.URL, document.Homepage}
}

var (
	conanFolder   = regexp.MustCompile(`(?m)^\s*folder:\s*["']?([\w.-]+)`)
	conanHomepage = regexp.MustCompile(`(?m)^\s*homepage\s*=\s*["']([^"']+)["']`)
)

// conanRecipeFolder reads the folder of a conan-center-index recipe's newest
// version from its config.yml, which lists versions newest first.
func conanRecipeFolder(body []byte) string {
	if match := conanFolder.FindSubmatch(body); match != nil {
		return string(match[1])
	}
	return ""
}

// conanCandidates reads a recipe's homepage. Its "url" attribute is always
// conan-center-index itself, so it is not a candidate.
func conanCandidates(body []byte) []string {
	if match := conanHomepage.FindSubmatch(body); match != nil {
		return []string{string(match[1])}
	}
	return nil
}

// depsDevDefaultVersion reads the default version from an api.deps.dev package document.
func depsDevDefaultVersion(body []byte) string {
	var document struct {
		Versions []struct {
			VersionKey struct{ Version string }
			IsDefault  bool
		}
	}
	if json.Unmarshal(body, &document) != nil || len(document.Versions) == 0 {
		return ""
	}
	for _, version := range document.Versions {
		if version.IsDefault {
			return version.VersionKey.Version
		}
	}
	return document.Versions[len(document.Versions)-1].VersionKey.Version
}

// depsDevCandidates reads an api.deps.dev version document's source links.
func depsDevCandidates(body []byte) []string {
	var document struct {
		Links []struct {
			Label, URL string
		}
		RelatedProjects []struct {
			ProjectKey   struct{ ID string }
			RelationType string
		}
	}
	if json.Unmarshal(body, &document) != nil {
		return nil
	}
	var candidates []string
	for _, project := range document.RelatedProjects {
		if project.RelationType == "SOURCE_REPO" {
			candidates = append(candidates, project.ProjectKey.ID)
		}
	}
	for _, link := range document.Links {
		if link.Label == "SOURCE_REPO" || link.Label == "ISSUE_TRACKER" {
			candidates = append(candidates, link.URL)
		}
	}
	for _, link := range document.Links {
		candidates = append(candidates, link.URL)
	}
	return candidates
}

// goVanity maps well-known Go module hosts to their GitHub repositories, so most modules
// resolve without a network call.
var goVanity = []struct {
	prefix string
	repo   func(parts []string) string
}{
	{"github.com/", func(parts []string) string { return at(parts, 1) + "/" + at(parts, 2) }},
	{"golang.org/x/", func(parts []string) string { return "golang/" + at(parts, 2) }},
	{"google.golang.org/grpc", func([]string) string { return "grpc/grpc-go" }},
	{"google.golang.org/protobuf", func([]string) string { return "protocolbuffers/protobuf-go" }},
	{"google.golang.org/api", func([]string) string { return "googleapis/google-api-go-client" }},
	{"go.uber.org/", func(parts []string) string { return "uber-go/" + at(parts, 1) }},
	{"k8s.io/", func(parts []string) string { return "kubernetes/" + at(parts, 1) }},
	{"sigs.k8s.io/", func(parts []string) string { return "kubernetes-sigs/" + at(parts, 1) }},
	{"gopkg.in/", gopkgIn},
}

// GoModuleRepo resolves well-known module paths without the network. It returns "" when unsure.
func GoModuleRepo(module string) string {
	parts := strings.Split(module, "/")
	for _, vanity := range goVanity {
		if strings.HasPrefix(module, vanity.prefix) || module == strings.TrimSuffix(vanity.prefix, "/") {
			if repo := vanity.repo(parts); !strings.HasSuffix(repo, "/") && !strings.HasPrefix(repo, "/") {
				return repo
			}
		}
	}
	return ""
}

var gopkgVersion = regexp.MustCompile(`\.v\d+$`)

// gopkgIn maps gopkg.in/yaml.v3 to go-yaml/yaml and gopkg.in/user/pkg.v1 to user/pkg.
func gopkgIn(parts []string) string {
	versionless := func(name string) string { return gopkgVersion.ReplaceAllString(name, "") }
	if len(parts) >= 3 {
		return parts[1] + "/" + versionless(parts[2])
	}
	name := versionless(at(parts, 1))
	return "go-" + name + "/" + name
}

func at(parts []string, index int) string {
	if index < len(parts) {
		return parts[index]
	}
	return ""
}

var goImportMeta = regexp.MustCompile(`<meta\s+name="go-(?:import|source)"\s+content="([^"]+)"`)

// goImportCandidates reads go-import and go-source meta tags from a ?go-get=1 page.
func goImportCandidates(body []byte) []string {
	var candidates []string
	for _, match := range goImportMeta.FindAllStringSubmatch(string(body), -1) {
		candidates = append(candidates, strings.Fields(match[1])...)
	}
	return candidates
}

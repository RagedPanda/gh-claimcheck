package registry

import (
	"testing"

	"github.com/RagedPanda/gh-claimcheck/internal/ghrepo"
)

func TestRegistryCandidates(t *testing.T) {
	cases := []struct {
		name       string
		candidates []string
		want       string
	}{
		{"npm object", npmCandidates([]byte(`{"repository": {"type": "git", "url": "https://github.com/babel/babel.git", "directory": "packages/babel-core"}}`)), "babel/babel"},
		{"npm shorthand", npmCandidates([]byte(`{"repository": "sindresorhus/ora"}`)), "sindresorhus/ora"},
		{"npm bugs fallback", npmCandidates([]byte(`{"bugs": {"url": "https://github.com/acme/x/issues"}, "homepage": "https://x.dev"}`)), "acme/x"},
		{"pypi", pypiCandidates([]byte(`{"info": {"project_urls": {"Documentation": "https://requests.readthedocs.io", "Source": "https://github.com/psf/requests"}, "home_page": null}}`)), "psf/requests"},
		{"crates", cratesCandidates([]byte(`{"crate": {"repository": "https://github.com/serde-rs/serde", "homepage": "https://serde.rs"}}`)), "serde-rs/serde"},
		{"rubygems", rubygemsCandidates([]byte(`{"source_code_uri": "https://github.com/rails/rails/tree/v8.1.4", "homepage_uri": "https://rubyonrails.org"}`)), "rails/rails"},
		{"packagist", packagistCandidates([]byte(`{"packages": {"guzzlehttp/guzzle": [{"source": {"url": "https://github.com/guzzle/guzzle.git"}}]}}`)), "guzzle/guzzle"},
		{"pub", pubCandidates([]byte(`{"latest": {"pubspec": {"repository": "https://github.com/rrousselGit/provider"}}}`)), "rrousselGit/provider"},
		{"cocoapods", cocoapodsCandidates([]byte(`{"source": {"git": "https://github.com/Alamofire/Alamofire.git", "tag": "5.9.1"}}`)), "Alamofire/Alamofire"},
		{"deps.dev", depsDevCandidates([]byte(`{"links": [{"label": "HOMEPAGE", "url": "https://square.github.io/okhttp/"}, {"label": "SOURCE_REPO", "url": "https://github.com/square/okhttp"}], "relatedProjects": [{"projectKey": {"id": "github.com/square/okhttp"}, "relationType": "SOURCE_REPO"}]}`)), "square/okhttp"},
		{"go-import", goImportCandidates([]byte(`<meta name="go-import" content="go.uber.org/zap git https://github.com/uber-go/zap">`)), "uber-go/zap"},
		{"bad json", npmCandidates([]byte(`not json`)), ""},
	}
	for _, tc := range cases {
		if got := ghrepo.First(tc.candidates...); got != tc.want {
			t.Errorf("%s: got %q, want %q (candidates %q)", tc.name, got, tc.want, tc.candidates)
		}
	}
}

func TestDepsDevDefaultVersion(t *testing.T) {
	body := []byte(`{"versions": [{"versionKey": {"version": "1.0"}}, {"versionKey": {"version": "2.0"}, "isDefault": true}, {"versionKey": {"version": "3.0-rc"}}]}`)
	if got := depsDevDefaultVersion(body); got != "2.0" {
		t.Errorf("default version = %q, want 2.0", got)
	}
}

func TestGoModuleRepo(t *testing.T) {
	cases := map[string]string{
		"github.com/cli/go-gh/v2":          "cli/go-gh",
		"golang.org/x/text":                "golang/text",
		"google.golang.org/grpc":           "grpc/grpc-go",
		"go.uber.org/zap":                  "uber-go/zap",
		"k8s.io/client-go":                 "kubernetes/client-go",
		"sigs.k8s.io/yaml":                 "kubernetes-sigs/yaml",
		"gopkg.in/yaml.v3":                 "go-yaml/yaml",
		"gopkg.in/natefinch/lumberjack.v2": "natefinch/lumberjack",
		"example.com/private/mod":          "",
		"github.com/onlyowner":             "",
	}
	for module, want := range cases {
		if got := GoModuleRepo(module); got != want {
			t.Errorf("GoModuleRepo(%q) = %q, want %q", module, got, want)
		}
	}
}

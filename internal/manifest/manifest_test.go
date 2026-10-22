package manifest

import (
	"testing"
	"testing/fstest"
)

func names(dependencies []Dependency) map[string]Dependency {
	byName := map[string]Dependency{}
	for _, dependency := range dependencies {
		byName[dependency.Name] = dependency
	}
	return byName
}

func expect(t *testing.T, file string, content string, want map[string][2]bool, absent ...string) {
	t.Helper()
	got := names(Parse(file, []byte(content)))
	for name, flags := range want {
		dependency, ok := got[name]
		if !ok {
			t.Errorf("%s: missing %q (got %v)", file, name, got)
			continue
		}
		if dependency.Direct != flags[0] || dependency.Dev != flags[1] {
			t.Errorf("%s: %q direct=%v dev=%v, want %v %v", file, name, dependency.Direct, dependency.Dev, flags[0], flags[1])
		}
	}
	for _, name := range absent {
		if _, ok := got[name]; ok {
			t.Errorf("%s: %q should not be listed", file, name)
		}
	}
}

var direct, directDev, transitive = [2]bool{true, false}, [2]bool{true, true}, [2]bool{false, false}

func TestPackageJSON(t *testing.T) {
	expect(t, "web/package.json", `{
		"name": "app",
		"dependencies": {"react": "^18.3.1", "@acme/ui": "workspace:*", "local": "file:../local"},
		"devDependencies": {"@babel/core": "^7.24.0"},
		"peerDependencies": {"vue": "^3"}
	}`, map[string][2]bool{"react": direct, "@babel/core": directDev, "vue": direct}, "@acme/ui", "local", "app")
}

func TestPackageLock(t *testing.T) {
	expect(t, "package-lock.json", `{"lockfileVersion": 3, "packages": {
		"": {"name": "app"},
		"node_modules/react": {"version": "18.3.1"},
		"node_modules/@babel/core": {"version": "7.24.0", "dev": true},
		"node_modules/@babel/core/node_modules/semver": {"version": "6.3.1", "dev": true},
		"node_modules/@acme/ui": {"link": true},
		"packages/ui": {"version": "1.0.0"}
	}}`, map[string][2]bool{"react": transitive, "@babel/core": {false, true}, "semver": {false, true}}, "@acme/ui", "app", "")
}

func TestYarnLock(t *testing.T) {
	expect(t, "yarn.lock", `# yarn lockfile v1

"@babel/core@^7.0.0", "@babel/core@^7.1.0":
  version "7.24.0"

lodash@^4.17.21:
  version "4.17.21"

"string-width-cjs@npm:string-width@^4.2.0":
  version "4.2.3"

__metadata:
  version: 6
`, map[string][2]bool{"@babel/core": transitive, "lodash": transitive, "string-width": transitive}, "string-width-cjs", "__metadata")
}

func TestPnpmLock(t *testing.T) {
	expect(t, "pnpm-lock.yaml", `lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      react:
        specifier: ^18

packages:

  '@babel/core@7.24.0':
    resolution: {integrity: sha512-x}

  /lodash@4.17.21:
    resolution: {integrity: sha512-y}

  react-dom@18.3.1(react@18.3.1):
    resolution: {integrity: sha512-z}

snapshots:
  semver@6.3.1: {}
`, map[string][2]bool{"@babel/core": transitive, "lodash": transitive, "react-dom": transitive}, "semver", "react")
}

func TestGoMod(t *testing.T) {
	expect(t, "go.mod", `module example.com/app

go 1.23

require github.com/spf13/cobra v1.8.0

require (
	github.com/cli/go-gh/v2 v2.12.2
	golang.org/x/text v0.20.0 // indirect
	// a comment
)
`, map[string][2]bool{"github.com/spf13/cobra": direct, "github.com/cli/go-gh/v2": direct, "golang.org/x/text": transitive}, "example.com/app")
}

func TestRequirements(t *testing.T) {
	expect(t, "requirements.txt", `# pinned
requests[socks]==2.32.3
Django>=5.0 ; python_version >= "3.10"
-r base.txt
-e git+https://github.com/acme/thing.git#egg=thing
zope.interface  # trailing comment
`, map[string][2]bool{"requests": direct, "django": direct, "zope-interface": direct}, "thing", "-r")
	expect(t, "requirements-dev.txt", "pytest\n", map[string][2]bool{"pytest": directDev})
}

func TestPyproject(t *testing.T) {
	expect(t, "pyproject.toml", `[project]
name = "app"
keywords = ["cli", "tool"]
dependencies = [
  "httpx>=0.27",
  "Pydantic[email]",
]

[project.optional-dependencies]
docs = ["mkdocs"]

[dependency-groups]
test = ["pytest", {include-group = "lint"}]

[tool.poetry.dependencies]
python = "^3.11"
rich = "^13"

[tool.poetry.group.dev.dependencies]
black = "^24"
`, map[string][2]bool{"httpx": direct, "pydantic": direct, "mkdocs": directDev, "pytest": directDev, "rich": direct, "black": directDev},
		"cli", "tool", "app", "python", "lint")
}

func TestPipfile(t *testing.T) {
	expect(t, "Pipfile", `[packages]
flask = "*"

[dev-packages]
pytest = "*"
`, map[string][2]bool{"flask": direct, "pytest": directDev})
}

func TestCargo(t *testing.T) {
	expect(t, "Cargo.toml", `[package]
name = "app"
version = "0.1.0"

[dependencies]
serde = { version = "1", features = ["derive"] }
tokio = "1"
local = { path = "../local" }

[dependencies.clap]
version = "4"

[dev-dependencies]
insta = "1"

[target.'cfg(windows)'.dependencies]
winapi = "0.3"
`, map[string][2]bool{"serde": direct, "tokio": direct, "clap": direct, "insta": directDev, "winapi": direct}, "local", "app", "version", "features")
	expect(t, "Cargo.lock", `version = 3

[[package]]
name = "app"
version = "0.1.0"

[[package]]
name = "serde"
version = "1.0.200"
source = "registry+https://github.com/rust-lang/crates.io-index"
`, map[string][2]bool{"serde": transitive}, "app")
}

func TestPoetryLock(t *testing.T) {
	expect(t, "poetry.lock", `[[package]]
name = "Charset_Normalizer"
version = "3.3.2"
`, map[string][2]bool{"charset-normalizer": transitive})
}

func TestUnsupported(t *testing.T) {
	if Parse("README.md", []byte("x")) != nil || Supported("Makefile") {
		t.Error("unsupported files should be ignored")
	}
}

func TestMerge(t *testing.T) {
	merged := Merge(
		[]Dependency{{Name: "b", Ecosystem: NPM}, {Name: "a", Ecosystem: NPM, Dev: true}},
		[]Dependency{{Name: "a", Ecosystem: NPM, Direct: true, Dev: false}, {Name: "a", Ecosystem: PyPI}},
	)
	if len(merged) != 3 {
		t.Fatalf("merged = %+v", merged)
	}
	if first := merged[0]; first.Name != "a" || first.Ecosystem != NPM || !first.Direct || first.Dev {
		t.Errorf("first = %+v, want direct non-dev npm:a", first)
	}
}

func TestDiscover(t *testing.T) {
	project := fstest.MapFS{
		"package.json":                    {Data: []byte(`{"dependencies": {"react": "^18"}}`)},
		"package-lock.json":               {Data: []byte(`{"packages": {"node_modules/react": {}, "node_modules/scheduler": {}}}`)},
		"node_modules/react/package.json": {Data: []byte(`{"dependencies": {"loose-envify": "^1"}}`)},
		"services/api/go.mod":             {Data: []byte("module x\n\nrequire github.com/acme/lib v1.0.0\n")},
		"a/b/c/d/e/f/g/package.json":      {Data: []byte(`{"dependencies": {"too-deep": "1"}}`)},
		"docs/README.md":                  {Data: []byte("# docs")},
	}
	scan, err := Discover(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Files) != 3 {
		t.Errorf("files = %v, want the root package files and services/api/go.mod", scan.Files)
	}
	got := names(scan.Dependencies)
	for _, name := range []string{"react", "scheduler", "github.com/acme/lib"} {
		if _, ok := got[name]; !ok {
			t.Errorf("missing %s in %v", name, got)
		}
	}
	for _, name := range []string{"loose-envify", "too-deep"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s should be skipped", name)
		}
	}
	if !got["react"].Direct || got["scheduler"].Direct {
		t.Errorf("react should be direct and scheduler transitive: %+v", scan.Dependencies)
	}
}

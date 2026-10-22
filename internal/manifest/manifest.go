// Package manifest reads dependency manifests and lock files. Parsers are pure:
// they take a file's name and content and return the dependencies it lists.
// Lock files add transitive dependencies (the libraries your libraries use).
package manifest

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

// Ecosystems, named after their package registries.
const (
	NPM       = "npm"
	Go        = "go"
	PyPI      = "pypi"
	Cargo     = "cargo"
	Maven     = "maven" // name is group:artifact
	RubyGems  = "rubygems"
	Packagist = "packagist"
	Pub       = "pub"
	Swift     = "swift" // name is the package's git URL
	CocoaPods = "cocoapods"
	NuGet     = "nuget"
	JSR       = "jsr"
	Conda     = "conda"
	Vcpkg     = "vcpkg"
	Conan     = "conan"
	Clojars   = "clojars" // name is group/artifact
	Hex       = "hex"
	GitHub    = "github" // name is owner/repo: Carthage and git dependencies
)

// Dependency is one package a project uses.
type Dependency struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
	Direct    bool   `json:"direct"` // listed in a manifest, not only pulled in by another package
	Dev       bool   `json:"dev"`    // only needed for development
}

// Key identifies a dependency across files.
func (dependency Dependency) Key() string { return dependency.Ecosystem + ":" + dependency.Name }

type parser func(content []byte) []Dependency

// format pairs a file test with the parser for that kind of file. The test
// gets the slash-separated path; most formats only look at the base name.
type format struct {
	matches func(name string) bool
	parse   parser
}

func byBase(test func(base string) bool, parse parser) format {
	return format{func(name string) bool { return test(path.Base(name)) }, parse}
}

func named(name string, parse parser) format {
	return byBase(func(base string) bool { return base == name }, parse)
}

func suffixed(suffix string, parse parser) format {
	return byBase(func(base string) bool { return strings.HasSuffix(base, suffix) }, parse)
}

func matching(pattern string, parse parser) format {
	return byBase(regexp.MustCompile(pattern).MatchString, parse)
}

// formats lists every supported manifest and lock file. To support a new kind
// of file, write a parser in the ecosystem's file and add it here.
var formats = []format{
	// JavaScript and TypeScript
	named("package.json", packageJSON),
	named("package-lock.json", packageLock),
	named("npm-shrinkwrap.json", packageLock),
	named("yarn.lock", yarnLock),
	named("pnpm-lock.yaml", pnpmLock),
	named("bun.lock", bunLock),
	named("deno.json", denoJSON),
	named("deno.jsonc", denoJSON),
	named("deno.lock", denoLock),
	// Go
	named("go.mod", goMod),
	// Python
	matching(`^requirements[-_.][\w.-]*(dev|test)[\w.-]*\.(txt|in)$`, requirements(true)), // before the general pattern: first match wins
	matching(`^requirements([-_.][\w.-]*)?\.(txt|in)$`, requirements(false)),
	named("pyproject.toml", pyproject),
	named("Pipfile", pipfile),
	named("poetry.lock", tomlPackages(PyPI, false)),
	named("uv.lock", tomlPackages(PyPI, false)),
	named("setup.py", setupPy),
	named("setup.cfg", setupCfg),
	matching(`^environment[\w.-]*\.ya?ml$`, condaEnvironment),
	// Rust
	named("Cargo.toml", cargoToml),
	named("Cargo.lock", tomlPackages(Cargo, true)),
	// Android, Kotlin and Java
	suffixed(".gradle", gradle),
	suffixed(".gradle.kts", gradle),
	suffixed(".versions.toml", versionCatalog),
	named("pom.xml", pomXML),
	// Scala and Clojure
	suffixed(".sbt", sbt),
	named("deps.edn", depsEDN),
	named("project.clj", projectCLJ),
	// C and C++
	named("vcpkg.json", vcpkgJSON),
	named("conanfile.txt", conanfileTxt),
	named("conanfile.py", conanfilePy),
	named("conan.lock", conanLock),
	// Elixir and Erlang
	named("mix.exs", mixExs),
	named("mix.lock", mixLock),
	named("rebar.config", rebarConfig),
	named("rebar.lock", rebarLock),
	// Ruby
	named("Gemfile", gemfile),
	named("gems.rb", gemfile),
	named("Gemfile.lock", gemfileLock),
	named("gems.locked", gemfileLock),
	// PHP
	named("composer.json", composerJSON),
	named("composer.lock", composerLock),
	// Dart and Flutter
	named("pubspec.yaml", pubspecYAML),
	named("pubspec.lock", pubspecLock),
	// Swift and iOS
	named("Package.swift", packageSwift),
	named("Package.resolved", packageResolved),
	named("Podfile.lock", podfileLock),
	named("Cartfile", cartfile(true, false)),
	named("Cartfile.private", cartfile(true, true)),
	named("Cartfile.resolved", cartfile(false, false)),
	// .NET
	suffixed(".csproj", msbuild),
	suffixed(".fsproj", msbuild),
	suffixed(".vbproj", msbuild),
	named("Directory.Packages.props", msbuild),
	named("packages.lock.json", nugetLock),
}

// Supported reports whether a file name is a manifest or lock file this package reads.
func Supported(name string) bool {
	_, ok := parserFor(name)
	return ok
}

func parserFor(name string) (parser, bool) {
	for _, candidate := range formats {
		if candidate.matches(name) {
			return candidate.parse, true
		}
	}
	return nil, false
}

// Parse returns the dependencies listed in one file, or nil if the file is not supported.
func Parse(name string, content []byte) []Dependency {
	if found, ok := parserFor(name); ok {
		return found(content)
	}
	return nil
}

// Merge combines dependencies from several files. A dependency is direct if any
// file lists it directly, and dev only if every file marks it dev. The result is
// sorted: direct before transitive, then by ecosystem and name.
func Merge(lists ...[]Dependency) []Dependency {
	merged := map[string]Dependency{}
	for _, list := range lists {
		for _, dependency := range list {
			existing, seen := merged[dependency.Key()]
			if seen {
				dependency.Direct = dependency.Direct || existing.Direct
				dependency.Dev = dependency.Dev && existing.Dev
			}
			merged[dependency.Key()] = dependency
		}
	}
	all := make([]Dependency, 0, len(merged))
	for _, dependency := range merged {
		all = append(all, dependency)
	}
	sort.Slice(all, func(left, right int) bool {
		if all[left].Direct != all[right].Direct {
			return all[left].Direct
		}
		return all[left].Key() < all[right].Key()
	})
	return all
}

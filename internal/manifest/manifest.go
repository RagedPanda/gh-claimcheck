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

// format pairs a file-name test with the parser for that kind of file.
type format struct {
	matches func(base string) bool
	parse   parser
}

func named(name string, parse parser) format {
	return format{func(base string) bool { return base == name }, parse}
}

func suffixed(suffix string, parse parser) format {
	return format{func(base string) bool { return strings.HasSuffix(base, suffix) }, parse}
}

func matching(pattern string, parse parser) format {
	expression := regexp.MustCompile(pattern)
	return format{expression.MatchString, parse}
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
	// Go
	named("go.mod", goMod),
	// Python
	matching(`^requirements[-_.][\w.-]*(dev|test)[\w.-]*\.(txt|in)$`, requirements(true)), // before the general pattern: first match wins
	matching(`^requirements([-_.][\w.-]*)?\.(txt|in)$`, requirements(false)),
	named("pyproject.toml", pyproject),
	named("Pipfile", pipfile),
	named("poetry.lock", tomlPackages(PyPI, false)),
	named("uv.lock", tomlPackages(PyPI, false)),
	// Rust
	named("Cargo.toml", cargoToml),
	named("Cargo.lock", tomlPackages(Cargo, true)),
	// Android, Kotlin and Java
	suffixed(".gradle", gradle),
	suffixed(".gradle.kts", gradle),
	suffixed(".versions.toml", versionCatalog),
	named("pom.xml", pomXML),
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
	base := path.Base(name)
	for _, candidate := range formats {
		if candidate.matches(base) {
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

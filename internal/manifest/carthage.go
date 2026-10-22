package manifest

import (
	"regexp"
	"strings"
)

var cartfileLine = regexp.MustCompile(`^\s*(github|git)\s+"([^"]+)"`)

// cartfile reads Carthage's Cartfile, Cartfile.private and Cartfile.resolved.
// Binary frameworks have no repository to search, so they are skipped.
func cartfile(direct, dev bool) parser {
	return func(content []byte) []Dependency {
		var found []Dependency
		for _, line := range strings.Split(string(content), "\n") {
			match := cartfileLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			if dependency, ok := carthageDependency(match[1], match[2], direct, dev); ok {
				found = append(found, dependency)
			}
		}
		return found
	}
}

func carthageDependency(kind, source string, direct, dev bool) (Dependency, bool) {
	if kind == "github" && !strings.Contains(source, "://") {
		return Dependency{Name: source, Ecosystem: GitHub, Direct: direct, Dev: dev}, true
	}
	return gitDependency(source, direct, dev)
}

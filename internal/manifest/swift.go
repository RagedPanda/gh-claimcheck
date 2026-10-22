package manifest

import (
	"encoding/json"
	"regexp"
	"strings"
)

var swiftPackageURL = regexp.MustCompile(`\.package\s*\([^)]*?url:\s*"([^"]+)"`)

// packageSwift reads `.package(url: "...")` declarations. Swift packages are named by their git URL.
func packageSwift(content []byte) []Dependency {
	var found []Dependency
	for _, match := range swiftPackageURL.FindAllStringSubmatch(string(content), -1) {
		found = append(found, Dependency{Name: match[1], Ecosystem: Swift, Direct: true})
	}
	return found
}

// packageResolved reads Package.resolved in its version 1 and version 2/3 layouts.
func packageResolved(content []byte) []Dependency {
	type pin struct {
		Location      string
		RepositoryURL string
	}
	var resolved struct {
		Pins   []pin
		Object struct{ Pins []pin }
	}
	if json.Unmarshal(content, &resolved) != nil {
		return nil
	}
	var found []Dependency
	for _, entry := range append(resolved.Pins, resolved.Object.Pins...) {
		if url := entry.Location + entry.RepositoryURL; url != "" {
			found = append(found, Dependency{Name: url, Ecosystem: Swift})
		}
	}
	return found
}

var podEntry = regexp.MustCompile(`^  - "?([\w.+-]+)`)

// podfileLock reads PODS (everything installed) and DEPENDENCIES (what the Podfile asks for).
// Subspecs such as "Firebase/Core" count as their root pod.
func podfileLock(content []byte) []Dependency {
	var found []Dependency
	section := ""
	for _, line := range strings.Split(string(content), "\n") {
		if line != "" && line[0] != ' ' {
			section = strings.TrimSuffix(strings.TrimSpace(line), ":")
			continue
		}
		if section != "PODS" && section != "DEPENDENCIES" {
			continue
		}
		if match := podEntry.FindStringSubmatch(line); match != nil {
			found = append(found, Dependency{Name: match[1], Ecosystem: CocoaPods, Direct: section == "DEPENDENCIES"})
		}
	}
	return found
}

package manifest

import (
	"encoding/json"
	"regexp"
	"strings"
)

var packageReference = regexp.MustCompile(`(?i)<Package(?:Reference|Version)\s+(?:Update|Include)\s*=\s*"([^"]+)"`)

// msbuild reads PackageReference and central PackageVersion items from project files.
func msbuild(content []byte) []Dependency {
	var found []Dependency
	for _, match := range packageReference.FindAllStringSubmatch(string(content), -1) {
		found = append(found, Dependency{Name: match[1], Ecosystem: NuGet, Direct: true})
	}
	return found
}

// nugetLock reads packages.lock.json, which lists every package per target framework.
func nugetLock(content []byte) []Dependency {
	var lock struct {
		Dependencies map[string]map[string]struct{ Type string }
	}
	if json.Unmarshal(content, &lock) != nil {
		return nil
	}
	var found []Dependency
	for _, packages := range lock.Dependencies {
		for name, entry := range packages {
			if entry.Type != "Project" {
				found = append(found, Dependency{Name: name, Ecosystem: NuGet, Direct: strings.EqualFold(entry.Type, "Direct")})
			}
		}
	}
	return found
}

package manifest

import (
	"encoding/json"
	"strings"
)

func composerJSON(content []byte) []Dependency {
	var manifest struct {
		Require    map[string]string
		RequireDev map[string]string `json:"require-dev"`
	}
	if json.Unmarshal(content, &manifest) != nil {
		return nil
	}
	var found []Dependency
	add := func(packages map[string]string, dev bool) {
		for name := range packages {
			if strings.Contains(name, "/") { // skips "php" and "ext-*" platform requirements
				found = append(found, Dependency{Name: strings.ToLower(name), Ecosystem: Packagist, Direct: true, Dev: dev})
			}
		}
	}
	add(manifest.Require, false)
	add(manifest.RequireDev, true)
	return found
}

func composerLock(content []byte) []Dependency {
	var lock struct {
		Packages    []struct{ Name string }
		PackagesDev []struct{ Name string } `json:"packages-dev"`
	}
	if json.Unmarshal(content, &lock) != nil {
		return nil
	}
	var found []Dependency
	for _, entry := range lock.Packages {
		found = append(found, Dependency{Name: strings.ToLower(entry.Name), Ecosystem: Packagist})
	}
	for _, entry := range lock.PackagesDev {
		found = append(found, Dependency{Name: strings.ToLower(entry.Name), Ecosystem: Packagist, Dev: true})
	}
	return found
}

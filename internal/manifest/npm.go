package manifest

import (
	"encoding/json"
	"strings"
)

func packageJSON(content []byte) []Dependency {
	var manifest struct {
		Dependencies         map[string]string
		DevDependencies      map[string]string
		PeerDependencies     map[string]string
		OptionalDependencies map[string]string
	}
	if json.Unmarshal(content, &manifest) != nil {
		return nil
	}
	var found []Dependency
	add := func(versions map[string]string, dev bool) {
		for name, version := range versions {
			if isRegistryVersion(version) {
				found = append(found, Dependency{Name: name, Ecosystem: NPM, Direct: true, Dev: dev})
			}
		}
	}
	add(manifest.Dependencies, false)
	add(manifest.PeerDependencies, false)
	add(manifest.OptionalDependencies, false)
	add(manifest.DevDependencies, true)
	return found
}

// isRegistryVersion drops local and workspace references, which are not published packages.
func isRegistryVersion(version string) bool {
	for _, prefix := range []string{"workspace:", "file:", "link:", "portal:", "github:", "git:", "git+", "http:", "https:"} {
		if strings.HasPrefix(version, prefix) {
			return false
		}
	}
	return true
}

func packageLock(content []byte) []Dependency {
	var lock struct {
		Packages map[string]struct {
			Dev  bool
			Link bool
		}
	}
	if json.Unmarshal(content, &lock) != nil {
		return nil
	}
	var found []Dependency
	for location, entry := range lock.Packages {
		index := strings.LastIndex(location, "node_modules/")
		if index < 0 || entry.Link {
			continue
		}
		found = append(found, Dependency{Name: location[index+len("node_modules/"):], Ecosystem: NPM, Dev: entry.Dev})
	}
	return found
}

// yarnLock reads entry headers such as `lodash@^4.17.21:` or `"@babel/core@^7.0.0", "@babel/core@^7.1.0":`.
func yarnLock(content []byte) []Dependency {
	var found []Dependency
	for _, line := range strings.Split(string(content), "\n") {
		if line == "" || line[0] == ' ' || line[0] == '#' || !strings.HasSuffix(line, ":") {
			continue
		}
		spec := strings.Trim(strings.SplitN(strings.TrimSuffix(line, ":"), ",", 2)[0], `" `)
		if name := nameBeforeVersion(spec); name != "" && !strings.HasPrefix(name, "__") {
			found = append(found, Dependency{Name: name, Ecosystem: NPM})
		}
	}
	return found
}

// pnpmLock reads keys of the top-level `packages:` map, such as `  /lodash@4.17.21:` or `  '@babel/core@7.24.0':`.
func pnpmLock(content []byte) []Dependency {
	var found []Dependency
	inPackages := false
	for _, line := range strings.Split(string(content), "\n") {
		if line != "" && line[0] != ' ' {
			inPackages = strings.TrimSpace(line) == "packages:"
			continue
		}
		if !inPackages || !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") || !strings.HasSuffix(line, ":") {
			continue
		}
		spec := strings.Trim(strings.TrimSuffix(strings.TrimSpace(line), ":"), `'"`)
		spec = strings.TrimPrefix(strings.SplitN(spec, "(", 2)[0], "/")
		if name := nameBeforeVersion(spec); name != "" {
			found = append(found, Dependency{Name: name, Ecosystem: NPM})
		}
	}
	return found
}

// nameBeforeVersion splits "name@version" and "@scope/name@version", and
// resolves yarn aliases such as "alias@npm:real-name@^1".
func nameBeforeVersion(spec string) string {
	if alias := strings.Index(spec, "@npm:"); alias > 0 {
		if real := nameBeforeVersion(spec[alias+len("@npm:"):]); real != "" {
			return real
		}
		spec = spec[:alias] + "@"
	}
	at := strings.LastIndex(spec, "@")
	if at <= 0 {
		return ""
	}
	return spec[:at]
}

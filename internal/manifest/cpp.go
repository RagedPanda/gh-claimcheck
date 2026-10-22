package manifest

import (
	"encoding/json"
	"regexp"
	"strings"
)

// vcpkgJSON reads a vcpkg manifest. A dependency is a port name or an object;
// host dependencies are build tools, so they count as dev.
func vcpkgJSON(content []byte) []Dependency {
	var manifest struct{ Dependencies []json.RawMessage }
	if json.Unmarshal(relaxedJSON(content), &manifest) != nil {
		return nil
	}
	var found []Dependency
	for _, raw := range manifest.Dependencies {
		var port struct {
			Name string
			Host bool
		}
		if json.Unmarshal(raw, &port.Name) != nil && json.Unmarshal(raw, &port) != nil {
			continue
		}
		if port.Name != "" {
			found = append(found, Dependency{Name: port.Name, Ecosystem: Vcpkg, Direct: true, Dev: port.Host})
		}
	}
	return found
}

// conanName takes the package name from a Conan reference such as "zlib/1.3.1@user/channel".
func conanName(reference string) string {
	name, _, _ := strings.Cut(strings.TrimSpace(reference), "/")
	return strings.TrimSpace(name)
}

func conanDependency(reference string, direct, dev bool) Dependency {
	return Dependency{Name: conanName(reference), Ecosystem: Conan, Direct: direct, Dev: dev}
}

var conanBuildSection = regexp.MustCompile(`^(tool|build|test)_requires$`)

// conanfileTxt reads [requires] and, as dev, [tool_requires] and [build_requires].
func conanfileTxt(content []byte) []Dependency {
	var found []Dependency
	section := ""
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
		case strings.HasPrefix(trimmed, "["):
			section = strings.Trim(trimmed, "[] ")
		case section == "requires":
			found = append(found, conanDependency(trimmed, true, false))
		case conanBuildSection.MatchString(section):
			found = append(found, conanDependency(trimmed, true, true))
		}
	}
	return found
}

var (
	conanCall      = regexp.MustCompile(`self\.(requires|tool_requires|build_requires|test_requires)\(\s*["']([^"']+)["']`)
	conanAttribute = regexp.MustCompile(`(?m)^[ \t]*(requires|tool_requires|build_requires|test_requires)[ \t]*=[ \t]*([^\n]+(?:\n[ \t]+["'(\[][^\n]*)*)`)
)

// conanfilePy reads requirements from a conanfile.py recipe: self.requires(...)
// calls and requires = ... attributes. Tool, build and test requirements count as dev.
func conanfilePy(content []byte) []Dependency {
	text := string(content)
	var found []Dependency
	for _, match := range conanCall.FindAllStringSubmatch(text, -1) {
		found = append(found, conanDependency(match[2], true, match[1] != "requires"))
	}
	for _, match := range conanAttribute.FindAllStringSubmatch(text, -1) {
		for _, reference := range quotedStrings(match[2]) {
			found = append(found, conanDependency(reference, true, match[1] != "requires"))
		}
	}
	return found
}

// conanLock reads a Conan 2 lock file, which lists every resolved package.
func conanLock(content []byte) []Dependency {
	var lock struct {
		Requires      []string
		BuildRequires []string `json:"build_requires"`
	}
	if json.Unmarshal(content, &lock) != nil {
		return nil
	}
	var found []Dependency
	for _, reference := range lock.Requires {
		found = append(found, conanDependency(reference, false, false))
	}
	for _, reference := range lock.BuildRequires {
		found = append(found, conanDependency(reference, false, true))
	}
	return found
}

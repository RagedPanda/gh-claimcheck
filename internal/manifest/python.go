package manifest

import (
	"regexp"
	"strings"
)

var pythonName = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]*)`)

// pythonRequirement extracts the package name from a PEP 508 string such as "requests[socks]>=2; python_version>'3'".
func pythonRequirement(requirement string) string {
	name := pythonName.FindString(strings.TrimSpace(requirement))
	return normalizePython(name)
}

// normalizePython applies PEP 503 name normalisation.
func normalizePython(name string) string {
	return strings.ToLower(pythonSeparators.ReplaceAllString(name, "-"))
}

var pythonSeparators = regexp.MustCompile(`[-_.]+`)

func requirements(dev bool) parser {
	return func(content []byte) []Dependency {
		var found []Dependency
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(strings.SplitN(line, " #", 2)[0])
			if line == "" || line[0] == '#' || line[0] == '-' || strings.Contains(line, "://") {
				continue
			}
			if name := pythonRequirement(line); name != "" {
				found = append(found, Dependency{Name: name, Ecosystem: PyPI, Direct: true, Dev: dev})
			}
		}
		return found
	}
}

// pyproject reads PEP 621 dependency arrays and Poetry dependency tables.
func pyproject(content []byte) []Dependency {
	var found []Dependency
	for _, section := range tomlSections(string(content)) {
		dev := strings.Contains(section.name, "dev") || strings.Contains(section.name, "test")
		switch {
		case section.name == "project" || section.name == "project.optional-dependencies" || section.name == "dependency-groups":
			groups := section.keys()
			if section.name == "project" {
				groups = []string{"dependencies"}
			}
			for _, group := range groups {
				for _, requirement := range quotedStrings(withoutInlineTables(section.arrayOf(group))) {
					if name := pythonRequirement(requirement); name != "" {
						found = append(found, Dependency{Name: name, Ecosystem: PyPI, Direct: true, Dev: dev || section.name != "project"})
					}
				}
			}
		case strings.HasPrefix(section.name, "tool.poetry.") && strings.HasSuffix(section.name, "dependencies"):
			for _, key := range section.keys() {
				if key != "python" {
					found = append(found, Dependency{Name: normalizePython(key), Ecosystem: PyPI, Direct: true, Dev: dev})
				}
			}
		}
	}
	return found
}

func pipfile(content []byte) []Dependency {
	var found []Dependency
	for _, section := range tomlSections(string(content)) {
		if section.name != "packages" && section.name != "dev-packages" {
			continue
		}
		for _, key := range section.keys() {
			found = append(found, Dependency{Name: normalizePython(key), Ecosystem: PyPI, Direct: true, Dev: section.name == "dev-packages"})
		}
	}
	return found
}

package manifest

import (
	"strings"
)

func cargoToml(content []byte) []Dependency {
	var found []Dependency
	for _, section := range tomlSections(string(content)) {
		name := section.name
		if strings.HasPrefix(name, "target.") {
			name = name[strings.LastIndex(name, ".")+1:]
		}
		dev := name == "dev-dependencies"
		switch {
		case name == "dependencies" || name == "dev-dependencies" || name == "build-dependencies" || name == "workspace.dependencies":
			for _, key := range section.keys() {
				if !strings.Contains(section.valueOf(key), "path =") {
					found = append(found, Dependency{Name: key, Ecosystem: Cargo, Direct: true, Dev: dev})
				}
			}
		case strings.HasPrefix(name, "dependencies.") || strings.HasPrefix(name, "dev-dependencies."):
			if !strings.Contains(section.body, "path =") {
				found = append(found, Dependency{Name: name[strings.Index(name, ".")+1:], Ecosystem: Cargo, Direct: true, Dev: strings.HasPrefix(name, "dev-")})
			}
		}
	}
	return found
}

// tomlPackages reads `[[package]]` entries from poetry.lock, uv.lock and Cargo.lock.
// When needSource is set, entries without a `source` (local workspace crates) are skipped.
func tomlPackages(ecosystem string, needSource bool) parser {
	return func(content []byte) []Dependency {
		var found []Dependency
		for _, section := range tomlSections(string(content)) {
			if section.name != "[package]" {
				continue
			}
			name := strings.Trim(section.valueOf("name"), `"`)
			if name == "" || (needSource && section.valueOf("source") == "") {
				continue
			}
			if ecosystem == PyPI {
				name = normalizePython(name)
			}
			found = append(found, Dependency{Name: name, Ecosystem: ecosystem})
		}
		return found
	}
}

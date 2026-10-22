package manifest

import (
	"strings"
)

func goMod(content []byte) []Dependency {
	var found []Dependency
	inBlock := false
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "require (":
			inBlock = true
			continue
		case inBlock && trimmed == ")":
			inBlock = false
			continue
		case strings.HasPrefix(trimmed, "require "):
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "require "))
		case !inBlock:
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 && !strings.HasPrefix(fields[0], "//") {
			found = append(found, Dependency{Name: fields[0], Ecosystem: Go, Direct: !strings.Contains(trimmed, "// indirect")})
		}
	}
	return found
}

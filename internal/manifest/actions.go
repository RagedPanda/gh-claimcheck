package manifest

import (
	"regexp"
	"strings"
)

var actionUses = regexp.MustCompile(`(?m)^[\s-]*uses:\s*["']?([^\s"'#]+)`)

// workflow reads the actions a GitHub Actions workflow or composite action uses.
// They run in CI, not in the shipped project, so they count as dev.
func workflow(content []byte) []Dependency {
	var found []Dependency
	for _, match := range actionUses.FindAllStringSubmatch(string(content), -1) {
		if slug, ok := actionRepo(match[1]); ok {
			found = append(found, Dependency{Name: slug, Ecosystem: GitHub, Direct: true, Dev: true})
		}
	}
	return found
}

// actionRepo turns "owner/repo/path@ref" into owner/repo. Local actions (./) and
// Docker images (docker://) are not repositories.
func actionRepo(uses string) (string, bool) {
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "docker://") || strings.Contains(uses, "${{") {
		return "", false
	}
	reference, _, _ := strings.Cut(uses, "@")
	parts := strings.Split(reference, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

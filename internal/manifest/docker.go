package manifest

import (
	"regexp"
	"strings"
)

var (
	dockerFrom   = regexp.MustCompile(`(?im)^\s*FROM\s+(?:--\S+\s+)*(\S+)(?:\s+AS\s+(\S+))?`)
	composeImage = regexp.MustCompile(`(?m)^\s*image:\s*["']?([^\s"'#]+)`)
)

// dockerfile reads the base images a Dockerfile builds on. Later stages that
// build FROM an earlier stage's name are not images.
func dockerfile(content []byte) []Dependency {
	stages := map[string]bool{}
	var found []Dependency
	for _, match := range dockerFrom.FindAllStringSubmatch(string(content), -1) {
		image := match[1]
		if !stages[strings.ToLower(image)] {
			if name, ok := imageName(image); ok {
				found = append(found, Dependency{Name: name, Ecosystem: Docker, Direct: true})
			}
		}
		if match[2] != "" {
			stages[strings.ToLower(match[2])] = true
		}
	}
	return found
}

// compose reads the images a Docker Compose file runs: usually the databases,
// caches and queues a project depends on.
func compose(content []byte) []Dependency {
	var found []Dependency
	for _, match := range composeImage.FindAllStringSubmatch(string(content), -1) {
		if name, ok := imageName(match[1]); ok {
			found = append(found, Dependency{Name: name, Ecosystem: Docker, Direct: true})
		}
	}
	return found
}

var imageTag = regexp.MustCompile(`(@sha256:[0-9a-f]+|:[\w.-]+)$`)

// imageName strips an image reference's tag and digest and Docker Hub's implicit
// prefixes, so "docker.io/library/postgres:16" becomes "postgres".
func imageName(reference string) (string, bool) {
	if reference == "scratch" || strings.Contains(reference, "$") {
		return "", false
	}
	name := strings.ToLower(imageTag.ReplaceAllString(reference, ""))
	for _, prefix := range []string{"docker.io/", "index.docker.io/", "registry-1.docker.io/"} {
		name = strings.TrimPrefix(name, prefix)
	}
	name = strings.TrimPrefix(name, "library/")
	return name, name != ""
}

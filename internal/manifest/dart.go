package manifest

import "strings"

// yamlBlock is a key at some indent and the more-indented lines under it.
type yamlBlock struct {
	key  string
	body string
}

// yamlChildren returns the blocks at exactly `indent` spaces under the top-level key `parent`.
// It is enough for pubspec files, not a general YAML parser.
func yamlChildren(content, parent string, indent int) []yamlBlock {
	var blocks []yamlBlock
	inParent := false
	prefix := strings.Repeat(" ", indent)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if line[0] != ' ' {
			inParent = strings.TrimSuffix(trimmed, ":") == parent && strings.HasSuffix(trimmed, ":")
			continue
		}
		if !inParent {
			continue
		}
		if strings.HasPrefix(line, prefix) && line[indent] != ' ' {
			key, rest, _ := strings.Cut(trimmed, ":")
			blocks = append(blocks, yamlBlock{key: strings.Trim(key, `"'`), body: rest})
			continue
		}
		if len(blocks) > 0 {
			blocks[len(blocks)-1].body += "\n" + trimmed
		}
	}
	return blocks
}

// pubspecYAML reads dependencies and dev_dependencies, skipping SDK, path and git packages.
func pubspecYAML(content []byte) []Dependency {
	var found []Dependency
	for _, section := range []string{"dependencies", "dev_dependencies"} {
		for _, block := range yamlChildren(string(content), section, 2) {
			if strings.Contains(block.body, "sdk:") || strings.Contains(block.body, "path:") || strings.Contains(block.body, "git:") {
				continue
			}
			found = append(found, Dependency{Name: block.key, Ecosystem: Pub, Direct: true, Dev: section == "dev_dependencies"})
		}
	}
	return found
}

// pubspecLock reads hosted packages and whether each is direct.
func pubspecLock(content []byte) []Dependency {
	var found []Dependency
	for _, block := range yamlChildren(string(content), "packages", 2) {
		if !strings.Contains(block.body, "source: hosted") {
			continue
		}
		found = append(found, Dependency{
			Name: block.key, Ecosystem: Pub,
			Direct: strings.Contains(block.body, `dependency: "direct`),
			Dev:    strings.Contains(block.body, `dependency: "direct dev"`),
		})
	}
	return found
}

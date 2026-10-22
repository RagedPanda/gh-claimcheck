package manifest

import (
	"regexp"
	"strings"
)

var (
	gemLine      = regexp.MustCompile(`^\s*gem\s+["']([\w.-]+)["']`)
	gemDevGroup  = regexp.MustCompile(`^\s*group\b.*:(development|test)\b.*\bdo\b`)
	gemLockEntry = regexp.MustCompile(`^    ([\w.-]+) \(`)
)

// gemfile reads `gem "name"` lines; gems inside a development or test group are dev.
func gemfile(content []byte) []Dependency {
	var found []Dependency
	devIndent := -1
	for _, line := range strings.Split(string(content), "\n") {
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		switch {
		case devIndent < 0 && gemDevGroup.MatchString(line):
			devIndent = indent
		case devIndent >= 0 && indent == devIndent && strings.TrimSpace(line) == "end":
			devIndent = -1
		}
		if match := gemLine.FindStringSubmatch(line); match != nil {
			found = append(found, Dependency{Name: match[1], Ecosystem: RubyGems, Direct: true, Dev: devIndent >= 0})
		}
	}
	return found
}

// gemfileLock reads the resolved gems under `specs:`; the 4-space entries are gems,
// deeper ones are their requirements.
func gemfileLock(content []byte) []Dependency {
	var found []Dependency
	inGems := false
	for _, line := range strings.Split(string(content), "\n") {
		if line != "" && line[0] != ' ' {
			inGems = line == "GEM"
			continue
		}
		if match := gemLockEntry.FindStringSubmatch(line); inGems && match != nil {
			found = append(found, Dependency{Name: match[1], Ecosystem: RubyGems})
		}
	}
	return found
}

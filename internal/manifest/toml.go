package manifest

import (
	"regexp"
	"strings"
)

// Minimal TOML reading: enough for dependency tables, not a general parser.

type tomlSection struct {
	name string
	body string
}

var tomlHeader = regexp.MustCompile(`^\[(\[?[^\]]+\]?)\]\s*(#.*)?$`)

func tomlSections(content string) []tomlSection {
	sections := []tomlSection{{}}
	for _, line := range strings.Split(content, "\n") {
		if match := tomlHeader.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			sections = append(sections, tomlSection{name: strings.TrimSpace(match[1])})
			continue
		}
		sections[len(sections)-1].body += line + "\n"
	}
	return sections
}

var tomlKey = regexp.MustCompile(`(?m)^\s*("?[A-Za-z0-9_.-]+"?)\s*=\s*(.*)$`)

// keys returns the top-level keys of a section, skipping lines inside multi-line arrays.
func (section tomlSection) keys() []string {
	var keys []string
	depth := 0
	for _, line := range strings.Split(section.body, "\n") {
		if depth == 0 {
			if match := tomlKey.FindStringSubmatch(line); match != nil {
				keys = append(keys, strings.Trim(match[1], `"`))
			}
		}
		depth += strings.Count(line, "[") - strings.Count(line, "]")
	}
	return keys
}

func (section tomlSection) valueOf(key string) string {
	for _, match := range tomlKey.FindAllStringSubmatch(section.body, -1) {
		if strings.Trim(match[1], `"`) == key {
			return strings.TrimSpace(match[2])
		}
	}
	return ""
}

// arrayOf returns the text of the `key = [ ... ]` array in the section, which may span lines.
func (section tomlSection) arrayOf(key string) string {
	for _, match := range tomlKey.FindAllStringSubmatchIndex(section.body, -1) {
		if strings.Trim(section.body[match[2]:match[3]], `"`) != key {
			continue
		}
		rest := section.body[match[4]:]
		if !strings.HasPrefix(rest, "[") {
			return ""
		}
		depth := 0
		for index, char := range rest {
			depth += map[rune]int{'[': 1, ']': -1}[char]
			if depth == 0 {
				return rest[:index+1]
			}
		}
		return rest
	}
	return ""
}

var inlineTable = regexp.MustCompile(`\{[^}]*\}`)

// withoutInlineTables drops entries such as {include-group = "test"}, which are not packages.
func withoutInlineTables(text string) string { return inlineTable.ReplaceAllString(text, "") }

var quoted = regexp.MustCompile(`"([^"]+)"|'([^']+)'`)

func quotedStrings(text string) []string {
	var values []string
	for _, match := range quoted.FindAllStringSubmatch(text, -1) {
		values = append(values, match[1]+match[2])
	}
	return values
}

package manifest

import (
	"regexp"
	"strings"
)

var (
	setupList   = regexp.MustCompile(`(?s)\b(install_requires|tests_require|setup_requires)\s*=\s*[\[(](.*?)[\])]`)
	setupExtras = regexp.MustCompile(`(?s)\bextras_require\s*=\s*\{(.*?)\}\s*[,)]`)
	bracketList = regexp.MustCompile(`(?s)\[(.*?)\]`)
)

// setupPy reads the literal requirement lists passed to setup(). Lists built by
// code at run time cannot be read without running it, and are skipped.
func setupPy(content []byte) []Dependency {
	text := string(content)
	var found []Dependency
	for _, match := range setupList.FindAllStringSubmatch(text, -1) {
		found = append(found, pythonRequirements(quotedStrings(match[2]), match[1] != "install_requires")...)
	}
	for _, match := range setupExtras.FindAllStringSubmatch(text, -1) {
		for _, list := range bracketList.FindAllStringSubmatch(match[1], -1) {
			found = append(found, pythonRequirements(quotedStrings(list[1]), false)...)
		}
	}
	return found
}

// setupCfg reads install_requires, tests_require and extras_require from setup.cfg.
func setupCfg(content []byte) []Dependency {
	var found []Dependency
	for _, section := range iniSections(string(content)) {
		for key, values := range section.values {
			switch {
			case section.name == "options" && key == "install_requires":
				found = append(found, pythonRequirements(values, false)...)
			case section.name == "options" && (key == "tests_require" || key == "setup_requires"):
				found = append(found, pythonRequirements(values, true)...)
			case section.name == "options.extras_require":
				found = append(found, pythonRequirements(values, isDevGroup(key))...)
			}
		}
	}
	return found
}

var devGroup = regexp.MustCompile(`(?i)^(dev|develop|test|tests|testing|lint|docs|typing)$`)

func isDevGroup(name string) bool { return devGroup.MatchString(name) }

func pythonRequirements(requirements []string, dev bool) []Dependency {
	var found []Dependency
	for _, requirement := range requirements {
		if name := pythonRequirement(requirement); name != "" && !strings.Contains(requirement, "://") {
			found = append(found, Dependency{Name: name, Ecosystem: PyPI, Direct: true, Dev: dev})
		}
	}
	return found
}

type iniSection struct {
	name   string
	values map[string][]string // a key's first-line value and its indented continuation lines
}

// iniSections parses INI files such as setup.cfg, where a value may continue on indented lines.
func iniSections(content string) []iniSection {
	var sections []iniSection
	key := ""
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";"):
			continue
		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			sections = append(sections, iniSection{name: strings.TrimSpace(trimmed[1 : len(trimmed)-1]), values: map[string][]string{}})
			key = ""
		case len(sections) == 0:
			continue
		case line[0] == ' ' || line[0] == '\t':
			if key != "" {
				current := sections[len(sections)-1]
				current.values[key] = append(current.values[key], trimmed)
			}
		default:
			name, value, ok := strings.Cut(trimmed, "=")
			if !ok {
				name, value, _ = strings.Cut(trimmed, ":")
			}
			key = strings.TrimSpace(name)
			current := sections[len(sections)-1]
			current.values[key] = nil
			if value = strings.TrimSpace(value); value != "" {
				current.values[key] = []string{value}
			}
		}
	}
	return sections
}

var condaSpec = regexp.MustCompile(`^(?:[\w.-]+::)?([A-Za-z0-9][\w.-]*)`)

// condaEnvironment reads a conda environment file: conda packages, and pip packages under "- pip:".
func condaEnvironment(content []byte) []Dependency {
	var found []Dependency
	inDependencies, pipIndent := false, -1
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			inDependencies, pipIndent = trimmed == "dependencies:", -1
			continue
		}
		item, isItem := strings.CutPrefix(trimmed, "- ")
		if !inDependencies || !isItem {
			continue
		}
		item = strings.Trim(strings.TrimSpace(strings.SplitN(item, " #", 2)[0]), `"'`)
		switch {
		case pipIndent >= 0 && indent > pipIndent:
			found = append(found, pythonRequirements([]string{item}, false)...)
		case item == "pip:":
			pipIndent = indent
		default:
			pipIndent = -1
			name := condaSpec.FindStringSubmatch(item)
			if name != nil && name[1] != "python" && name[1] != "pip" {
				found = append(found, Dependency{Name: strings.ToLower(name[1]), Ecosystem: Conda, Direct: true})
			}
		}
	}
	return found
}

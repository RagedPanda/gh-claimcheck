package manifest

import (
	"regexp"
	"strings"
)

var (
	mixTuple   = regexp.MustCompile(`\{\s*:(\w+)\s*,([^{}]*)\}`)
	mixOnly    = regexp.MustCompile(`only:\s*(\[[^\]]*\]|:\w+)`)
	mixHexName = regexp.MustCompile(`hex:\s*:?"?(\w+)`)
	mixGitHub  = regexp.MustCompile(`github:\s*"([^"]+)"`)
	mixGit     = regexp.MustCompile(`git:\s*"([^"]+)"`)
)

// mixExs reads the deps list in an Elixir mix.exs. A dependency limited with
// only: to environments other than :prod counts as dev.
func mixExs(content []byte) []Dependency {
	text := string(content)
	start := strings.Index(text, "defp deps")
	if start < 0 {
		start = strings.Index(text, "deps:")
	}
	if start < 0 {
		return nil
	}
	var found []Dependency
	for _, match := range mixTuple.FindAllStringSubmatch(bracketed(text, start), -1) {
		if dependency, ok := mixDependency(match[1], match[2]); ok {
			found = append(found, dependency)
		}
	}
	return found
}

func mixDependency(name, options string) (Dependency, bool) {
	dev := false
	if only := mixOnly.FindStringSubmatch(options); only != nil {
		dev = !strings.Contains(only[1], ":prod")
	}
	switch {
	case strings.Contains(options, "path:") || strings.Contains(options, "in_umbrella:"):
		return Dependency{}, false
	case mixGitHub.MatchString(options):
		return Dependency{Name: mixGitHub.FindStringSubmatch(options)[1], Ecosystem: GitHub, Direct: true, Dev: dev}, true
	case mixGit.MatchString(options):
		return gitDependency(mixGit.FindStringSubmatch(options)[1], true, dev)
	case mixHexName.MatchString(options):
		name = mixHexName.FindStringSubmatch(options)[1]
	}
	return Dependency{Name: name, Ecosystem: Hex, Direct: true, Dev: dev}, true
}

var (
	mixLockHex = regexp.MustCompile(`\{:hex,\s*:"?(\w+)"?,`)
	mixLockGit = regexp.MustCompile(`\{:git,\s*"([^"]+)"`)
)

// mixLock reads every package Mix resolved, from Hex or git.
func mixLock(content []byte) []Dependency {
	text := string(content)
	var found []Dependency
	for _, match := range mixLockHex.FindAllStringSubmatch(text, -1) {
		found = append(found, Dependency{Name: match[1], Ecosystem: Hex})
	}
	for _, match := range mixLockGit.FindAllStringSubmatch(text, -1) {
		if dependency, ok := gitDependency(match[1], false, false); ok {
			found = append(found, dependency)
		}
	}
	return found
}

var (
	rebarDeps    = regexp.MustCompile(`\{\s*deps\s*,`)
	rebarGitURL  = regexp.MustCompile(`\{\s*git\s*,\s*"([^"]+)"`)
	rebarTopName = regexp.MustCompile(`^\s*(?:\{\s*)?([a-z]\w*)`)
)

// rebarConfig reads {deps, [...]} lists from rebar.config. Lists inside
// {profiles, ...} (test, docs) count as dev.
func rebarConfig(content []byte) []Dependency {
	text := string(content)
	profiles := strings.Index(text, "{profiles")
	var found []Dependency
	for _, location := range rebarDeps.FindAllStringIndex(text, -1) {
		dev := profiles >= 0 && location[0] > profiles
		for _, entry := range topLevelItems(bracketed(text, location[1])) {
			if dependency, ok := rebarDependency(entry, dev); ok {
				found = append(found, dependency)
			}
		}
	}
	return found
}

func rebarDependency(entry string, dev bool) (Dependency, bool) {
	if url := rebarGitURL.FindStringSubmatch(entry); url != nil {
		return gitDependency(url[1], true, dev)
	}
	name := rebarTopName.FindStringSubmatch(entry)
	if name == nil {
		return Dependency{}, false
	}
	return Dependency{Name: name[1], Ecosystem: Hex, Direct: true, Dev: dev}, true
}

// topLevelItems splits "[a, {b, "1"}, c]" into its comma-separated items, keeping nested terms whole.
func topLevelItems(list string) []string {
	list = strings.TrimSpace(list)
	if len(list) < 2 {
		return nil
	}
	var items []string
	depth, start := 0, 1
	for index := 1; index < len(list)-1; index++ {
		switch list[index] {
		case '"':
			index = stringEnd(list, index) - 1
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ',':
			if depth == 0 {
				items = append(items, strings.TrimSpace(list[start:index]))
				start = index + 1
			}
		}
	}
	if last := strings.TrimSpace(list[start : len(list)-1]); last != "" {
		items = append(items, last)
	}
	return items
}

var (
	rebarLockPackage = regexp.MustCompile(`\{<<"([^"]+)">>\s*,\s*\{pkg\s*,\s*<<"([^"]+)">>\s*,[^}]*\}\s*,\s*(\d+)\s*\}`)
	rebarLockGit     = regexp.MustCompile(`\{<<"[^"]+">>\s*,\s*\{git\s*,\s*"([^"]+)"[^}]*\}[^}]*\}\s*,\s*(\d+)\s*\}`)
)

// rebarLock reads rebar.lock. Each entry ends with its depth: 0 is a direct dependency.
func rebarLock(content []byte) []Dependency {
	text := string(content)
	var found []Dependency
	for _, match := range rebarLockPackage.FindAllStringSubmatch(text, -1) {
		found = append(found, Dependency{Name: match[2], Ecosystem: Hex, Direct: match[3] == "0"})
	}
	for _, match := range rebarLockGit.FindAllStringSubmatch(text, -1) {
		if dependency, ok := gitDependency(match[1], match[2] == "0", false); ok {
			found = append(found, dependency)
		}
	}
	return found
}

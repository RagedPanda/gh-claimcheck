package manifest

import (
	"encoding/json"
	"strings"
)

// relaxedJSON turns JSONC (comments, trailing commas), as used by bun.lock and
// deno.jsonc, into plain JSON. Strings are copied untouched.
func relaxedJSON(content []byte) []byte {
	var out strings.Builder
	text := string(content)
	for index := 0; index < len(text); index++ {
		char := text[index]
		switch {
		case char == '"':
			end := stringEnd(text, index)
			out.WriteString(text[index:end])
			index = end - 1
		case strings.HasPrefix(text[index:], "//"):
			for index < len(text) && text[index] != '\n' {
				index++
			}
			out.WriteByte('\n')
		case strings.HasPrefix(text[index:], "/*"):
			end := strings.Index(text[index+2:], "*/")
			if end < 0 {
				return []byte(out.String())
			}
			index += end + 3
		case char == ',' && nextIsClosing(text[index+1:]):
			// trailing comma: drop it
		default:
			out.WriteByte(char)
		}
	}
	return []byte(out.String())
}

// stringEnd returns the index just past the JSON string starting at start.
func stringEnd(text string, start int) int {
	for index := start + 1; index < len(text); index++ {
		switch text[index] {
		case '\\':
			index++
		case '"':
			return index + 1
		}
	}
	return len(text)
}

func nextIsClosing(rest string) bool {
	trimmed := strings.TrimLeft(rest, " \t\r\n")
	return strings.HasPrefix(trimmed, "}") || strings.HasPrefix(trimmed, "]")
}

// packageName returns the name in "name@version" or "@scope/name@version", or
// the whole spec when there is no version. Unlike nameBeforeVersion it splits
// at the first "@" after the scope, so peer suffixes like "a@1_b@2" are safe.
func packageName(spec string) string {
	at := strings.Index(spec[min(1, len(spec)):], "@")
	if at < 0 {
		return spec
	}
	return spec[:at+1]
}

type bunWorkspace struct {
	Dependencies         map[string]string
	DevDependencies      map[string]string
	OptionalDependencies map[string]string
	PeerDependencies     map[string]string
}

// bunLock reads Bun's text lock file (Bun 1.2 and later). Every workspace's own
// dependencies are direct; the rest of "packages" is transitive.
func bunLock(content []byte) []Dependency {
	var lock struct {
		Workspaces map[string]bunWorkspace
		Packages   map[string][]json.RawMessage
	}
	if json.Unmarshal(relaxedJSON(content), &lock) != nil {
		return nil
	}
	var found []Dependency
	listed := map[string]bool{}
	add := func(versions map[string]string, dev bool) {
		for name, version := range versions {
			if isRegistryVersion(version) {
				found = append(found, Dependency{Name: name, Ecosystem: NPM, Direct: true, Dev: dev})
				listed[name] = true
			}
		}
	}
	for _, workspace := range lock.Workspaces {
		add(workspace.Dependencies, false)
		add(workspace.OptionalDependencies, false)
		add(workspace.PeerDependencies, false)
		add(workspace.DevDependencies, true)
	}
	for _, entry := range lock.Packages {
		var spec string
		if len(entry) == 0 || json.Unmarshal(entry[0], &spec) != nil {
			continue
		}
		name := packageName(spec)
		if version := strings.TrimPrefix(spec, name+"@"); name != "" && !listed[name] && isRegistryVersion(version) {
			found = append(found, Dependency{Name: name, Ecosystem: NPM})
		}
	}
	return found
}

// denoJSON reads the import map in deno.json: "jsr:" and "npm:" specifiers.
// URL imports (deno.land/x, esm.sh) are skipped.
func denoJSON(content []byte) []Dependency {
	var config struct {
		Imports map[string]string
	}
	if json.Unmarshal(relaxedJSON(content), &config) != nil {
		return nil
	}
	var found []Dependency
	for _, specifier := range config.Imports {
		if dependency, ok := denoSpecifier(specifier); ok {
			dependency.Direct = true
			found = append(found, dependency)
		}
	}
	return found
}

func denoSpecifier(specifier string) (Dependency, bool) {
	for prefix, ecosystem := range map[string]string{"jsr:": JSR, "npm:": NPM} {
		if rest, ok := strings.CutPrefix(specifier, prefix); ok {
			rest = strings.TrimPrefix(rest, "/")
			if name := packageName(rest); name != "" {
				return Dependency{Name: name, Ecosystem: ecosystem}, true
			}
		}
	}
	return Dependency{}, false
}

// denoLock reads deno.lock, version 3 (sections under "packages") and 4 or later (top level).
func denoLock(content []byte) []Dependency {
	type sections struct {
		JSR map[string]json.RawMessage `json:"jsr"`
		NPM map[string]json.RawMessage `json:"npm"`
	}
	var lock struct {
		sections
		Packages sections `json:"packages"`
	}
	if json.Unmarshal(content, &lock) != nil {
		return nil
	}
	var found []Dependency
	for _, section := range []sections{lock.sections, lock.Packages} {
		for key := range section.JSR {
			found = append(found, Dependency{Name: packageName(key), Ecosystem: JSR})
		}
		for key := range section.NPM {
			found = append(found, Dependency{Name: packageName(key), Ecosystem: NPM})
		}
	}
	return found
}

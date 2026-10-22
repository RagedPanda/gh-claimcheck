package manifest

import (
	"regexp"
	"strings"
)

var (
	sbtModule       = regexp.MustCompile(`"([\w.-]+)"\s*(%%%|%%|%)\s*"([\w.-]+)"\s*%\s*(?:"[^"]*"|[\w.]+)(\s*%\s*(?:"[\w-]+"|[A-Za-z]+))?`)
	sbtScalaVersion = regexp.MustCompile(`scalaVersion\s*:=\s*"(\d+)\.(\d+)`)
	sbtDevScope     = regexp.MustCompile(`(?i)test|it|provided`)
)

// sbt reads library dependencies from .sbt files. A "%%" module is published
// with the Scala version in its artifact name, e.g. cats-core_3 or cats-core_2.13.
func sbt(content []byte) []Dependency {
	text := string(content)
	suffix := "_2.13"
	if version := sbtScalaVersion.FindStringSubmatch(text); version != nil {
		suffix = "_" + version[1] + "." + version[2]
		if version[1] == "3" {
			suffix = "_3"
		}
	}
	plugins := strings.Contains(text, "addSbtPlugin")
	var found []Dependency
	for _, match := range sbtModule.FindAllStringSubmatch(text, -1) {
		artifact := match[3]
		if match[2] != "%" {
			artifact += suffix
		}
		dev := plugins || sbtDevScope.MatchString(match[4])
		found = append(found, Dependency{Name: match[1] + ":" + artifact, Ecosystem: Maven, Direct: true, Dev: dev})
	}
	return found
}

var (
	ednLibrary = regexp.MustCompile(`(?:^|[\s\[{(,])([A-Za-z][\w.*+!?-]*(?:/[\w.*+!?-]+)?)\s+\{([^{}]*)\}`)
	ednString  = func(key string) *regexp.Regexp { return regexp.MustCompile(key + `\s+"([^"]+)"`) }
	ednGitURL  = ednString(`:git/url`)
)

// depsEDN reads Clojure CLI deps.edn: Maven/Clojars coordinates and git
// libraries. Dependencies inside :aliases (test, dev, build tools) count as dev.
func depsEDN(content []byte) []Dependency {
	text := string(content)
	aliases := strings.Index(text, ":aliases")
	var found []Dependency
	for _, match := range ednLibrary.FindAllStringSubmatchIndex(text, -1) {
		library, options := text[match[2]:match[3]], text[match[4]:match[5]]
		dev := aliases >= 0 && match[0] > aliases
		if dependency, ok := clojureLibrary(library, options); ok {
			dependency.Direct, dependency.Dev = true, dev
			found = append(found, dependency)
		}
	}
	return found
}

func clojureLibrary(library, options string) (Dependency, bool) {
	switch {
	case strings.Contains(options, ":mvn/version"):
		return Dependency{Name: clojarsName(library), Ecosystem: Clojars}, true
	case strings.Contains(options, ":git/"):
		if url := ednGitURL.FindStringSubmatch(options); url != nil {
			return gitDependency(url[1], true, false)
		}
		// io.github.owner/repo and com.github.owner/repo name their GitHub repository
		for _, prefix := range []string{"io.github.", "com.github."} {
			if rest, ok := strings.CutPrefix(library, prefix); ok {
				return Dependency{Name: rest, Ecosystem: GitHub}, true
			}
		}
	}
	return Dependency{}, false
}

// clojarsName gives a library its group: in Clojure, "cheshire" means "cheshire/cheshire".
func clojarsName(library string) string {
	if strings.Contains(library, "/") {
		return library
	}
	return library + "/" + library
}

var leinCoordinate = regexp.MustCompile(`\[\s*([A-Za-z][\w.*+!?-]*(?:/[\w.*+!?-]+)?)\s+"[^"]*"`)

// projectCLJ reads Leiningen's :dependencies and :plugins vectors. Plugins, and
// dependencies declared inside :profiles, count as dev.
func projectCLJ(content []byte) []Dependency {
	text := string(content)
	profiles := strings.Index(text, ":profiles")
	var found []Dependency
	for _, keyword := range []string{":dependencies", ":plugins"} {
		for offset := 0; ; {
			at := strings.Index(text[offset:], keyword)
			if at < 0 {
				break
			}
			at += offset
			vector := bracketed(text, at+len(keyword))
			dev := keyword == ":plugins" || (profiles >= 0 && at > profiles)
			for _, match := range leinCoordinate.FindAllStringSubmatch(vector, -1) {
				found = append(found, Dependency{Name: clojarsName(match[1]), Ecosystem: Clojars, Direct: true, Dev: dev})
			}
			offset = at + len(keyword)
		}
	}
	return found
}

// bracketed returns the balanced [...] that starts at the first "[" at or after from,
// ignoring brackets inside strings.
func bracketed(text string, from int) string {
	start := strings.Index(text[from:], "[")
	if start < 0 {
		return ""
	}
	start += from
	depth := 0
	for index := start; index < len(text); index++ {
		switch text[index] {
		case '"':
			index = stringEnd(text, index) - 1
		case '[':
			depth++
		case ']':
			if depth--; depth == 0 {
				return text[start : index+1]
			}
		}
	}
	return text[start:]
}

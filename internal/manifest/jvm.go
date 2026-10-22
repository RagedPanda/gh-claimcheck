package manifest

import (
	"regexp"
	"strings"
)

// Gradle dependency declarations in Groovy or Kotlin DSL, such as
//
//	implementation("com.squareup.okhttp3:okhttp:4.12.0")
//	testImplementation 'junit:junit:4.13.2'
//	implementation(platform("androidx.compose:compose-bom:2024.10.00"))
var gradleCoordinate = regexp.MustCompile(`(?m)\b(\w+)\s*\(?\s*(?:(?:enforced)?[Pp]latform\s*\(\s*)?["']([\w.-]+):([\w.-]+)(?::[^"']*)?["']`)

// Map notation: implementation group: 'com.google.guava', name: 'guava', version: '33.0'
var gradleMap = regexp.MustCompile(`(?m)\b(\w+)\s*\(?\s*group\s*[:=]\s*["']([\w.-]+)["']\s*,\s*name\s*[:=]\s*["']([\w.-]+)["']`)

// gradleConfigurations are the configurations that declare dependencies; anything else
// with a "group:artifact" string (plugin ids, repositories) is ignored.
var gradleConfiguration = regexp.MustCompile(`(?i)(implementation|api|compileOnly|runtimeOnly|compile|runtime|kapt|ksp|annotationProcessor|classpath|lintChecks|coreLibraryDesugaring)$`)

func gradle(content []byte) []Dependency {
	var found []Dependency
	for _, expression := range []*regexp.Regexp{gradleCoordinate, gradleMap} {
		for _, match := range expression.FindAllStringSubmatch(string(content), -1) {
			configuration := match[1]
			if !gradleConfiguration.MatchString(configuration) {
				continue
			}
			found = append(found, Dependency{
				Name: match[2] + ":" + match[3], Ecosystem: Maven, Direct: true,
				Dev: strings.HasPrefix(configuration, "test") || strings.HasPrefix(configuration, "androidTest"),
			})
		}
	}
	return found
}

// versionCatalog reads the [libraries] table of a Gradle version catalog (gradle/libs.versions.toml).
func versionCatalog(content []byte) []Dependency {
	var found []Dependency
	for _, section := range tomlSections(string(content)) {
		if section.name != "libraries" {
			continue
		}
		for _, key := range section.keys() {
			if coordinate := catalogCoordinate(section.valueOf(key)); coordinate != "" {
				found = append(found, Dependency{Name: coordinate, Ecosystem: Maven, Direct: true})
			}
		}
	}
	return found
}

var (
	catalogModule = regexp.MustCompile(`module\s*=\s*"([\w.-]+:[\w.-]+)"`)
	catalogGroup  = regexp.MustCompile(`group\s*=\s*"([\w.-]+)"`)
	catalogName   = regexp.MustCompile(`name\s*=\s*"([\w.-]+)"`)
	catalogString = regexp.MustCompile(`^"([\w.-]+):([\w.-]+)(:[^"]*)?"$`)
)

// catalogCoordinate reads "g:a:v", { module = "g:a" } or { group = "g", name = "a" }.
func catalogCoordinate(value string) string {
	if match := catalogString.FindStringSubmatch(value); match != nil {
		return match[1] + ":" + match[2]
	}
	if match := catalogModule.FindStringSubmatch(value); match != nil {
		return match[1]
	}
	group, name := catalogGroup.FindStringSubmatch(value), catalogName.FindStringSubmatch(value)
	if group != nil && name != nil {
		return group[1] + ":" + name[1]
	}
	return ""
}

var (
	pomDependency = regexp.MustCompile(`(?s)<dependency>(.*?)</dependency>`)
	pomGroup      = regexp.MustCompile(`<groupId>\s*([^<$]+?)\s*</groupId>`)
	pomArtifact   = regexp.MustCompile(`<artifactId>\s*([^<$]+?)\s*</artifactId>`)
	pomTestScope  = regexp.MustCompile(`<scope>\s*test\s*</scope>`)
)

// pomXML reads <dependency> blocks; coordinates that use ${properties} are skipped.
func pomXML(content []byte) []Dependency {
	var found []Dependency
	for _, block := range pomDependency.FindAllStringSubmatch(string(content), -1) {
		group, artifact := pomGroup.FindStringSubmatch(block[1]), pomArtifact.FindStringSubmatch(block[1])
		if group == nil || artifact == nil {
			continue
		}
		found = append(found, Dependency{Name: group[1] + ":" + artifact[1], Ecosystem: Maven, Direct: true, Dev: pomTestScope.MatchString(block[1])})
	}
	return found
}

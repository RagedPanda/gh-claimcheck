package manifest

import (
	"io/fs"
	"strings"
)

// maxFileSize skips unusually large files; real lock files are well under it.
const maxFileSize = 32 << 20

// maxDepth bounds how deep Discover looks, which covers monorepo packages.
const maxDepth = 6

// skippedDirs are installed, generated or vendored code, not the project's own manifests.
var skippedDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true, "target": true, "out": true,
	".git": true, ".venv": true, "venv": true, "env": true, "__pycache__": true, "site-packages": true,
	"third_party": true, ".next": true, ".nuxt": true, ".cache": true, "coverage": true, "bower_components": true,
}

// Scan is what Discover found.
type Scan struct {
	Files        []string     // manifest and lock files that were read
	Dependencies []Dependency // merged and sorted, direct first
}

// Discover walks a project tree, reads every supported manifest and lock file, and merges what they list.
func Discover(project fs.FS) (Scan, error) {
	var scan Scan
	var lists [][]Dependency
	err := fs.WalkDir(project, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		if entry.IsDir() {
			if name != "." && (skippedDirs[entry.Name()] || strings.Count(name, "/") >= maxDepth) {
				return fs.SkipDir
			}
			return nil
		}
		if !Supported(name) {
			return nil
		}
		if info, err := entry.Info(); err != nil || info.Size() > maxFileSize {
			return nil
		}
		content, err := fs.ReadFile(project, name)
		if err != nil {
			return nil
		}
		scan.Files = append(scan.Files, name)
		lists = append(lists, Parse(name, content))
		return nil
	})
	scan.Dependencies = Merge(lists...)
	return scan, err
}

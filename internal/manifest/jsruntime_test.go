package manifest

import (
	"encoding/json"
	"testing"
)

func TestRelaxedJSON(t *testing.T) {
	var parsed struct {
		URL  string
		List []int
	}
	err := json.Unmarshal(relaxedJSON([]byte(`{
  // a comment
  "url": "https://x.dev/a//b", /* block */
  "list": [1, 2,],
}`)), &parsed)
	if err != nil || parsed.URL != "https://x.dev/a//b" || len(parsed.List) != 2 {
		t.Errorf("relaxedJSON: %v %+v", err, parsed)
	}
	expect(t, "deno.jsonc", `{
  // comments and trailing commas are allowed in deno.jsonc
  "imports": {
    "@std/assert": "jsr:@std/assert@^1.0.0",
    "chalk": "npm:chalk@5",
    "local": "./src/mod.ts",
  },
}`, map[string][2]bool{"@std/assert": direct, "chalk": direct}, "local")
}

func TestPackageName(t *testing.T) {
	for spec, want := range map[string]string{"@std/path@1.0.0": "@std/path", "chalk@5": "chalk", "chalk": "chalk", "@scope/x": "@scope/x"} {
		if got := packageName(spec); got != want {
			t.Errorf("packageName(%q) = %q, want %q", spec, got, want)
		}
	}
}

func TestBunLock(t *testing.T) {
	expect(t, "bun.lock", `{
  "lockfileVersion": 1,
  "workspaces": {
    "": {
      "name": "app",
      "dependencies": { "hono": "^4.6.0" },
      "devDependencies": { "@types/bun": "latest" },
    },
  },
  "packages": {
    "hono": ["hono@4.6.3", "", {}, "sha512-x"],
    "@types/bun": ["@types/bun@1.1.10", "", {}, "sha512-y"],
    "bun-types": ["bun-types@1.1.29", "", {}, "sha512-z"],
    "mine": ["mine@github:me/mine#abc", {}],
  },
}`, map[string][2]bool{"hono": direct, "@types/bun": directDev, "bun-types": transitive}, "app", "mine")
}

func TestDenoLock(t *testing.T) {
	expect(t, "deno.lock", `{
  "version": "4",
  "specifiers": { "jsr:@std/path@1": "1.0.8" },
  "jsr": { "@std/path@1.0.8": { "integrity": "x" } },
  "npm": { "chalk@5.3.0": { "integrity": "y" } }
}`, map[string][2]bool{"@std/path": transitive, "chalk": transitive})
}

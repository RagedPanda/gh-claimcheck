package manifest

import "testing"

func TestWorkflow(t *testing.T) {
	content := `on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: "actions/setup-go@v5"
      - uses: github/codeql-action/init@v3 # a sub-action
      - uses: ./.github/actions/local
      - uses: docker://alpine:3.20
  reuse:
    uses: acme/workflows/.github/workflows/ci.yml@main
`
	want := map[string][2]bool{"actions/checkout": directDev, "actions/setup-go": directDev, "github/codeql-action": directDev, "acme/workflows": directDev}
	expect(t, ".github/workflows/ci.yml", content, want, "./.github/actions/local", "alpine")
	expect(t, "action.yml", "runs:\n  using: composite\n  steps:\n    - uses: actions/cache@v4\n", map[string][2]bool{"actions/cache": directDev})
	if Supported("ci.yml") || Supported("docs/.github/notes.yml") || !Supported("pkg/.github/workflows/release.yaml") {
		t.Error("workflow files must be matched by their .github/workflows folder")
	}
}

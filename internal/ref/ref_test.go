package ref

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		input       string
		defaultRepo string
		want        string
	}{
		{"cli/cli#123", "", "cli/cli#123"},
		{"cli/cli/issues/123", "", "cli/cli#123"},
		{"https://github.com/cli/cli/issues/123", "", "cli/cli#123"},
		{"github.com/cli/cli/issues/123#issuecomment-1", "", "cli/cli#123"},
		{"https://github.com/date-fns/tz/issues/80?x=1", "", "date-fns/tz#80"},
		{"#42", "acme/lib", "acme/lib#42"},
		{"42", "acme/lib", "acme/lib#42"},
		{" vitest-dev/eslint-plugin-vitest#980 ", "", "vitest-dev/eslint-plugin-vitest#980"},
	}
	for _, tc := range cases {
		got, err := Parse(tc.input, tc.defaultRepo)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tc.input, err)
			continue
		}
		if got.String() != tc.want {
			t.Errorf("Parse(%q) = %s, want %s", tc.input, got, tc.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, input := range []string{"", "cli", "cli/cli", "#12", "https://github.com/cli/cli/pull/9", "cli/cli#0", "cli/cli#abc"} {
		if got, err := Parse(input, ""); err == nil {
			t.Errorf("Parse(%q) = %s, want an error", input, got)
		}
	}
}

func TestParseRepo(t *testing.T) {
	if owner, name, err := ParseRepo("date-fns/date-fns"); err != nil || owner != "date-fns" || name != "date-fns" {
		t.Errorf("ParseRepo = %q %q %v", owner, name, err)
	}
	if _, _, err := ParseRepo("nope"); err == nil {
		t.Error("expected an error")
	}
}

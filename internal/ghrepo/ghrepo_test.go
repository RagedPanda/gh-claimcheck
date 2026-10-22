package ghrepo

import "testing"

func TestFromURL(t *testing.T) {
	cases := map[string]string{
		"git+https://github.com/lodash/lodash.git":               "lodash/lodash",
		"https://github.com/babel/babel.git":                     "babel/babel",
		"git@github.com:serde-rs/serde.git":                      "serde-rs/serde",
		"ssh://git@github.com/acme/lib":                          "acme/lib",
		"github:sindresorhus/ora":                                "sindresorhus/ora",
		"https://github.com/rails/rails/tree/v8.1.4":             "rails/rails",
		"https://www.github.com/psf/requests#readme":             "psf/requests",
		"github.com/square/okhttp":                               "square/okhttp",
		"https://github.com/sponsors/someone":                    "",
		"https://gitlab.com/acme/lib":                            "",
		"https://notgithub.com/acme/lib":                         "",
		"":                                                       "",
		"https://github.com/date-fns/date-fns/tree/main/pkgs/tz": "date-fns/date-fns",
	}
	for input, want := range cases {
		if got := FromURL(input); got != want {
			t.Errorf("FromURL(%q) = %q, want %q", input, got, want)
		}
	}
}

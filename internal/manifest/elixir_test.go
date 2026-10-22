package manifest

import (
	"reflect"
	"testing"
)

func TestMixExs(t *testing.T) {
	expect(t, "mix.exs", `defmodule App.MixProject do
  defp deps do
    [
      {:phoenix, "~> 1.7"},
      {:jason, ">= 0.0.0"},
      {:credo, "~> 1.7", only: [:dev, :test], runtime: false},
      {:telemetry, "~> 1.0", only: [:dev, :prod]},
      {:my_fork, github: "acme/my_fork"},
      {:pkg_alias, "~> 1.0", hex: :real_pkg},
      {:sibling, in_umbrella: true},
      {:local, path: "../local"}
    ]
  end
end
`, map[string][2]bool{
		"phoenix": direct, "jason": direct, "credo": directDev, "telemetry": direct, "acme/my_fork": direct, "real_pkg": direct,
	}, "sibling", "local", "pkg_alias")
	expect(t, "mix.lock", `%{
  "jason": {:hex, :jason, "1.4.4", "b9226785a9aa77b6857ca22832cffa5d5011a667207eb2a0ad56adb5db443b8a", [:mix], [], "hexpm", "c5eb0cab91f094599f94d55bc63409236a8ec69a21a67814529e8d5f6cc90b3b"},
  "fork": {:git, "https://github.com/acme/fork.git", "abc", []},
}`, map[string][2]bool{"jason": transitive, "acme/fork": transitive})
}

func TestRebar(t *testing.T) {
	expect(t, "rebar.config", `{erl_opts, [debug_info]}.
{deps, [
    cowboy,
    {jsx, "3.1.0"},
    {meck, {git, "https://github.com/eproxus/meck.git", {tag, "0.9.2"}}}
]}.
{profiles, [{test, [{deps, [proper]}]}]}.
`, map[string][2]bool{"cowboy": direct, "jsx": direct, "eproxus/meck": direct, "proper": directDev}, "debug_info")
	expect(t, "rebar.lock", `{"1.2.0",
[{<<"cowboy">>,{pkg,<<"cowboy">>,<<"2.12.0">>},0},
 {<<"cowlib">>,{pkg,<<"cowlib">>,<<"2.13.0">>},1},
 {<<"meck">>,{git,"https://github.com/eproxus/meck.git",{ref,"abc"}},0}]}.
[{pkg_hash,[{<<"cowboy">>, <<"8A7A">>}]}].
`, map[string][2]bool{"cowboy": direct, "cowlib": transitive, "eproxus/meck": direct})
}

func TestTopLevelItems(t *testing.T) {
	got := topLevelItems(`[a, {b, "1,2"}, {c, {git, "u"}}]`)
	want := []string{"a", `{b, "1,2"}`, `{c, {git, "u"}}`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("topLevelItems = %q", got)
	}
}

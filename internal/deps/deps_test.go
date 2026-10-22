package deps

import (
	"reflect"
	"testing"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
	"github.com/RagedPanda/gh-claimcheck/internal/manifest"
)

func dependency(name string, direct, dev bool) manifest.Dependency {
	return manifest.Dependency{Name: name, Ecosystem: manifest.NPM, Direct: direct, Dev: dev}
}

func TestKeep(t *testing.T) {
	all := []manifest.Dependency{dependency("a", true, false), dependency("b", true, true), dependency("c", false, false)}
	cases := []struct {
		filter Filter
		want   []string
	}{
		{Filter{}, []string{"a", "b", "c"}},
		{Filter{DirectOnly: true}, []string{"a", "b"}},
		{Filter{NoDev: true}, []string{"a", "c"}},
		{Filter{DirectOnly: true, NoDev: true}, []string{"a"}},
	}
	for _, tc := range cases {
		var names []string
		for _, kept := range Keep(all, tc.filter) {
			names = append(names, kept.Name)
		}
		if !reflect.DeepEqual(names, tc.want) {
			t.Errorf("%+v: got %v, want %v", tc.filter, names, tc.want)
		}
	}
}

func TestTargets(t *testing.T) {
	dependencies := []manifest.Dependency{
		dependency("@babel/helper-plugin-utils", false, false),
		dependency("@babel/core", true, false),
		dependency("lodash", false, false),
		dependency("unresolved", true, false),
		dependency("Lodash-Fork", false, false),
	}
	repos := map[string]string{
		"npm:@babel/helper-plugin-utils": "babel/babel",
		"npm:@babel/core":                "babel/babel",
		"npm:lodash":                     "lodash/lodash",
		"npm:Lodash-Fork":                "Lodash/Lodash",
	}
	want := []Target{
		{Repo: "babel/babel", Via: "@babel/core", Direct: true},
		{Repo: "lodash/lodash", Via: "lodash", Direct: false},
	}
	if got := Targets(dependencies, repos); !reflect.DeepEqual(got, want) {
		t.Errorf("Targets = %+v, want %+v", got, want)
	}
}

func TestChunk(t *testing.T) {
	got := Chunk([]string{"a", "b", "c", "d", "e"}, 2)
	want := [][]string{{"a", "b"}, {"c", "d"}, {"e"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Chunk = %v, want %v", got, want)
	}
	if Chunk(nil, 2) != nil {
		t.Error("Chunk(nil) should be nil")
	}
}

func TestSearchGroups(t *testing.T) {
	targets := []Target{{Repo: "a/1", Direct: true}, {Repo: "a/2", Direct: true}, {Repo: "a/3", Direct: true}, {Repo: "b/1"}}
	want := [][]string{{"a/1", "a/2"}, {"a/3"}, {"b/1"}}
	if got := SearchGroups(targets, 2); !reflect.DeepEqual(got, want) {
		t.Errorf("SearchGroups = %v, want %v", got, want)
	}
}

func TestRank(t *testing.T) {
	day := func(offset int) time.Time { return time.Date(2026, 10, 1+offset, 0, 0, 0, 0, time.UTC) }
	issue := func(repo string, number, created int) facts.Candidate {
		return facts.Candidate{Repo: repo, Number: number, URL: repo + "#" + string(rune('0'+number)), CreatedAt: day(created)}
	}
	targets := []Target{{Repo: "babel/babel", Via: "@babel/core", Direct: true}, {Repo: "lodash/lodash", Via: "lodash"}}
	results := [][]facts.Candidate{
		{issue("lodash/lodash", 1, 9), issue("babel/babel", 2, 1)},
		{issue("babel/babel", 3, 5), issue("Babel/Babel", 2, 1), issue("other/repo", 4, 9)},
	}
	got := Rank(results, targets, 10)
	var order []int
	for _, found := range got {
		order = append(order, found.Issue.Number)
	}
	if want := []int{3, 2, 1}; !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
	if got[0].Target.Via != "@babel/core" || got[2].Target.Via != "lodash" {
		t.Errorf("via labels wrong: %+v", got)
	}
	if limited := Rank(results, targets, 2); len(limited) != 2 {
		t.Errorf("limit not applied: %d", len(limited))
	}
}

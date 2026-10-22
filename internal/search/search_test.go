package search

import (
	"testing"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
)

func TestQuery(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		options Options
		want    string
	}{
		{"defaults", Options{MaxComments: -1}, "is:issue is:open no:assignee -linked:pr archived:false"},
		{
			"everything",
			Options{Repos: []string{"cli/cli"}, Orgs: []string{"unjs"}, Languages: []string{"go"}, Labels: []string{"bug"},
				GoodFirstIssue: true, Days: 30, MaxComments: 2, Terms: "timeout"},
			`is:issue is:open no:assignee -linked:pr archived:false repo:cli/cli org:unjs language:go label:bug label:"good first issue" comments:0..2 created:>=2026-09-22 timeout`,
		},
		{"blank values dropped", Options{Orgs: []string{"", " "}, MaxComments: 0}, "is:issue is:open no:assignee -linked:pr archived:false comments:0..0"},
	}
	for _, tc := range cases {
		if got := Query(tc.options, now); got != tc.want {
			t.Errorf("%s:\n got  %s\n want %s", tc.name, got, tc.want)
		}
	}
}

func TestHasStars(t *testing.T) {
	keep := HasStars(100)
	if keep(facts.Candidate{Stars: 99}) || !keep(facts.Candidate{Stars: 100}) {
		t.Error("HasStars(100) should keep 100 stars and drop 99")
	}
}

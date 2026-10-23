package search

import (
	"testing"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
)

func TestLooksLikeSpam(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		{"", true},
		{"thanks ", true},
		{"لعه", true},
		{"https://www.snapchat.com/add/someone", true},
		{"59mMMoCgtLvv8zpBXZwQiPCHfxYmSfSMeQ61yzuXGW1cAZ9q9obCvjEt2jzQ", true},
		{"Sparse checkout does not retry transient fetch failures", false},
		{"tzOffset: wrong sign", false},
	}
	for _, tc := range cases {
		if got := LooksLikeSpam(tc.title); got != tc.want {
			t.Errorf("LooksLikeSpam(%q) = %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestWorthwhile(t *testing.T) {
	keep := Worthwhile(100)
	cases := []struct {
		name      string
		candidate facts.Candidate
		want      bool
	}{
		{"real issue", facts.Candidate{Title: "Crash on empty input", Stars: 100}, true},
		{"too few stars", facts.Candidate{Title: "Crash on empty input", Stars: 99}, false},
		{"spam title", facts.Candidate{Title: "thanks", Stars: 5000}, false},
	}
	for _, tc := range cases {
		if got := keep(tc.candidate); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

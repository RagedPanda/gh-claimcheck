package render

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/check"
	"github.com/RagedPanda/gh-claimcheck/internal/facts"
	"github.com/RagedPanda/gh-claimcheck/internal/manifest"
)

func sampleResult() Result {
	return Result{
		Ref:   "acme/lib#7",
		Issue: facts.Issue{Number: 7, Title: "parse() drops the last line", URL: "https://github.com/acme/lib/issues/7"},
		Report: check.Report{Verdict: check.Caution, Summary: "An open PR looks like the same fix", Findings: []check.Finding{
			{Level: check.Warn, Title: "An open PR looks like the same fix", Detail: "#11 by @dan", URL: "https://github.com/acme/lib/pull/11"},
			{Level: check.Pass, Title: "Issue is open"},
		}},
	}
}

func TestFullPlain(t *testing.T) {
	out := Full(sampleResult(), Plain)
	for _, want := range []string{"acme/lib#7  parse() drops the last line", "  ! An open PR looks like the same fix", "    #11 by @dan", "    https://github.com/acme/lib/pull/11", "  ✓ Issue is open", "CAUTION  An open PR looks like the same fix", "ask on the issue"} {
		if !strings.Contains(out, want) {
			t.Errorf("Full output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("Plain palette must not emit ANSI escapes")
	}
}

func TestFullYoursAdvice(t *testing.T) {
	result := sampleResult()
	result.Report = check.Report{Verdict: check.Go, Summary: "You are already on it", Yours: true}
	if out := Full(result, Plain); !strings.Contains(out, `"Fixes #7"`) {
		t.Errorf("advice should name the issue number:\n%s", out)
	}
}

func TestFullError(t *testing.T) {
	if out := Full(Result{Ref: "a/b#1", Err: errors.New("a/b#1: not found")}, Plain); out != "✗ a/b#1: not found\n" {
		t.Errorf("error output = %q", out)
	}
}

func TestBriefAlignsColumns(t *testing.T) {
	out := Brief([]Result{sampleResult(), {Ref: "x/y#1", Err: errors.New("boom")}}, Plain)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 || strings.Index(lines[0], "acme/lib#7") != strings.Index(lines[1], "x/y#1") {
		t.Errorf("columns not aligned:\n%s", out)
	}
}

func TestCandidates(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	report := check.Report{Verdict: check.Go}
	out := Candidates([]Candidate{{
		Issue:    facts.Candidate{Repo: "acme/lib", Number: 7, Title: strings.Repeat("x", 100), Stars: 25700, Comments: 2, CreatedAt: now.AddDate(0, 0, -3)},
		Report:   &report,
		HasCheck: true,
	}}, now, Plain)
	for _, want := range []string{"GO", "acme/lib#7", "★25.7k", "2c", "3d", strings.Repeat("x", 77) + "..."} {
		if !strings.Contains(out, want) {
			t.Errorf("Candidates output missing %q:\n%s", want, out)
		}
	}
	if empty := Candidates(nil, now, Plain); !strings.HasPrefix(empty, "No matching issues") {
		t.Errorf("empty output = %q", empty)
	}
}

func TestCandidatesVia(t *testing.T) {
	now := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	issue := facts.Candidate{Repo: "babel/babel", Number: 7, Title: "Crash", CreatedAt: now}
	out := Candidates([]Candidate{{Issue: issue, Via: "@babel/core"}, {Issue: issue}}, now, Plain)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[0], "babel/babel#7  via @babel/core") || strings.Contains(lines[1], "via") {
		t.Errorf("via column wrong:\n%s", out)
	}
}

func TestDependencies(t *testing.T) {
	dependencies := []manifest.Dependency{
		{Name: "react", Ecosystem: manifest.NPM, Direct: true},
		{Name: "vitest", Ecosystem: manifest.NPM, Direct: true, Dev: true},
		{Name: "left-pad-private", Ecosystem: manifest.NPM},
		{Name: "timed-out", Ecosystem: manifest.NPM},
	}
	out := Dependencies(dependencies, map[string]string{"npm:react": "facebook/react", "npm:vitest": "vitest-dev/vitest"}, map[string]bool{"npm:timed-out": true}, Plain)
	for _, want := range []string{"npm  react             direct       facebook/react", "direct, dev  vitest-dev/vitest", "transitive   not on GitHub", "timed-out         transitive   lookup failed"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if empty := Dependencies(nil, nil, nil, Plain); !strings.HasPrefix(empty, "No dependencies found") {
		t.Errorf("empty = %q", empty)
	}
}

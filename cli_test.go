package main

import (
	"errors"
	"io"
	"testing"

	"github.com/RagedPanda/gh-claimcheck/internal/check"
	"github.com/RagedPanda/gh-claimcheck/internal/render"
)

func TestParseCheckAllowsFlagsAnywhere(t *testing.T) {
	config, err := parseCheck([]string{"12", "--brief", "-R", "acme/lib", "acme/lib#3", "--exit-status"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.issues) != 2 || config.issues[1] != "acme/lib#3" || config.repo != "acme/lib" || !config.brief || !config.exitStatus {
		t.Errorf("config = %+v", config)
	}
}

func TestParseCheckHelp(t *testing.T) {
	for _, args := range [][]string{{}, {"--help"}, {"-h"}} {
		if _, err := parseCheck(args, io.Discard); !errors.Is(err, errHelp) {
			t.Errorf("parseCheck(%v) error = %v, want errHelp", args, err)
		}
	}
	if _, err := parseCheck([]string{"--nope"}, io.Discard); err == nil || errors.Is(err, errHelp) {
		t.Errorf("unknown flag error = %v", err)
	}
}

func TestParseFind(t *testing.T) {
	config, err := parseFind([]string{"timezone", "--org", "unjs,vitejs", "--lang", "go", "--good-first-issue", "--go-only", "-s", "500", "parser"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	options := config.options
	if options.Terms != "timezone parser" || len(options.Orgs) != 2 || options.Languages[0] != "go" || !options.GoodFirstIssue {
		t.Errorf("options = %+v", options)
	}
	if !config.check || config.minStars != 500 || config.limit != 20 || options.Days != 60 || options.MaxComments != 3 {
		t.Errorf("config = %+v (--go-only should imply --check; defaults 20/60/3)", config)
	}
	if _, err := parseFind([]string{"--limit", "0"}, io.Discard); err == nil {
		t.Error("--limit 0 should be rejected")
	}
}

func TestExitCode(t *testing.T) {
	result := func(verdict string) render.Result { return render.Result{Report: check.Report{Verdict: verdict}} }
	cases := []struct {
		results []render.Result
		want    int
	}{
		{[]render.Result{result(check.Go)}, 0},
		{[]render.Result{result(check.Go), result(check.Caution)}, 3},
		{[]render.Result{result(check.Caution), result(check.Skip)}, 2},
		{[]render.Result{result(check.Skip), {Err: errors.New("x")}}, 1},
	}
	for _, tc := range cases {
		if got := exitCode(tc.results); got != tc.want {
			t.Errorf("exitCode = %d, want %d", got, tc.want)
		}
	}
}

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/RagedPanda/gh-claimcheck/internal/check"
	"github.com/RagedPanda/gh-claimcheck/internal/render"
	"github.com/RagedPanda/gh-claimcheck/internal/search"
)

const checkUsage = `Check whether an issue is free to work on before you claim it.

USAGE
  gh claimcheck <issue>... [flags]
  gh claimcheck find [flags]

ISSUE can be a URL, owner/repo#123, or #123 / 123 inside a clone (or with --repo).

It looks for open or merged PRs that already fix the issue (including in the
repository the project moved to), people who already claimed it, assignees,
maintainer pushback, blocking labels, whether outside PRs get merged, and the
project's contributing and AI-use rules. Then it says GO, CAUTION or SKIP.

FLAGS
  -R, --repo owner/repo   Repository for bare issue numbers
  -b, --brief             One line per issue
      --json              Print the reports as JSON
      --exit-status       Exit 2 if any issue is SKIP, 3 if any is CAUTION
  -h, --help              Show this help

EXAMPLES
  gh claimcheck https://github.com/cli/cli/issues/123
  gh claimcheck cli/cli#123 cli/cli#456 --brief
  gh claimcheck 123                     # inside a clone of the repo
  gh claimcheck find --help             # find issues to work on
`

const findUsage = `Find open, unassigned issues with no linked PR, and optionally check each one.

USAGE
  gh claimcheck find [search terms] [flags]

FLAGS
  -R, --repo owner/repo     Search one repository (repeatable)
      --org name            Search one organisation or user (repeatable)
  -l, --lang language       Only this language, e.g. go, typescript (repeatable)
      --label name          Require a label (repeatable)
      --good-first-issue    Only issues labelled "good first issue"
      --help-wanted         Only issues labelled "help wanted"
      --days n              Created in the last n days (default 60, 0 for any)
      --max-comments n      At most n comments (default 3, -1 for any)
  -s, --min-stars n         Only repositories with at least n stars
  -L, --limit n             Number of issues (default 20, max 100)
  -c, --check               Run the full check on each result
      --go-only             With --check, only show issues that come out GO
      --json                Print results as JSON
  -h, --help                Show this help

EXAMPLES
  gh claimcheck find --good-first-issue --lang go
  gh claimcheck find --org vitejs --days 14 --check
  gh claimcheck find "timezone" --repo date-fns/date-fns --max-comments 0
  gh claimcheck find --lang typescript --min-stars 1000 --check --go-only
`

// errHelp signals that help was printed and the program should exit cleanly.
var errHelp = errors.New("help requested")

type checkConfig struct {
	issues     []string
	repo       string
	brief      bool
	json       bool
	exitStatus bool
}

type findConfig struct {
	options  search.Options
	limit    int
	minStars int
	check    bool
	goOnly   bool
	json     bool
}

// listFlag collects a repeatable, comma-separable flag.
type listFlag struct{ values *[]string }

func (list listFlag) String() string { return "" }

func (list listFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*list.values = append(*list.values, part)
		}
	}
	return nil
}

func parseCheck(args []string, usageOut io.Writer) (checkConfig, error) {
	var config checkConfig
	flags := newFlagSet("claimcheck", checkUsage, usageOut)
	stringFlag(flags, &config.repo, "", "repo", "R")
	boolFlag(flags, &config.brief, "brief", "b")
	boolFlag(flags, &config.json, "json")
	boolFlag(flags, &config.exitStatus, "exit-status")
	positional, err := parseInterspersed(flags, args)
	if err != nil {
		return config, err
	}
	if len(positional) == 0 {
		fmt.Fprint(usageOut, checkUsage)
		return config, errHelp
	}
	config.issues = positional
	return config, nil
}

func parseFind(args []string, usageOut io.Writer) (findConfig, error) {
	config := findConfig{options: search.Options{Days: 60, MaxComments: 3}, limit: 20}
	flags := newFlagSet("claimcheck find", findUsage, usageOut)
	listVar(flags, &config.options.Repos, "repo", "R")
	listVar(flags, &config.options.Orgs, "org")
	listVar(flags, &config.options.Languages, "lang", "l")
	listVar(flags, &config.options.Labels, "label")
	boolFlag(flags, &config.options.GoodFirstIssue, "good-first-issue")
	boolFlag(flags, &config.options.HelpWanted, "help-wanted")
	intFlag(flags, &config.options.Days, config.options.Days, "days")
	intFlag(flags, &config.options.MaxComments, config.options.MaxComments, "max-comments")
	intFlag(flags, &config.limit, config.limit, "limit", "L")
	intFlag(flags, &config.minStars, 0, "min-stars", "s")
	boolFlag(flags, &config.check, "check", "c")
	boolFlag(flags, &config.goOnly, "go-only")
	boolFlag(flags, &config.json, "json")
	positional, err := parseInterspersed(flags, args)
	if err != nil {
		return config, err
	}
	if config.limit < 1 || config.limit > 100 {
		return config, fmt.Errorf("--limit must be between 1 and 100")
	}
	if config.goOnly && !config.check {
		config.check = true
	}
	config.options.Terms = strings.Join(positional, " ")
	return config, nil
}

func newFlagSet(name, usage string, usageOut io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() { fmt.Fprint(usageOut, usage) }
	return flags
}

// parseInterspersed lets flags appear before, between or after positional arguments.
func parseInterspersed(flags *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				flags.Usage()
				return nil, errHelp
			}
			return nil, fmt.Errorf("%w (see --help)", err)
		}
		rest := flags.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

func boolFlag(flags *flag.FlagSet, target *bool, names ...string) {
	for _, name := range names {
		flags.BoolVar(target, name, false, "")
	}
}

func stringFlag(flags *flag.FlagSet, target *string, fallback string, names ...string) {
	for _, name := range names {
		flags.StringVar(target, name, fallback, "")
	}
}

func intFlag(flags *flag.FlagSet, target *int, fallback int, names ...string) {
	for _, name := range names {
		flags.IntVar(target, name, fallback, "")
	}
}

func listVar(flags *flag.FlagSet, target *[]string, names ...string) {
	for _, name := range names {
		flags.Var(listFlag{target}, name, "")
	}
}

// exitCode maps results to the --exit-status contract: 1 error, 2 SKIP, 3 CAUTION, 0 GO.
func exitCode(results []render.Result) int {
	code := 0
	for _, result := range results {
		switch {
		case result.Err != nil:
			return 1
		case result.Report.Verdict == check.Skip:
			code = 2
		case result.Report.Verdict == check.Caution && code == 0:
			code = 3
		}
	}
	return code
}

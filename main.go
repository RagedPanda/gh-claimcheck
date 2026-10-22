// gh-claimcheck is a GitHub CLI extension that checks whether an issue is
// free to work on before you claim it, and finds issues that are.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/cli/go-gh/v2/pkg/term"

	"github.com/RagedPanda/gh-claimcheck/internal/check"
	"github.com/RagedPanda/gh-claimcheck/internal/facts"
	"github.com/RagedPanda/gh-claimcheck/internal/github"
	"github.com/RagedPanda/gh-claimcheck/internal/parallel"
	"github.com/RagedPanda/gh-claimcheck/internal/ref"
	"github.com/RagedPanda/gh-claimcheck/internal/render"
	"github.com/RagedPanda/gh-claimcheck/internal/search"
)

// concurrency bounds parallel checks so search and GraphQL rate limits hold.
const concurrency = 3

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	terminal := term.FromEnv()
	palette := render.Plain
	if terminal.IsColorEnabled() {
		palette = render.ANSI
	}
	var err error
	code := 0
	if len(args) > 0 && args[0] == "find" {
		err = runFind(args[1:], stdout, palette)
	} else {
		code, err = runCheck(args, stdout, palette)
	}
	if errors.Is(err, errHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "claimcheck:", err)
		return 1
	}
	return code
}

func runCheck(args []string, stdout io.Writer, palette render.Palette) (int, error) {
	config, err := parseCheck(args, stdout)
	if err != nil {
		return 0, err
	}
	defaultRepo := config.repo
	if defaultRepo == "" {
		if current, err := repository.Current(); err == nil {
			defaultRepo = current.Owner + "/" + current.Name
		}
	}
	issues, err := parseIssues(config.issues, defaultRepo)
	if err != nil {
		return 0, err
	}
	client, err := github.New()
	if err != nil {
		return 0, err
	}
	results := checkAll(context.Background(), client, issues, time.Now())
	if config.json {
		return exitStatus(config, results), writeJSON(stdout, jsonResults(results))
	}
	if config.brief {
		fmt.Fprint(stdout, render.Brief(results, palette))
		return exitStatus(config, results), nil
	}
	for index, result := range results {
		if index > 0 {
			fmt.Fprintln(stdout)
		}
		fmt.Fprint(stdout, render.Full(result, palette))
	}
	return exitStatus(config, results), nil
}

func runFind(args []string, stdout io.Writer, palette render.Palette) error {
	config, err := parseFind(args, stdout)
	if err != nil {
		return err
	}
	client, err := github.New()
	if err != nil {
		return err
	}
	ctx, now := context.Background(), time.Now()
	found, err := client.FindIssues(ctx, search.Query(config.options, now), config.limit, search.HasStars(config.minStars))
	if err != nil {
		return err
	}
	candidates := make([]render.Candidate, len(found))
	for index, issue := range found {
		candidates[index] = render.Candidate{Issue: issue, HasCheck: config.check}
	}
	if config.check {
		candidates = withChecks(ctx, client, candidates, now)
	}
	if config.goOnly {
		candidates = onlyGo(candidates)
	}
	if config.json {
		return writeJSON(stdout, jsonCandidates(candidates))
	}
	fmt.Fprint(stdout, render.Candidates(candidates, now, palette))
	return nil
}

func parseIssues(inputs []string, defaultRepo string) ([]ref.Issue, error) {
	issues := make([]ref.Issue, 0, len(inputs))
	for _, input := range inputs {
		issue, err := ref.Parse(input, defaultRepo)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// checkAll checks issues in parallel and returns results in input order.
func checkAll(ctx context.Context, client *github.Client, issues []ref.Issue, now time.Time) []render.Result {
	results := make([]render.Result, len(issues))
	parallel.ForEach(len(issues), concurrency, func(index int) {
		results[index] = checkOne(ctx, client, issues[index], now)
	})
	return results
}

func checkOne(ctx context.Context, client *github.Client, issue ref.Issue, now time.Time) render.Result {
	gathered, err := client.Gather(ctx, issue, now)
	if err != nil {
		return render.Result{Ref: issue.String(), Err: err}
	}
	return render.Result{Ref: issue.String(), Issue: gathered.Issue, Report: check.Run(gathered)}
}

func withChecks(ctx context.Context, client *github.Client, candidates []render.Candidate, now time.Time) []render.Candidate {
	checked := make([]render.Candidate, len(candidates))
	parallel.ForEach(len(candidates), concurrency, func(index int) {
		candidate := candidates[index]
		issue := ref.Issue{Number: candidate.Issue.Number}
		issue.Owner, issue.Name, _ = ref.ParseRepo(candidate.Issue.Repo)
		result := checkOne(ctx, client, issue, now)
		candidate.Err = result.Err
		if result.Err == nil {
			report := result.Report
			candidate.Report = &report
		}
		checked[index] = candidate
	})
	return checked
}

func onlyGo(candidates []render.Candidate) []render.Candidate {
	var kept []render.Candidate
	for _, candidate := range candidates {
		if candidate.Report != nil && candidate.Report.Verdict == check.Go {
			kept = append(kept, candidate)
		}
	}
	return kept
}

func exitStatus(config checkConfig, results []render.Result) int {
	if !config.exitStatus {
		for _, result := range results {
			if result.Err != nil {
				return 1
			}
		}
		return 0
	}
	return exitCode(results)
}

type jsonResult struct {
	Issue   string          `json:"issue"`
	Title   string          `json:"title,omitempty"`
	URL     string          `json:"url,omitempty"`
	Error   string          `json:"error,omitempty"`
	Verdict string          `json:"verdict,omitempty"`
	Summary string          `json:"summary,omitempty"`
	Checks  []check.Finding `json:"findings,omitempty"`
}

func jsonResults(results []render.Result) []jsonResult {
	out := make([]jsonResult, len(results))
	for index, result := range results {
		out[index] = jsonResult{Issue: result.Ref, Title: result.Issue.Title, URL: result.Issue.URL,
			Verdict: result.Report.Verdict, Summary: result.Report.Summary, Checks: result.Report.Findings}
		if result.Err != nil {
			out[index] = jsonResult{Issue: result.Ref, Error: result.Err.Error()}
		}
	}
	return out
}

type jsonCandidate struct {
	Issue     string        `json:"issue"`
	Title     string        `json:"title"`
	URL       string        `json:"url"`
	Comments  int           `json:"comments"`
	Stars     int           `json:"stars"`
	Labels    []string      `json:"labels"`
	CreatedAt time.Time     `json:"createdAt"`
	Error     string        `json:"error,omitempty"`
	Report    *check.Report `json:"report,omitempty"`
}

func jsonCandidates(candidates []render.Candidate) []jsonCandidate {
	out := make([]jsonCandidate, len(candidates))
	for index, candidate := range candidates {
		out[index] = toJSONCandidate(candidate.Issue, candidate.Report, candidate.Err)
	}
	return out
}

func toJSONCandidate(issue facts.Candidate, report *check.Report, err error) jsonCandidate {
	converted := jsonCandidate{
		Issue: fmt.Sprintf("%s#%d", issue.Repo, issue.Number), Title: issue.Title, URL: issue.URL,
		Comments: issue.Comments, Stars: issue.Stars, Labels: nonNil(issue.Labels), CreatedAt: issue.CreatedAt, Report: report,
	}
	if err != nil {
		converted.Error = err.Error()
	}
	return converted
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func writeJSON(out io.Writer, value interface{}) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

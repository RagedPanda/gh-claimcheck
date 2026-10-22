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
	"path/filepath"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/cli/go-gh/v2/pkg/term"

	"github.com/RagedPanda/gh-claimcheck/internal/check"
	"github.com/RagedPanda/gh-claimcheck/internal/deps"
	"github.com/RagedPanda/gh-claimcheck/internal/facts"
	"github.com/RagedPanda/gh-claimcheck/internal/github"
	"github.com/RagedPanda/gh-claimcheck/internal/manifest"
	"github.com/RagedPanda/gh-claimcheck/internal/parallel"
	"github.com/RagedPanda/gh-claimcheck/internal/ref"
	"github.com/RagedPanda/gh-claimcheck/internal/registry"
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
	switch {
	case len(args) > 0 && args[0] == "find":
		err = runFind(args[1:], stdout, palette)
	case len(args) > 0 && args[0] == "deps":
		err = runDeps(args[1:], stdout, stderr, palette)
	default:
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
	return showCandidates(ctx, client, config, candidates, stdout, now, palette)
}

// showCandidates optionally checks the found issues, then prints them.
func showCandidates(ctx context.Context, client *github.Client, config findConfig, candidates []render.Candidate, stdout io.Writer, now time.Time, palette render.Palette) error {
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

// searchChunk bounds the repo: qualifiers in one search query.
const searchChunk = 50

func runDeps(args []string, stdout, stderr io.Writer, palette render.Palette) error {
	progress := progressWriter(stderr)
	config, err := parseDeps(args, stdout)
	if err != nil {
		return err
	}
	if info, err := os.Stat(config.dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not a directory", config.dir)
	}
	scan, err := manifest.Discover(os.DirFS(config.dir))
	if err != nil {
		return err
	}
	if len(scan.Files) == 0 {
		return fmt.Errorf("no manifest or lock files in %s (see gh claimcheck deps --help for the files it reads)", config.dir)
	}
	dependencies := deps.Keep(scan.Dependencies, config.filter)
	fmt.Fprintf(progress, "Read %d files: %d dependencies (%d direct). Looking up their repositories...\n",
		len(scan.Files), len(dependencies), countDirect(dependencies))
	ctx, now := context.Background(), time.Now()
	cachePath := registryCachePath()
	cache := registry.DecodeCache(readOrEmpty(cachePath))
	resolution := registry.New().Resolve(ctx, dependencies, cache, now)
	writeCache(cachePath, cache.With(resolution.Looked, now))
	if len(resolution.Failed) > 0 {
		fmt.Fprintf(stderr, "claimcheck: could not look up %d of %d dependencies%s; they were skipped.\n",
			len(resolution.Failed), len(dependencies), rateLimitNote(resolution.RateLimited))
	}
	repoByKey := resolution.Repos
	if config.list {
		if config.find.json {
			return writeJSON(stdout, jsonDependencies(dependencies, repoByKey, resolution.Failed))
		}
		fmt.Fprint(stdout, render.Dependencies(dependencies, repoByKey, resolution.Failed, palette))
		return nil
	}
	targets := deps.Targets(dependencies, repoByKey)
	if len(targets) == 0 {
		return fmt.Errorf("found no GitHub repositories among %d dependencies in %s (try --list)", len(dependencies), config.dir)
	}
	client, err := github.New()
	if err != nil {
		return err
	}
	chunks := deps.SearchGroups(targets, searchChunk)
	fmt.Fprintf(progress, "Searching %d repositories...\n", len(targets))
	results := make([][]facts.Candidate, len(chunks))
	errs := make([]error, len(chunks))
	parallel.ForEach(len(chunks), concurrency, func(index int) {
		options := config.find.options
		options.Repos = chunks[index]
		results[index], errs[index] = client.FindIssues(ctx, search.Query(options, now), config.find.limit, search.HasStars(config.find.minStars))
	})
	if err := errors.Join(errs...); err != nil {
		return err
	}
	found := deps.Rank(results, targets, config.find.limit)
	candidates := make([]render.Candidate, len(found))
	for index, each := range found {
		candidates[index] = render.Candidate{Issue: each.Issue, HasCheck: config.find.check, Via: each.Target.Via}
	}
	return showCandidates(ctx, client, config.find, candidates, stdout, now, palette)
}

func rateLimitNote(hosts []string) string {
	if len(hosts) == 0 {
		return ""
	}
	return fmt.Sprintf(" (%s asked us to slow down; run it again in a few minutes, answers so far are cached)", strings.Join(hosts, ", "))
}

// registryCachePath is where registry answers are kept between runs, or "" if there is no cache directory.
func registryCachePath() string {
	directory, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(directory, "gh-claimcheck", "registry.json")
}

func readOrEmpty(path string) []byte {
	if path == "" {
		return nil
	}
	content, _ := os.ReadFile(path)
	return content
}

// writeCache saves the cache; a cache that cannot be written only makes the next run slower.
func writeCache(path string, cache registry.Cache) {
	if path == "" || os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	temporary := path + ".tmp"
	if os.WriteFile(temporary, cache.Encode(), 0o644) == nil {
		_ = os.Rename(temporary, path)
	}
}

// progressWriter shows progress only to a person watching, never in pipes or logs.
func progressWriter(stderr io.Writer) io.Writer {
	if file, ok := stderr.(*os.File); ok && term.IsTerminal(file) {
		return stderr
	}
	return io.Discard
}

func countDirect(dependencies []manifest.Dependency) int {
	count := 0
	for _, dependency := range dependencies {
		if dependency.Direct {
			count++
		}
	}
	return count
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
	Via       string        `json:"via,omitempty"`
	Error     string        `json:"error,omitempty"`
	Report    *check.Report `json:"report,omitempty"`
}

func jsonCandidates(candidates []render.Candidate) []jsonCandidate {
	out := make([]jsonCandidate, len(candidates))
	for index, candidate := range candidates {
		out[index] = toJSONCandidate(candidate.Issue, candidate.Report, candidate.Err)
		out[index].Via = candidate.Via
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

type jsonDependency struct {
	manifest.Dependency
	Repo         string `json:"repo,omitempty"`
	LookupFailed bool   `json:"lookupFailed,omitempty"`
}

func jsonDependencies(dependencies []manifest.Dependency, repoByKey map[string]string, failed map[string]bool) []jsonDependency {
	out := make([]jsonDependency, len(dependencies))
	for index, dependency := range dependencies {
		out[index] = jsonDependency{Dependency: dependency, Repo: repoByKey[dependency.Key()], LookupFailed: failed[dependency.Key()]}
	}
	return out
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

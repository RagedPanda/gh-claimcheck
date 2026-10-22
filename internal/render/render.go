// Package render formats reports for the terminal. It only builds strings,
// so the caller decides where they go.
package render

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/check"
	"github.com/RagedPanda/gh-claimcheck/internal/facts"
)

// Palette colours output. Plain returns text unchanged, for pipes and NO_COLOR.
type Palette struct {
	Red, Yellow, Green, Cyan, Gray, Bold func(string) string
}

// Plain is a palette that adds no colour.
var Plain = Palette{Red: same, Yellow: same, Green: same, Cyan: same, Gray: same, Bold: same}

// ANSI is a palette for colour terminals.
var ANSI = Palette{
	Red: ansi("31"), Yellow: ansi("33"), Green: ansi("32"), Cyan: ansi("36"), Gray: ansi("90"), Bold: ansi("1"),
}

// Result pairs an issue with its report, or the error that stopped the check.
type Result struct {
	Issue  facts.Issue
	Report check.Report
	Err    error
	Ref    string
}

// Full renders a complete report for one issue.
func Full(result Result, palette Palette) string {
	if result.Err != nil {
		return palette.Red("✗ ") + result.Err.Error() + "\n"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "%s  %s\n", palette.Bold(result.Ref), result.Issue.Title)
	fmt.Fprintf(&out, "%s\n\n", palette.Gray(result.Issue.URL))
	for _, finding := range result.Report.Findings {
		fmt.Fprintf(&out, "  %s %s\n", symbol(finding.Level, palette), finding.Title)
		if finding.Detail != "" {
			fmt.Fprintf(&out, "    %s\n", palette.Gray(finding.Detail))
		}
		if finding.URL != "" && !strings.Contains(finding.Detail, finding.URL) {
			fmt.Fprintf(&out, "    %s\n", palette.Gray(finding.URL))
		}
	}
	fmt.Fprintf(&out, "\n  %s  %s\n", verdict(result.Report.Verdict, palette), result.Report.Summary)
	fmt.Fprintf(&out, "  %s\n", palette.Gray(advice(result.Report, result.Issue.Number)))
	return out.String()
}

// Brief renders one line per issue.
func Brief(results []Result, palette Palette) string {
	var out strings.Builder
	writer := tabwriter.NewWriter(&out, 0, 0, 2, ' ', 0)
	for _, result := range results {
		if result.Err != nil {
			fmt.Fprintf(writer, "%s\t%s\t%s\n", palette.Red("ERROR"), result.Ref, result.Err.Error())
			continue
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\n", verdict(result.Report.Verdict, palette), result.Ref, result.Report.Summary)
	}
	writer.Flush()
	return out.String()
}

// Candidate is a search result, with a report when it was checked.
type Candidate struct {
	Issue    facts.Candidate
	Report   *check.Report
	Err      error
	HasCheck bool
}

// Candidates renders search results as a table.
func Candidates(candidates []Candidate, now time.Time, palette Palette) string {
	if len(candidates) == 0 {
		return "No matching issues. Try a longer --days window, more --max-comments, or a wider scope.\n"
	}
	var out strings.Builder
	writer := tabwriter.NewWriter(&out, 0, 0, 2, ' ', 0)
	for _, candidate := range candidates {
		issue := candidate.Issue
		columns := []string{}
		if candidate.HasCheck {
			columns = append(columns, candidateVerdict(candidate, palette))
		}
		columns = append(columns,
			fmt.Sprintf("%s#%d", issue.Repo, issue.Number),
			palette.Gray(fmt.Sprintf("★%s", compact(issue.Stars))),
			palette.Gray(fmt.Sprintf("%dc", issue.Comments)),
			palette.Gray(age(issue.CreatedAt, now)),
			clip(issue.Title, 80),
		)
		fmt.Fprintln(writer, strings.Join(columns, "\t"))
	}
	writer.Flush()
	return out.String()
}

func candidateVerdict(candidate Candidate, palette Palette) string {
	switch {
	case candidate.Err != nil:
		return palette.Red("ERROR")
	case candidate.Report == nil:
		return ""
	}
	return verdict(candidate.Report.Verdict, palette)
}

func symbol(level string, palette Palette) string {
	switch level {
	case check.Block:
		return palette.Red("✗")
	case check.Warn:
		return palette.Yellow("!")
	case check.Pass:
		return palette.Green("✓")
	}
	return palette.Cyan("·")
}

func verdict(name string, palette Palette) string {
	padded := fmt.Sprintf("%-7s", name)
	switch name {
	case check.Skip:
		return palette.Bold(palette.Red(padded))
	case check.Caution:
		return palette.Bold(palette.Yellow(padded))
	}
	return palette.Bold(palette.Green(padded))
}

func advice(report check.Report, number int) string {
	if report.Yours && report.Verdict == check.Go {
		return fmt.Sprintf("Keep going, and link your PR to the issue with \"Fixes #%d\" in its description.", number)
	}
	switch report.Verdict {
	case check.Skip:
		return "Someone is already on it, or it cannot be worked on. Pick another issue."
	case check.Caution:
		return "Look into the points above, and ask on the issue before you start."
	}
	return "Comment on the issue to say you are working on it, then open a PR."
}

func age(then, now time.Time) string {
	days := int(now.Sub(then).Hours() / 24)
	if days < 1 {
		return "today"
	}
	return fmt.Sprintf("%dd", days)
}

func compact(count int) string {
	if count >= 1000 {
		return fmt.Sprintf("%.1fk", float64(count)/1000)
	}
	return fmt.Sprint(count)
}

func clip(value string, limit int) string {
	if len([]rune(value)) <= limit {
		return value
	}
	return string([]rune(value)[:limit-3]) + "..."
}

func same(value string) string { return value }

func ansi(code string) func(string) string {
	return func(value string) string { return "\x1b[" + code + "m" + value + "\x1b[0m" }
}

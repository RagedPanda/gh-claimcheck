// Package text reads intent out of free text: claim comments, maintainer
// pushback, AI contribution policies, "this project has moved" notices,
// issue labels and search keywords. Every function is pure.
package text

import (
	"regexp"
	"sort"
	"strings"
)

var claimPatterns = compileAll(
	`\b(i'?d like to|i would like to|i want to|i'?d love to|i'?d be happy to|happy to|can i|could i|may i|let me|i'?ll|i will|i'?m going to|i am going to|i can|i could)\s+(try to\s+)?(work on|take|pick( this| it)? up|tackle|fix|handle|submit|open|send|raise|grab|look into|create|make|prepare)\b`,
	`\bassign (this |it )?(issue )?(to )?me\b`,
	`^\s*/(assign|take|claim)\b`,
	`\b(i'?m|i am) (currently )?(working on|looking into) (this|it|a fix)\b`,
	`\bi('?ve| have)? (just |already )?(opened|submitted|created|raised|sent|pushed|prepared|wrote|made)\b.{0,40}\b(pr|pull request|merge request|fix|branch|patch)\b`,
	`\b(i have|i'?ve got|here'?s) (a )?(fix|branch|patch)\b`,
	`\b(fix|patch) is (on|in) (a|my) (branch|fork)\b`,
)

// IsClaim reports whether a comment says its author is taking on the issue.
func IsClaim(body string) bool {
	return matchesAny(claimPatterns, normalize(body))
}

var declinePatterns = compileAll(
	`\bwon'?t fix\b`, `\bwontfix\b`, `\bnot a bug\b`, `\bworking as (intended|designed|expected)\b`,
	`\b(this is )?(by|as) design\b`, `\bduplicate of\b`, `\bnot planned\b`,
	`\b(we )?(won'?t|will not|do not|don'?t) (accept|merge|take) (a )?(pr|prs|pull requests?|changes?)\b`,
	`\bclosing (this|as)\b`,
)

// IsDecline reports whether a comment pushes back on the issue being fixed.
func IsDecline(body string) bool {
	return matchesAny(declinePatterns, normalize(body))
}

var blockingLabel = regexp.MustCompile(`(?i)won'?t ?fix|wontfix|duplicate|invalid|not planned|blocked|on.?hold|stale|needs?.?(decision|discussion|triage|repro|reproduction|info|feedback|design)|pending.?(triage|approval)|discussion|question`)

var welcomingLabel = regexp.MustCompile(`(?i)good.?first.?issue|help.?wanted|accepting.?prs?|prs?.?welcome|contributions?.?welcome|up.?for.?grabs|first.?timers?|beginner|easy`)

// BlockingLabels returns the labels that say the issue is not ready for a PR.
func BlockingLabels(labels []string) []string { return filter(labels, blockingLabel.MatchString) }

// WelcomingLabels returns the labels that invite outside contributors.
func WelcomingLabels(labels []string) []string { return filter(labels, welcomingLabel.MatchString) }

// AI policy stances, from strictest to loosest.
const (
	AIRestricted = "restricted"
	AIDisclose   = "disclose"
	AIMentioned  = "mentioned"
)

// AIPolicy is what a contributing guide says about AI-assisted contributions.
type AIPolicy struct {
	Stance  string // AIRestricted, AIDisclose, AIMentioned or empty when not mentioned
	Excerpt string
}

// "AI" is matched case-sensitively so words like "said" or "maintain" never match.
var aiMention = regexp.MustCompile(`\bAI\b|(?i)\b(llms?|chatgpt|copilot|claude|gemini|generative|machine[- ]generated|ai[- ](generated|assisted|tools?|agents?))\b`)

var aiRestricted = regexp.MustCompile(`(?i)\b(not|no longer|never) (be )?(accept|accepted|allow|allowed|welcome|permitted|merge|merged)|prohibit|\bban(ned)?\b|forbid|will be (closed|rejected)|(do not|don'?t|must not|may not) (submit|use|open|send)|not acceptable`)

var aiDisclose = regexp.MustCompile(`(?i)disclos|must (state|mention|note|say|indicate)|clearly (state|mark|label)|in your own words|understand (the|your|every)|be able to explain`)

// ReadAIPolicy finds what a contributing guide says about AI-assisted contributions.
func ReadAIPolicy(guide string) AIPolicy {
	lines := strings.Split(guide, "\n")
	var mentions []string
	for index, line := range lines {
		if aiMention.MatchString(line) {
			mentions = append(mentions, paragraphAround(lines, index))
		}
	}
	return policyFrom(mentions)
}

func policyFrom(mentions []string) AIPolicy {
	if len(mentions) == 0 {
		return AIPolicy{}
	}
	for _, stance := range []struct {
		name    string
		pattern *regexp.Regexp
	}{{AIRestricted, aiRestricted}, {AIDisclose, aiDisclose}} {
		for _, mention := range mentions {
			if stance.pattern.MatchString(mention) {
				return AIPolicy{Stance: stance.name, Excerpt: excerpt(mention)}
			}
		}
	}
	return AIPolicy{Stance: AIMentioned, Excerpt: excerpt(mentions[0])}
}

func paragraphAround(lines []string, index int) string {
	start, end := index, index
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" && index-start < 3 {
		start--
	}
	for end < len(lines)-1 && strings.TrimSpace(lines[end+1]) != "" && end-index < 3 {
		end++
	}
	return strings.Join(lines[start:end+1], " ")
}

var movedNotice = regexp.MustCompile(`(?i)(has |have |been |was |is )?(moved|migrated|relocated|transferred)( permanently)? (to|into)|now lives (in|at)|development (continues|happens) (in|at)|deprecated in favou?r of|superseded by|no longer maintained|is archived`)

var githubRepo = regexp.MustCompile(`github\.com/([A-Za-z0-9-]+/[A-Za-z0-9._-]+)`)

// MovedTo reads a README for a notice that the project moved. It returns the
// repository it moved to (empty when the notice names none) and the notice line.
func MovedTo(readme, currentRepo string) (string, string) {
	lines := strings.Split(readme, "\n")
	for index, line := range lines[:min(len(lines), 40)] {
		if !movedNotice.MatchString(line) {
			continue
		}
		return movedTarget(lines, index, currentRepo), strings.TrimSpace(line)
	}
	return "", ""
}

func movedTarget(lines []string, index int, currentRepo string) string {
	nearby := strings.Join(lines[index:min(len(lines), index+3)], " ")
	for _, match := range githubRepo.FindAllStringSubmatch(nearby, -1) {
		target := strings.TrimSuffix(strings.TrimSuffix(match[1], ".git"), ".")
		if !strings.EqualFold(target, currentRepo) {
			return target
		}
	}
	return ""
}

var (
	codeSpan   = regexp.MustCompile("`([^`]+)`")
	identifier = regexp.MustCompile(`[A-Za-z_$][\w$]*(?:[.:#][\w$]+)+|[a-z]+[A-Z][\w$]*|[A-Za-z]+_[\w]+|[\w$]+\(\)`)
	word       = regexp.MustCompile(`[A-Za-z][\w-]{3,}`)
)

// Keywords picks up to three search terms from an issue title: code spans
// first, then identifiers, then the longest uncommon words.
func Keywords(title string) []string {
	var picked []string
	add := func(term string) {
		term = strings.Trim(term, "()`'\".,:;")
		if len(term) >= 3 && !contains(picked, term) && len(picked) < 3 {
			picked = append(picked, term)
		}
	}
	for _, match := range codeSpan.FindAllStringSubmatch(title, -1) {
		add(firstToken(match[1]))
	}
	for _, match := range identifier.FindAllString(title, -1) {
		add(match)
	}
	for _, candidate := range longestWords(title) {
		add(candidate)
	}
	return picked
}

func firstToken(code string) string {
	if fields := strings.FieldsFunc(code, func(r rune) bool { return strings.ContainsRune(" (<{[=,", r) }); len(fields) > 0 {
		return fields[0]
	}
	return code
}

var stopWords = map[string]bool{
	"about": true, "after": true, "again": true, "always": true, "because": true, "before": true,
	"being": true, "between": true, "broken": true, "cannot": true, "could": true, "doesn't": true,
	"error": true, "every": true, "fails": true, "failing": true, "first": true, "incorrect": true,
	"instead": true, "issue": true, "never": true, "other": true, "problem": true, "should": true,
	"since": true, "still": true, "their": true, "there": true, "these": true, "those": true,
	"through": true, "under": true, "using": true, "value": true, "when": true, "where": true,
	"which": true, "while": true, "with": true, "without": true, "would": true, "wrong": true,
	"returns": true, "return": true, "throws": true, "works": true, "working": true, "correctly": true,
}

func longestWords(title string) []string {
	var words []string
	for _, candidate := range word.FindAllString(title, -1) {
		if len(candidate) >= 5 && !stopWords[strings.ToLower(candidate)] {
			words = append(words, candidate)
		}
	}
	sort.SliceStable(words, func(left, right int) bool { return len(words[left]) > len(words[right]) })
	return words
}

// excerpt shortens a paragraph to one readable line.
func excerpt(paragraph string) string {
	collapsed := strings.Join(strings.Fields(paragraph), " ")
	if len(collapsed) <= 160 {
		return collapsed
	}
	return strings.TrimSpace(collapsed[:157]) + "..."
}

// normalize lowercases text, unifies apostrophes and drops quoted reply lines,
// so a quoted claim by someone else is not read as a new claim.
func normalize(body string) string {
	var kept []string
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), ">") {
			kept = append(kept, line)
		}
	}
	return strings.NewReplacer("’", "'", "‘", "'").Replace(strings.ToLower(strings.Join(kept, "\n")))
}

func compileAll(patterns ...string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, len(patterns))
	for index, pattern := range patterns {
		compiled[index] = regexp.MustCompile(`(?m)` + pattern)
	}
	return compiled
}

func matchesAny(patterns []*regexp.Regexp, text string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func filter(values []string, keep func(string) bool) []string {
	var kept []string
	for _, value := range values {
		if keep(value) {
			kept = append(kept, value)
		}
	}
	return kept
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

// Similarity is the share of the issue title's significant words that also
// appear in another title, from 0 to 1. Plurals are folded, so "offset" matches "offsets".
func Similarity(issueTitle, otherTitle string) float64 {
	issueWords := significantWords(issueTitle)
	if len(issueWords) == 0 {
		return 0
	}
	otherWords := significantWords(otherTitle)
	shared := 0
	for term := range issueWords {
		if otherWords[term] {
			shared++
		}
	}
	return float64(shared) / float64(len(issueWords))
}

var similarityNoise = map[string]bool{"fix": true, "fixe": true, "bug": true, "issue": true, "when": true, "the": true, "and": true, "for": true, "with": true, "not": true, "does": true}

func significantWords(title string) map[string]bool {
	terms := map[string]bool{}
	for _, candidate := range word.FindAllString(strings.ToLower(title), -1) {
		candidate = strings.TrimSuffix(candidate, "s")
		if len(candidate) >= 3 && !stopWords[candidate] && !similarityNoise[candidate] {
			terms[candidate] = true
		}
	}
	return terms
}

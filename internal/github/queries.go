package github

import (
	"strings"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
	"github.com/RagedPanda/gh-claimcheck/internal/text"
)

// GraphQL rejects unused fragments, so each query appends only the ones it uses.
const (
	fileFragment = `
fragment file on GitObject { ... on Blob { text } ... on Tree { entries { name } } }`
	prFragment = `
fragment pr on PullRequest { number title url state isDraft createdAt author { login } repository { nameWithOwner } }`
)

// Community file locations, in the order GitHub itself looks for them.
const communityFiles = `
contributingRoot: object(expression: "HEAD:CONTRIBUTING.md") { ...file }
contributingGithub: object(expression: "HEAD:.github/CONTRIBUTING.md") { ...file }
contributingDocs: object(expression: "HEAD:docs/CONTRIBUTING.md") { ...file }
contributingLower: object(expression: "HEAD:.github/contributing.md") { ...file }
templateGithub: object(expression: "HEAD:.github/pull_request_template.md") { ...file }
templateGithubUpper: object(expression: "HEAD:.github/PULL_REQUEST_TEMPLATE.md") { ...file }
templateRoot: object(expression: "HEAD:pull_request_template.md") { ...file }
templateDocs: object(expression: "HEAD:docs/pull_request_template.md") { ...file }
templateDir: object(expression: "HEAD:.github/PULL_REQUEST_TEMPLATE") { ...file }
`

var issueQuery = `
query($owner: String!, $name: String!, $number: Int!) {
  viewer { login }
  repository(owner: $owner, name: $name) {
    nameWithOwner
    isArchived
    ` + communityFiles + `
    agents: object(expression: "HEAD:AGENTS.md") { ...file }
    aiPolicy: object(expression: "HEAD:AI_POLICY.md") { ...file }
    pullRequests(states: MERGED, first: 40, orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes { mergedAt authorAssociation author { login __typename } }
    }
    issue(number: $number) {
      title body url state stateReason createdAt locked
      author { login }
      assignees(first: 10) { nodes { login } }
      labels(first: 30) { nodes { name } }
      comments(last: 60) { totalCount nodes { author { login } authorAssociation body createdAt } }
      closedByPullRequestsReferences(first: 10, includeClosedPrs: true) { nodes { ...pr } }
      timelineItems(first: 100, itemTypes: [CROSS_REFERENCED_EVENT, CONNECTED_EVENT]) {
        nodes {
          __typename
          ... on CrossReferencedEvent { source { ...pr } }
          ... on ConnectedEvent { subject { ...pr } }
        }
      }
    }
  }
}
` + fileFragment + prFragment

var orgDefaultsQuery = `
query($owner: String!) {
  repository(owner: $owner, name: ".github") {
    ` + communityFiles + `
  }
}
` + fileFragment

var prSearchQuery = `
query($q: String!) {
  search(query: $q, type: ISSUE, first: 10) { nodes { ...pr } }
}
` + prFragment

type login struct {
	Login    string
	Typename string `json:"__typename"`
}

type blob struct {
	Text    string
	Entries []struct{ Name string }
}

type namedBlob struct {
	path string
	blob *blob
}

type communityBlobs struct {
	ContributingRoot    *blob
	ContributingGithub  *blob
	ContributingDocs    *blob
	ContributingLower   *blob
	TemplateGithub      *blob
	TemplateGithubUpper *blob
	TemplateRoot        *blob
	TemplateDocs        *blob
	TemplateDir         *blob
}

func (files communityBlobs) contributingFiles() []namedBlob {
	return []namedBlob{
		{".github/CONTRIBUTING.md", files.ContributingGithub},
		{"CONTRIBUTING.md", files.ContributingRoot},
		{"docs/CONTRIBUTING.md", files.ContributingDocs},
		{".github/contributing.md", files.ContributingLower},
	}
}

func (files communityBlobs) templateFiles() []namedBlob {
	return []namedBlob{
		{".github/pull_request_template.md", files.TemplateGithub},
		{".github/PULL_REQUEST_TEMPLATE.md", files.TemplateGithubUpper},
		{"pull_request_template.md", files.TemplateRoot},
		{"docs/pull_request_template.md", files.TemplateDocs},
		{".github/PULL_REQUEST_TEMPLATE/", files.TemplateDir},
	}
}

type prNode struct {
	Number     int
	Title      string
	URL        string
	State      string
	IsDraft    bool
	CreatedAt  time.Time
	Author     *login
	Repository struct{ NameWithOwner string }
}

func (node prNode) toPullRequest(source string) facts.PullRequest {
	return facts.PullRequest{
		Repo: node.Repository.NameWithOwner, Number: node.Number, Title: node.Title, URL: node.URL,
		State: node.State, IsDraft: node.IsDraft, Author: loginOf(node.Author), CreatedAt: node.CreatedAt, Source: source,
	}
}

type issueNode struct {
	Title       string
	Body        string
	URL         string
	State       string
	StateReason string
	CreatedAt   time.Time
	Locked      bool
	Author      *login
	Assignees   struct{ Nodes []login }
	Labels      struct{ Nodes []struct{ Name string } }
	Comments    struct {
		TotalCount int
		Nodes      []struct {
			Author            *login
			AuthorAssociation string
			Body              string
			CreatedAt         time.Time
		}
	}
	ClosedByPullRequestsReferences struct{ Nodes []prNode }
	TimelineItems                  struct {
		Nodes []struct {
			Source  *prNode
			Subject *prNode
		}
	}
}

func (node issueNode) toIssue(repo string) facts.Issue {
	issue := facts.Issue{
		Repo: repo, Title: node.Title, Body: node.Body, URL: node.URL, State: node.State, StateReason: node.StateReason,
		Author: loginOf(node.Author), CreatedAt: node.CreatedAt, Locked: node.Locked, TotalComments: node.Comments.TotalCount,
	}
	issue.Number = numberFromURL(node.URL)
	for _, assignee := range node.Assignees.Nodes {
		issue.Assignees = append(issue.Assignees, assignee.Login)
	}
	for _, label := range node.Labels.Nodes {
		issue.Labels = append(issue.Labels, label.Name)
	}
	for _, comment := range node.Comments.Nodes {
		issue.Comments = append(issue.Comments, facts.Comment{
			Author: loginOf(comment.Author), Association: comment.AuthorAssociation, Body: comment.Body, CreatedAt: comment.CreatedAt,
		})
	}
	return issue
}

// linkedPRs returns pull requests that close the issue or reference it.
func (node issueNode) linkedPRs() []facts.PullRequest {
	var linked []facts.PullRequest
	add := func(pr *prNode) {
		if pr != nil && pr.Number > 0 {
			linked = append(linked, pr.toPullRequest(facts.SourceLinked))
		}
	}
	for index := range node.ClosedByPullRequestsReferences.Nodes {
		add(&node.ClosedByPullRequestsReferences.Nodes[index])
	}
	for _, item := range node.TimelineItems.Nodes {
		add(item.Source)
		add(item.Subject)
	}
	return linked
}

type repositoryNode struct {
	communityBlobs
	NameWithOwner string
	IsArchived    bool
	Agents        *blob
	AIPolicy      *blob
	PullRequests  struct {
		Nodes []struct {
			MergedAt          time.Time
			AuthorAssociation string
			Author            *login
		}
	}
	Issue *issueNode
}

func (node repositoryNode) toRepo() facts.Repo {
	repo := facts.Repo{NameWithOwner: node.NameWithOwner, Archived: node.IsArchived}
	repo.ContributingPath, repo.ContributingText = firstFile(node.contributingFiles())
	repo.PRTemplatePath, _ = firstFile(node.templateFiles())
	repo.AIPolicyPath, repo.AIPolicyText = aiPolicySource(node, repo)
	for _, pr := range node.PullRequests.Nodes {
		repo.MergedPRs = append(repo.MergedPRs, facts.MergedPR{
			Author: loginOf(pr.Author), Association: pr.AuthorAssociation, MergedAt: pr.MergedAt,
			IsBot: pr.Author == nil || pr.Author.Typename == "Bot",
		})
	}
	return repo
}

// aiPolicySource picks the file whose AI policy is strictest.
func aiPolicySource(node repositoryNode, repo facts.Repo) (string, string) {
	candidates := []namedBlob{{"AI_POLICY.md", node.AIPolicy}, {repo.ContributingPath, &blob{Text: repo.ContributingText}}, {"AGENTS.md", node.Agents}}
	rank := map[string]int{text.AIRestricted: 3, text.AIDisclose: 2, text.AIMentioned: 1}
	bestPath, bestText, bestRank := "", "", 0
	for _, candidate := range candidates {
		if candidate.blob == nil || candidate.path == "" {
			continue
		}
		if stanceRank := rank[text.ReadAIPolicy(candidate.blob.Text).Stance]; stanceRank > bestRank {
			bestPath, bestText, bestRank = candidate.path, candidate.blob.Text, stanceRank
		}
	}
	return bestPath, bestText
}

type issueResponse struct {
	Viewer     login
	Repository repositoryNode
}

type orgDefaultsResponse struct {
	Repository *communityBlobs
}

type searchResponse struct {
	Search struct{ Nodes []prNode }
}

func loginOf(author *login) string {
	if author == nil {
		return "ghost"
	}
	return author.Login
}

func numberFromURL(url string) int {
	number := 0
	for _, char := range url[strings.LastIndex(url, "/")+1:] {
		if char < '0' || char > '9' {
			break
		}
		number = number*10 + int(char-'0')
	}
	return number
}

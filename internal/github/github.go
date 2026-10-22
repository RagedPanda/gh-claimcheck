// Package github gathers facts from the GitHub API. It is the only package
// that talks to the network; it turns API responses into plain facts.
package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
	"github.com/RagedPanda/gh-claimcheck/internal/ref"
	"github.com/RagedPanda/gh-claimcheck/internal/text"
)

// Client reads from the GitHub API with the user's gh login.
type Client struct {
	graphql *api.GraphQLClient
	rest    *api.RESTClient
}

// New returns a client authenticated the same way as gh.
func New() (*Client, error) {
	graphql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, explainSetup(err)
	}
	rest, err := api.NewRESTClient(api.ClientOptions{Headers: map[string]string{"Accept": "application/vnd.github.raw+json"}})
	if err != nil {
		return nil, explainSetup(err)
	}
	return &Client{graphql: graphql, rest: rest}, nil
}

// Gather collects the facts about one issue.
func (client *Client) Gather(ctx context.Context, issue ref.Issue, now time.Time) (facts.Facts, error) {
	var response issueResponse
	err := client.graphql.DoWithContext(ctx, issueQuery, map[string]interface{}{"owner": issue.Owner, "name": issue.Name, "number": issue.Number}, &response)
	if err != nil {
		return facts.Facts{}, explain(err, issue)
	}
	if response.Repository.Issue == nil {
		return facts.Facts{}, fmt.Errorf("%s: not found, or it is a pull request", issue)
	}

	repo := response.Repository.toRepo()
	client.addOrgContributing(ctx, issue.Owner, &repo)
	repo.MovedTo, repo.MovedNotice = text.MovedTo(client.readme(ctx, issue.Repo()), issue.Repo())

	issueFacts := facts.Facts{
		Viewer: response.Viewer.Login,
		Now:    now,
		Issue:  response.Repository.Issue.toIssue(issue.Repo()),
		Repo:   repo,
	}
	issueFacts.PullRequests = mergePRs(response.Repository.Issue.linkedPRs(), client.titleMatches(ctx, issueFacts))
	return issueFacts, nil
}

// titleMatches searches pull requests by the issue title's keywords, in the
// repository and in the one it moved to.
func (client *Client) titleMatches(ctx context.Context, issueFacts facts.Facts) []facts.PullRequest {
	keywords := text.Keywords(issueFacts.Issue.Title)
	if len(keywords) == 0 {
		return nil
	}
	terms := quoteAll(keywords[:min(len(keywords), 2)])
	var found []facts.PullRequest
	for _, repo := range nonEmpty(issueFacts.Repo.NameWithOwner, issueFacts.Repo.MovedTo) {
		found = append(found, client.searchPRs(ctx, fmt.Sprintf("repo:%s is:pr %s", repo, terms))...)
	}
	return found
}

func (client *Client) searchPRs(ctx context.Context, query string) []facts.PullRequest {
	var response searchResponse
	if err := client.graphql.DoWithContext(ctx, prSearchQuery, map[string]interface{}{"q": query}, &response); err != nil {
		return nil
	}
	var found []facts.PullRequest
	for _, node := range response.Search.Nodes {
		if node.Number > 0 {
			found = append(found, node.toPullRequest(facts.SourceTitleMatch))
		}
	}
	return found
}

// addOrgContributing falls back to the organisation's default community files
// in its .github repository when the repository has none of its own.
func (client *Client) addOrgContributing(ctx context.Context, owner string, repo *facts.Repo) {
	if repo.ContributingPath != "" && repo.PRTemplatePath != "" {
		return
	}
	var response orgDefaultsResponse
	// The .github repository is optional, so a "not found" error is expected and ignored.
	_ = client.graphql.DoWithContext(ctx, orgDefaultsQuery, map[string]interface{}{"owner": owner}, &response)
	if response.Repository == nil {
		return
	}
	prefix := owner + "/.github: "
	if repo.ContributingPath == "" {
		if path, body := firstFile(response.Repository.contributingFiles()); path != "" {
			repo.ContributingPath, repo.ContributingText = prefix+path, body
			if repo.AIPolicyPath == "" && text.ReadAIPolicy(body).Stance != "" {
				repo.AIPolicyPath, repo.AIPolicyText = repo.ContributingPath, body
			}
		}
	}
	if repo.PRTemplatePath == "" {
		if path, _ := firstFile(response.Repository.templateFiles()); path != "" {
			repo.PRTemplatePath = prefix + path
		}
	}
}

func (client *Client) readme(ctx context.Context, repo string) string {
	response, err := client.rest.RequestWithContext(ctx, http.MethodGet, "repos/"+repo+"/readme", nil)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		return ""
	}
	return string(body)
}

// Viewer returns the login gh is authenticated as.
func (client *Client) Viewer(ctx context.Context) (string, error) {
	var response struct {
		Viewer struct{ Login string }
	}
	err := client.graphql.DoWithContext(ctx, `query{viewer{login}}`, nil, &response)
	return response.Viewer.Login, err
}

// mergePRs combines linked and title-matched pull requests, keeping the linked
// record when both found the same one.
func mergePRs(linked, matched []facts.PullRequest) []facts.PullRequest {
	seen := map[string]bool{}
	var merged []facts.PullRequest
	for _, pr := range append(linked, matched...) {
		key := strings.ToLower(fmt.Sprintf("%s#%d", pr.Repo, pr.Number))
		if !seen[key] {
			seen[key] = true
			merged = append(merged, pr)
		}
	}
	return merged
}

// errNotLoggedIn is returned when there is no usable gh login.
var errNotLoggedIn = errors.New("not logged in to GitHub; run `gh auth login`")

// explainSetup turns go-gh's missing-token error into the fix.
func explainSetup(err error) error {
	if strings.Contains(err.Error(), "authentication token not found") {
		return errNotLoggedIn
	}
	return err
}

func unauthorized(err error) bool {
	var httpError *api.HTTPError
	return errors.As(err, &httpError) && httpError.StatusCode == http.StatusUnauthorized
}

func explain(err error, issue ref.Issue) error {
	if unauthorized(err) {
		return errNotLoggedIn
	}
	var graphqlError *api.GraphQLError
	if errors.As(err, &graphqlError) && strings.Contains(err.Error(), "Could not resolve") {
		return fmt.Errorf("%s: not found; check the repository name and issue number, and that you can see the repository", issue)
	}
	return fmt.Errorf("%s: %w", issue, err)
}

func firstFile(files []namedBlob) (string, string) {
	for _, file := range files {
		if file.blob != nil && (file.blob.Text != "" || len(file.blob.Entries) > 0) {
			return file.path, file.blob.Text
		}
	}
	return "", ""
}

func quoteAll(terms []string) string {
	quoted := make([]string, len(terms))
	for index, term := range terms {
		quoted[index] = `"` + strings.ReplaceAll(term, `"`, "") + `"`
	}
	return strings.Join(quoted, " ")
}

func nonEmpty(values ...string) []string {
	var kept []string
	for _, value := range values {
		if value != "" {
			kept = append(kept, value)
		}
	}
	return kept
}

func explainSearch(err error) error {
	if unauthorized(err) {
		return errNotLoggedIn
	}
	return fmt.Errorf("search failed: %w", err)
}

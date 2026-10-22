package github

import (
	"context"
	"time"

	"github.com/RagedPanda/gh-claimcheck/internal/facts"
)

const issueSearchQuery = `
query($q: String!, $first: Int!, $after: String) {
  search(query: $q, type: ISSUE, first: $first, after: $after) {
    pageInfo { hasNextPage endCursor }
    nodes {
      ... on Issue {
        number title url createdAt
        comments { totalCount }
        labels(first: 10) { nodes { name } }
        repository { nameWithOwner stargazerCount }
      }
    }
  }
}`

type candidateNode struct {
	Number    int
	Title     string
	URL       string
	CreatedAt time.Time
	Comments  struct{ TotalCount int }
	Labels    struct{ Nodes []struct{ Name string } }
	Repo      struct {
		NameWithOwner  string
		StargazerCount int
	} `json:"repository"`
}

func (node candidateNode) toCandidate() facts.Candidate {
	candidate := facts.Candidate{
		Repo: node.Repo.NameWithOwner, Number: node.Number, Title: node.Title, URL: node.URL,
		Comments: node.Comments.TotalCount, Stars: node.Repo.StargazerCount, CreatedAt: node.CreatedAt,
	}
	for _, label := range node.Labels.Nodes {
		candidate.Labels = append(candidate.Labels, label.Name)
	}
	return candidate
}

// maxSearchPages bounds how far FindIssues pages through results looking for matches.
const maxSearchPages = 5

// FindIssues runs an issue search and returns up to limit results that keep accepts,
// paging through at most maxSearchPages pages of 100.
func (client *Client) FindIssues(ctx context.Context, query string, limit int, keep func(facts.Candidate) bool) ([]facts.Candidate, error) {
	var candidates []facts.Candidate
	var after *string
	for page := 0; page < maxSearchPages && len(candidates) < limit; page++ {
		var response struct {
			Search struct {
				PageInfo struct {
					HasNextPage bool
					EndCursor   string
				}
				Nodes []candidateNode
			}
		}
		variables := map[string]interface{}{"q": query, "first": 100, "after": after}
		if err := client.graphql.DoWithContext(ctx, issueSearchQuery, variables, &response); err != nil {
			return nil, explainSearch(err)
		}
		for _, node := range response.Search.Nodes {
			if candidate := node.toCandidate(); node.Number > 0 && keep(candidate) && len(candidates) < limit {
				candidates = append(candidates, candidate)
			}
		}
		if !response.Search.PageInfo.HasNextPage {
			break
		}
		cursor := response.Search.PageInfo.EndCursor
		after = &cursor
	}
	return candidates, nil
}

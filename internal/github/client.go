package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type RateLimitInfo struct {
	Remaining int
	Limit     int
	Reset     time.Time
}

type Client struct {
	token      string
	httpClient *http.Client
	rateLimit  RateLimitInfo
}

func GetGHToken() (string, error) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}
	if token := os.Getenv("GH_TOKEN"); token != "" {
		return token, nil
	}

	cmd := exec.Command("gh", "auth", "token")
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("github authentication not found. Run 'gh auth login' to authenticate")
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", errors.New("empty token returned by 'gh auth token'")
	}
	return token, nil
}

func NewClient(token string) (*Client, error) {
	if token == "" {
		var err error
		token, err = GetGHToken()
		if err != nil {
			return nil, err
		}
	}

	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		rateLimit: RateLimitInfo{
			Remaining: 5000,
			Limit:     5000,
			Reset:     time.Now().Add(1 * time.Hour),
		},
	}, nil
}

func (c *Client) RateLimit() RateLimitInfo {
	return c.rateLimit
}

func (c *Client) updateRateLimits(resp *http.Response) {
	if rem := resp.Header.Get("X-Ratelimit-Remaining"); rem != "" {
		if val, err := strconv.Atoi(rem); err == nil {
			c.rateLimit.Remaining = val
		}
	}
	if lim := resp.Header.Get("X-Ratelimit-Limit"); lim != "" {
		if val, err := strconv.Atoi(lim); err == nil {
			c.rateLimit.Limit = val
		}
	}
	if rst := resp.Header.Get("X-Ratelimit-Reset"); rst != "" {
		if val, err := strconv.ParseInt(rst, 10, 64); err == nil {
			c.rateLimit.Reset = time.Unix(val, 0)
		}
	}
}

func (c *Client) GetAuthenticatedUser(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	c.updateRateLimits(resp)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get user: HTTP %d", resp.StatusCode)
	}

	var res struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Login, nil
}

type gqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Client) queryGraphQL(ctx context.Context, query string, variables map[string]interface{}, target interface{}) error {
	body, err := json.Marshal(gqlRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.updateRateLimits(resp)

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("graphql request failed (status %d): %s", resp.StatusCode, string(data))
	}

	var gqlResp gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return err
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", gqlResp.Errors[0].Message)
	}

	return json.Unmarshal(gqlResp.Data, target)
}

// FetchRepositoriesAndPRs fetches both repos and open PRs for an owner (org or user)
func (c *Client) FetchRepositoriesAndPRs(ctx context.Context, owner string) ([]Repository, []PullRequest, error) {
	query := `
query($login: String!) {
  repositoryOwner(login: $login) {
    repositories(first: 50, orderBy: {field: PUSHED_AT, direction: DESC}) {
      nodes {
        name
        description
        pushedAt
        isPrivate
        url
        sshUrl
        defaultBranchRef {
          name
        }
        pullRequests(first: 20, states: OPEN, orderBy: {field: UPDATED_AT, direction: DESC}) {
          nodes {
            number
            title
            author {
              login
            }
            createdAt
            updatedAt
            isDraft
            reviewDecision
            headRefName
            url
            labels(first: 5) {
              nodes {
                name
              }
            }
            comments {
              totalCount
            }
            reviewThreads(first: 30) {
              totalCount
              nodes {
                isResolved
              }
            }
          }
        }
      }
    }
  }
}
`
	var resp struct {
		RepositoryOwner struct {
			Repositories struct {
				Nodes []struct {
					Name             string    `json:"name"`
					Description      *string   `json:"description"`
					PushedAt         time.Time `json:"pushedAt"`
					IsPrivate        bool      `json:"isPrivate"`
					URL              string    `json:"url"`
					SSHURL           string    `json:"sshUrl"`
					DefaultBranchRef *struct {
						Name string `json:"name"`
					} `json:"defaultBranchRef"`
					PullRequests struct {
						Nodes []struct {
							Number         int       `json:"number"`
							Title          string    `json:"title"`
							Author         *struct {
								Login string `json:"login"`
							} `json:"author"`
							CreatedAt      time.Time `json:"createdAt"`
							UpdatedAt      time.Time `json:"updatedAt"`
							IsDraft        bool      `json:"isDraft"`
							ReviewDecision *string   `json:"reviewDecision"`
							HeadRefName    string    `json:"headRefName"`
							URL            string    `json:"url"`
							Labels         struct {
								Nodes []struct {
									Name string `json:"name"`
								} `json:"nodes"`
							} `json:"labels"`
							Comments struct {
								TotalCount int `json:"totalCount"`
							} `json:"comments"`
							ReviewThreads struct {
								TotalCount int `json:"totalCount"`
								Nodes      []struct {
									IsResolved bool `json:"isResolved"`
								} `json:"nodes"`
							} `json:"reviewThreads"`
						} `json:"nodes"`
					} `json:"pullRequests"`
				} `json:"nodes"`
			} `json:"repositories"`
		} `json:"repositoryOwner"`
	}

	err := c.queryGraphQL(ctx, query, map[string]interface{}{"login": owner}, &resp)
	if err != nil {
		return nil, nil, err
	}

	var repos []Repository
	var prs []PullRequest

	for _, node := range resp.RepositoryOwner.Repositories.Nodes {
		desc := ""
		if node.Description != nil {
			desc = *node.Description
		}
		defBranch := "main"
		if node.DefaultBranchRef != nil && node.DefaultBranchRef.Name != "" {
			defBranch = node.DefaultBranchRef.Name
		}

		repo := Repository{
			Name:          node.Name,
			FullName:      owner + "/" + node.Name,
			Description:   desc,
			PushedAt:      node.PushedAt,
			DefaultBranch: defBranch,
			IsPrivate:     node.IsPrivate,
			URL:           node.URL,
			SSHURL:        node.SSHURL,
		}
		repos = append(repos, repo)

		for _, p := range node.PullRequests.Nodes {
			author := "unknown"
			if p.Author != nil {
				author = p.Author.Login
			}
			reviewDecision := "NONE"
			if p.ReviewDecision != nil {
				reviewDecision = *p.ReviewDecision
			}
			var labels []string
			for _, l := range p.Labels.Nodes {
				labels = append(labels, l.Name)
			}

			unresolved := 0
			for _, t := range p.ReviewThreads.Nodes {
				if !t.IsResolved {
					unresolved++
				}
			}

			prs = append(prs, PullRequest{
				Number:             p.Number,
				Repo:               node.Name,
				Title:              p.Title,
				Author:             author,
				CreatedAt:          p.CreatedAt,
				UpdatedAt:          p.UpdatedAt,
				IsDraft:            p.IsDraft,
				ReviewDecision:     reviewDecision,
				HeadRefName:        p.HeadRefName,
				URL:                p.URL,
				Labels:             labels,
				TotalComments:      p.Comments.TotalCount + p.ReviewThreads.TotalCount,
				UnresolvedComments: unresolved,
			})
		}
	}

	return repos, prs, nil
}

// FetchSquadActivity retrieves public events for a list of squad members
func (c *Client) FetchSquadActivity(ctx context.Context, members []string) ([]SquadEvent, error) {
	var events []SquadEvent
	if len(members) == 0 {
		return events, nil
	}

	for _, member := range members {
		url := fmt.Sprintf("https://api.github.com/users/%s/events/public?per_page=10", member)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}
		c.updateRateLimits(resp)
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		var rawEvents []struct {
			Type      string    `json:"type"`
			CreatedAt time.Time `json:"created_at"`
			Repo      struct {
				Name string `json:"name"`
			} `json:"repo"`
			Payload json.RawMessage `json:"payload"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&rawEvents); err == nil {
			for _, ev := range rawEvents {
				action := ""
				details := ""
				target := ev.Repo.Name

				switch ev.Type {
				case "PushEvent":
					action = "Pushed commits"
					var p struct {
						Commits []struct {
							Message string `json:"message"`
						} `json:"commits"`
					}
					_ = json.Unmarshal(ev.Payload, &p)
					if len(p.Commits) > 0 {
						details = strings.Split(p.Commits[0].Message, "\n")[0]
					}
				case "PullRequestEvent":
					var p struct {
						Action      string `json:"action"`
						PullRequest struct {
							Number int    `json:"number"`
							Title  string `json:"title"`
						} `json:"pull_request"`
					}
					_ = json.Unmarshal(ev.Payload, &p)
					action = fmt.Sprintf("PR %s", p.Action)
					target = fmt.Sprintf("%s#%d", ev.Repo.Name, p.PullRequest.Number)
					details = p.PullRequest.Title
				case "IssueCommentEvent":
					action = "Commented"
					var p struct {
						Comment struct {
							Body string `json:"body"`
						} `json:"comment"`
					}
					_ = json.Unmarshal(ev.Payload, &p)
					details = strings.Split(p.Comment.Body, "\n")[0]
				default:
					action = strings.TrimSuffix(ev.Type, "Event")
				}

				if len(details) > 60 {
					details = details[:57] + "..."
				}

				events = append(events, SquadEvent{
					Actor:     member,
					Action:    action,
					Repo:      ev.Repo.Name,
					Target:    target,
					Details:   details,
					Timestamp: ev.CreatedAt,
				})
			}
		}
		resp.Body.Close()
	}

	return events, nil
}

// TriggerCopilotReview requests a Copilot review on a PR
func (c *Client) TriggerCopilotReview(ctx context.Context, owner, repo string, prNumber int) error {
	// Request github-copilot as a reviewer
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/requested_reviewers", owner, repo, prNumber)
	body, _ := json.Marshal(map[string]interface{}{
		"reviewers": []string{"github-copilot"},
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.updateRateLimits(resp)

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}

	// Fallback: post a comment with /copilot review
	commentURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repo, prNumber)
	commentBody, _ := json.Marshal(map[string]string{
		"body": "/copilot review",
	})
	creq, err := http.NewRequestWithContext(ctx, "POST", commentURL, bytes.NewReader(commentBody))
	if err != nil {
		return err
	}
	creq.Header.Set("Authorization", "Bearer "+c.token)
	cresp, err := c.httpClient.Do(creq)
	if err != nil {
		return err
	}
	defer cresp.Body.Close()
	c.updateRateLimits(cresp)

	if cresp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to trigger copilot review: HTTP %d", cresp.StatusCode)
	}
	return nil
}

// CloneRepo clones a repository into targetDir
func CloneRepo(ctx context.Context, repoFullName, targetDir string) error {
	parentDir := targetDir
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "gh", "repo", "clone", repoFullName, targetDir)
	return cmd.Run()
}

package github

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type RateLimitInfo struct {
	Remaining int       `json:"remaining"`
	Limit     int       `json:"limit"`
	Used      int       `json:"used"`
	Reset     time.Time `json:"reset"`
}

type ClientOptions struct {
	Token              string
	ProxyURL           string
	InsecureSkipVerify bool
	Timeout            time.Duration
	CachePath          string
}

type Client struct {
	token            string
	httpClient       *http.Client
	rateLimit        RateLimitInfo // Core REST
	graphqlRateLimit RateLimitInfo // GraphQL
	proxyUsed        string
	cache            *ETagCache
}

func (c *Client) ProxyUsed() string {
	return c.proxyUsed
}

func getGitProxy() string {
	for _, key := range []string{"https.proxy", "http.proxy"} {
		cmd := exec.Command("git", "config", "--get", key)
		if out, err := cmd.Output(); err == nil {
			val := strings.TrimSpace(string(out))
			if val != "" {
				return val
			}
		}
	}
	return ""
}

func getEnvProxy() string {
	for _, envKey := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if val := strings.TrimSpace(os.Getenv(envKey)); val != "" {
			return val
		}
	}
	return ""
}

// ResolveProxy resolves proxy configuration in priority order:
// 1. Explicitly configured proxy URL
// 2. Shell environment variables (HTTPS_PROXY, HTTP_PROXY, ALL_PROXY)
// 3. Git configuration (git config --get https.proxy / http.proxy)
func ResolveProxy(explicit string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		return p
	}
	if p := getEnvProxy(); p != "" {
		return p
	}
	return getGitProxy()
}

func normalizeProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	return url.Parse(raw)
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

// NewClientWithOptions creates a new GitHub API client with custom options (proxy, TLS, timeouts)
func NewClientWithOptions(opts ClientOptions) (*Client, error) {
	if opts.Token == "" {
		var err error
		opts.Token, err = GetGHToken()
		if err != nil {
			return nil, err
		}
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	proxyStr := ResolveProxy(opts.ProxyURL)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: opts.InsecureSkipVerify,
	}

	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if certFile := os.Getenv("SSL_CERT_FILE"); certFile != "" {
		if certData, err := os.ReadFile(certFile); err == nil {
			rootCAs.AppendCertsFromPEM(certData)
		}
	}
	tlsConfig.RootCAs = rootCAs

	transport := &http.Transport{
		TLSClientConfig:       tlsConfig,
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if proxyStr != "" {
		proxyURL, err := normalizeProxyURL(proxyStr)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyStr, err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)

		// Propagate to child processes (gh, git)
		if os.Getenv("HTTPS_PROXY") == "" {
			_ = os.Setenv("HTTPS_PROXY", proxyStr)
		}
		if os.Getenv("HTTP_PROXY") == "" {
			_ = os.Setenv("HTTP_PROXY", proxyStr)
		}
	}

	if opts.InsecureSkipVerify {
		_ = os.Setenv("GIT_SSL_NO_VERIFY", "true")
	}

	// ETag Cache resolution
	cachePath := opts.CachePath
	if cachePath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			cachePath = filepath.Join(home, ".config", "grubby", "cache.json")
		}
	}
	var etagCache *ETagCache
	if cachePath != "" {
		etagCache = NewETagCache(cachePath)
	}

	transportWithCache := &ETagTransport{
		Base:  transport,
		Cache: etagCache,
	}

	return &Client{
		token: opts.Token,
		httpClient: &http.Client{
			Transport: transportWithCache,
			Timeout:   timeout,
		},
		proxyUsed: proxyStr,
		cache:     etagCache,
		rateLimit: RateLimitInfo{
			Remaining: 5000,
			Limit:     5000,
			Used:      0,
			Reset:     time.Now().Add(1 * time.Hour),
		},
		graphqlRateLimit: RateLimitInfo{
			Remaining: 5000,
			Limit:     5000,
			Used:      0,
			Reset:     time.Now().Add(1 * time.Hour),
		},
	}, nil
}

// NewClient creates a new GitHub API client using default options
func NewClient(token string) (*Client, error) {
	return NewClientWithOptions(ClientOptions{
		Token: token,
	})
}

func (c *Client) RateLimit() RateLimitInfo {
	return c.rateLimit
}

func (c *Client) GraphQLRateLimit() RateLimitInfo {
	return c.graphqlRateLimit
}

func (c *Client) updateRateLimits(resp *http.Response) {
	if resp == nil {
		return
	}

	target := &c.rateLimit
	if resp.Header.Get("X-Ratelimit-Resource") == "graphql" {
		target = &c.graphqlRateLimit
	}

	if rem := resp.Header.Get("X-Ratelimit-Remaining"); rem != "" {
		if val, err := strconv.Atoi(rem); err == nil {
			target.Remaining = val
		}
	}
	if lim := resp.Header.Get("X-Ratelimit-Limit"); lim != "" {
		if val, err := strconv.Atoi(lim); err == nil {
			target.Limit = val
		}
	}
	if used := resp.Header.Get("X-Ratelimit-Used"); used != "" {
		if val, err := strconv.Atoi(used); err == nil {
			target.Used = val
		}
	} else if target.Limit > 0 && target.Limit >= target.Remaining {
		target.Used = target.Limit - target.Remaining
	}
	if rst := resp.Header.Get("X-Ratelimit-Reset"); rst != "" {
		if val, err := strconv.ParseInt(rst, 10, 64); err == nil {
			target.Reset = time.Unix(val, 0)
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

	maxRetries := 2
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*500) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")

		resp, err = c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}

		c.updateRateLimits(resp)

		// Transient gateway / server errors: retry
		if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout {
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
			continue
		}

		// Non-retryable status or 200 OK
		break
	}

	if resp == nil || resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout {
		proxyNotice := ""
		if c.proxyUsed != "" {
			proxyNotice = fmt.Sprintf(" (via proxy: %s)", c.proxyUsed)
		} else {
			proxyNotice = " (no proxy configured; if behind a corporate proxy, set HTTPS_PROXY, git config http.proxy, or use --proxy)"
		}
		if lastErr != nil {
			return fmt.Errorf("graphql request failed%s: %w", proxyNotice, lastErr)
		}
		return fmt.Errorf("graphql request failed%s", proxyNotice)
	}
	defer resp.Body.Close()

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

// FetchRepositoriesAndPRs fetches repos, open PRs, commit events, and vulnerability summaries for an owner (org or user).
// Decoupled into optimized repository and indexed search queries to avoid GitHub 10s backend timeouts on large enterprise orgs.
func (c *Client) FetchRepositoriesAndPRs(ctx context.Context, owner string) ([]Repository, []PullRequest, []SquadEvent, map[string]SecurityAlertSummary, error) {
	// Query 1: Repositories, default branch commit history, and vulnerability alerts
	repoQueryWithAlerts := `
query($login: String!) {
  repositoryOwner(login: $login) {
    repositories(first: 50, orderBy: {field: PUSHED_AT, direction: DESC}) {
      nodes {
        name
        description
        pushedAt
        updatedAt
        isPrivate
        url
        sshUrl
        defaultBranchRef {
          name
          target {
            ... on Commit {
              history(first: 20) {
                nodes {
                  oid
                  message
                  committedDate
                  author {
                    name
                    user {
                      login
                    }
                  }
                }
              }
            }
          }
        }
        vulnerabilityAlerts(first: 25, states: OPEN) {
          totalCount
          nodes {
            securityVulnerability {
              severity
              package {
                name
              }
            }
          }
        }
      }
    }
  }
}
`
	type repoNode struct {
		Name             string    `json:"name"`
		Description      *string   `json:"description"`
		PushedAt         time.Time `json:"pushedAt"`
		UpdatedAt        time.Time `json:"updatedAt"`
		IsPrivate        bool      `json:"isPrivate"`
		URL              string    `json:"url"`
		SSHURL           string    `json:"sshUrl"`
		DefaultBranchRef *struct {
			Name   string `json:"name"`
			Target *struct {
				History struct {
					Nodes []struct {
						OID           string    `json:"oid"`
						Message       string    `json:"message"`
						CommittedDate time.Time `json:"committedDate"`
						Author        struct {
							Name string `json:"name"`
							User *struct {
								Login string `json:"login"`
							} `json:"user"`
						} `json:"author"`
					} `json:"nodes"`
				} `json:"history"`
			} `json:"target"`
		} `json:"defaultBranchRef"`
		VulnerabilityAlerts struct {
			TotalCount int `json:"totalCount"`
			Nodes      []struct {
				SecurityVulnerability struct {
					Severity string `json:"severity"`
					Package  struct {
						Name string `json:"name"`
					} `json:"package"`
				} `json:"securityVulnerability"`
			} `json:"nodes"`
		} `json:"vulnerabilityAlerts"`
	}

	var repoResp struct {
		RepositoryOwner struct {
			Repositories struct {
				Nodes []repoNode `json:"nodes"`
			} `json:"repositories"`
		} `json:"repositoryOwner"`
	}

	err := c.queryGraphQL(ctx, repoQueryWithAlerts, map[string]interface{}{"login": owner}, &repoResp)
	if err != nil {
		// Fallback: Query repositories without vulnerabilityAlerts if org permissions / Dependabot disabled
		repoQueryFallback := `
query($login: String!) {
  repositoryOwner(login: $login) {
    repositories(first: 50, orderBy: {field: PUSHED_AT, direction: DESC}) {
      nodes {
        name
        description
        pushedAt
        updatedAt
        isPrivate
        url
        sshUrl
        defaultBranchRef {
          name
          target {
            ... on Commit {
              history(first: 20) {
                nodes {
                  oid
                  message
                  committedDate
                  author {
                    name
                    user {
                      login
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}
`
		var fallbackResp struct {
			RepositoryOwner struct {
				Repositories struct {
					Nodes []repoNode `json:"nodes"`
				} `json:"repositories"`
			} `json:"repositoryOwner"`
		}
		fallbackErr := c.queryGraphQL(ctx, repoQueryFallback, map[string]interface{}{"login": owner}, &fallbackResp)
		if fallbackErr != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to fetch repositories: %w (fallback error: %v)", err, fallbackErr)
		}
		repoResp = fallbackResp
	}

	var repos []Repository
	var commitEvents []SquadEvent
	securityMap := make(map[string]SecurityAlertSummary)

	for _, node := range repoResp.RepositoryOwner.Repositories.Nodes {
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
			UpdatedAt:     node.UpdatedAt,
			DefaultBranch: defBranch,
			IsPrivate:     node.IsPrivate,
			URL:           node.URL,
			SSHURL:        node.SSHURL,
		}
		repos = append(repos, repo)

		// Parse commit events from default branch
		if node.DefaultBranchRef != nil && node.DefaultBranchRef.Target != nil {
			for _, cNode := range node.DefaultBranchRef.Target.History.Nodes {
				actor := ""
				if cNode.Author.User != nil && cNode.Author.User.Login != "" {
					actor = cNode.Author.User.Login
				} else if cNode.Author.Name != "" {
					actor = cNode.Author.Name
				}
				msg := strings.Split(cNode.Message, "\n")[0]
				if len(msg) > 60 {
					msg = msg[:57] + "..."
				}
				normRepo := NormalizeRepoName(node.Name)
				commitEvents = append(commitEvents, SquadEvent{
					Actor:     actor,
					Action:    "Committed",
					Repo:      normRepo,
					Target:    fmt.Sprintf("%s:%s", normRepo, defBranch),
					Details:   msg,
					Timestamp: cNode.CommittedDate,
				})
			}
		}

		// Parse Vulnerability Alerts
		crit, high, med, low := 0, 0, 0, 0
		var packages []string
		seenPkg := make(map[string]bool)
		for _, va := range node.VulnerabilityAlerts.Nodes {
			pkgName := va.SecurityVulnerability.Package.Name
			if pkgName != "" && !seenPkg[pkgName] {
				seenPkg[pkgName] = true
				packages = append(packages, pkgName)
			}
			switch strings.ToUpper(va.SecurityVulnerability.Severity) {
			case "CRITICAL":
				crit++
			case "HIGH":
				high++
			case "MODERATE":
				med++
			case "LOW":
				low++
			}
		}

		securityMap[node.Name] = SecurityAlertSummary{
			Repo:        node.Name,
			Critical:    crit,
			High:        high,
			Medium:      med,
			Low:         low,
			Total:       node.VulnerabilityAlerts.TotalCount,
			Packages:    packages,
			LastScanned: time.Now(),
		}
	}

	// Query 2: Fetch open PRs across the organization/user using GitHub indexed search API
	// This evaluates in <300ms across thousands of repos without triggering backend timeouts
	prSearchQuery := `
query($searchQuery: String!) {
  search(query: $searchQuery, type: ISSUE, first: 50) {
    issueCount
    nodes {
      ... on PullRequest {
        number
        title
        repository {
          name
        }
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
`
	var prResp struct {
		Search struct {
			IssueCount int `json:"issueCount"`
			Nodes      []struct {
				Number     int    `json:"number"`
				Title      string `json:"title"`
				Repository struct {
					Name string `json:"name"`
				} `json:"repository"`
				Author *struct {
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
		} `json:"search"`
	}

	searchString := fmt.Sprintf("is:pr is:open user:%s sort:updated-desc", owner)
	var prs []PullRequest

	if prErr := c.queryGraphQL(ctx, prSearchQuery, map[string]interface{}{"searchQuery": searchString}, &prResp); prErr == nil {
		for _, p := range prResp.Search.Nodes {
			if p.Number == 0 || p.Repository.Name == "" {
				continue
			}
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
				Repo:               p.Repository.Name,
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

	return repos, prs, commitEvents, securityMap, nil
}

type OrgMember struct {
	ID         string `json:"id"`
	DatabaseID int64  `json:"database_id"`
	Login      string `json:"login"`
	Name       string `json:"name"`
	AvatarURL  string `json:"avatar_url"`
}

func (m OrgMember) FormattedID() string {
	if m.DatabaseID > 0 {
		return fmt.Sprintf("%d", m.DatabaseID)
	}
	return m.ID
}

// FetchOrgMembers retrieves the members of an organization (up to 100)
func (c *Client) FetchOrgMembers(ctx context.Context, org string) ([]OrgMember, error) {
	query := `
query($login: String!) {
  organization(login: $login) {
    membersWithRole(first: 100) {
      nodes {
        id
        databaseId
        login
        name
        avatarUrl(size: 64)
      }
    }
  }
}
`
	var resp struct {
		Organization *struct {
			MembersWithRole struct {
				Nodes []struct {
					ID         string  `json:"id"`
					DatabaseID *int64  `json:"databaseId"`
					Login      string  `json:"login"`
					Name       *string `json:"name"`
					AvatarURL  string  `json:"avatarUrl"`
				} `json:"nodes"`
			} `json:"membersWithRole"`
		} `json:"organization"`
	}

	err := c.queryGraphQL(ctx, query, map[string]interface{}{"login": org}, &resp)
	if err == nil && resp.Organization != nil && len(resp.Organization.MembersWithRole.Nodes) > 0 {
		var members []OrgMember
		for _, n := range resp.Organization.MembersWithRole.Nodes {
			name := n.Login
			if n.Name != nil && strings.TrimSpace(*n.Name) != "" {
				name = *n.Name
			}
			var dbID int64
			if n.DatabaseID != nil {
				dbID = *n.DatabaseID
			}
			members = append(members, OrgMember{
				ID:         n.ID,
				DatabaseID: dbID,
				Login:      n.Login,
				Name:       name,
				AvatarURL:  n.AvatarURL,
			})
		}
		return members, nil
	}

	// Fallback to REST API /orgs/{org}/members
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.github.com/orgs/%s/members?per_page=100", org), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	c.updateRateLimits(httpResp)

	if httpResp.StatusCode == http.StatusOK {
		var raw []struct {
			ID        int64  `json:"id"`
			NodeID    string `json:"node_id"`
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		}
		if err := json.NewDecoder(httpResp.Body).Decode(&raw); err == nil && len(raw) > 0 {
			var members []OrgMember
			for _, r := range raw {
				members = append(members, OrgMember{
					ID:         r.NodeID,
					DatabaseID: r.ID,
					Login:      r.Login,
					Name:       r.Login,
					AvatarURL:  r.AvatarURL,
				})
			}
			return members, nil
		}
	}

	return nil, fmt.Errorf("no members found for org %s", org)
}

// FetchUserProfile retrieves detailed profile information (id, name, avatarUrl) for a user.
func (c *Client) FetchUserProfile(ctx context.Context, login string) (*OrgMember, error) {
	query := `
query($login: String!) {
  user(login: $login) {
    id
    databaseId
    login
    name
    avatarUrl(size: 64)
  }
}
`
	var resp struct {
		User *struct {
			ID         string  `json:"id"`
			DatabaseID *int64  `json:"databaseId"`
			Login      string  `json:"login"`
			Name       *string `json:"name"`
			AvatarURL  string  `json:"avatarUrl"`
		} `json:"user"`
	}

	err := c.queryGraphQL(ctx, query, map[string]interface{}{"login": login}, &resp)
	if err == nil && resp.User != nil {
		name := resp.User.Login
		if resp.User.Name != nil && strings.TrimSpace(*resp.User.Name) != "" {
			name = *resp.User.Name
		}
		var dbID int64
		if resp.User.DatabaseID != nil {
			dbID = *resp.User.DatabaseID
		}
		return &OrgMember{
			ID:         resp.User.ID,
			DatabaseID: dbID,
			Login:      resp.User.Login,
			Name:       name,
			AvatarURL:  resp.User.AvatarURL,
		}, nil
	}

	// Fallback to REST API /users/{login}
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.github.com/users/%s", url.PathEscape(login)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	c.updateRateLimits(httpResp)

	if httpResp.StatusCode == http.StatusOK {
		var raw struct {
			ID        int64   `json:"id"`
			NodeID    string  `json:"node_id"`
			Login     string  `json:"login"`
			Name      *string `json:"name"`
			AvatarURL string  `json:"avatar_url"`
		}
		if err := json.NewDecoder(httpResp.Body).Decode(&raw); err == nil {
			name := raw.Login
			if raw.Name != nil && strings.TrimSpace(*raw.Name) != "" {
				name = *raw.Name
			}
			return &OrgMember{
				ID:         raw.NodeID,
				DatabaseID: raw.ID,
				Login:      raw.Login,
				Name:       name,
				AvatarURL:  raw.AvatarURL,
			}, nil
		}
	}

	return nil, fmt.Errorf("user not found: %s", login)
}

// FetchUserProfiles retrieves detailed profile information for multiple users.
// It uses aliased GraphQL queries to fetch all members in a single request.
func (c *Client) FetchUserProfiles(ctx context.Context, logins []string) (map[string]OrgMember, error) {
	profiles := make(map[string]OrgMember)
	if len(logins) == 0 {
		return profiles, nil
	}

	// Deduplicate logins
	unique := make(map[string]string)
	for _, l := range logins {
		lClean := strings.TrimSpace(l)
		if lClean != "" {
			unique[strings.ToLower(lClean)] = lClean
		}
	}

	var cleanLogins []string
	for _, l := range unique {
		cleanLogins = append(cleanLogins, l)
	}

	// Build batch GraphQL query
	var queryParts []string
	queryParts = append(queryParts, "query {")
	for i, l := range cleanLogins {
		escaped := strings.ReplaceAll(l, `"`, `\"`)
		queryParts = append(queryParts, fmt.Sprintf(`  u%d: user(login: "%s") { id databaseId login name avatarUrl(size: 64) }`, i, escaped))
	}
	queryParts = append(queryParts, "}")
	query := strings.Join(queryParts, "\n")

	var rawResp map[string]interface{}
	err := c.queryGraphQL(ctx, query, nil, &rawResp)
	if err == nil && rawResp != nil {
		for i, l := range cleanLogins {
			alias := fmt.Sprintf("u%d", i)
			if uData, ok := rawResp[alias].(map[string]interface{}); ok && uData != nil {
				loginStr, _ := uData["login"].(string)
				if loginStr == "" {
					loginStr = l
				}
				idStr, _ := uData["id"].(string)
				var dbID int64
				if f, ok := uData["databaseId"].(float64); ok {
					dbID = int64(f)
				}
				nameStr := loginStr
				if n, ok := uData["name"].(string); ok && strings.TrimSpace(n) != "" {
					nameStr = n
				}
				avatarURL, _ := uData["avatarUrl"].(string)

				profiles[strings.ToLower(loginStr)] = OrgMember{
					ID:         idStr,
					DatabaseID: dbID,
					Login:      loginStr,
					Name:       nameStr,
					AvatarURL:  avatarURL,
				}
			}
		}
	}

	// For any login that was not resolved, try individual FetchUserProfile as fallback
	for _, l := range cleanLogins {
		if _, ok := profiles[strings.ToLower(l)]; !ok {
			if prof, err := c.FetchUserProfile(ctx, l); err == nil && prof != nil {
				profiles[strings.ToLower(prof.Login)] = *prof
			}
		}
	}

	return profiles, nil
}

// DownloadAvatar downloads an avatar image using the configured HTTP client (respecting proxy/TLS).
func (c *Client) DownloadAvatar(ctx context.Context, avatarURL string) ([]byte, error) {
	if avatarURL == "" {
		return nil, fmt.Errorf("empty avatar url")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", avatarURL, nil)
	if err != nil {
		return nil, err
	}
	if strings.Contains(avatarURL, "api.github.com") {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("avatar download failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
}

// FetchSquadActivity retrieves public and private events for a list of squad members and merges with commit history
func (c *Client) FetchSquadActivity(ctx context.Context, members []string, commitEvents []SquadEvent) ([]SquadEvent, error) {
	var events []SquadEvent
	if len(members) == 0 {
		return events, nil
	}

	memberMap := make(map[string]bool)
	for _, m := range members {
		memberMap[strings.ToLower(strings.TrimSpace(m))] = true
	}

	// Filter commitEvents to only include squad members
	for _, ce := range commitEvents {
		if memberMap[strings.ToLower(ce.Actor)] {
			events = append(events, ce)
		}
	}

	for _, member := range members {
		// Use /users/{member}/events which returns both public and private events for authenticated user/collaborators
		url := fmt.Sprintf("https://api.github.com/users/%s/events?per_page=100", member)
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
				repoName := NormalizeRepoName(ev.Repo.Name)
				target := repoName

				switch ev.Type {
				case "PushEvent":
					var p struct {
						Ref     string `json:"ref"`
						Commits []struct {
							Message string `json:"message"`
						} `json:"commits"`
					}
					_ = json.Unmarshal(ev.Payload, &p)
					branch := strings.TrimPrefix(p.Ref, "refs/heads/")
					if branch != "" {
						target = fmt.Sprintf("%s:%s", repoName, branch)
					}
					if len(p.Commits) > 0 {
						action = fmt.Sprintf("Pushed %d commit", len(p.Commits))
						if len(p.Commits) > 1 {
							action += "s"
						}
						details = strings.Split(p.Commits[0].Message, "\n")[0]
					} else {
						action = "Pushed commits"
					}
				case "CreateEvent":
					var p struct {
						Ref     string `json:"ref"`
						RefType string `json:"ref_type"`
					}
					_ = json.Unmarshal(ev.Payload, &p)
					action = fmt.Sprintf("Created %s", p.RefType)
					if p.Ref != "" {
						details = p.Ref
					}
				case "DeleteEvent":
					var p struct {
						Ref     string `json:"ref"`
						RefType string `json:"ref_type"`
					}
					_ = json.Unmarshal(ev.Payload, &p)
					action = fmt.Sprintf("Deleted %s", p.RefType)
					if p.Ref != "" {
						details = p.Ref
					}
				case "PullRequestEvent":
					var p struct {
						Action      string `json:"action"`
						PullRequest struct {
							Number int    `json:"number"`
							Title  string `json:"title"`
							Merged bool   `json:"merged"`
						} `json:"pull_request"`
					}
					_ = json.Unmarshal(ev.Payload, &p)
					if p.Action == "closed" && p.PullRequest.Merged {
						action = "PR merged"
					} else {
						action = fmt.Sprintf("PR %s", p.Action)
					}
					target = fmt.Sprintf("%s#%d", repoName, p.PullRequest.Number)
					details = p.PullRequest.Title
				case "PullRequestReviewEvent":
					var p struct {
						Action string `json:"action"`
						Review struct {
							State string `json:"state"`
						} `json:"review"`
						PullRequest struct {
							Number int    `json:"number"`
							Title  string `json:"title"`
						} `json:"pull_request"`
					}
					_ = json.Unmarshal(ev.Payload, &p)
					action = fmt.Sprintf("Review %s", strings.ToLower(p.Review.State))
					target = fmt.Sprintf("%s#%d", repoName, p.PullRequest.Number)
					details = p.PullRequest.Title
				case "IssueCommentEvent":
					action = "Commented"
					var p struct {
						Issue struct {
							Number int `json:"number"`
						} `json:"issue"`
						Comment struct {
							Body string `json:"body"`
						} `json:"comment"`
					}
					_ = json.Unmarshal(ev.Payload, &p)
					if p.Issue.Number > 0 {
						target = fmt.Sprintf("%s#%d", repoName, p.Issue.Number)
					}
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
					Repo:      repoName,
					Target:    target,
					Details:   details,
					Timestamp: ev.CreatedAt,
				})
			}
		}
		resp.Body.Close()
	}

	// Deduplicate events that might appear as both PushEvent and Commit
	var deduplicated []SquadEvent
	seen := make(map[string]bool)
	for _, ev := range events {
		key := fmt.Sprintf("%s|%s|%s|%d", ev.Actor, NormalizeRepoName(ev.Repo), ev.Details, ev.Timestamp.Unix())
		if !seen[key] {
			seen[key] = true
			deduplicated = append(deduplicated, ev)
		}
	}

	// Sort events by timestamp descending (newest first)
	for i := 0; i < len(deduplicated)-1; i++ {
		for j := i + 1; j < len(deduplicated); j++ {
			if deduplicated[j].Timestamp.After(deduplicated[i].Timestamp) {
				deduplicated[i], deduplicated[j] = deduplicated[j], deduplicated[i]
			}
		}
	}

	return deduplicated, nil
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

// FetchNotifications retrieves unread and recent GitHub notifications for the authenticated user
func (c *Client) FetchNotifications(ctx context.Context) ([]Notification, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/notifications?all=true", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.updateRateLimits(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch notifications: HTTP %d", resp.StatusCode)
	}

	var rawList []struct {
		ID        string    `json:"id"`
		Unread    bool      `json:"unread"`
		Reason    string    `json:"reason"`
		UpdatedAt time.Time `json:"updated_at"`
		Subject   struct {
			Title string `json:"title"`
			URL   string `json:"url"`
			Type  string `json:"type"`
		} `json:"subject"`
		Repository struct {
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			HTMLURL  string `json:"html_url"`
		} `json:"repository"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawList); err != nil {
		return nil, err
	}

	notifications := make([]Notification, 0, len(rawList))
	for _, raw := range rawList {
		webURL := formatNotificationWebURL(raw.Subject.URL, raw.Repository.HTMLURL)

		notifications = append(notifications, Notification{
			ID:         raw.ID,
			Unread:     raw.Unread,
			Reason:     raw.Reason,
			UpdatedAt:  raw.UpdatedAt,
			Repository: raw.Repository.FullName,
			RepoURL:    raw.Repository.HTMLURL,
			Title:      raw.Subject.Title,
			Type:       raw.Subject.Type,
			URL:        webURL,
		})
	}

	return notifications, nil
}

// MarkNotificationRead marks a notification thread as read on GitHub
func (c *Client) MarkNotificationRead(ctx context.Context, threadID string) error {
	req, err := http.NewRequestWithContext(ctx, "PATCH", "https://api.github.com/notifications/threads/"+threadID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.updateRateLimits(resp)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusResetContent {
		return nil
	}
	return fmt.Errorf("failed to mark notification as read: HTTP %d", resp.StatusCode)
}

// MarkAllNotificationsRead marks all notifications as read on GitHub
func (c *Client) MarkAllNotificationsRead(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "PUT", "https://api.github.com/notifications", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.updateRateLimits(resp)

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusResetContent || resp.StatusCode == http.StatusAccepted {
		return nil
	}
	return fmt.Errorf("failed to mark all notifications as read: HTTP %d", resp.StatusCode)
}

func formatNotificationWebURL(apiURL, repoHTMLURL string) string {
	if apiURL != "" && strings.Contains(apiURL, "api.github.com/repos/") {
		webURL := strings.Replace(apiURL, "https://api.github.com/repos/", "https://github.com/", 1)
		webURL = strings.Replace(webURL, "/pulls/", "/pull/", 1)
		return webURL
	}
	return repoHTMLURL
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

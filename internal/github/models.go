package github

import (
	"strings"
	"time"
)

// NormalizeRepoName normalizes a repository identifier by stripping any URL prefixes,
// owner/organization prefix (e.g. "org/repo" -> "repo"), trailing .git, and whitespace.
func NormalizeRepoName(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "https://github.com/")
	s = strings.TrimPrefix(s, "http://github.com/")
	s = strings.TrimPrefix(s, "git@github.com:")
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")
	if idx := strings.LastIndex(s, "/"); idx != -1 {
		s = s[idx+1:]
	}
	if s == "" {
		return "unknown"
	}
	return strings.TrimSpace(s)
}

type Repository struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	PushedAt      time.Time `json:"pushed_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	DefaultBranch string    `json:"default_branch"`
	IsPrivate     bool      `json:"is_private"`
	URL           string    `json:"url"`
	SSHURL        string    `json:"ssh_url"`
}

// LastUpdated returns the latest timestamp between PushedAt and UpdatedAt.
func (r Repository) LastUpdated() time.Time {
	if r.PushedAt.After(r.UpdatedAt) {
		return r.PushedAt
	}
	if !r.UpdatedAt.IsZero() {
		return r.UpdatedAt
	}
	return r.PushedAt
}

type PullRequest struct {
	Number             int       `json:"number"`
	Repo               string    `json:"repo"`
	Title              string    `json:"title"`
	Author             string    `json:"author"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	State              string    `json:"state"`
	IsDraft            bool      `json:"is_draft"`
	ReviewDecision     string    `json:"review_decision"` // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, NONE
	Labels             []string  `json:"labels"`
	TotalComments      int       `json:"total_comments"`
	UnresolvedComments int       `json:"unresolved_comments"`
	URL                string    `json:"url"`
	HeadRefName        string    `json:"head_ref_name"`
}

func (pr PullRequest) DaysStale() int {
	if pr.UpdatedAt.IsZero() {
		return 0
	}
	days := int(time.Since(pr.UpdatedAt).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

type SquadEvent struct {
	Actor     string    `json:"actor"`
	Action    string    `json:"action"` // Opened PR, Merged PR, Commented, Pushed, Approved
	Repo      string    `json:"repo"`
	Target    string    `json:"target"`
	Details   string    `json:"details"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
}

type SecurityAlertSummary struct {
	Repo        string    `json:"repo"`
	Critical    int       `json:"critical"`
	High        int       `json:"high"`
	Medium      int       `json:"medium"`
	Low         int       `json:"low"`
	Total       int       `json:"total"`
	Packages    []string  `json:"packages"`
	LastScanned time.Time `json:"last_scanned"`
}

type Notification struct {
	ID         string    `json:"id"`
	Unread     bool      `json:"unread"`
	Reason     string    `json:"reason"`
	UpdatedAt  time.Time `json:"updated_at"`
	Repository string    `json:"repository"`
	RepoURL    string    `json:"repo_url"`
	Title      string    `json:"title"`
	Type       string    `json:"type"`
	URL        string    `json:"url"`
}

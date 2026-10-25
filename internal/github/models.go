package github

import "time"

type Repository struct {
	Name          string    `json:"name"`
	FullName      string    `json:"full_name"`
	Description   string    `json:"description"`
	PushedAt      time.Time `json:"pushed_at"`
	DefaultBranch string    `json:"default_branch"`
	IsPrivate     bool      `json:"is_private"`
	URL           string    `json:"url"`
	SSHURL        string    `json:"ssh_url"`
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

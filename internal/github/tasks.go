package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TaskPriority string

const (
	PriorityHigh   TaskPriority = "high"
	PriorityMedium TaskPriority = "medium"
	PriorityLow    TaskPriority = "low"

	LabelPRReview = "pr-review"
)

// Rank returns sorting rank (1 = high, 2 = medium, 3 = low).
func (p TaskPriority) Rank() int {
	switch p {
	case PriorityHigh:
		return 1
	case PriorityMedium:
		return 2
	case PriorityLow:
		return 3
	default:
		return 2 // Default is medium
	}
}

// Label returns the GitHub label associated with this priority.
func (p TaskPriority) Label() string {
	switch p {
	case PriorityHigh:
		return "priority: high"
	case PriorityMedium:
		return "priority: medium"
	case PriorityLow:
		return "priority: low"
	default:
		return "priority: medium"
	}
}

// ParsePriority inspects issue labels and determines the priority.
// "if there are no labels on the task it is deemed medium priority."
func ParsePriority(labels []string) TaskPriority {
	for _, l := range labels {
		lower := strings.ToLower(strings.TrimSpace(l))
		if lower == "priority: high" || lower == "high" || lower == "p1" || lower == "priority-high" {
			return PriorityHigh
		}
		if lower == "priority: low" || lower == "low" || lower == "p3" || lower == "priority-low" {
			return PriorityLow
		}
		if lower == "priority: medium" || lower == "medium" || lower == "p2" || lower == "priority-medium" || lower == "normal" {
			return PriorityMedium
		}
	}
	return PriorityMedium
}

// FilterTasksForSquad filters a list of tasks for the specified squad name.
// If a task has a "squad:<name>" label, it only matches if <name> equals squadName.
// If a task has NO squad label (e.g. legacy tasks), it is included by default for backwards compatibility.
func FilterTasksForSquad(tasks []Task, squadName string) []Task {
	if strings.TrimSpace(squadName) == "" {
		return tasks
	}
	targetLabel := "squad:" + strings.ToLower(strings.TrimSpace(squadName))
	var filtered []Task
	for _, t := range tasks {
		hasSquadLabel := false
		matchesSquad := false
		for _, l := range t.Labels {
			clean := strings.ToLower(strings.TrimSpace(l))
			if strings.HasPrefix(clean, "squad:") {
				hasSquadLabel = true
				if clean == targetLabel {
					matchesSquad = true
					break
				}
			}
		}
		if matchesSquad || !hasSquadLabel {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

type Task struct {
	ID          int64        `json:"id"`
	Number      int          `json:"number"`
	Title       string       `json:"title"`
	Body        string       `json:"body"`
	State       string       `json:"state"` // "open" or "closed"
	Priority    TaskPriority `json:"priority"`
	RepoOwner   string       `json:"repo_owner"`
	RepoName    string       `json:"repo_name"`
	Author      string       `json:"author"`
	Assignees   []string     `json:"assignees"`
	Labels      []string     `json:"labels"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	ClosedAt    *time.Time   `json:"closed_at,omitempty"`
	URL         string       `json:"html_url"`
	IsSquadTask bool         `json:"is_squad_task"`
}

// IsPRReview returns true if this task represents a pull request review.
func (t Task) IsPRReview() bool {
	for _, l := range t.Labels {
		clean := strings.ToLower(strings.TrimSpace(l))
		if clean == LabelPRReview || clean == "pr review" || clean == "pr_review" {
			return true
		}
	}
	return false
}

// PRURL extracts the linked pull request URL from the task body or returns an empty string.
func (t Task) PRURL() string {
	for _, word := range strings.Fields(t.Body) {
		idx := strings.Index(word, "https://github.com/")
		if idx >= 0 {
			sub := word[idx:]
			if strings.Contains(sub, "/pull/") {
				sub = strings.TrimRight(sub, ".,);\"'>)]")
				return sub
			}
		}
	}
	return ""
}

// ParsePRURL parses a GitHub pull request URL into owner, repo, and PR number.
func ParsePRURL(prURL string) (owner, repo string, number int, ok bool) {
	prURL = strings.TrimSpace(prURL)
	idx := strings.Index(prURL, "github.com/")
	if idx == -1 {
		return "", "", 0, false
	}
	path := prURL[idx+len("github.com/"):]
	parts := strings.Split(path, "/")
	if len(parts) >= 4 && parts[2] == "pull" {
		owner = parts[0]
		repo = parts[1]
		num, err := strconv.Atoi(parts[3])
		if err == nil && num > 0 {
			return owner, repo, num, true
		}
	}
	return "", "", 0, false
}

type rawGithubIssue struct {
	ID        int64      `json:"id"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	HTMLURL   string     `json:"html_url"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
	Labels []struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	} `json:"labels"`
	PullRequest *struct{} `json:"pull_request,omitempty"`
}

// EnsureTaskRepository checks if a backing repository exists, and creates it if not.
func (c *Client) EnsureTaskRepository(ctx context.Context, owner, repo string, isOrg bool) error {
	cleanOwner := strings.TrimSpace(owner)
	cleanRepo := strings.TrimSpace(repo)
	if cleanOwner == "" || cleanRepo == "" {
		return fmt.Errorf("owner and repo must not be empty")
	}

	// 1. Check if repo already exists
	checkURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", url.PathEscape(cleanOwner), url.PathEscape(cleanRepo))
	req, err := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Repo exists; ensure priority labels exist
		_ = c.ensurePriorityLabels(ctx, cleanOwner, cleanRepo)
		return nil
	}

	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unexpected status checking repo: %d", resp.StatusCode)
	}

	// 2. Repo does not exist, create it
	createBody, _ := json.Marshal(map[string]interface{}{
		"name":        cleanRepo,
		"description": "Tasks repository managed by Grubby",
		"private":     true,
		"auto_init":   true,
	})

	var createURL string
	if isOrg {
		createURL = fmt.Sprintf("https://api.github.com/orgs/%s/repos", url.PathEscape(cleanOwner))
	} else {
		createURL = "https://api.github.com/user/repos"
	}

	creq, err := http.NewRequestWithContext(ctx, "POST", createURL, bytes.NewReader(createBody))
	if err != nil {
		return err
	}
	creq.Header.Set("Authorization", "Bearer "+c.token)
	creq.Header.Set("Accept", "application/vnd.github+json")
	creq.Header.Set("Content-Type", "application/json")

	cresp, err := c.do(creq)
	if err != nil {
		return err
	}
	c.updateRateLimits(cresp)
	defer cresp.Body.Close()

	// If org creation failed (e.g. 404/403 because user isn't org admin or owner is user account), fallback to user repos
	if isOrg && (cresp.StatusCode == http.StatusNotFound || cresp.StatusCode == http.StatusForbidden) {
		fallbackReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/user/repos", bytes.NewReader(createBody))
		if err == nil {
			fallbackReq.Header.Set("Authorization", "Bearer "+c.token)
			fallbackReq.Header.Set("Accept", "application/vnd.github+json")
			fallbackReq.Header.Set("Content-Type", "application/json")
			if fbResp, fbErr := c.do(fallbackReq); fbErr == nil {
				c.updateRateLimits(fbResp)
				fbResp.Body.Close()
			}
		}
	}

	_ = c.ensurePriorityLabels(ctx, cleanOwner, cleanRepo)
	return nil
}

func (c *Client) ensurePriorityLabels(ctx context.Context, owner, repo string) error {
	labels := []struct {
		Name  string `json:"name"`
		Color string `json:"color"`
		Desc  string `json:"description"`
	}{
		{Name: "priority: high", Color: "d73a4a", Desc: "High priority task"},
		{Name: "priority: medium", Color: "fbca04", Desc: "Medium priority task"},
		{Name: "priority: low", Color: "0e8a16", Desc: "Low priority task"},
		{Name: LabelPRReview, Color: "8250df", Desc: "Pull request review task"},
	}

	for _, l := range labels {
		body, _ := json.Marshal(l)
		labelURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/labels", url.PathEscape(owner), url.PathEscape(repo))
		req, err := http.NewRequestWithContext(ctx, "POST", labelURL, bytes.NewReader(body))
		if err != nil {
			continue
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Content-Type", "application/json")

		if resp, err := c.do(req); err == nil {
			c.updateRateLimits(resp)
			resp.Body.Close()
		}
	}
	return nil
}

// FetchTasks retrieves open and closed issues from the specified repository and maps them into Tasks.
func (c *Client) FetchTasks(ctx context.Context, owner, repo string, isSquad bool) ([]Task, error) {
	return c.FetchTasksFiltered(ctx, owner, repo, "", isSquad)
}

// FetchTasksFiltered retrieves open and closed issues from the specified repository, optionally filtered by a GitHub label.
func (c *Client) FetchTasksFiltered(ctx context.Context, owner, repo string, labelFilter string, isSquad bool) ([]Task, error) {
	cleanOwner := strings.TrimSpace(owner)
	cleanRepo := strings.TrimSpace(repo)
	if cleanOwner == "" || cleanRepo == "" {
		return nil, nil
	}

	fetchURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues?state=all&per_page=100&sort=updated&direction=desc",
		url.PathEscape(cleanOwner), url.PathEscape(cleanRepo))
	if labelFilter != "" {
		fetchURL += fmt.Sprintf("&labels=%s", url.QueryEscape(labelFilter))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch tasks from %s/%s: status %d", cleanOwner, cleanRepo, resp.StatusCode)
	}

	var rawIssues []rawGithubIssue
	if err := json.NewDecoder(resp.Body).Decode(&rawIssues); err != nil {
		return nil, fmt.Errorf("failed to decode issues: %w", err)
	}

	var tasks []Task
	for _, raw := range rawIssues {
		// Ignore pull requests returned by GitHub's issues endpoint
		if raw.PullRequest != nil {
			continue
		}

		var labelNames []string
		for _, l := range raw.Labels {
			labelNames = append(labelNames, l.Name)
		}

		var assignees []string
		for _, a := range raw.Assignees {
			assignees = append(assignees, a.Login)
		}

		priority := ParsePriority(labelNames)

		tasks = append(tasks, Task{
			ID:          raw.ID,
			Number:      raw.Number,
			Title:       raw.Title,
			Body:        raw.Body,
			State:       raw.State,
			Priority:    priority,
			RepoOwner:   cleanOwner,
			RepoName:    cleanRepo,
			Author:      raw.User.Login,
			Assignees:   assignees,
			Labels:      labelNames,
			CreatedAt:   raw.CreatedAt,
			UpdatedAt:   raw.UpdatedAt,
			ClosedAt:    raw.ClosedAt,
			URL:         raw.HTMLURL,
			IsSquadTask: isSquad,
		})
	}

	return tasks, nil
}

// CreateTask creates a new issue in the target repository with default priority label.
func (c *Client) CreateTask(ctx context.Context, owner, repo, title, body string, priority TaskPriority, assignees []string, isSquad bool) (*Task, error) {
	return c.CreateTaskWithLabels(ctx, owner, repo, title, body, priority, nil, assignees, isSquad)
}

// CreateTaskWithLabels creates a new issue in the target repository with specified extra labels (such as pr-review).
func (c *Client) CreateTaskWithLabels(ctx context.Context, owner, repo, title, body string, priority TaskPriority, extraLabels []string, assignees []string, isSquad bool) (*Task, error) {
	cleanOwner := strings.TrimSpace(owner)
	cleanRepo := strings.TrimSpace(repo)
	cleanTitle := strings.TrimSpace(title)

	if cleanOwner == "" || cleanRepo == "" {
		return nil, fmt.Errorf("repo owner and name required")
	}
	if cleanTitle == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}

	// Ensure repo exists
	_ = c.EnsureTaskRepository(ctx, cleanOwner, cleanRepo, isSquad)

	labels := []string{priority.Label()}
	for _, l := range extraLabels {
		clean := strings.TrimSpace(l)
		if clean != "" && clean != priority.Label() {
			labels = append(labels, clean)
		}
	}

	payload := map[string]interface{}{
		"title":  cleanTitle,
		"body":   body,
		"labels": labels,
	}
	if len(assignees) > 0 {
		payload["assignees"] = assignees
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	postURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", url.PathEscape(cleanOwner), url.PathEscape(cleanRepo))
	req, err := http.NewRequestWithContext(ctx, "POST", postURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create task on %s/%s: status %d: %s", cleanOwner, cleanRepo, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var raw rawGithubIssue
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode created task: %w", err)
	}

	var labelNames []string
	for _, l := range raw.Labels {
		labelNames = append(labelNames, l.Name)
	}
	var assignedLogins []string
	for _, a := range raw.Assignees {
		assignedLogins = append(assignedLogins, a.Login)
	}

	task := &Task{
		ID:          raw.ID,
		Number:      raw.Number,
		Title:       raw.Title,
		Body:        raw.Body,
		State:       raw.State,
		Priority:    priority,
		RepoOwner:   cleanOwner,
		RepoName:    cleanRepo,
		Author:      raw.User.Login,
		Assignees:   assignedLogins,
		Labels:      labelNames,
		CreatedAt:   raw.CreatedAt,
		UpdatedAt:   raw.UpdatedAt,
		URL:         raw.HTMLURL,
		IsSquadTask: isSquad,
	}

	return task, nil
}

// CloseTask closes an issue (marks as completed).
func (c *Client) CloseTask(ctx context.Context, owner, repo string, number int) error {
	payload := map[string]string{
		"state": "closed",
	}
	jsonBytes, _ := json.Marshal(payload)

	patchURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", url.PathEscape(owner), url.PathEscape(repo), number)
	req, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to close task #%d: status %d", number, resp.StatusCode)
	}
	return nil
}

// IsPRClosedOrMerged checks if a pull request is closed or merged.
func (c *Client) IsPRClosedOrMerged(ctx context.Context, owner, repo string, number int) (bool, error) {
	cleanOwner := strings.TrimSpace(owner)
	cleanRepo := strings.TrimSpace(repo)
	if cleanOwner == "" || cleanRepo == "" || number <= 0 {
		return false, fmt.Errorf("invalid PR coordinates")
	}

	getURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", url.PathEscape(cleanOwner), url.PathEscape(cleanRepo), number)
	req, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.do(req)
	if err != nil {
		return false, err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to fetch PR #%d: status %d", number, resp.StatusCode)
	}

	var rawPR struct {
		State  string `json:"state"`
		Merged bool   `json:"merged"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawPR); err != nil {
		return false, err
	}

	return rawPR.State == "closed" || rawPR.Merged, nil
}

// LinkPRToIssue updates the PR description to include a closing reference ("Resolves <issueURL>")
// so that GitHub natively links the issue in the PR sidebar and automatically closes it when merged into the default branch.
func (c *Client) LinkPRToIssue(ctx context.Context, prOwner, prRepo string, prNumber int, issueURL string) error {
	cleanOwner := strings.TrimSpace(prOwner)
	cleanRepo := strings.TrimSpace(prRepo)
	cleanIssueURL := strings.TrimSpace(issueURL)
	if cleanOwner == "" || cleanRepo == "" || prNumber <= 0 || cleanIssueURL == "" {
		return fmt.Errorf("invalid parameters for linking PR to issue")
	}

	getURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", url.PathEscape(cleanOwner), url.PathEscape(cleanRepo), prNumber)
	req, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch PR #%d: status %d", prNumber, resp.StatusCode)
	}

	var rawPR struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawPR); err != nil {
		return err
	}

	// If already linked, nothing to do
	if strings.Contains(rawPR.Body, cleanIssueURL) {
		return nil
	}

	// Append closing reference
	updatedBody := rawPR.Body
	if strings.TrimSpace(updatedBody) == "" {
		updatedBody = fmt.Sprintf("Resolves %s", cleanIssueURL)
	} else {
		updatedBody = fmt.Sprintf("%s\n\nResolves %s", updatedBody, cleanIssueURL)
	}

	patchPayload, err := json.Marshal(map[string]string{
		"body": updatedBody,
	})
	if err != nil {
		return err
	}

	patchReq, err := http.NewRequestWithContext(ctx, "PATCH", getURL, bytes.NewReader(patchPayload))
	if err != nil {
		return err
	}
	patchReq.Header.Set("Authorization", "Bearer "+c.token)
	patchReq.Header.Set("Accept", "application/vnd.github+json")
	patchReq.Header.Set("Content-Type", "application/json")

	patchResp, err := c.do(patchReq)
	if err != nil {
		return err
	}
	c.updateRateLimits(patchResp)
	defer patchResp.Body.Close()

	if patchResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update PR #%d description: status %d", prNumber, patchResp.StatusCode)
	}
	return nil
}

type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

type PRStatus struct {
	State    string
	Merged   bool
	MergedAt *time.Time
	ClosedAt *time.Time
}

// BatchGetPRStatuses retrieves the state and merged status of multiple pull requests in a single GraphQL call.
func (c *Client) BatchGetPRStatuses(ctx context.Context, refs []PRRef) (map[PRRef]PRStatus, error) {
	result := make(map[PRRef]PRStatus)
	if len(refs) == 0 {
		return result, nil
	}

	unique := make(map[PRRef]bool)
	var cleanRefs []PRRef
	for _, r := range refs {
		if r.Owner != "" && r.Repo != "" && r.Number > 0 && !unique[r] {
			unique[r] = true
			cleanRefs = append(cleanRefs, r)
		}
	}
	if len(cleanRefs) == 0 {
		return result, nil
	}

	var queryParts []string
	queryParts = append(queryParts, "query {")
	for i, r := range cleanRefs {
		escapedOwner := strings.ReplaceAll(r.Owner, `"`, `\"`)
		escapedRepo := strings.ReplaceAll(r.Repo, `"`, `\"`)
		queryParts = append(queryParts, fmt.Sprintf(
			`  p%d: repository(owner: "%s", name: "%s") { pullRequest(number: %d) { state merged mergedAt closedAt } }`,
			i, escapedOwner, escapedRepo, r.Number,
		))
	}
	queryParts = append(queryParts, "}")
	query := strings.Join(queryParts, "\n")

	var rawResp map[string]map[string]struct {
		State    string     `json:"state"`
		Merged   bool       `json:"merged"`
		MergedAt *time.Time `json:"mergedAt"`
		ClosedAt *time.Time `json:"closedAt"`
	}

	if err := c.queryGraphQL(ctx, query, nil, &rawResp); err != nil {
		return nil, err
	}

	for i, r := range cleanRefs {
		key := fmt.Sprintf("p%d", i)
		if repoData, ok := rawResp[key]; ok {
			if prData, prOk := repoData["pullRequest"]; prOk {
				result[r] = PRStatus{
					State:    prData.State,
					Merged:   prData.Merged,
					MergedAt: prData.MergedAt,
					ClosedAt: prData.ClosedAt,
				}
			}
		}
	}

	return result, nil
}

// CheckAndAutoClosePRTasks inspects all open tasks to check if their linked pull requests have been closed or merged,
// automatically closing the corresponding GitHub issues and returning the updated open and done task slices.
// It batches all PR status checks into a single GraphQL query to minimize network calls and rate limit usage.
func (c *Client) CheckAndAutoClosePRTasks(ctx context.Context, tasks []Task) (openTasks []Task, doneTasks []Task, autoClosedCount int) {
	type taskPRInfo struct {
		taskIndex int
		ref       PRRef
	}

	var openPRTasks []taskPRInfo
	for i, t := range tasks {
		if t.State == "closed" {
			doneTasks = append(doneTasks, t)
			continue
		}

		if !t.IsPRReview() {
			openTasks = append(openTasks, t)
			continue
		}

		prURL := t.PRURL()
		owner, repo, number, ok := ParsePRURL(prURL)
		if !ok {
			openTasks = append(openTasks, t)
			continue
		}

		openPRTasks = append(openPRTasks, taskPRInfo{
			taskIndex: i,
			ref:       PRRef{Owner: owner, Repo: repo, Number: number},
		})
	}

	if len(openPRTasks) == 0 {
		return openTasks, doneTasks, 0
	}

	var refs []PRRef
	for _, item := range openPRTasks {
		refs = append(refs, item.ref)
	}

	statuses, err := c.BatchGetPRStatuses(ctx, refs)
	if err != nil {
		// Fallback: keep tasks open if batch query fails
		for _, item := range openPRTasks {
			openTasks = append(openTasks, tasks[item.taskIndex])
		}
		return openTasks, doneTasks, 0
	}

	for _, item := range openPRTasks {
		t := tasks[item.taskIndex]
		st, found := statuses[item.ref]
		if found && (st.State == "CLOSED" || st.State == "MERGED" || st.Merged) {
			_ = c.CloseTask(ctx, t.RepoOwner, t.RepoName, t.Number)
			now := time.Now()
			if st.ClosedAt != nil {
				now = *st.ClosedAt
			}
			t.State = "closed"
			t.ClosedAt = &now
			t.UpdatedAt = now
			doneTasks = append(doneTasks, t)
			autoClosedCount++
		} else {
			openTasks = append(openTasks, t)
		}
	}

	return openTasks, doneTasks, autoClosedCount
}

// UpdateTaskPriority replaces priority labels with the new priority label.
func (c *Client) UpdateTaskPriority(ctx context.Context, owner, repo string, number int, priority TaskPriority, existingLabels []string) error {
	var newLabels []string
	for _, l := range existingLabels {
		lower := strings.ToLower(strings.TrimSpace(l))
		if strings.HasPrefix(lower, "priority:") || lower == "high" || lower == "medium" || lower == "low" || lower == "p1" || lower == "p2" || lower == "p3" {
			continue
		}
		newLabels = append(newLabels, l)
	}
	newLabels = append(newLabels, priority.Label())

	payload := map[string]interface{}{
		"labels": newLabels,
	}
	jsonBytes, _ := json.Marshal(payload)

	patchURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", url.PathEscape(owner), url.PathEscape(repo), number)
	req, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update priority for task #%d: status %d", number, resp.StatusCode)
	}
	return nil
}

// UpdateTaskAssignees updates the assignees list for an issue.
func (c *Client) UpdateTaskAssignees(ctx context.Context, owner, repo string, number int, assignees []string) error {
	payload := map[string]interface{}{
		"assignees": assignees,
	}
	jsonBytes, _ := json.Marshal(payload)

	patchURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", url.PathEscape(owner), url.PathEscape(repo), number)
	req, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update assignees for task #%d: status %d", number, resp.StatusCode)
	}
	return nil
}

// ReopenTask reopens a closed issue (marks as active).
func (c *Client) ReopenTask(ctx context.Context, owner, repo string, number int) error {
	payload := map[string]string{
		"state": "open",
	}
	jsonBytes, _ := json.Marshal(payload)

	patchURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", url.PathEscape(owner), url.PathEscape(repo), number)
	req, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to reopen task #%d: status %d", number, resp.StatusCode)
	}
	return nil
}

// UpdateTask updates the title, body, priority, and assignees of an existing task issue.
func (c *Client) UpdateTask(ctx context.Context, owner, repo string, number int, title, body string, priority TaskPriority, assignees []string, existingLabels []string, isSquad bool) (*Task, error) {
	cleanOwner := strings.TrimSpace(owner)
	cleanRepo := strings.TrimSpace(repo)
	cleanTitle := strings.TrimSpace(title)

	if cleanOwner == "" || cleanRepo == "" {
		return nil, fmt.Errorf("repo owner and name required")
	}
	if cleanTitle == "" {
		return nil, fmt.Errorf("task title cannot be empty")
	}

	var newLabels []string
	for _, l := range existingLabels {
		lower := strings.ToLower(strings.TrimSpace(l))
		if strings.HasPrefix(lower, "priority:") || lower == "high" || lower == "medium" || lower == "low" || lower == "p1" || lower == "p2" || lower == "p3" {
			continue
		}
		newLabels = append(newLabels, l)
	}
	newLabels = append(newLabels, priority.Label())

	payload := map[string]interface{}{
		"title":     cleanTitle,
		"body":      body,
		"labels":    newLabels,
		"assignees": assignees,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	patchURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", url.PathEscape(cleanOwner), url.PathEscape(cleanRepo), number)
	req, err := http.NewRequestWithContext(ctx, "PATCH", patchURL, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	c.updateRateLimits(resp)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to update task #%d: status %d", number, resp.StatusCode)
	}

	var raw rawGithubIssue
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode updated task: %w", err)
	}

	var labelNames []string
	for _, l := range raw.Labels {
		labelNames = append(labelNames, l.Name)
	}
	var assignedLogins []string
	for _, a := range raw.Assignees {
		assignedLogins = append(assignedLogins, a.Login)
	}

	task := &Task{
		ID:          raw.ID,
		Number:      raw.Number,
		Title:       raw.Title,
		Body:        raw.Body,
		State:       raw.State,
		Priority:    priority,
		RepoOwner:   cleanOwner,
		RepoName:    cleanRepo,
		Author:      raw.User.Login,
		Assignees:   assignedLogins,
		Labels:      labelNames,
		CreatedAt:   raw.CreatedAt,
		UpdatedAt:   raw.UpdatedAt,
		ClosedAt:    raw.ClosedAt,
		URL:         raw.HTMLURL,
		IsSquadTask: isSquad,
	}

	return task, nil
}

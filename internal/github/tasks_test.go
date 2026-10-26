package github

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParsePriority(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		expected TaskPriority
	}{
		{
			name:     "priority: high",
			labels:   []string{"bug", "priority: high"},
			expected: PriorityHigh,
		},
		{
			name:     "high shorthand",
			labels:   []string{"high"},
			expected: PriorityHigh,
		},
		{
			name:     "priority: low",
			labels:   []string{"priority: low"},
			expected: PriorityLow,
		},
		{
			name:     "low shorthand",
			labels:   []string{"documentation", "low"},
			expected: PriorityLow,
		},
		{
			name:     "priority: medium",
			labels:   []string{"priority: medium"},
			expected: PriorityMedium,
		},
		{
			name:     "no labels deemed medium priority",
			labels:   []string{},
			expected: PriorityMedium,
		},
		{
			name:     "nil labels deemed medium priority",
			labels:   nil,
			expected: PriorityMedium,
		},
		{
			name:     "unrelated labels deemed medium priority",
			labels:   []string{"enhancement", "ui"},
			expected: PriorityMedium,
		},
	}

	for _, tc := range tests {
		actual := ParsePriority(tc.labels)
		if actual != tc.expected {
			t.Errorf("[%s] expected priority %v, got %v", tc.name, tc.expected, actual)
		}
	}
}

func TestTaskPriority_RankAndLabel(t *testing.T) {
	if PriorityHigh.Rank() != 1 {
		t.Errorf("expected PriorityHigh.Rank() == 1, got %d", PriorityHigh.Rank())
	}
	if PriorityMedium.Rank() != 2 {
		t.Errorf("expected PriorityMedium.Rank() == 2, got %d", PriorityMedium.Rank())
	}
	if PriorityLow.Rank() != 3 {
		t.Errorf("expected PriorityLow.Rank() == 3, got %d", PriorityLow.Rank())
	}

	if PriorityHigh.Label() != "priority: high" {
		t.Errorf("expected priority: high, got %s", PriorityHigh.Label())
	}
	if PriorityMedium.Label() != "priority: medium" {
		t.Errorf("expected priority: medium, got %s", PriorityMedium.Label())
	}
	if PriorityLow.Label() != "priority: low" {
		t.Errorf("expected priority: low, got %s", PriorityLow.Label())
	}
}

func TestTask_IsPRReview(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		expected bool
	}{
		{
			name:     "contains pr-review label",
			labels:   []string{"priority: medium", "pr-review"},
			expected: true,
		},
		{
			name:     "contains PR-REVIEW uppercase",
			labels:   []string{"PR-REVIEW"},
			expected: true,
		},
		{
			name:     "contains pr review with space",
			labels:   []string{"pr review"},
			expected: true,
		},
		{
			name:     "regular task without pr label",
			labels:   []string{"priority: high", "bug"},
			expected: false,
		},
		{
			name:     "no labels",
			labels:   nil,
			expected: false,
		},
	}

	for _, tc := range tests {
		task := Task{Labels: tc.labels}
		if task.IsPRReview() != tc.expected {
			t.Errorf("[%s] expected IsPRReview == %v, got %v", tc.name, tc.expected, task.IsPRReview())
		}
	}
}

func TestTask_PRURL(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "standard review body",
			body:     "Review pull request: https://github.com/my-org/my-repo/pull/42\n\nRepository: my-repo\nAuthor: @alice",
			expected: "https://github.com/my-org/my-repo/pull/42",
		},
		{
			name:     "markdown link",
			body:     "Please check [PR](https://github.com/my-org/my-repo/pull/105) for changes.",
			expected: "https://github.com/my-org/my-repo/pull/105",
		},
		{
			name:     "trailing punctuation",
			body:     "Linked PR: https://github.com/my-org/my-repo/pull/99.",
			expected: "https://github.com/my-org/my-repo/pull/99",
		},
		{
			name:     "no PR in body",
			body:     "General task without pull request link",
			expected: "",
		},
	}

	for _, tc := range tests {
		task := Task{Body: tc.body}
		actual := task.PRURL()
		if actual != tc.expected {
			t.Errorf("[%s] expected PRURL %q, got %q", tc.name, tc.expected, actual)
		}
	}
}

func TestParsePRURL(t *testing.T) {
	tests := []struct {
		url        string
		wantOwner  string
		wantRepo   string
		wantNumber int
		wantOK     bool
	}{
		{
			url:        "https://github.com/simonbronner/best-resume-ever/pull/1",
			wantOwner:  "simonbronner",
			wantRepo:   "best-resume-ever",
			wantNumber: 1,
			wantOK:     true,
		},
		{
			url:        "https://github.com/my-org/my-repo/pull/42/",
			wantOwner:  "my-org",
			wantRepo:   "my-repo",
			wantNumber: 42,
			wantOK:     true,
		},
		{
			url:        "https://github.com/my-org/my-repo/issues/42",
			wantOwner:  "",
			wantRepo:   "",
			wantNumber: 0,
			wantOK:     false,
		},
		{
			url:        "invalid url",
			wantOwner:  "",
			wantRepo:   "",
			wantNumber: 0,
			wantOK:     false,
		},
	}

	for _, tc := range tests {
		owner, repo, number, ok := ParsePRURL(tc.url)
		if ok != tc.wantOK || owner != tc.wantOwner || repo != tc.wantRepo || number != tc.wantNumber {
			t.Errorf("ParsePRURL(%q) = (%q, %q, %d, %v), want (%q, %q, %d, %v)",
				tc.url, owner, repo, number, ok, tc.wantOwner, tc.wantRepo, tc.wantNumber, tc.wantOK)
		}
	}
}

func TestBatchGetPRStatuses(t *testing.T) {
	mock := mockRoundTripper(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.String(), "graphql") {
			t.Errorf("expected request to graphql, got %s", req.URL.String())
		}
		bodyBytes, _ := io.ReadAll(req.Body)
		bodyStr := string(bodyBytes)
		if !strings.Contains(bodyStr, "p0: repository") || !strings.Contains(bodyStr, "p1: repository") {
			t.Errorf("graphql query missing expected aliases: %s", bodyStr)
		}

		respJSON := `{
			"data": {
				"p0": {
					"pullRequest": {
						"state": "OPEN",
						"merged": false,
						"mergedAt": null,
						"closedAt": null
					}
				},
				"p1": {
					"pullRequest": {
						"state": "MERGED",
						"merged": true,
						"mergedAt": "2026-10-25T10:00:00Z",
						"closedAt": "2026-10-25T10:00:00Z"
					}
				}
			}
		}`

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(respJSON))),
			Request:    req,
		}, nil
	})

	client := &Client{
		token: "dummy",
		httpClient: &http.Client{
			Transport: mock,
		},
	}

	refs := []PRRef{
		{Owner: "owner1", Repo: "repo1", Number: 10},
		{Owner: "owner2", Repo: "repo2", Number: 20},
	}

	statuses, err := client.BatchGetPRStatuses(context.Background(), refs)
	if err != nil {
		t.Fatalf("BatchGetPRStatuses failed: %v", err)
	}

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	st0 := statuses[refs[0]]
	if st0.State != "OPEN" || st0.Merged {
		t.Errorf("expected OPEN and not merged for ref 0, got %+v", st0)
	}

	st1 := statuses[refs[1]]
	if st1.State != "MERGED" || !st1.Merged || st1.ClosedAt == nil {
		t.Errorf("expected MERGED with ClosedAt for ref 1, got %+v", st1)
	}
}

func TestCheckAndAutoClosePRTasks(t *testing.T) {
	closedCalled := false
	mock := mockRoundTripper(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.String(), "graphql") {
			respJSON := `{
				"data": {
					"p0": {
						"pullRequest": {
							"state": "MERGED",
							"merged": true,
							"mergedAt": "2026-10-26T00:00:00Z",
							"closedAt": "2026-10-26T00:00:00Z"
						}
					},
					"p1": {
						"pullRequest": {
							"state": "OPEN",
							"merged": false
						}
					}
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(respJSON))),
				Request:    req,
			}, nil
		}

		if req.Method == "PATCH" && strings.Contains(req.URL.Path, "/issues/1") {
			closedCalled = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"state":"closed"}`))),
				Request:    req,
			}, nil
		}

		t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Request:    req,
		}, nil
	})

	client := &Client{
		token: "dummy",
		httpClient: &http.Client{
			Transport: mock,
		},
	}

	tasks := []Task{
		{
			ID:        1,
			Number:    1,
			RepoOwner: "owner1",
			RepoName:  "repo1",
			Title:     "Review PR 1",
			Body:      "Review: https://github.com/owner1/repo1/pull/1",
			Labels:    []string{"pr-review"},
			State:     "open",
		},
		{
			ID:        2,
			Number:    2,
			RepoOwner: "owner1",
			RepoName:  "repo1",
			Title:     "Review PR 2",
			Body:      "Review: https://github.com/owner1/repo1/pull/2",
			Labels:    []string{"pr-review"},
			State:     "open",
		},
		{
			ID:        3,
			Number:    3,
			RepoOwner: "owner1",
			RepoName:  "repo1",
			Title:     "Regular task",
			Body:      "No PR here",
			State:     "open",
		},
		{
			ID:        4,
			Number:    4,
			RepoOwner: "owner1",
			RepoName:  "repo1",
			Title:     "Already done",
			State:     "closed",
		},
	}

	openTasks, doneTasks, autoClosedCount := client.CheckAndAutoClosePRTasks(context.Background(), tasks)

	if autoClosedCount != 1 {
		t.Errorf("expected autoClosedCount == 1, got %d", autoClosedCount)
	}
	if !closedCalled {
		t.Errorf("expected CloseTask API call for task 1")
	}

	if len(openTasks) != 2 {
		t.Errorf("expected 2 open tasks (t2, t3), got %d", len(openTasks))
	}
	if len(doneTasks) != 2 {
		t.Errorf("expected 2 done tasks (t1, t4), got %d", len(doneTasks))
	}

	// Verify t1 was closed
	var foundT1 bool
	for _, dt := range doneTasks {
		if dt.ID == 1 {
			foundT1 = true
			if dt.State != "closed" {
				t.Errorf("expected t1 to be closed, got %s", dt.State)
			}
		}
	}
	if !foundT1 {
		t.Errorf("expected t1 in doneTasks")
	}
}

package github

import (
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

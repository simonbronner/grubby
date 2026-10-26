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

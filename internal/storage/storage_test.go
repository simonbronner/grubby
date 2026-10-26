package storage

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"grubby/internal/github"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	return db
}

func TestRepositoriesCRUDAndTiers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now()
	repos := []github.Repository{
		{
			Name:          "hot-repo",
			FullName:      "my-org/hot-repo",
			Description:   "Active daily repo",
			PushedAt:      now.Add(-1 * time.Hour),
			UpdatedAt:     now.Add(-1 * time.Hour),
			DefaultBranch: "main",
			IsPrivate:     true,
			URL:           "https://github.com/my-org/hot-repo",
		},
		{
			Name:          "active-repo",
			FullName:      "my-org/active-repo",
			Description:   "Pushed 10 days ago",
			PushedAt:      now.Add(-10 * 24 * time.Hour),
			UpdatedAt:     now.Add(-10 * 24 * time.Hour),
			DefaultBranch: "main",
			IsPrivate:     false,
			URL:           "https://github.com/my-org/active-repo",
		},
		{
			Name:          "dormant-repo",
			FullName:      "my-org/dormant-repo",
			Description:   "Untouched for 6 months",
			PushedAt:      now.Add(-180 * 24 * time.Hour),
			UpdatedAt:     now.Add(-180 * 24 * time.Hour),
			DefaultBranch: "main",
			IsPrivate:     false,
			URL:           "https://github.com/my-org/dormant-repo",
		},
	}

	if err := db.SaveRepositories(repos, "my-org"); err != nil {
		t.Fatalf("SaveRepositories failed: %v", err)
	}

	loaded, err := db.GetRepositories()
	if err != nil {
		t.Fatalf("GetRepositories failed: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 repositories, got %d", len(loaded))
	}

	// Update tiers with PR on active-repo
	prs := []github.PullRequest{
		{
			Repo:   "active-repo",
			Number: 1,
			Title:  "Open PR",
		},
	}
	if err := db.UpdateRepoTiers(prs); err != nil {
		t.Fatalf("UpdateRepoTiers failed: %v", err)
	}

	// active-repo has open PR so it should be promoted to Hot!
	if tier := db.GetRepoTier("active-repo"); tier != TierHot {
		t.Errorf("expected active-repo to be TierHot due to open PR, got %v", tier)
	}

	// hot-repo pushed 1h ago, should be TierHot
	if tier := db.GetRepoTier("hot-repo"); tier != TierHot {
		t.Errorf("expected hot-repo to be TierHot, got %v", tier)
	}

	// dormant-repo pushed 180d ago, no PRs, should be TierDormant
	if tier := db.GetRepoTier("dormant-repo"); tier != TierDormant {
		t.Errorf("expected dormant-repo to be TierDormant, got %v", tier)
	}

	// Touch dormant repo -> should become TierHot immediately!
	if err := db.TouchRepo("dormant-repo"); err != nil {
		t.Fatalf("TouchRepo failed: %v", err)
	}
	if tier := db.GetRepoTier("dormant-repo"); tier != TierHot {
		t.Errorf("expected touched dormant-repo to become TierHot, got %v", tier)
	}

	// Verify Tier Counts
	hot, active, dormant, err := db.GetTierCounts()
	if err != nil {
		t.Fatalf("GetTierCounts failed: %v", err)
	}
	if hot != 3 { // all 3 are hot now
		t.Errorf("expected 3 hot repos, got hot=%d active=%d dormant=%d", hot, active, dormant)
	}
}

func TestTasksLocalFirstAndDebouncing(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	task := github.Task{
		ID:          1001,
		Number:      42,
		RepoOwner:   "my-org",
		RepoName:    "core",
		Title:       "Initial Title",
		Body:        "Initial Notes",
		State:       "open",
		Priority:    github.PriorityMedium,
		Author:      "bronners",
		Assignees:   []string{"bronners"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsSquadTask: false,
	}

	// Save task to SQLite (0ms UI latency)
	if err := db.SaveTask(&task, "synced"); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	tasks, err := db.GetTasks(false, "open")
	if err != nil {
		t.Fatalf("GetTasks failed: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Initial Title" {
		t.Fatalf("expected 1 task with 'Initial Title', got %v", tasks)
	}

	// Test Debouncer coalescing 3 rapid edits into 1 write
	var coalescedCount int64
	var flushedCount int64
	var finalTitle string

	debouncer := NewTaskDebouncer(
		50*time.Millisecond,
		func() {
			atomic.AddInt64(&coalescedCount, 1)
		},
		func(job *TaskDebounceJob, err error) {
			atomic.AddInt64(&flushedCount, 1)
			finalTitle = job.Title
		},
	)

	flushCallback := func(ctx context.Context, job *TaskDebounceJob) error {
		return nil
	}

	// Edit 1
	debouncer.Enqueue(TaskDebounceJob{
		TaskID:    1001,
		Number:    42,
		RepoOwner: "my-org",
		RepoName:  "core",
		Title:     "Title Edit 1",
	}, flushCallback)

	// Rapid Edit 2 (before 50ms timer)
	time.Sleep(10 * time.Millisecond)
	debouncer.Enqueue(TaskDebounceJob{
		TaskID:    1001,
		Number:    42,
		RepoOwner: "my-org",
		RepoName:  "core",
		Title:     "Title Edit 2",
	}, flushCallback)

	// Rapid Edit 3 (before 50ms timer)
	time.Sleep(10 * time.Millisecond)
	debouncer.Enqueue(TaskDebounceJob{
		TaskID:    1001,
		Number:    42,
		RepoOwner: "my-org",
		RepoName:  "core",
		Title:     "Final Title",
	}, flushCallback)

	// Wait for debounce window to fire
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt64(&coalescedCount) != 2 {
		t.Errorf("expected 2 coalesced edits (saved requests), got %d", coalescedCount)
	}
	if atomic.LoadInt64(&flushedCount) != 1 {
		t.Errorf("expected exactly 1 flush callback, got %d", flushedCount)
	}
	if finalTitle != "Final Title" {
		t.Errorf("expected final title 'Final Title', got %q", finalTitle)
	}
}

func TestRequestStatsPersistence(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if err := db.IncrementRequestsMade(15); err != nil {
		t.Fatalf("IncrementRequestsMade failed: %v", err)
	}
	if err := db.IncrementRequestsSaved(45); err != nil {
		t.Fatalf("IncrementRequestsSaved failed: %v", err)
	}

	made, saved, err := db.GetRequestStats()
	if err != nil {
		t.Fatalf("GetRequestStats failed: %v", err)
	}
	if made != 15 {
		t.Errorf("expected 15 requests made, got %d", made)
	}
	if saved != 45 {
		t.Errorf("expected 45 requests saved, got %d", saved)
	}
}

func TestIsDependabot(t *testing.T) {
	tests := []struct {
		actor    string
		expected bool
	}{
		{"@dependabot[bot]", true},
		{"dependabot[bot]", true},
		{"dependabot", true},
		{"app/dependabot", true},
		{"@dependabot", true},
		{"Dependabot[Bot]", true},
		{"alice", false},
		{"bob", false},
		{"github-actions[bot]", false},
		{"", false},
	}

	for _, tc := range tests {
		if got := IsDependabot(tc.actor); got != tc.expected {
			t.Errorf("IsDependabot(%q) = %v; want %v", tc.actor, got, tc.expected)
		}
	}
}

func TestActivityBasedTiers(t *testing.T) {
	now := time.Now()

	// 1. Repo pushed 2 days ago (<= 4d) -> Hot
	rHot := github.Repository{
		Name:     "recent-push",
		PushedAt: now.Add(-2 * 24 * time.Hour),
	}
	if tier := CalculateRepoTier(rHot, time.Time{}, nil, nil); tier != TierHot {
		t.Errorf("expected recent-push to be TierHot, got %v", tier)
	}

	// 2. Repo pushed 10 days ago (> 4d and <= 42d) -> Active
	rActive := github.Repository{
		Name:     "moderate-push",
		PushedAt: now.Add(-10 * 24 * time.Hour),
	}
	if tier := CalculateRepoTier(rActive, time.Time{}, nil, nil); tier != TierActive {
		t.Errorf("expected moderate-push to be TierActive, got %v", tier)
	}

	// 3. Repo pushed 50 days ago (> 42d / 6w) -> Dormant
	rDormant := github.Repository{
		Name:     "old-repo",
		PushedAt: now.Add(-50 * 24 * time.Hour),
	}
	if tier := CalculateRepoTier(rDormant, time.Time{}, nil, nil); tier != TierDormant {
		t.Errorf("expected old-repo to be TierDormant, got %v", tier)
	}

	// 4. Dormant repo with Dependabot PR 1 day ago -> Must stay Dormant!
	dependabotPR := []github.PullRequest{
		{
			Repo:      "old-repo",
			Author:    "@dependabot[bot]",
			UpdatedAt: now.Add(-1 * 24 * time.Hour),
		},
	}
	if tier := CalculateRepoTier(rDormant, time.Time{}, dependabotPR, nil); tier != TierDormant {
		t.Errorf("expected old-repo with dependabot PR to stay TierDormant, got %v", tier)
	}

	// 5. Dormant repo with Dependabot commit and push 1 day ago -> Must stay Dormant!
	rDormantWithBotPush := github.Repository{
		Name:     "old-repo",
		PushedAt: now.Add(-1 * 24 * time.Hour),
	}
	dependabotEvent := []github.SquadEvent{
		{
			Actor:     "dependabot[bot]",
			Action:    "Committed",
			Repo:      "old-repo",
			Timestamp: now.Add(-1 * 24 * time.Hour),
		},
	}
	if tier := CalculateRepoTier(rDormantWithBotPush, time.Time{}, nil, dependabotEvent); tier != TierDormant {
		t.Errorf("expected old-repo with dependabot push/commit to stay TierDormant, got %v", tier)
	}

	// 6. Dormant repo with human PR 3 days ago -> Hot!
	humanPR := []github.PullRequest{
		{
			Repo:      "old-repo",
			Author:    "alice",
			UpdatedAt: now.Add(-3 * 24 * time.Hour),
		},
	}
	if tier := CalculateRepoTier(rDormant, time.Time{}, humanPR, nil); tier != TierHot {
		t.Errorf("expected old-repo with human PR within 3d to be TierHot, got %v", tier)
	}

	// 7. Dormant repo with human commit 2 weeks ago -> Active!
	humanCommit := []github.SquadEvent{
		{
			Actor:     "bob",
			Action:    "Committed",
			Repo:      "old-repo",
			Timestamp: now.Add(-14 * 24 * time.Hour),
		},
	}
	if tier := CalculateRepoTier(rDormant, time.Time{}, nil, humanCommit); tier != TierActive {
		t.Errorf("expected old-repo with human commit 2w ago to be TierActive, got %v", tier)
	}
}

func TestDeletePullRequest(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now()
	prs := []github.PullRequest{
		{
			Repo:           "grubby",
			Number:         42,
			Title:          "Feature PR",
			Author:         "alice",
			ReviewDecision: "APPROVED",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		{
			Repo:           "grubby",
			Number:         43,
			Title:          "Another PR",
			Author:         "bob",
			ReviewDecision: "REVIEW_REQUIRED",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}
	if err := db.SavePullRequests(prs); err != nil {
		t.Fatalf("SavePullRequests failed: %v", err)
	}

	saved, err := db.GetPullRequests()
	if err != nil || len(saved) != 2 {
		t.Fatalf("expected 2 PRs, got %d (err: %v)", len(saved), err)
	}

	// Delete PR 42
	if err := db.DeletePullRequest("grubby", 42); err != nil {
		t.Fatalf("DeletePullRequest failed: %v", err)
	}

	remaining, err := db.GetPullRequests()
	if err != nil {
		t.Fatalf("GetPullRequests failed: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Number != 43 {
		t.Fatalf("expected 1 PR with number 43, got %+v", remaining)
	}
}

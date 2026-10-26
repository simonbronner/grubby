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

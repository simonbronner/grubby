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
			Name:          "cold-repo",
			FullName:      "my-org/cold-repo",
			Description:   "Untouched for 6 months",
			PushedAt:      now.Add(-180 * 24 * time.Hour),
			UpdatedAt:     now.Add(-180 * 24 * time.Hour),
			DefaultBranch: "main",
			IsPrivate:     false,
			URL:           "https://github.com/my-org/cold-repo",
		},
		{
			Name:          "dead-repo",
			FullName:      "my-org/dead-repo",
			Description:   "Untouched for 2 years",
			PushedAt:      now.Add(-700 * 24 * time.Hour),
			UpdatedAt:     now.Add(-700 * 24 * time.Hour),
			DefaultBranch: "main",
			IsPrivate:     false,
			URL:           "https://github.com/my-org/dead-repo",
		},
	}

	if err := db.SaveRepositories(repos, "my-org"); err != nil {
		t.Fatalf("SaveRepositories failed: %v", err)
	}

	loaded, err := db.GetRepositories()
	if err != nil {
		t.Fatalf("GetRepositories failed: %v", err)
	}
	if len(loaded) != 4 {
		t.Fatalf("expected 4 repositories, got %d", len(loaded))
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

	// cold-repo pushed 180d ago, no PRs, should be TierCold
	if tier := db.GetRepoTier("cold-repo"); tier != TierCold {
		t.Errorf("expected cold-repo to be TierCold, got %v", tier)
	}

	// dead-repo pushed 700d ago, no PRs, should be TierDead
	if tier := db.GetRepoTier("dead-repo"); tier != TierDead {
		t.Errorf("expected dead-repo to be TierDead, got %v", tier)
	}

	// Touch cold repo -> should NOT change its tier (still TierCold), but records last_interacted_at!
	if err := db.TouchRepo("cold-repo"); err != nil {
		t.Fatalf("TouchRepo failed: %v", err)
	}
	if tier := db.GetRepoTier("cold-repo"); tier != TierCold {
		t.Errorf("expected touched cold-repo to remain TierCold, got %v", tier)
	}
	interacted := db.GetRepoLastInteractedAt("cold-repo")
	if interacted.IsZero() || time.Since(interacted) > 5*time.Second {
		t.Errorf("expected valid recent last_interacted_at timestamp, got %v", interacted)
	}

	// Verify Tier Counts: 2 Hot (hot-repo & active-repo with PR), 0 Warm, 1 Cold, 1 Dead
	hot, warm, cold, dead, err := db.GetTierCounts()
	if err != nil {
		t.Fatalf("GetTierCounts failed: %v", err)
	}
	if hot != 2 || warm != 0 || cold != 1 || dead != 1 {
		t.Errorf("expected hot=2 warm=0 cold=1 dead=1, got hot=%d warm=%d cold=%d dead=%d", hot, warm, cold, dead)
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
	if tier := CalculateRepoTier(rHot, nil, nil); tier != TierHot {
		t.Errorf("expected recent-push to be TierHot, got %v", tier)
	}

	// 2. Repo pushed 10 days ago (> 4d and <= 42d) -> Warm
	rWarm := github.Repository{
		Name:     "moderate-push",
		PushedAt: now.Add(-10 * 24 * time.Hour),
	}
	if tier := CalculateRepoTier(rWarm, nil, nil); tier != TierWarm {
		t.Errorf("expected moderate-push to be TierWarm, got %v", tier)
	}

	// 3. Repo pushed 50 days ago (> 42d / 6w and <= 12m) -> Cold
	rCold := github.Repository{
		Name:     "old-repo",
		PushedAt: now.Add(-50 * 24 * time.Hour),
	}
	if tier := CalculateRepoTier(rCold, nil, nil); tier != TierCold {
		t.Errorf("expected old-repo to be TierCold, got %v", tier)
	}

	// 4. Repo pushed 400 days ago (> 12m) -> Dead
	rDead := github.Repository{
		Name:     "dead-repo",
		PushedAt: now.Add(-400 * 24 * time.Hour),
	}
	if tier := CalculateRepoTier(rDead, nil, nil); tier != TierDead {
		t.Errorf("expected dead-repo to be TierDead, got %v", tier)
	}

	// 5. Zero activity repo -> Dead
	rZero := github.Repository{Name: "zero-repo"}
	if tier := CalculateRepoTier(rZero, nil, nil); tier != TierDead {
		t.Errorf("expected zero-repo to be TierDead, got %v", tier)
	}

	// 6. Cold repo with Dependabot PR 1 day ago -> Must stay Cold!
	dependabotPR := []github.PullRequest{
		{
			Repo:      "old-repo",
			Author:    "@dependabot[bot]",
			UpdatedAt: now.Add(-1 * 24 * time.Hour),
		},
	}
	if tier := CalculateRepoTier(rCold, dependabotPR, nil); tier != TierCold {
		t.Errorf("expected old-repo with dependabot PR to stay TierCold, got %v", tier)
	}

	// 7. Dead repo with Dependabot commit and push 1 day ago -> Must stay Dead!
	rDeadWithBotPush := github.Repository{
		Name:     "dead-repo",
		PushedAt: now.Add(-1 * 24 * time.Hour),
	}
	dependabotEvent := []github.SquadEvent{
		{
			Actor:     "dependabot[bot]",
			Action:    "Committed",
			Repo:      "dead-repo",
			Timestamp: now.Add(-1 * 24 * time.Hour),
		},
	}
	if tier := CalculateRepoTier(rDeadWithBotPush, nil, dependabotEvent); tier != TierDead {
		t.Errorf("expected dead-repo with dependabot push/commit to stay TierDead, got %v", tier)
	}

	// 8. Cold repo with human PR 3 days ago -> Hot!
	humanPR := []github.PullRequest{
		{
			Repo:      "old-repo",
			Author:    "alice",
			UpdatedAt: now.Add(-3 * 24 * time.Hour),
		},
	}
	if tier := CalculateRepoTier(rCold, humanPR, nil); tier != TierHot {
		t.Errorf("expected old-repo with human PR within 3d to be TierHot, got %v", tier)
	}

	// 9. Cold repo with human commit 2 weeks ago -> Warm!
	humanCommit := []github.SquadEvent{
		{
			Actor:     "bob",
			Action:    "Committed",
			Repo:      "old-repo",
			Timestamp: now.Add(-14 * 24 * time.Hour),
		},
	}
	if tier := CalculateRepoTier(rCold, nil, humanCommit); tier != TierWarm {
		t.Errorf("expected old-repo with human commit 2w ago to be TierWarm, got %v", tier)
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

func TestRepoLabelsStorage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// 1. Save labels for repo 101 in squad "core-team"
	err := db.SaveRepoLabels(101, "auth-service", []string{"backend", "auth", "core"}, "core-team")
	if err != nil {
		t.Fatalf("SaveRepoLabels failed: %v", err)
	}

	// 2. Save labels for repo 102 in squad "core-team"
	err = db.SaveRepoLabels(102, "web-app", []string{"frontend", "react"}, "core-team")
	if err != nil {
		t.Fatalf("SaveRepoLabels failed: %v", err)
	}

	// 3. Query by squad
	byID, err := db.GetRepoLabelsForSquad("core-team")
	if err != nil {
		t.Fatalf("GetRepoLabelsForSquad failed: %v", err)
	}
	if len(byID[101]) != 3 || len(byID[102]) != 2 {
		t.Errorf("unexpected labels by ID: %+v", byID)
	}

	byName, err := db.GetRepoLabelsByNameForSquad("core-team")
	if err != nil {
		t.Fatalf("GetRepoLabelsByNameForSquad failed: %v", err)
	}
	if len(byName["auth-service"]) != 3 || len(byName["web-app"]) != 2 {
		t.Errorf("unexpected labels by name: %+v", byName)
	}

	// 4. Query all unique labels in squad
	unique, err := db.GetAllUniqueLabels("core-team")
	if err != nil {
		t.Fatalf("GetAllUniqueLabels failed: %v", err)
	}
	if len(unique) != 5 {
		t.Errorf("expected 5 unique labels, got %d: %v", len(unique), unique)
	}

	// 5. Update labels for repo 101 (overwrite)
	err = db.SaveRepoLabels(101, "auth-service", []string{"backend", "security"}, "core-team")
	if err != nil {
		t.Fatalf("SaveRepoLabels overwrite failed: %v", err)
	}
	updated, _ := db.GetRepoLabelsForSquad("core-team")
	if len(updated[101]) != 2 || updated[101][0] != "auth" && updated[101][1] != "security" && updated[101][0] != "backend" {
		t.Errorf("expected updated labels [backend, security], got: %v", updated[101])
	}
}

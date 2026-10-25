package ui

import (
	"strings"
	"testing"
	"time"

	"grubby/internal/config"
	"grubby/internal/github"
)

func TestSquadDashboard_TwoWeekAggregation(t *testing.T) {
	now := time.Now()

	cfg := &config.Config{
		ActiveSquad: "core-team",
		Squads: []config.SquadConfig{
			{
				Name:        "core-team",
				DisplayName: "Core Team",
				Members:     []string{"alice", "bob", "inactive-user"},
			},
		},
	}

	m := Model{
		Config: cfg,
		Width:  120,
		Height: 30,
		OrgMembers: []github.OrgMember{
			{Login: "alice", Name: "Alice Smith"},
			{Login: "bob", Name: "Bob Jones"},
		},
		SquadEvents: []github.SquadEvent{
			// Within 14 days
			{Actor: "alice", Action: "Committed", Repo: "auth-service", Target: "auth-service:main", Details: "feat: add oauth", Timestamp: now.Add(-2 * 24 * time.Hour)},
			{Actor: "alice", Action: "Pushed 2 commits", Repo: "auth-service", Target: "auth-service:main", Details: "fix: token refresh", Timestamp: now.Add(-3 * 24 * time.Hour)},
			{Actor: "alice", Action: "Review approved", Repo: "payment-api", Target: "payment-api#10", Details: "LGTM!", Timestamp: now.Add(-1 * 24 * time.Hour)},
			{Actor: "bob", Action: "Committed", Repo: "auth-service", Target: "auth-service:main", Details: "refactor: db pool", Timestamp: now.Add(-5 * 24 * time.Hour)},
			{Actor: "bob", Action: "PR merged", Repo: "payment-api", Target: "payment-api#12", Details: "Merged Stripe webhook", Timestamp: now.Add(-4 * 24 * time.Hour)},

			// Older than 14 days (should be excluded)
			{Actor: "alice", Action: "Committed", Repo: "old-repo", Target: "old-repo:main", Details: "ancient commit", Timestamp: now.Add(-20 * 24 * time.Hour)},
		},
	}

	dashboard := m.computeSquadDashboard()

	// 1. Verify excluded old-repo
	for _, r := range dashboard.ActiveRepos {
		if r.RepoName == "old-repo" {
			t.Fatalf("expected old-repo (20d old) to be excluded from last 2 weeks")
		}
	}

	// 2. Verify most active repo ranking
	// auth-service has 1 commit from alice + 1 push from alice + 1 commit from bob = 3 actions
	// payment-api has 1 review from alice + 1 merge from bob = 2 actions
	if len(dashboard.ActiveRepos) != 2 {
		t.Fatalf("expected 2 active repos in last 14 days, got %d", len(dashboard.ActiveRepos))
	}
	if dashboard.ActiveRepos[0].RepoName != "auth-service" {
		t.Errorf("expected auth-service to be #1 most active, got %s", dashboard.ActiveRepos[0].RepoName)
	}
	if dashboard.ActiveRepos[0].TotalActions != 3 {
		t.Errorf("expected 3 total actions for auth-service, got %d", dashboard.ActiveRepos[0].TotalActions)
	}
	if len(dashboard.ActiveRepos[0].Contributors) != 2 {
		t.Errorf("expected 2 contributors on auth-service, got %d", len(dashboard.ActiveRepos[0].Contributors))
	}

	// 3. Verify member contributions
	// Alice: 1 commit, 1 push, 1 review = 3 actions
	// Bob: 1 commit, 1 merge = 2 actions
	// inactive-user: 0 actions
	if len(dashboard.MemberContributions) != 3 {
		t.Fatalf("expected 3 squad members, got %d", len(dashboard.MemberContributions))
	}

	alice := dashboard.MemberContributions[0]
	if alice.Login != "alice" || alice.TotalActions != 3 {
		t.Errorf("expected alice with 3 actions, got %+v", alice)
	}
	if alice.Commits != 1 || alice.Pushes != 1 || alice.Reviews != 1 {
		t.Errorf("unexpected alice breakdown: %+v", alice)
	}

	bob := dashboard.MemberContributions[1]
	if bob.Login != "bob" || bob.TotalActions != 2 {
		t.Errorf("expected bob with 2 actions, got %+v", bob)
	}
	if bob.Merges != 1 || bob.Commits != 1 {
		t.Errorf("unexpected bob breakdown: %+v", bob)
	}

	inactive := dashboard.MemberContributions[2]
	if inactive.Login != "inactive-user" || inactive.TotalActions != 0 {
		t.Errorf("expected inactive-user with 0 actions, got %+v", inactive)
	}

	// 4. Verify rendering
	rendered := m.renderSquadView()
	if !strings.Contains(rendered, "Squad Dashboard: Core Team") {
		t.Errorf("expected rendered view to contain dashboard title, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "MOST ACTIVE REPOSITORIES") {
		t.Errorf("expected rendered view to contain active repos section")
	}
	if !strings.Contains(rendered, "SQUAD MEMBER CONTRIBUTIONS") {
		t.Errorf("expected rendered view to contain member contributions section")
	}
	if !strings.Contains(rendered, "@alice (Alice Smith)") {
		t.Errorf("expected rendered view to show Alice Smith with handle")
	}
}

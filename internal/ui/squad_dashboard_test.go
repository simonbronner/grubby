package ui

import (
	"fmt"
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

func TestSquadDashboard_DeterministicSortingJitterFree(t *testing.T) {
	now := time.Now()

	cfg := &config.Config{
		ActiveSquad: "infra",
		Squads: []config.SquadConfig{
			{
				Name:        "infra",
				DisplayName: "Infrastructure",
				Members:     []string{"member-z", "member-a", "member-m", "member-c"},
			},
		},
	}

	// Create 4 repos with equal total actions, equal commits, equal reviews
	events := []github.SquadEvent{
		{Actor: "member-z", Action: "Committed", Repo: "repo-delta", Timestamp: now.Add(-1 * time.Hour)},
		{Actor: "member-a", Action: "Committed", Repo: "repo-beta", Timestamp: now.Add(-1 * time.Hour)},
		{Actor: "member-m", Action: "Committed", Repo: "repo-alpha", Timestamp: now.Add(-1 * time.Hour)},
		{Actor: "member-c", Action: "Committed", Repo: "repo-charlie", Timestamp: now.Add(-1 * time.Hour)},
	}

	m := Model{
		Config:      cfg,
		Width:       120,
		Height:      30,
		SquadEvents: events,
	}

	// Compute dashboard repeatedly across 50 iterations to simulate 1s timer ticks
	var firstOrder []string
	var firstMembers []string
	for i := 0; i < 50; i++ {
		dash := m.computeSquadDashboard()

		var currentOrder []string
		for _, r := range dash.ActiveRepos {
			currentOrder = append(currentOrder, r.RepoName)
		}

		var currentMembers []string
		for _, mem := range dash.MemberContributions {
			currentMembers = append(currentMembers, mem.Login)
		}

		if i == 0 {
			firstOrder = currentOrder
			firstMembers = currentMembers
			// Verify alphabetical tiebreak: alpha, beta, charlie, delta
			expectedRepos := []string{"repo-alpha", "repo-beta", "repo-charlie", "repo-delta"}
			for idx, exp := range expectedRepos {
				if firstOrder[idx] != exp {
					t.Fatalf("expected repo #%d to be %s, got %s", idx, exp, firstOrder[idx])
				}
			}
			// Verify alphabetical member tiebreak: member-a, member-c, member-m, member-z
			expectedMembers := []string{"member-a", "member-c", "member-m", "member-z"}
			for idx, exp := range expectedMembers {
				if firstMembers[idx] != exp {
					t.Fatalf("expected member #%d to be %s, got %s", idx, exp, firstMembers[idx])
				}
			}
		} else {
			// Ensure order NEVER changes across ticks (jitter-free)
			for idx := range currentOrder {
				if currentOrder[idx] != firstOrder[idx] {
					t.Fatalf("jitter detected at iteration %d: order changed from %v to %v", i, firstOrder, currentOrder)
				}
			}
			for idx := range currentMembers {
				if currentMembers[idx] != firstMembers[idx] {
					t.Fatalf("member jitter detected at iteration %d: order changed from %v to %v", i, firstMembers, currentMembers)
				}
			}
		}
	}
}

func TestSquadDashboard_CleanBorderlessAndUserDetails(t *testing.T) {
	now := time.Now()

	cfg := &config.Config{
		ActiveSquad: "core",
		Squads: []config.SquadConfig{
			{
				Name:        "core",
				DisplayName: "Core",
				Members:     []string{"simonb", "octo"},
			},
		},
	}

	m := Model{
		Config: cfg,
		Width:  100,
		Height: 35,
		OrgMembers: []github.OrgMember{
			{
				ID:         "MDQ6VXNlcjEyNDgyMjUz",
				DatabaseID: 12482253,
				Login:      "simonb",
				Name:       "Simon Bronner",
			},
			{
				ID:         "MDQ6VXNlcjU4MzIzMQ==",
				DatabaseID: 583231,
				Login:      "octo",
				Name:       "The Octocat",
			},
		},
		SquadEvents: []github.SquadEvent{
			{Actor: "simonb", Action: "Committed", Repo: "grubby", Timestamp: now.Add(-1 * time.Hour)},
		},
		SquadCursor: 0, // simonb selected
	}

	rendered := m.renderSquadView()

	// 1. Check user ID and display name
	if !strings.Contains(rendered, "@simonb (Simon Bronner)") {
		t.Errorf("expected rendered view to show @simonb (Simon Bronner)")
	}
	if !strings.Contains(rendered, "ID: 12482253") {
		t.Errorf("expected rendered view to show ID: 12482253")
	}
	if !strings.Contains(rendered, "@octo (The Octocat)") {
		t.Errorf("expected rendered view to show @octo (The Octocat)")
	}
	if !strings.Contains(rendered, "ID: 583231") {
		t.Errorf("expected rendered view to show ID: 583231")
	}

	// 2. Check selected cursor prefix
	if !strings.Contains(rendered, "▶ @simonb (Simon Bronner)") {
		t.Errorf("expected rendered view to contain selected cursor prefix ▶")
	}
}

func TestAvatar_ITerm2NativeGraphicsAndBoundingBox(t *testing.T) {
	// 1. Test BuildITerm2ImageSequence standard
	seq := BuildITerm2ImageSequence("aGVsbG8=", 8, 4)
	expectedSeq := "\x1b]1337;File=inline=1;width=8;height=4;preserveAspectRatio=1:aGVsbG8=\x07"
	if !strings.Contains(seq, expectedSeq) {
		t.Errorf("expected sequence to contain %q, got %q", expectedSeq, seq)
	}

	// 2. Test RenderAvatarBoundingBox
	bbox := RenderAvatarBoundingBox(8, 4)
	lines := strings.Split(bbox, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines in bounding box, got %d", len(lines))
	}
	for i, l := range lines {
		if len(l) != 8 {
			t.Errorf("line %d: expected width 8, got %d", i, len(l))
		}
	}

	// 3. Test ExtractInitials & GetUserAvatarColor
	if init := ExtractInitials("Simon Bronner", "simonb"); init != "SB" {
		t.Errorf("expected SB, got %q", init)
	}
	if init := ExtractInitials("John F. Kennedy", "jfk"); init != "JK" {
		t.Errorf("expected JK, got %q", init)
	}
	if init := ExtractInitials("", "octocat"); init != "OC" {
		t.Errorf("expected OC, got %q", init)
	}
	if init := ExtractInitials("", "@bronners"); init != "BR" {
		t.Errorf("expected BR, got %q", init)
	}

	c1 := GetUserAvatarColor("simonb")
	c2 := GetUserAvatarColor("simonb")
	if c1 != c2 {
		t.Errorf("expected deterministic color for same login, got %v and %v", c1, c2)
	}

	// 4. Test RenderASCIIInitialsAvatar
	avatarBox := RenderASCIIInitialsAvatar("Simon Bronner", "simonb", 8, 4)
	if !strings.Contains(avatarBox, "SB") {
		t.Errorf("expected avatar box to contain initials 'SB', got %q", avatarBox)
	}
	boxLines := strings.Split(avatarBox, "\n")
	if len(boxLines) != 4 {
		t.Fatalf("expected 4 lines in avatar box, got %d", len(boxLines))
	}

	// 5. Test RenderFallbackAvatar
	fb8x4 := RenderFallbackAvatar(8, 4)
	fbLines := strings.Split(fb8x4, "\n")
	if len(fbLines) != 4 {
		t.Fatalf("expected 4 lines of fallback avatar, got %d", len(fbLines))
	}

	// 6. Test EmitITerm2ImagesCmd with nil/empty and placements
	if cmd := EmitITerm2ImagesCmd(nil); cmd != nil {
		t.Errorf("expected nil cmd for nil placements")
	}
	if cmd := EmitITerm2ImagesCmd([]AvatarPlacement{}); cmd != nil {
		t.Errorf("expected nil cmd for empty placements")
	}
	cmd := EmitITerm2ImagesCmd([]AvatarPlacement{
		{Row: 16, Col: 3, Width: 8, Height: 4, RawBytes: []byte("test")},
	})
	if cmd == nil {
		t.Errorf("expected non-nil cmd for valid placements")
	}
}

func TestSquadAvatarPlacements_ExactRowAlignmentAndBounds(t *testing.T) {
	now := time.Now()
	cfg := &config.Config{
		ActiveSquad: "core",
		Squads: []config.SquadConfig{
			{
				Name:        "core",
				DisplayName: "Core Team",
				Members:     []string{"simonb", "octocat"},
			},
		},
	}

	m := Model{
		Config:            cfg,
		Width:             120,
		Height:            35,
		ActiveTab:         TabSquad,
		InitialDataLoaded: true,
		SplashDismissed:   true,
		OrgMembers: []github.OrgMember{
			{Login: "simonb", Name: "Simon Bronner"},
			{Login: "octocat", Name: "The Octocat"},
		},
		SquadEvents: []github.SquadEvent{
			{Actor: "simonb", Action: "Committed", Repo: "grubby", Timestamp: now.Add(-1 * time.Hour)},
		},
		SquadCursor: 0,
	}

	view := m.View()
	if !strings.Contains(view, "SB") {
		t.Errorf("expected squad view to contain ASCII initials SB for Simon Bronner")
	}
	if !strings.Contains(view, "TO") {
		t.Errorf("expected squad view to contain ASCII initials TO for The Octocat")
	}

	// 2. Test scrolled view (cursor at index 4 with 6 members)
	cfg.Squads[0].Members = []string{"u1", "u2", "u3", "u4", "u5", "u6"}
	m.SquadCursor = 4 // Scrolled down
	m.Height = 35     // visiblePanes = 4

	scrolledView := m.View()
	if !strings.Contains(scrolledView, "@u5") {
		t.Errorf("expected scrolled view to contain active cursor member @u5")
	}
}

func TestSquadDashboard_RepoNameNormalization(t *testing.T) {
	now := time.Now()
	cfg := &config.Config{
		ActiveSquad: "core",
		Squads: []config.SquadConfig{
			{
				Name:    "core",
				Members: []string{"alice", "bob"},
			},
		},
	}

	m := Model{
		Config: cfg,
		Width:  120,
		Height: 35,
		Repos: []github.Repository{
			{Name: "payment-api"},
		},
		SquadEvents: []github.SquadEvent{
			// Same repo "payment-api" represented with org prefix, bare name, and url/git
			{Actor: "alice", Action: "Committed", Repo: "acmeco/payment-api", Timestamp: now.Add(-1 * time.Hour)},
			{Actor: "bob", Action: "Pushed 1 commit", Repo: "payment-api", Timestamp: now.Add(-2 * time.Hour)},
			{Actor: "alice", Action: "PR merged", Repo: "git@github.com:acmeco/payment-api.git", Timestamp: now.Add(-3 * time.Hour)},
		},
	}

	dashboard := m.computeSquadDashboard()

	// Should consolidate into exactly 1 active repository
	if len(dashboard.ActiveRepos) != 1 {
		t.Fatalf("expected 1 consolidated active repo, got %d: %+v", len(dashboard.ActiveRepos), dashboard.ActiveRepos)
	}

	r := dashboard.ActiveRepos[0]
	if r.RepoName != "payment-api" {
		t.Errorf("expected repo name 'payment-api', got %q", r.RepoName)
	}
	if r.TotalActions != 3 {
		t.Errorf("expected 3 total actions, got %d", r.TotalActions)
	}
	if r.Commits != 1 || r.Pushes != 1 || r.Merges != 1 {
		t.Errorf("unexpected breakdown: commits=%d, pushes=%d, merges=%d", r.Commits, r.Pushes, r.Merges)
	}
	if len(r.Contributors) != 2 {
		t.Errorf("expected 2 unique contributors, got %d (%v)", len(r.Contributors), r.Contributors)
	}
}

func TestSquadDashboard_Top10ActiveReposAndZeroExclusion(t *testing.T) {
	now := time.Now()
	cfg := &config.Config{
		ActiveSquad: "core",
		Squads: []config.SquadConfig{
			{
				Name:    "core",
				Members: []string{"dev1"},
			},
		},
	}

	// Create 12 active repos and 1 inactive repo
	var events []github.SquadEvent
	for i := 1; i <= 12; i++ {
		repoName := fmt.Sprintf("service-%02d", i)
		// Give service-01 12 actions down to service-12 with 1 action
		for action := 0; action < (13 - i); action++ {
			events = append(events, github.SquadEvent{
				Actor:     "dev1",
				Action:    "Committed",
				Repo:      repoName,
				Timestamp: now.Add(time.Duration(-action) * time.Hour),
			})
		}
	}

	m := Model{
		Config:      cfg,
		Width:       140,
		Height:      50,
		SquadEvents: events,
	}

	dashboard := m.computeSquadDashboard()

	// All 12 repos had actions > 0, so all 12 are in ActiveRepos
	if len(dashboard.ActiveRepos) != 12 {
		t.Fatalf("expected 12 active repos, got %d", len(dashboard.ActiveRepos))
	}

	// Render the view and check top 10 display
	rendered := m.renderSquadView()

	// Ranks #1 through #10 should be in the rendered output
	for i := 1; i <= 10; i++ {
		rankStr := fmt.Sprintf("#%-2d ", i)
		if !strings.Contains(rendered, rankStr) {
			t.Errorf("expected rendered output to contain rank %q", rankStr)
		}
	}

	// Rank #11 and #12 should NOT be in the rendered output because of displayLimit = 10
	if strings.Contains(rendered, "#11") {
		t.Errorf("expected rendered output not to contain rank #11")
	}
	if strings.Contains(rendered, "#12") {
		t.Errorf("expected rendered output not to contain rank #12")
	}
}

func TestSquadDashboard_ContributorContributions14DaysAllRepos(t *testing.T) {
	now := time.Now()
	cfg := &config.Config{
		ActiveSquad: "core",
		Squads: []config.SquadConfig{
			{
				Name:    "core",
				Members: []string{"alice"},
			},
		},
	}

	// Alice contributes to 15 different repos in 14 days
	var events []github.SquadEvent
	for i := 1; i <= 15; i++ {
		events = append(events, github.SquadEvent{
			Actor:     "alice",
			Action:    "Committed",
			Repo:      fmt.Sprintf("repo-%02d", i),
			Timestamp: now.Add(time.Duration(-i*12) * time.Hour), // within 14 days
		})
	}
	// And 1 event older than 14 days
	events = append(events, github.SquadEvent{
		Actor:     "alice",
		Action:    "Committed",
		Repo:      "ancient-repo",
		Timestamp: now.Add(-15 * 24 * time.Hour),
	})

	m := Model{
		Config:      cfg,
		Width:       140,
		Height:      50,
		SquadEvents: events,
	}

	dashboard := m.computeSquadDashboard()
	if len(dashboard.MemberContributions) != 1 {
		t.Fatalf("expected 1 member contribution, got %d", len(dashboard.MemberContributions))
	}

	alice := dashboard.MemberContributions[0]
	// Should include all 15 events in 14 days across all 15 repos
	if alice.TotalActions != 15 {
		t.Errorf("expected 15 total actions for Alice across all repos, got %d", alice.TotalActions)
	}
	if alice.Commits != 15 {
		t.Errorf("expected 15 commits for Alice, got %d", alice.Commits)
	}

	// Top repos should have up to 3 entries
	if len(alice.TopRepos) != 3 {
		t.Errorf("expected 3 top repos, got %d: %v", len(alice.TopRepos), alice.TopRepos)
	}
}

func TestSquadDashboard_SecurityOverviewTable(t *testing.T) {
	cfg := &config.Config{
		ActiveSquad: "core",
		Squads: []config.SquadConfig{
			{
				Name:        "core",
				DisplayName: "Core Team",
				Members:     []string{"alice"},
				Repos:       []string{"auth-svc", "gateway-svc"},
			},
		},
	}

	secSummaries := map[string]github.SecurityAlertSummary{
		"auth-svc": {
			Critical: 2,
			High:     3,
			Medium:   4,
			Low:      1,
			Total:    10,
		},
		"gateway-svc": {
			Critical: 0,
			High:     0,
			Medium:   0,
			Low:      0,
			Total:    0,
		},
	}

	m := Model{
		Config:            cfg,
		Width:             120,
		Height:            40,
		SecuritySummaries: secSummaries,
		SquadEvents: []github.SquadEvent{
			{
				Actor:     "alice",
				Action:    "Committed",
				Repo:      "auth-svc",
				Timestamp: time.Now().Add(-1 * time.Hour),
			},
		},
	}

	rendered := m.renderSquadView()

	// 1. Check for Security Overview title and headers
	if !strings.Contains(rendered, "SQUAD SECURITY OVERVIEW") {
		t.Fatalf("expected view to contain 'SQUAD SECURITY OVERVIEW', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "CRIT") || !strings.Contains(rendered, "HIGH") || !strings.Contains(rendered, "MED") || !strings.Contains(rendered, "REST") {
		t.Fatalf("expected view to contain vulnerability severity columns, got:\n%s", rendered)
	}

	// 2. Check for auth-svc and its alert counts
	if !strings.Contains(rendered, "auth-svc") {
		t.Fatalf("expected view to contain owned vulnerable repo 'auth-svc', got:\n%s", rendered)
	}

	// 3. Test zero vulnerabilities empty state
	secSummaries["auth-svc"] = github.SecurityAlertSummary{Total: 0}
	m.SecuritySummaries = secSummaries
	renderedClean := m.renderSquadView()
	if !strings.Contains(renderedClean, "0 vulnerabilities across 2 squad repos") {
		t.Fatalf("expected clean state message, got:\n%s", renderedClean)
	}

	// 4. Test no repos owned empty state
	cfg.Squads[0].Repos = nil
	renderedNoRepos := m.renderSquadView()
	if !strings.Contains(renderedNoRepos, "No repositories assigned to squad yet") {
		t.Fatalf("expected no repos assigned message, got:\n%s", renderedNoRepos)
	}
}


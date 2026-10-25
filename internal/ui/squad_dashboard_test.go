package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
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

func TestSquadDashboard_BorderIntegrityAndUserDetails(t *testing.T) {
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

	// 2. Check border alignment integrity
	lines := strings.Split(rendered, "\n")
	for lineIdx, line := range lines {
		// If line contains card box characters, verify proper margin and no broken left indent
		if strings.Contains(line, "╭") || strings.Contains(line, "│") || strings.Contains(line, "╰") {
			// Find the index of the first box character
			boxCharIdx := -1
			for charIdx, r := range line {
				if r == '╭' || r == '│' || r == '╰' {
					boxCharIdx = charIdx
					break
				}
			}
			// MarginLeft(2) means the border character should always start at column 2 (2 leading spaces)
			if boxCharIdx != 2 {
				t.Errorf("broken border alignment on line %d (expected box char at column 2, got column %d): %q", lineIdx, boxCharIdx, line)
			}
		}
	}
}

func TestAvatar_RenderAndFallback(t *testing.T) {
	// 1. Create a synthetic 16x16 test image (blue circle on white background)
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 50, G: 100, B: 200, A: 255})
		}
	}
	buf := bytes.NewBuffer(nil)
	if err := png.Encode(buf, img); err != nil {
		t.Fatalf("png encode error: %v", err)
	}

	// 2. Test RenderAvatarArt 8x4
	art, err := RenderAvatarArt(buf.Bytes(), 8, 4, true)
	if err != nil {
		t.Fatalf("RenderAvatarArt failed: %v", err)
	}
	lines := strings.Split(art, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines of avatar art, got %d", len(lines))
	}
	// Verify ANSI escapes present
	if !strings.Contains(art, "\x1b[38;2;") {
		t.Errorf("expected 24-bit TrueColor foreground escape in avatar art")
	}

	// 3. Test RenderFallbackAvatar
	fb8x4 := RenderFallbackAvatar(8, 4)
	fbLines := strings.Split(fb8x4, "\n")
	if len(fbLines) != 4 {
		t.Fatalf("expected 4 lines of fallback avatar, got %d", len(fbLines))
	}
	if !strings.Contains(fb8x4, "👤") {
		t.Errorf("expected 👤 icon in fallback avatar")
	}
}

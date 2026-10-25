package ui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"grubby/internal/config"
	"grubby/internal/github"
)

func createTestPRModel() Model {
	prs := make([]github.PullRequest, 50)
	for i := 0; i < 50; i++ {
		prs[i] = github.PullRequest{
			Number: i + 1,
			Repo:   "repo-test",
			Title:  "PR Title test",
			Author: "dev-user",
		}
	}

	prfi := textinput.New()
	prfi.Prompt = "🔍 "

	mi := textinput.New()

	cfg := &config.Config{
		Version:     1,
		ActiveSquad: "backend",
		Squads: []config.SquadConfig{
			{Name: "backend", DisplayName: "Backend Team", Members: []string{"alice"}},
		},
	}

	orgMembers := []github.OrgMember{
		{Login: "alice", Name: "Alice Smith"},
		{Login: "bob", Name: "Bob Jones"},
		{Login: "charlie", Name: "Charlie Brown"},
		{Login: "dependabot", Name: "Dependabot Bot"},
	}

	return Model{
		ActiveTab:     TabPRs,
		Config:        cfg,
		PRs:           prs,
		OrgMembers:    orgMembers,
		PRFilterInput: prfi,
		ModalInput:    mi,
		Width:         120,
		Height:        24,
	}
}

func TestPRsKeyHandling_Navigation(t *testing.T) {
	m := createTestPRModel()

	// Initial cursor
	if m.PRCursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.PRCursor)
	}

	// Down arrow
	res, _ := m.handlePRsKeys(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(Model)
	if m.PRCursor != 1 {
		t.Fatalf("expected cursor 1 after down, got %d", m.PRCursor)
	}

	// Page down
	res, _ = m.handlePRsKeys(tea.KeyMsg{Type: tea.KeyPgDown})
	m = res.(Model)
	if m.PRCursor <= 1 {
		t.Fatalf("expected cursor > 1 after pgdown, got %d", m.PRCursor)
	}

	// End
	res, _ = m.handlePRsKeys(tea.KeyMsg{Type: tea.KeyEnd})
	m = res.(Model)
	if m.PRCursor != 49 {
		t.Fatalf("expected cursor 49 after end, got %d", m.PRCursor)
	}

	// Home
	res, _ = m.handlePRsKeys(tea.KeyMsg{Type: tea.KeyHome})
	m = res.(Model)
	if m.PRCursor != 0 {
		t.Fatalf("expected cursor 0 after home, got %d", m.PRCursor)
	}
}

func TestPRsFiltering(t *testing.T) {
	m := createTestPRModel()
	m.PRs[10].Title = "Fix critical security bug in auth"
	m.PRs[10].Repo = "auth-service"

	// Filter by "security"
	m.PRFilter = "security"
	display := m.getDisplayPRs()
	if len(display) != 1 || display[0].Number != 11 {
		t.Fatalf("expected 1 matched PR #11, got %d", len(display))
	}

	// Clear filter with esc
	res, _ := m.handlePRsKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)
	if m.PRFilter != "" {
		t.Fatalf("expected cleared filter, got %s", m.PRFilter)
	}
	if len(m.getDisplayPRs()) != 50 {
		t.Fatalf("expected 50 PRs after clear, got %d", len(m.getDisplayPRs()))
	}
}

func TestOrgMemberFiltering(t *testing.T) {
	m := createTestPRModel()

	// No filter
	m.ModalInput.SetValue("")
	filtered := m.getFilteredOrgMembers()
	if len(filtered) != 4 {
		t.Fatalf("expected 4 org members, got %d", len(filtered))
	}

	// Filter by username "bob"
	m.ModalInput.SetValue("bob")
	filtered = m.getFilteredOrgMembers()
	if len(filtered) != 1 || filtered[0].Login != "bob" {
		t.Fatalf("expected bob, got %+v", filtered)
	}

	// Filter by full name "Brown"
	m.ModalInput.SetValue("Brown")
	filtered = m.getFilteredOrgMembers()
	if len(filtered) != 1 || filtered[0].Login != "charlie" {
		t.Fatalf("expected charlie, got %+v", filtered)
	}
}

func TestPRRowRendering_AgeAndAlignment(t *testing.T) {
	m := createTestPRModel()
	// Add PR with zero CreatedAt
	m.PRs = append(m.PRs, github.PullRequest{
		Number:         101,
		Repo:           "corp-repo",
		Title:          "PR with zero time",
		Author:         "developer",
		ReviewDecision: "APPROVED",
	})

	// Add PR with stale age
	m.PRs = append(m.PRs, github.PullRequest{
		Number:         102,
		Repo:           "corp-repo",
		Title:          "Stale PR",
		Author:         "dependabot",
		ReviewDecision: "CHANGES_REQUESTED",
	})

	output := m.renderPRsView()
	if output == "" {
		t.Fatalf("expected non-empty output")
	}

	// Verify that formatTimeAgo handles zero time gracefully
	if got := formatTimeAgo(time.Time{}); got != "-" {
		t.Errorf("expected '-' for zero time, got %q", got)
	}

	// Verify DaysStale handles zero time gracefully
	zeroPR := github.PullRequest{}
	if got := zeroPR.DaysStale(); got != 0 {
		t.Errorf("expected 0 for zero PR DaysStale, got %d", got)
	}
}

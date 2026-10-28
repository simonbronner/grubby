package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"grubby/internal/config"
	"grubby/internal/github"
	"grubby/internal/storage"
)

func createTestModelWithSquadAndStatus() Model {
	cfg := &config.Config{
		ActiveSquad: "backend",
		Squads: []config.SquadConfig{
			{
				Name:        "backend",
				DisplayName: "backend",
				Members:     []string{"alice", "bob"},
			},
			{
				Name:        "frontend",
				DisplayName: "frontend",
				Members:     []string{"charlie"},
			},
		},
	}

	ti := textinput.New()
	ti.Focus()

	m := Model{
		ActiveTab:         TabSquad,
		Config:            cfg,
		CurrentOrg:        "my-org",
		AuthUser:          "tester",
		Width:             120,
		Height:            30,
		InitialDataLoaded: true,
		LastRefreshTime:   time.Now().Add(-45 * time.Second),
		LastStatusCheck:   time.Now().Add(-10 * time.Second),
		ModalInput:        ti,
		OrgMembers: []github.OrgMember{
			{Login: "alice", Name: "Alice Smith"},
			{Login: "bob", Name: "Bob Jones"},
			{Login: "dave", Name: "Dave Williams"},
			{Login: "eve", Name: "Eve Adams"},
		},
		PrevComponentStatuses: make(map[string]string),
		SelectedSquadMembers:  make(map[string]bool),
		RateLimit: github.RateLimitInfo{
			Remaining: 4950,
			Limit:     5000,
		},
	}

	return m
}

func TestManageSquadModal_Flow(t *testing.T) {
	m := createTestModelWithSquadAndStatus()

	// 1. Press 'm' to open Manage Squad modal
	res, _ := m.handleSquadKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	m = res.(Model)

	if m.ActiveModal != ModalManageSquad {
		t.Fatalf("expected ActiveModal to be ModalManageSquad, got %v", m.ActiveModal)
	}
	if m.SquadManageMode != SquadModeOverview {
		t.Fatalf("expected SquadManageMode to be SquadModeOverview, got %d", m.SquadManageMode)
	}

	overview := m.renderManageSquadModal()
	if !strings.Contains(overview, "backend") || !strings.Contains(overview, "@alice") {
		t.Fatalf("expected overview to show backend squad and @alice, got:\n%s", overview)
	}

	// 2. Press 'a' to enter Add Members mode
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = res.(Model)
	if m.SquadManageMode != SquadModeAdd {
		t.Fatalf("expected SquadModeAdd, got %d", m.SquadManageMode)
	}

	// Navigate down to 'dave' and toggle with Space
	m.ModalCursor = 2 // dave
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeySpace})
	m = res.(Model)
	if !m.SelectedSquadMembers["dave"] {
		t.Fatalf("expected dave to be selected for addition")
	}

	// Press Enter to confirm adding dave
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	squad := m.Config.GetActiveSquad()
	foundDave := false
	for _, mem := range squad.Members {
		if mem == "dave" {
			foundDave = true
			break
		}
	}
	if !foundDave {
		t.Fatalf("expected dave to be added to squad members: %v", squad.Members)
	}

	// 3. Press 'd' to enter Remove Members mode
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = res.(Model)
	if m.SquadManageMode != SquadModeRemove {
		t.Fatalf("expected SquadModeRemove, got %d", m.SquadManageMode)
	}

	// Select bob for removal
	m.ModalCursor = 1 // bob
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeySpace})
	m = res.(Model)
	if !m.SelectedSquadMembers["bob"] {
		t.Fatalf("expected bob to be selected for removal")
	}

	// Confirm removal
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	squad = m.Config.GetActiveSquad()
	for _, mem := range squad.Members {
		if mem == "bob" {
			t.Fatalf("expected bob to be removed from squad members: %v", squad.Members)
		}
	}

	// 4. Press 'r' to enter Rename mode
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = res.(Model)
	if m.SquadManageMode != SquadModeRename {
		t.Fatalf("expected SquadModeRename, got %d", m.SquadManageMode)
	}
	m.ModalInput.SetValue("core-backend")
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	squad = m.Config.GetActiveSquad()
	if squad.Name != "core-backend" {
		t.Fatalf("expected squad renamed to core-backend, got %s", squad.Name)
	}

	// 5. Press 'x' to enter Delete Confirm mode
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = res.(Model)
	if m.SquadManageMode != SquadModeDeleteConfirm {
		t.Fatalf("expected SquadModeDeleteConfirm, got %d", m.SquadManageMode)
	}

	// Confirm deletion with 'y'
	res, _ = m.handleManageSquadModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = res.(Model)
	if m.ActiveModal != ModalNone {
		t.Fatalf("expected modal closed after delete, got %v", m.ActiveModal)
	}
	// Active squad should fall back to frontend
	if m.Config.ActiveSquad != "frontend" {
		t.Fatalf("expected active squad to switch to frontend, got %s", m.Config.ActiveSquad)
	}
}

func TestTopBarAndFooterRendering(t *testing.T) {
	m := createTestModelWithSquadAndStatus()

	// Set GitHub status
	m.GitHubStatus = &github.StatusSummary{
		Status: struct {
			Indicator   string `json:"indicator"`
			Description string `json:"description"`
		}{
			Indicator:   "none",
			Description: "All Systems Operational",
		},
		Components: []github.StatusComponent{
			{Name: "Git Operations", Status: "operational"},
			{Name: "API Requests", Status: "operational"},
		},
	}

	topBar := m.renderTopBar()
	// Must contain mascot
	if !strings.Contains(topBar, "ᘛ( ᵔ ᴗ ᵔ*)ᘚ") {
		t.Errorf("expected topBar to contain Hobbit mascot")
	}
	// Must contain tabs including Status
	if (!strings.Contains(topBar, "1: Repos") && !strings.Contains(topBar, "1: Repositories")) || !strings.Contains(topBar, "Status") {
		t.Errorf("expected topBar to contain tabs, got:\n%s", topBar)
	}
	// Must contain status healthy dot (green)
	if !strings.Contains(topBar, "●") {
		t.Errorf("expected topBar to contain status healthy dot ●, got:\n%s", topBar)
	}
	// Must contain security vulnerabilities indicator in top bar
	if !strings.Contains(topBar, "🛡️ Clean") {
		t.Errorf("expected topBar to contain security summary, got:\n%s", topBar)
	}
	// Must contain consolidated refresh & rate limit
	if !strings.Contains(topBar, "⟳") || (!strings.Contains(topBar, "reqs/h") && !strings.Contains(topBar, "/h")) {
		t.Errorf("expected topBar to contain consolidated refresh & rate indicator, got:\n%s", topBar)
	}

	footer := m.renderFooter()
	// Single-line footer: Must contain Squad help shortcuts on left
	if !strings.Contains(footer, "[m] Manage Squad") {
		t.Errorf("expected footer to contain [m] Manage Squad help key")
	}
	// Right side: Must contain consolidated identity '{username} @ {org} in {squad}'
	if !strings.Contains(footer, "tester @ my-org in backend") {
		t.Errorf("expected footer to contain consolidated identity 'tester @ my-org in backend', got:\n%s", footer)
	}
	// The separate metadata bar line must be gone (single-line footer, no newline)
	if strings.Contains(footer, "\n") {
		t.Errorf("expected single-line footer without separate metadata bar, got multiline footer:\n%s", footer)
	}
}

func TestGitHubStatusView_RenderingAndKeys(t *testing.T) {
	m := createTestModelWithSquadAndStatus()
	m.ActiveTab = TabStatus

	m.GitHubStatus = &github.StatusSummary{
		Status: struct {
			Indicator   string `json:"indicator"`
			Description string `json:"description"`
		}{
			Indicator:   "minor",
			Description: "Minor Service Outage",
		},
		Components: []github.StatusComponent{
			{Name: "Git Operations", Status: "operational", Description: "Pulls and pushes"},
			{Name: "Actions", Status: "degraded_performance", Description: "Workflow execution"},
		},
		Incidents: []github.StatusIncident{
			{
				Name:   "Actions Queue Delays",
				Status: "investigating",
				Impact: "minor",
				IncidentUpdates: []github.IncidentUpdate{
					{Body: "Investigating runner queue delays", CreatedAt: time.Now()},
				},
			},
		},
	}

	view := m.renderStatusView()
	if !strings.Contains(view, "MINOR SERVICE OUTAGE") {
		t.Errorf("expected view to render outage banner, got:\n%s", view)
	}
	if !strings.Contains(view, "Actions Queue Delays") {
		t.Errorf("expected view to render active incident")
	}
	if !strings.Contains(view, "Git Operations") || !strings.Contains(view, "DEGRADED") {
		t.Errorf("expected components table to show Git Operations and DEGRADED Actions")
	}

	// Test navigation keys
	m.StatusCursor = 0
	res, _ := m.handleStatusKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = res.(Model)
	if m.StatusCursor != 1 {
		t.Fatalf("expected StatusCursor to move to 1, got %d", m.StatusCursor)
	}

	res, _ = m.handleStatusKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = res.(Model)
	if m.StatusCursor != 0 {
		t.Fatalf("expected StatusCursor to move to 0, got %d", m.StatusCursor)
	}
}

func TestGitHubStatusLoadedMsg_DegradationAndRestoration(t *testing.T) {
	m := createTestModelWithSquadAndStatus()

	// Initial status (both operational)
	m.PrevComponentStatuses = map[string]string{
		"Git Operations": "operational",
		"Actions":        "operational",
	}

	// Receive msg where Actions becomes degraded
	summaryDegraded := &github.StatusSummary{
		Status: struct {
			Indicator   string `json:"indicator"`
			Description string `json:"description"`
		}{
			Indicator:   "minor",
			Description: "Degraded performance",
		},
		Components: []github.StatusComponent{
			{ID: "git-op", Name: "Git Operations", Status: "operational"},
			{ID: "act-run", Name: "Actions", Status: "degraded_performance", Description: "Delayed runs"},
		},
	}

	res, _ := m.Update(GitHubStatusLoadedMsg{Summary: summaryDegraded})
	m = res.(Model)

	if len(m.Notifications) == 0 {
		t.Fatalf("expected a notification to be generated on degradation")
	}
	if !strings.Contains(m.Notifications[0].Title, "Actions Degraded") {
		t.Errorf("expected notification title 'Actions Degraded', got %s", m.Notifications[0].Title)
	}

	// Now receive msg where Actions is restored
	summaryRestored := &github.StatusSummary{
		Status: struct {
			Indicator   string `json:"indicator"`
			Description string `json:"description"`
		}{
			Indicator:   "none",
			Description: "All Systems Operational",
		},
		Components: []github.StatusComponent{
			{ID: "git-op", Name: "Git Operations", Status: "operational"},
			{ID: "act-run", Name: "Actions", Status: "operational"},
		},
	}

	res, _ = m.Update(GitHubStatusLoadedMsg{Summary: summaryRestored})
	m = res.(Model)

	if len(m.Notifications) < 2 {
		t.Fatalf("expected recovery notification to be generated")
	}
	if !strings.Contains(m.Notifications[0].Title, "Actions Operational") {
		t.Errorf("expected recovery notification title 'Actions Operational', got %s", m.Notifications[0].Title)
	}
}

func TestSelectSquadModal_ListsAvailableSquadsFromConfigAndDB(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	// Seed DB with discovered squads
	err = db.SaveSquads([]github.SquadDefinition{
		{Name: "platform", DisplayName: "Platform Team", Members: []string{"alice", "bob"}},
		{Name: "security", DisplayName: "Security Team", Members: []string{"charlie"}},
	})
	if err != nil {
		t.Fatalf("failed to save squads in db: %v", err)
	}

	cfg := &config.Config{
		ActiveSquad: "backend",
		Squads: []config.SquadConfig{
			{Name: "backend", DisplayName: "Backend Team", Members: []string{"dev1"}},
		},
	}

	m := Model{
		Config:          cfg,
		DB:              db,
		ActiveModal:     ModalSelectSquad,
		ModalCursor:     0,
		SplashDismissed: true,
	}

	available := m.getAvailableSquads()
	if len(available) != 3 {
		t.Fatalf("expected 3 squads (1 config + 2 db), got %d", len(available))
	}

	// Verify render shows all available squads
	rendered := m.renderSelectSquadModal()
	if !strings.Contains(rendered, "Backend Team (Active)") {
		t.Errorf("expected rendered modal to show active squad, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Platform Team") {
		t.Errorf("expected rendered modal to show Platform Team, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Security Team") {
		t.Errorf("expected rendered modal to show Security Team, got:\n%s", rendered)
	}

	// Move cursor down to Platform Team
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = res.(Model)
	if m.ModalCursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.ModalCursor)
	}

	// Press Enter to select Platform Team
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if m.ActiveModal != ModalNone {
		t.Errorf("expected modal to close, got %v", m.ActiveModal)
	}
	if m.Config.ActiveSquad != "platform" {
		t.Errorf("expected active squad to switch to platform, got: %s", m.Config.ActiveSquad)
	}
	// Verify it was added to Config.Squads as well
	found := false
	for _, s := range m.Config.Squads {
		if s.Name == "platform" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected platform squad to be added to Config.Squads")
	}
}

func TestDataLoadedMsg_SyncsSquads(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		ActiveSquad: "core",
		Squads: []config.SquadConfig{
			{Name: "core", DisplayName: "Core", Members: []string{"simon"}},
		},
	}

	m := Model{
		Config: cfg,
		DB:     db,
	}

	msg := DataLoadedMsg{
		Squads: []github.SquadDefinition{
			{Name: "sre", DisplayName: "Site Reliability", Members: []string{"ops1", "ops2"}},
		},
	}

	res, _ := m.Update(msg)
	m = res.(Model)

	// Check Config.Squads
	if len(m.Config.Squads) != 2 {
		t.Fatalf("expected 2 squads in config, got %d", len(m.Config.Squads))
	}
	if m.Config.Squads[1].Name != "sre" {
		t.Errorf("expected sre squad in config, got %s", m.Config.Squads[1].Name)
	}

	// Check DB.GetSquads
	dbSquads, err := db.GetSquads()
	if err != nil {
		t.Fatalf("failed to get squads from db: %v", err)
	}
	if len(dbSquads) != 1 || dbSquads[0].Name != "sre" {
		t.Errorf("unexpected squads in db: %+v", dbSquads)
	}
}


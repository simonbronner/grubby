package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"grubby/internal/config"
	"grubby/internal/git"
	"grubby/internal/github"
)

func createTestModel() Model {
	cfg := &config.Config{
		Version:      1,
		WorkspaceDir: "~/workspaces",
		Groups: []config.GroupConfig{
			{Name: "backend", DisplayName: "Backend Services", Repos: []string{"auth-service", "api-gateway"}},
			{Name: "frontend", DisplayName: "Frontend Web", Repos: []string{"web-client"}},
		},
	}

	repos := []github.Repository{
		{Name: "auth-service", FullName: "acme/auth-service"},
		{Name: "api-gateway", FullName: "acme/api-gateway"},
		{Name: "web-client", FullName: "acme/web-client"},
		{Name: "docs", FullName: "acme/docs"},
	}

	secSummaries := map[string]github.SecurityAlertSummary{
		"auth-service": {Total: 2, Critical: 1, High: 1},
		"api-gateway":  {Total: 8, Critical: 3, High: 2, Medium: 3},
		"web-client":   {Total: 1, Critical: 0, High: 0, Medium: 1},
		"docs":         {Total: 0},
	}

	rfi := textinput.New()
	rfi.Placeholder = "Type repository name substring..."
	rfi.Prompt = "🔍 "

	return Model{
		ActiveTab:         TabRepos,
		Config:            cfg,
		Repos:             repos,
		SecuritySummaries: secSummaries,
		GitStatuses:       make(map[string]git.RepoGitStatus),
		RepoFilterInput:   rfi,
	}
}

func TestGetDisplayRepos_NameFilter(t *testing.T) {
	m := createTestModel()

	// Default: all 4 repos
	display := m.getDisplayRepos()
	if len(display) != 4 {
		t.Fatalf("expected 4 repos, got %d", len(display))
	}

	// Filter by substring "api"
	m.RepoFilter = "api"
	display = m.getDisplayRepos()
	if len(display) != 1 || display[0].Name != "api-gateway" {
		t.Fatalf("expected api-gateway, got %+v", display)
	}

	// Filter case-insensitive "AUTH"
	m.RepoFilter = "AUTH"
	display = m.getDisplayRepos()
	if len(display) != 1 || display[0].Name != "auth-service" {
		t.Fatalf("expected auth-service, got %+v", display)
	}

	// Clear filter
	m.RepoFilter = ""
	display = m.getDisplayRepos()
	if len(display) != 4 {
		t.Fatalf("expected 4 repos after clear, got %d", len(display))
	}
}

func TestGetDisplayRepos_GroupFilter(t *testing.T) {
	m := createTestModel()

	// Filter by group "backend" (auth-service, api-gateway)
	m.RepoGroupFilter = "backend"
	display := m.getDisplayRepos()
	if len(display) != 2 {
		t.Fatalf("expected 2 backend repos, got %d", len(display))
	}

	// Filter by group "frontend" (web-client)
	m.RepoGroupFilter = "frontend"
	display = m.getDisplayRepos()
	if len(display) != 1 || display[0].Name != "web-client" {
		t.Fatalf("expected web-client, got %+v", display)
	}

	// Combined group + name filter
	m.RepoGroupFilter = "backend"
	m.RepoFilter = "auth"
	display = m.getDisplayRepos()
	if len(display) != 1 || display[0].Name != "auth-service" {
		t.Fatalf("expected auth-service for combined filter, got %+v", display)
	}
}

func TestGetDisplayRepos_SortByVulnerabilities(t *testing.T) {
	m := createTestModel()

	m.RepoSortVulnerabilities = true
	display := m.getDisplayRepos()

	// Order should be:
	// 1. api-gateway (Total: 8, Crit: 3)
	// 2. auth-service (Total: 2, Crit: 1)
	// 3. web-client (Total: 1, Crit: 0)
	// 4. docs (Total: 0)
	if len(display) != 4 {
		t.Fatalf("expected 4 repos, got %d", len(display))
	}

	if display[0].Name != "api-gateway" {
		t.Errorf("expected #1 api-gateway, got %s", display[0].Name)
	}
	if display[1].Name != "auth-service" {
		t.Errorf("expected #2 auth-service, got %s", display[1].Name)
	}
	if display[2].Name != "web-client" {
		t.Errorf("expected #3 web-client, got %s", display[2].Name)
	}
	if display[3].Name != "docs" {
		t.Errorf("expected #4 docs, got %s", display[3].Name)
	}
}

func TestRepoClonedMsg_ImmediateDetection(t *testing.T) {
	m := createTestModel()

	// Initially repo is not cloned
	if st, ok := m.GitStatuses["auth-service"]; ok && st.IsCloned {
		t.Fatalf("expected auth-service to not be cloned initially")
	}

	// Simulate RepoClonedMsg
	msg := RepoClonedMsg{
		RepoName:  "auth-service",
		TargetDir: "~/workspaces/auth-service",
		Status: git.RepoGitStatus{
			RepoName:  "auth-service",
			LocalPath: "~/workspaces/auth-service",
			IsCloned:  true,
			Branch:    "main",
			State:     git.StateSynced,
		},
		Err: nil,
	}

	updatedModel, _ := m.Update(msg)
	mod := updatedModel.(Model)

	st, found := mod.GitStatuses["auth-service"]
	if !found || !st.IsCloned {
		t.Fatalf("expected auth-service to be marked cloned immediately, got: %+v", st)
	}
	if st.Branch != "main" {
		t.Errorf("expected branch main, got %s", st.Branch)
	}
	if st.State != git.StateSynced {
		t.Errorf("expected StateSynced, got %v", st.State)
	}
}

func TestReposKeyHandling_FilterAndSortKeys(t *testing.T) {
	m := createTestModel()

	// Press 'v' -> opens ModalFilterGroup
	updated, _ := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = updated.(Model)
	if m.ActiveModal != ModalFilterGroup {
		t.Errorf("expected ModalFilterGroup, got %v", m.ActiveModal)
	}

	// Press 'esc' inside modal -> closes modal
	updated, _ = m.handleFilterGroupModalKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.ActiveModal != ModalNone {
		t.Errorf("expected ModalNone after esc, got %v", m.ActiveModal)
	}

	// Press 'f' -> opens ModalFilterRepoName
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = updated.(Model)
	if m.ActiveModal != ModalFilterRepoName {
		t.Errorf("expected ModalFilterRepoName, got %v", m.ActiveModal)
	}

	// Type query and press enter inside filter modal
	m.RepoFilterInput.SetValue("gateway")
	updated, _ = m.handleFilterNameModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.ActiveModal != ModalNone {
		t.Errorf("expected ModalNone after enter, got %v", m.ActiveModal)
	}
	if m.RepoFilter != "gateway" {
		t.Errorf("expected RepoFilter 'gateway', got %q", m.RepoFilter)
	}
	if len(m.getDisplayRepos()) != 1 {
		t.Errorf("expected 1 match for 'gateway', got %d", len(m.getDisplayRepos()))
	}

	// Press 'esc' on repos tab -> clears filter
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.RepoFilter != "" {
		t.Errorf("expected cleared RepoFilter, got %q", m.RepoFilter)
	}

	// Press 's' -> toggles sort by vulnerabilities
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)
	if !m.RepoSortVulnerabilities {
		t.Errorf("expected RepoSortVulnerabilities=true")
	}

	// Press 's' again -> toggles off
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)
	if m.RepoSortVulnerabilities {
		t.Errorf("expected RepoSortVulnerabilities=false")
	}
}

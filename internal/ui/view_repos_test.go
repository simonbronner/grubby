package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"grubby/internal/config"
	"grubby/internal/git"
	"grubby/internal/github"
	"grubby/internal/storage"
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
		SplashDismissed:   true,
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

	// Filter by group name "backend" via unified filter (auth-service, api-gateway)
	m.RepoFilter = "backend"
	display := m.getDisplayRepos()
	if len(display) != 2 {
		t.Fatalf("expected 2 backend repos, got %d", len(display))
	}

	// Filter by group name "frontend" via unified filter (web-client)
	m.RepoFilter = "frontend"
	display = m.getDisplayRepos()
	if len(display) != 1 || display[0].Name != "web-client" {
		t.Fatalf("expected web-client, got %+v", display)
	}

	// Filter by repo name "auth" via unified filter
	m.RepoFilter = "auth"
	display = m.getDisplayRepos()
	if len(display) != 1 || display[0].Name != "auth-service" {
		t.Fatalf("expected auth-service for auth filter, got %+v", display)
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

	// Press '/' -> opens ModalFilterRepoName
	updated, _ := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updated.(Model)
	if m.ActiveModal != ModalFilterRepoName {
		t.Errorf("expected ModalFilterRepoName, got %v", m.ActiveModal)
	}

	// Press 'esc' inside modal -> closes modal
	updated, _ = m.handleFilterNameModalKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.ActiveModal != ModalNone {
		t.Errorf("expected ModalNone after esc, got %v", m.ActiveModal)
	}

	// Press '/' again, type query and press enter inside filter modal
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

	// Filter by group name "backend"
	m.RepoFilterInput.SetValue("backend")
	updated, _ = m.handleFilterNameModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if len(m.getDisplayRepos()) != 2 {
		t.Errorf("expected 2 matches for group 'backend', got %d", len(m.getDisplayRepos()))
	}

	// Press 'esc' again -> clears
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)

	// Press 'v' -> toggles sort by vulnerabilities
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = updated.(Model)
	if !m.RepoSortVulnerabilities {
		t.Errorf("expected RepoSortVulnerabilities=true")
	}

	// Press 'v' again -> toggles off
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = updated.(Model)
	if m.RepoSortVulnerabilities {
		t.Errorf("expected RepoSortVulnerabilities=false")
	}

	// Test uncloned repo auto-clone triggers
	// docs is not cloned in createTestModel()
	m.RepoCursor = 3 // docs
	updated, cmd := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = updated.(Model)
	if cmd == nil {
		t.Errorf("expected non-nil clone command when pressing 's' on uncloned repo")
	}
	if !strings.Contains(m.StatusMsg, "Cloning") {
		t.Errorf("expected status msg to indicate cloning, got %q", m.StatusMsg)
	}

	updated, cmd = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(Model)
	if cmd == nil {
		t.Errorf("expected non-nil clone command when pressing 'e' on uncloned repo")
	}

	updated, cmd = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = updated.(Model)
	if cmd == nil {
		t.Errorf("expected non-nil clone command when pressing 't' on uncloned repo")
	}
}

func TestRenderReposView_TmuxSessionColumn(t *testing.T) {
	m := createTestModel()
	m.Width = 120
	m.Height = 25

	rendered := m.renderReposView()

	// Verify TMUX column is present and BRANCH is dropped
	if !strings.Contains(rendered, "TMUX") {
		t.Errorf("expected rendered repos view to contain 'TMUX' header column, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "BRANCH") {
		t.Errorf("expected rendered repos view to drop 'BRANCH' column")
	}
}

func TestGetDisplayRepos_SortByInteractedThenUpdated(t *testing.T) {
	now := time.Now()

	cfg := &config.Config{
		Version: 1,
		RepoInteractions: map[string]time.Time{
			"repo-interacted-old": now.Add(-2 * time.Hour),
			"repo-interacted-new": now.Add(-10 * time.Minute),
		},
	}

	repos := []github.Repository{
		{Name: "repo-never-touched-old", PushedAt: now.Add(-5 * 24 * time.Hour)},
		{Name: "repo-never-touched-new", PushedAt: now.Add(-1 * time.Hour)},
		{Name: "repo-interacted-old", PushedAt: now.Add(-10 * 24 * time.Hour)},
		{Name: "repo-interacted-new", PushedAt: now.Add(-12 * 24 * time.Hour)},
		{Name: "repo-tmux-session", PushedAt: now.Add(-20 * 24 * time.Hour)},
	}

	// repo-tmux-session has an active tmux session with latest activity 5 mins ago
	tmuxSessions := map[string]TmuxSessionInfo{
		"repo-tmux-session": {
			Windows:      1,
			LastActivity: now.Add(-5 * time.Minute),
		},
	}

	m := Model{
		Config:       cfg,
		Repos:        repos,
		TmuxSessions: tmuxSessions,
		GitStatuses:  make(map[string]git.RepoGitStatus),
	}

	display := m.getDisplayRepos()
	if len(display) != 5 {
		t.Fatalf("expected 5 repos, got %d", len(display))
	}

	// Expected order:
	// 1. repo-tmux-session (interacted via tmux 5m ago)
	// 2. repo-interacted-new (interacted 10m ago)
	// 3. repo-interacted-old (interacted 2h ago)
	// 4. repo-never-touched-new (updated 1h ago)
	// 5. repo-never-touched-old (updated 5d ago)
	expected := []string{
		"repo-tmux-session",
		"repo-interacted-new",
		"repo-interacted-old",
		"repo-never-touched-new",
		"repo-never-touched-old",
	}

	for i, exp := range expected {
		if display[i].Name != exp {
			t.Errorf("pos %d: expected %s, got %s", i, exp, display[i].Name)
		}
	}
}

func TestReposKeyHandling_RecordInteractionMovesToTop(t *testing.T) {
	now := time.Now()

	cfg := &config.Config{
		Version: 1,
		RepoInteractions: map[string]time.Time{
			"repo-a": now.Add(-10 * time.Minute),
		},
	}

	repos := []github.Repository{
		{Name: "repo-a", PushedAt: now.Add(-2 * time.Hour)},
		{Name: "repo-b", PushedAt: now.Add(-1 * time.Hour)},
	}

	m := Model{
		Config:       cfg,
		Repos:        repos,
		TmuxSessions: make(map[string]TmuxSessionInfo),
		GitStatuses: map[string]git.RepoGitStatus{
			"repo-b": {RepoName: "repo-b", IsCloned: true, LocalPath: "/tmp/repo-b"},
		},
	}

	// Initially repo-a is at index 0 because it was interacted with 10m ago
	display := m.getDisplayRepos()
	if display[0].Name != "repo-a" {
		t.Fatalf("expected repo-a at pos 0 initially, got %s", display[0].Name)
	}

	// Cursor is on repo-b, user presses Enter to open web
	for idx, it := range m.getRepoTableItems() {
		if it.Kind == ItemRepo && it.Repo != nil && it.Repo.Name == "repo-b" {
			m.RepoCursor = idx
			break
		}
	}
	updated, _ := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// repo-b was interacted with just now, so it should move to pos 0 in getDisplayRepos()
	newDisplay := m.getDisplayRepos()
	if newDisplay[0].Name != "repo-b" {
		t.Errorf("expected repo-b to move to pos 0 after interaction, got %s", newDisplay[0].Name)
	}
	// And cursor follows repo-b in table items
	items := m.getRepoTableItems()
	if m.RepoCursor < len(items) && (items[m.RepoCursor].Repo == nil || items[m.RepoCursor].Repo.Name != "repo-b") {
		t.Errorf("expected cursor to follow repo-b, got item: %+v", items[m.RepoCursor])
	}
}

func TestRepoTiersRenderingAndSorting(t *testing.T) {
	now := time.Now()
	repos := []github.Repository{
		{Name: "dead-repo", PushedAt: now.Add(-400 * 24 * time.Hour)},
		{Name: "cold-repo", PushedAt: now.Add(-60 * 24 * time.Hour)},
		{Name: "warm-repo", PushedAt: now.Add(-10 * 24 * time.Hour)},
		{Name: "hot-repo", PushedAt: now.Add(-1 * time.Hour)},
	}

	tiers := map[string]storage.RepoTier{
		"dead-repo": storage.TierDead,
		"cold-repo": storage.TierCold,
		"warm-repo": storage.TierWarm,
		"hot-repo":  storage.TierHot,
	}

	m := Model{
		Repos:        repos,
		RepoTiers:    tiers,
		GitStatuses:  make(map[string]git.RepoGitStatus),
		Width:        140,
		Height:       30,
		TmuxSessions: make(map[string]TmuxSessionInfo),
	}

	// 1. Verify sorting: Hot comes first, then Warm, then Cold, then Dead
	sorted := m.getDisplayRepos()
	if len(sorted) != 4 {
		t.Fatalf("expected 4 repos, got %d", len(sorted))
	}
	if sorted[0].Name != "hot-repo" {
		t.Errorf("expected hot-repo first, got %s", sorted[0].Name)
	}
	if sorted[1].Name != "warm-repo" {
		t.Errorf("expected warm-repo second, got %s", sorted[1].Name)
	}
	if sorted[2].Name != "cold-repo" {
		t.Errorf("expected cold-repo third, got %s", sorted[2].Name)
	}
	if sorted[3].Name != "dead-repo" {
		t.Errorf("expected dead-repo fourth, got %s", sorted[3].Name)
	}

	// 2. Verify rendering: tier headers as tree nodes, and TIER column is removed
	view := m.renderReposView()
	if strings.Contains(view, "  TIER  ") {
		t.Errorf("expected 'TIER' header column to be removed, got:\n%s", view)
	}
	if !strings.Contains(view, "▼ 🔥 HOT") || !strings.Contains(view, "▼ 🌞 WARM") || !strings.Contains(view, "▼ 🧊 COLD") || !strings.Contains(view, "▶ 💀 DEAD") {
		t.Errorf("expected tier tree nodes, got:\n%s", view)
	}
	// Dead tier is collapsed by default: dead-repo should not be visible
	if strings.Contains(view, "dead-repo") {
		t.Errorf("expected dead-repo to be hidden when Dead tier is collapsed, got:\n%s", view)
	}
	// Space to expand hint shown on collapsed Dead tier
	if !strings.Contains(view, "[Space to expand]") {
		t.Errorf("expected '[Space to expand]' hint on collapsed Dead tier, got:\n%s", view)
	}

	// 3. Expand Dead tier -> dead-repo should appear
	m.CollapsedTiers = map[storage.RepoTier]bool{storage.TierDead: false}
	viewExpanded := m.renderReposView()
	if !strings.Contains(viewExpanded, "dead-repo") {
		t.Errorf("expected dead-repo to be visible after expanding Dead tier, got:\n%s", viewExpanded)
	}

	// 4. Verify tier count summary banner
	if !strings.Contains(view, "🔥 1 Hot") || !strings.Contains(view, "🌞 1 Warm") || !strings.Contains(view, "🧊 1 Cold") || !strings.Contains(view, "💀 1 Dead") {
		t.Errorf("expected tier count summary in header banner, got:\n%s", view)
	}
}

func TestRepoOpenPRsFilteredWithPKey(t *testing.T) {
	repos := []github.Repository{
		{Name: "my-service", FullName: "my-org/my-service"},
		{Name: "other-repo", FullName: "my-org/other-repo"},
	}
	m := Model{
		ActiveTab:  TabRepos,
		Repos:      repos,
		Width:      140,
		Height:     30,
	}
	for idx, it := range m.getRepoTableItems() {
		if it.Kind == ItemRepo && it.Repo != nil && it.Repo.Name == "my-service" {
			m.RepoCursor = idx
			break
		}
	}

	updated, cmd := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	upModel := updated.(Model)

	if upModel.ActiveTab != TabPRs {
		t.Errorf("expected ActiveTab to be TabPRs (%d), got %d", TabPRs, upModel.ActiveTab)
	}
	if upModel.PRFilter != "my-service" {
		t.Errorf("expected PRFilter to be 'my-service', got %q", upModel.PRFilter)
	}
	if upModel.PRCursor != 0 {
		t.Errorf("expected PRCursor to be 0, got %d", upModel.PRCursor)
	}
	if cmd == nil {
		t.Errorf("expected status toast cmd, got nil")
	}
}

func TestRepoInteractionTouchesSQLite(t *testing.T) {
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	defer db.Close()

	now := time.Now()
	repos := []github.Repository{
		{Name: "my-cold-repo", FullName: "org/my-cold-repo", PushedAt: now.Add(-60 * 24 * time.Hour)},
	}
	if err := db.SaveRepositories(repos, "org"); err != nil {
		t.Fatalf("SaveRepositories failed: %v", err)
	}
	_ = db.SetRepoTier("my-cold-repo", storage.TierCold)

	m := Model{
		DB:           db,
		Repos:        repos,
		RepoTiers:    map[string]storage.RepoTier{"my-cold-repo": storage.TierCold},
		GitStatuses:  make(map[string]git.RepoGitStatus),
		Width:        120,
		Height:       30,
		TmuxSessions: make(map[string]TmuxSessionInfo),
	}

	if tier := m.getRepoTier("my-cold-repo"); tier != storage.TierCold {
		t.Fatalf("expected initial tier cold, got %v", tier)
	}

	// User interacts with repo (Enter key on my-cold-repo)
	for idx, it := range m.getRepoTableItems() {
		if it.Kind == ItemRepo && it.Repo != nil && it.Repo.Name == "my-cold-repo" {
			m.RepoCursor = idx
			break
		}
	}
	updated, _ := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	// Interaction must NOT change tier (it stays TierCold!)
	if tier := m.getRepoTier("my-cold-repo"); tier != storage.TierCold {
		t.Errorf("expected in-memory tier to remain TierCold, got %v", tier)
	}
	if dbTier := db.GetRepoTier("my-cold-repo"); dbTier != storage.TierCold {
		t.Errorf("expected SQLite db tier to remain TierCold, got %v", dbTier)
	}

	// SQLite database records last_interacted_at
	interacted := db.GetRepoLastInteractedAt("my-cold-repo")
	if interacted.IsZero() || time.Since(interacted) > 5*time.Second {
		t.Errorf("expected valid recent last_interacted_at, got %v", interacted)
	}
}

func TestRepoInteractionPushesToTopOfTier(t *testing.T) {
	now := time.Now()
	repos := []github.Repository{
		{Name: "hot-1", PushedAt: now.Add(-1 * time.Hour)},
		{Name: "cold-a", PushedAt: now.Add(-50 * 24 * time.Hour)},
		{Name: "cold-b", PushedAt: now.Add(-45 * 24 * time.Hour)}, // pushed more recently than cold-a
		{Name: "dead-1", PushedAt: now.Add(-500 * 24 * time.Hour)},
	}

	tiers := map[string]storage.RepoTier{
		"hot-1":  storage.TierHot,
		"cold-a": storage.TierCold,
		"cold-b": storage.TierCold,
		"dead-1": storage.TierDead,
	}

	m := Model{
		Repos:        repos,
		RepoTiers:    tiers,
		GitStatuses:  make(map[string]git.RepoGitStatus),
		Width:        140,
		Height:       30,
		TmuxSessions: make(map[string]TmuxSessionInfo),
	}

	// Initially, cold-b comes before cold-a because it was pushed more recently (45d vs 50d)
	sorted := m.getDisplayRepos()
	if sorted[1].Name != "cold-b" || sorted[2].Name != "cold-a" {
		t.Fatalf("expected [hot-1, cold-b, cold-a, dead-1], got %s, %s, %s, %s",
			sorted[0].Name, sorted[1].Name, sorted[2].Name, sorted[3].Name)
	}

	// User interacts with cold-a (inspecting it)
	m = m.recordRepoInteraction("cold-a")

	// cold-a must still be TierCold!
	if tier := m.getRepoTier("cold-a"); tier != storage.TierCold {
		t.Errorf("expected cold-a to stay TierCold, got %v", tier)
	}

	// Now cold-a must be pushed to the top of the Cold tier (ahead of cold-b, but behind hot-1)!
	sortedAfter := m.getDisplayRepos()
	if sortedAfter[0].Name != "hot-1" {
		t.Errorf("expected hot-1 to remain first, got %s", sortedAfter[0].Name)
	}
	if sortedAfter[1].Name != "cold-a" {
		t.Errorf("expected interacted cold-a to be pushed to top of Cold tier (position 1), got %s", sortedAfter[1].Name)
	}
	if sortedAfter[2].Name != "cold-b" {
		t.Errorf("expected cold-b at position 2, got %s", sortedAfter[2].Name)
	}
	if sortedAfter[3].Name != "dead-1" {
		t.Errorf("expected dead-1 to remain at position 3, got %s", sortedAfter[3].Name)
	}
}

func TestCollapsibleTierTree_TogglingAndAutoExpansion(t *testing.T) {
	now := time.Now()
	repos := []github.Repository{
		{ID: 101, Name: "hot-repo", PushedAt: now.Add(-1 * time.Hour)},
		{ID: 102, Name: "warm-repo", PushedAt: now.Add(-5 * 24 * time.Hour)},
		{ID: 103, Name: "dead-repo", PushedAt: now.Add(-400 * 24 * time.Hour)},
	}
	tiers := map[string]storage.RepoTier{
		"hot-repo":  storage.TierHot,
		"warm-repo": storage.TierWarm,
		"dead-repo": storage.TierDead,
	}

	m := Model{
		Repos:           repos,
		RepoTiers:       tiers,
		GitStatuses:     make(map[string]git.RepoGitStatus),
		Width:           120,
		Height:          30,
		SplashDismissed: true,
		CollapsedTiers:  map[storage.RepoTier]bool{storage.TierDead: true},
	}

	// 1. Initial State:
	// Hot tier (Header + hot-repo)
	// Warm tier (Header + warm-repo)
	// Dead tier (Header only, collapsed)
	items := m.getRepoTableItems()
	if len(items) != 5 {
		t.Fatalf("expected 5 table items (Hot Hdr + Repo, Warm Hdr + Repo, Dead Hdr), got %d", len(items))
	}
	if items[4].Kind != ItemTierHeader || items[4].Tier != storage.TierDead || items[4].Expanded {
		t.Errorf("expected item 4 to be collapsed Dead tier header, got: %+v", items[4])
	}

	// 2. Cursor on Dead tier header (index 4), press Space -> expands
	m.RepoCursor = 4
	updated, _ := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updated.(Model)
	if m.isTierCollapsed(storage.TierDead) {
		t.Fatalf("expected Dead tier to be expanded after Space")
	}
	itemsAfterExpand := m.getRepoTableItems()
	if len(itemsAfterExpand) != 6 {
		t.Fatalf("expected 6 items after expanding Dead tier (now includes dead-repo), got %d", len(itemsAfterExpand))
	}
	if itemsAfterExpand[5].Kind != ItemRepo || itemsAfterExpand[5].Repo.Name != "dead-repo" {
		t.Errorf("expected item 5 to be dead-repo, got: %+v", itemsAfterExpand[5])
	}

	// 3. Cursor on dead-repo (index 5), press '[' -> collapses Dead tier and lands cursor back on Dead header
	m.RepoCursor = 5
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = updated.(Model)
	if !m.isTierCollapsed(storage.TierDead) {
		t.Fatalf("expected Dead tier to be collapsed after pressing '[' on child repo")
	}
	if m.RepoCursor != 4 {
		t.Errorf("expected cursor to jump to Dead header at index 4, got %d", m.RepoCursor)
	}

	// 4. Auto-expansion during search ('/' or '#')
	// Filter by "dead"
	m.RepoFilter = "dead"
	if m.isTierCollapsed(storage.TierDead) {
		t.Errorf("expected Dead tier to auto-expand when search query is active")
	}
	autoExpandedItems := m.getRepoTableItems()
	// Only Dead tier matches "dead" -> Dead Header + dead-repo = 2 items
	if len(autoExpandedItems) != 2 {
		t.Fatalf("expected 2 items for 'dead' query (Header + dead-repo), got %d", len(autoExpandedItems))
	}

	// 5. Esc clears search and restores collapsed state
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.RepoFilter != "" {
		t.Errorf("expected RepoFilter to be cleared, got %q", m.RepoFilter)
	}
	if !m.isTierCollapsed(storage.TierDead) {
		t.Errorf("expected Dead tier to be collapsed again after clearing search")
	}
}

func TestSummaryColumnAndInlineLabelsRendering(t *testing.T) {
	now := time.Now()
	repos := []github.Repository{
		{
			ID:          101,
			Name:        "auth-svc",
			Description: "Authentication and authorization service for core APIs",
			PushedAt:    now.Add(-1 * time.Hour),
		},
	}
	tiers := map[string]storage.RepoTier{
		"auth-svc": storage.TierHot,
	}
	secSummaries := map[string]github.SecurityAlertSummary{
		"auth-svc": {Total: 5, Critical: 1, High: 2, Medium: 2},
	}
	prs := []github.PullRequest{
		{Repo: "auth-svc", Number: 10, State: "OPEN"},
		{Repo: "auth-svc", Number: 11, State: "OPEN"},
	}
	personalTasks := []github.Task{
		{RepoName: "auth-svc", Number: 1, State: "open"},
	}
	squadTasks := []github.Task{
		{RepoName: "auth-svc", Number: 2, State: "open"},
		{RepoName: "auth-svc", Number: 3, State: "open"},
	}

	m := Model{
		Repos:             repos,
		RepoTiers:         tiers,
		SecuritySummaries: secSummaries,
		PRs:               prs,
		PersonalTasks:     personalTasks,
		SquadTasks:        squadTasks,
		RepoLabels: map[int64][]string{
			101: {"backend", "core"},
		},
		RepoLabelColors: map[string]string{
			"backend": "1d76db",
			"core":    "5319e7",
		},
		GitStatuses:     make(map[string]git.RepoGitStatus),
		Width:           160,
		Height:          30,
		SplashDismissed: true,
	}

	view := m.renderReposView()

	// 1. Verify column header is (Vulns - PRs - Issues)
	if !strings.Contains(view, "(Vulns - PRs - Issues)") {
		t.Errorf("expected '(Vulns - PRs - Issues)' column header, got:\n%s", view)
	}

	// 2. Verify inline labels #backend and #core are rendered
	if !strings.Contains(view, "#backend") || !strings.Contains(view, "#core") {
		t.Errorf("expected inline labels #backend and #core in repo view, got:\n%s", view)
	}

	// 3. Verify repository About / description text is rendered (chopped with … if needed)
	if !strings.Contains(view, "Authentication and au") {
		t.Errorf("expected repository About text in repo view, got:\n%s", view)
	}

	// 4. Verify (Vulns - PRs - Issues) format: ( 1 2 2 ) - ( 2 ) - ( 3 ) without commas
	if !strings.Contains(view, "( 1 2 2 )") {
		t.Errorf("expected '( 1 2 2 )' vulnerabilities without commas, got:\n%s", view)
	}
	if !strings.Contains(view, "( 2 )") {
		t.Errorf("expected ( 2 ) PR count in column, got:\n%s", view)
	}
	if !strings.Contains(view, "( 3 )") {
		t.Errorf("expected ( 3 ) task count in column, got:\n%s", view)
	}

	// 5. Test clean repo with 0 vulnerabilities renders green ( 0 0 0 ) without commas
	m.SecuritySummaries["auth-svc"] = github.SecurityAlertSummary{Total: 0}
	viewClean := m.renderReposView()
	if !strings.Contains(viewClean, "( 0 0 0 )") {
		t.Errorf("expected '( 0 0 0 )' without commas for 0 vulnerabilities, got:\n%s", viewClean)
	}

	// 6. Verify single spinner indicator in top tier bar when Loading is true
	m.Loading = true
	loadingView := m.renderReposView()
	if !strings.Contains(loadingView, "Syncing...") {
		t.Errorf("expected 'Syncing...' in tier bar when Loading is true, got:\n%s", loadingView)
	}
}

func TestHotkeyHashOpensLabelFilterModal(t *testing.T) {
	repos := []github.Repository{
		{ID: 101, Name: "auth-svc"},
	}
	m := Model{
		Repos:           repos,
		Width:           120,
		Height:          30,
		SplashDismissed: true,
	}

	// Press '#' -> opens ModalFilterGroup
	updated, _ := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'#'}})
	m = updated.(Model)
	if m.ActiveModal != ModalFilterGroup {
		t.Errorf("expected ModalFilterGroup when pressing '#', got %v", m.ActiveModal)
	}
}

func TestHotkeyLOpensLabelsModal(t *testing.T) {
	repos := []github.Repository{
		{ID: 101, Name: "auth-svc"},
	}
	m := Model{
		Repos:           repos,
		Width:           120,
		Height:          30,
		SplashDismissed: true,
	}

	// Move cursor to the repo (index 1 under Warm header)
	items := m.getRepoTableItems()
	for idx, it := range items {
		if it.Kind == ItemRepo && it.Repo != nil && it.Repo.Name == "auth-svc" {
			m.RepoCursor = idx
			break
		}
	}

	// Press 'l' -> opens ModalManageGroups (Manage Labels)
	updated, _ := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(Model)
	if m.ActiveModal != ModalManageGroups {
		t.Errorf("expected ModalManageGroups when pressing 'l', got %v", m.ActiveModal)
	}
}

func TestHideAndUnhideRepository(t *testing.T) {
	now := time.Now()
	repos := []github.Repository{
		{ID: 101, Name: "auth-svc", PushedAt: now.Add(-1 * time.Hour)},
		{ID: 102, Name: "core-tasks", PushedAt: now.Add(-10 * time.Minute)},
		{ID: 103, Name: "dead-repo", PushedAt: now.Add(-500 * 24 * time.Hour)},
	}
	tiers := map[string]storage.RepoTier{
		"auth-svc":   storage.TierHot,
		"core-tasks": storage.TierHot,
		"dead-repo":  storage.TierDead,
	}

	m := Model{
		Repos:           repos,
		RepoTiers:       tiers,
		GitStatuses:     make(map[string]git.RepoGitStatus),
		Width:           140,
		Height:          30,
		SplashDismissed: true,
		CollapsedTiers: map[storage.RepoTier]bool{
			storage.TierDead:   true,
			storage.TierHidden: true,
		},
		HiddenRepos: make(map[string]bool),
	}

	// 1. Initially core-tasks is in Hot tier
	if m.getRepoTier("core-tasks") != storage.TierHot {
		t.Fatalf("expected core-tasks to be TierHot initially, got %v", m.getRepoTier("core-tasks"))
	}

	// Select core-tasks in items
	items := m.getRepoTableItems()
	for idx, it := range items {
		if it.Kind == ItemRepo && it.Repo != nil && it.Repo.Name == "core-tasks" {
			m.RepoCursor = idx
			break
		}
	}

	// 2. Press 'h' on core-tasks -> hides it!
	updated, _ := m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(Model)

	if !m.isRepoHidden("core-tasks") {
		t.Fatalf("expected core-tasks to be hidden after pressing 'h'")
	}
	if tier := m.getRepoTier("core-tasks"); tier != storage.TierHidden {
		t.Fatalf("expected core-tasks tier to become TierHidden, got %v", tier)
	}

	// 3. Verify Hidden section is underneath Dead tier and collapsed by default
	newItems := m.getRepoTableItems()
	foundDeadHeader := false
	foundHiddenHeader := false
	for _, it := range newItems {
		if it.Kind == ItemTierHeader {
			if it.Tier == storage.TierDead {
				foundDeadHeader = true
			}
			if it.Tier == storage.TierHidden {
				if !foundDeadHeader {
					t.Errorf("expected Hidden tier header to appear AFTER Dead tier header")
				}
				foundHiddenHeader = true
				if it.Expanded {
					t.Errorf("expected Hidden tier to be collapsed by default")
				}
			}
		}
		if it.Kind == ItemRepo && it.Repo.Name == "core-tasks" {
			t.Errorf("expected hidden repo core-tasks to be hidden when Hidden tier is collapsed")
		}
	}
	if !foundHiddenHeader {
		t.Fatalf("expected Hidden tier header in table items")
	}

	// 4. Verify View shows Hidden tier header
	view := m.renderReposView()
	if !strings.Contains(view, "🙈 HIDDEN") {
		t.Errorf("expected '🙈 HIDDEN' section header in view, got:\n%s", view)
	}
	if !strings.Contains(view, "1 Hidden") {
		t.Errorf("expected '1 Hidden' in tier summary bar, got:\n%s", view)
	}

	// 5. Expand Hidden tier -> core-tasks is visible with dimmed style
	m.CollapsedTiers[storage.TierHidden] = false
	viewExpanded := m.renderReposView()
	if !strings.Contains(viewExpanded, "core-tasks") {
		t.Errorf("expected core-tasks to be visible after expanding Hidden tier, got:\n%s", viewExpanded)
	}

	// 6. Press 'h' on core-tasks again -> unhides it!
	for idx, it := range m.getRepoTableItems() {
		if it.Kind == ItemRepo && it.Repo != nil && it.Repo.Name == "core-tasks" {
			m.RepoCursor = idx
			break
		}
	}
	updated, _ = m.handleReposKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(Model)

	if m.isRepoHidden("core-tasks") {
		t.Fatalf("expected core-tasks to be unhidden after pressing 'h' again")
	}
	if tier := m.getRepoTier("core-tasks"); tier != storage.TierHot {
		t.Fatalf("expected core-tasks tier to restore to TierHot, got %v", tier)
	}
}




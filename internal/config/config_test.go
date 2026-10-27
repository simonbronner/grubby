package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}

	// Test Group operations
	added, err := cfg.ToggleRepoInGroup("api-gateway", "backend")
	if err != nil || !added {
		t.Fatalf("expected repo added to group, got added=%v, err=%v", added, err)
	}

	groups := cfg.GetGroupsForRepo("api-gateway")
	if len(groups) != 1 || groups[0] != "backend" {
		t.Errorf("expected group backend, got %v", groups)
	}

	// Add to second group
	added2, err := cfg.ToggleRepoInGroup("api-gateway", "core")
	if err != nil || !added2 {
		t.Fatalf("expected repo added to second group")
	}

	groups = cfg.GetGroupsForRepo("api-gateway")
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}

	if !cfg.IsRepoInGroup("backend", "api-gateway") {
		t.Errorf("expected api-gateway to be in backend group")
	}
	if !cfg.IsRepoInGroup("core", "api-gateway") {
		t.Errorf("expected api-gateway to be in core group")
	}
	if cfg.IsRepoInGroup("frontend", "api-gateway") {
		t.Errorf("expected api-gateway NOT to be in frontend group")
	}

	// Test Squad operations
	err = cfg.AddMemberToSquad("platform", "alice")
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	err = cfg.SetActiveSquad("platform")
	if err != nil {
		t.Fatalf("failed to set active squad: %v", err)
	}

	squad := cfg.GetActiveSquad()
	if squad == nil || squad.Name != "platform" || len(squad.Members) != 1 || squad.Members[0] != "alice" {
		t.Fatalf("unexpected active squad: %+v", squad)
	}

	// Reload from disk to verify persistence
	cfgReloaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}

	if cfgReloaded.ActiveSquad != "platform" {
		t.Errorf("expected active squad platform, got %s", cfgReloaded.ActiveSquad)
	}

	reloadedGroups := cfgReloaded.GetGroupsForRepo("api-gateway")
	if len(reloadedGroups) != 2 {
		t.Errorf("expected 2 groups on reload, got %d", len(reloadedGroups))
	}

	// Test CollapseHome
	home, _ := os.UserHomeDir()
	collapsed := CollapseHome(filepath.Join(home, "workspaces", "grubby"))
	if collapsed != "~/workspaces/grubby" {
		t.Errorf("expected ~/workspaces/grubby, got %s", collapsed)
	}

	// Test RenameSquad, SetSquadMembers, and DeleteSquad
	err = cfg.SetSquadMembers("platform", []string{"alice", "bob", "charlie"})
	if err != nil {
		t.Fatalf("failed to set squad members: %v", err)
	}
	if len(cfg.GetActiveSquad().Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(cfg.GetActiveSquad().Members))
	}

	err = cfg.RenameSquad("platform", "infrastructure")
	if err != nil {
		t.Fatalf("failed to rename squad: %v", err)
	}
	if cfg.ActiveSquad != "infrastructure" {
		t.Errorf("expected active squad to be renamed to infrastructure, got %s", cfg.ActiveSquad)
	}

	err = cfg.DeleteSquad("infrastructure")
	if err != nil {
		t.Fatalf("failed to delete squad: %v", err)
	}
	if cfg.GetActiveSquad() != nil {
		t.Errorf("expected nil active squad after deleting only squad, got %+v", cfg.GetActiveSquad())
	}

	// Test Hidden Repos
	if cfg.IsRepoHidden("core-tasks") {
		t.Errorf("expected core-tasks not hidden initially")
	}
	nowHidden, err := cfg.ToggleRepoHidden("core-tasks")
	if err != nil || !nowHidden {
		t.Fatalf("expected ToggleRepoHidden to return true, got %v, err: %v", nowHidden, err)
	}
	if !cfg.IsRepoHidden("core-tasks") {
		t.Errorf("expected core-tasks to be hidden")
	}
	// Case insensitivity
	if !cfg.IsRepoHidden("CORE-TASKS") {
		t.Errorf("expected CORE-TASKS to be hidden case-insensitively")
	}
	nowHidden2, err := cfg.ToggleRepoHidden("core-tasks")
	if err != nil || nowHidden2 {
		t.Fatalf("expected ToggleRepoHidden to unhide, got %v, err: %v", nowHidden2, err)
	}
	if cfg.IsRepoHidden("core-tasks") {
		t.Errorf("expected core-tasks to be unhidden")
	}

	// Test Central Tasks Repo Resolution
	// 1. Unset: defaultOrg/grubby-tasks
	o, r := cfg.GetCentralTasksRepo("acme-corp")
	if o != "acme-corp" || r != "grubby-tasks" {
		t.Errorf("expected acme-corp/grubby-tasks, got %s/%s", o, r)
	}
	if fn := cfg.GetCentralTasksRepoFullName("acme-corp"); fn != "acme-corp/grubby-tasks" {
		t.Errorf("expected acme-corp/grubby-tasks, got %s", fn)
	}

	// 2. Explicit repo name only
	_ = cfg.SetCentralTasksRepo("tasks")
	o, r = cfg.GetCentralTasksRepo("acme-corp")
	if o != "acme-corp" || r != "tasks" {
		t.Errorf("expected acme-corp/tasks, got %s/%s", o, r)
	}

	// 3. Explicit owner/repo
	_ = cfg.SetCentralTasksRepo("custom-org/central-workspace")
	o, r = cfg.GetCentralTasksRepo("acme-corp")
	if o != "custom-org" || r != "central-workspace" {
		t.Errorf("expected custom-org/central-workspace, got %s/%s", o, r)
	}
	if fn := cfg.GetCentralTasksRepoFullName("acme-corp"); fn != "custom-org/central-workspace" {
		t.Errorf("expected custom-org/central-workspace, got %s", fn)
	}

	// Test Squad Repo Ownership
	_ = cfg.CreateSquad("security", "SecOps", []string{"eve"})
	if cfg.IsRepoOwnedBySquad("security", "auth-service") {
		t.Errorf("expected auth-service not owned initially")
	}

	// Claim ownership
	owned, err := cfg.ToggleRepoOwnership("security", "auth-service")
	if err != nil || !owned {
		t.Fatalf("expected ToggleRepoOwnership to return true, got %v, err: %v", owned, err)
	}
	if !cfg.IsRepoOwnedBySquad("security", "auth-service") {
		t.Errorf("expected auth-service to be owned by security squad")
	}

	// Another squad can also own the same repo (multiple squad owners)
	_ = cfg.CreateSquad("core", "Core Platform", []string{"simon"})
	owned2, _ := cfg.ToggleRepoOwnership("core", "auth-service")
	if !owned2 || !cfg.IsRepoOwnedBySquad("core", "auth-service") {
		t.Errorf("expected core squad to also own auth-service")
	}

	squadsForRepo := cfg.GetSquadsForRepo("auth-service")
	if len(squadsForRepo) != 2 {
		t.Fatalf("expected 2 squads owning auth-service, got %v", squadsForRepo)
	}

	reposForSecurity := cfg.GetReposForSquad("security")
	if len(reposForSecurity) != 1 || reposForSecurity[0] != "auth-service" {
		t.Errorf("expected [auth-service] for security squad, got %v", reposForSecurity)
	}

	// Unassign ownership
	unowned, _ := cfg.ToggleRepoOwnership("security", "auth-service")
	if unowned || cfg.IsRepoOwnedBySquad("security", "auth-service") {
		t.Errorf("expected auth-service to be unassigned from security")
	}
}

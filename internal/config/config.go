package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type SquadConfig struct {
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"display_name"`
	GithubTeam  string   `yaml:"github_team,omitempty"`
	Members     []string `yaml:"members"`
	Repos       []string `yaml:"repos,omitempty"`
}

type GroupConfig struct {
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"display_name"`
	Repos       []string `yaml:"repos"`
}

type SearchConfig struct {
	Engine     string   `yaml:"engine"` // "ripgrep"
	IgnoreDirs []string `yaml:"ignore_dirs"`
}

type Config struct {
	Version            int            `yaml:"version"`
	DefaultOrg         string         `yaml:"default_org"`
	WorkspaceDir       string         `yaml:"workspace_dir"`
	Editor             string         `yaml:"editor"`
	ActiveSquad        string         `yaml:"active_squad"`
	Squads             []SquadConfig  `yaml:"squads"`
	Groups             []GroupConfig  `yaml:"groups"`
	Search             SearchConfig         `yaml:"search"`
	RepoInteractions   map[string]time.Time `yaml:"repo_interactions,omitempty"`
	Proxy              string               `yaml:"proxy,omitempty"`
	InsecureSkipVerify bool           `yaml:"insecure_skip_verify,omitempty"`
	HiddenRepos        []string       `yaml:"hidden_repos,omitempty"`
	CentralTasksRepo   string         `yaml:"central_tasks_repo,omitempty"`

	filePath string     `yaml:"-"`
	mu       sync.Mutex `yaml:"-"`
}

// DefaultConfigPath returns ~/.config/grubby/config.yaml
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "grubby", "config.yaml"), nil
}

// ExpandHome expands leading ~ in file paths
func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// CollapseHome replaces the user's home directory path with ~/
func CollapseHome(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// Load loads or creates the config at path
func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}
	path = ExpandHome(path)

	cfg := &Config{
		Version:      1,
		WorkspaceDir: "~/workspaces",
		Editor:       os.Getenv("EDITOR"),
		Search: SearchConfig{
			Engine: "ripgrep",
			IgnoreDirs: []string{
				".git", "node_modules", "vendor", "dist", "build", ".next",
			},
		},
		filePath: path,
	}
	if cfg.Editor == "" {
		cfg.Editor = "nvim"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create directory and initial file
			if err := cfg.Save(); err != nil {
				return nil, fmt.Errorf("failed to initialize config file: %w", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("invalid config format in %s: %w", path, err)
	}
	cfg.filePath = path
	return cfg, nil
}

// Save persists the config atomically to disk
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.filePath == "" {
		return nil
	}

	dir := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	tmpFile := c.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("failed to write tmp config: %w", err)
	}

	if err := os.Rename(tmpFile, c.filePath); err != nil {
		return fmt.Errorf("failed to atomically replace config: %w", err)
	}
	return nil
}

// Group Helpers

func (c *Config) GetGroupsForRepo(repoName string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var matched []string
	for _, g := range c.Groups {
		for _, r := range g.Repos {
			if strings.EqualFold(r, repoName) {
				matched = append(matched, g.Name)
				break
			}
		}
	}
	return matched
}

func (c *Config) GetAllGroupNames() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	var names []string
	for _, g := range c.Groups {
		names = append(names, g.Name)
	}
	return names
}

func (c *Config) IsRepoInGroup(groupName, repoName string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, g := range c.Groups {
		if strings.EqualFold(g.Name, groupName) {
			for _, r := range g.Repos {
				if strings.EqualFold(r, repoName) {
					return true
				}
			}
			return false
		}
	}
	return false
}

func (c *Config) GetGroup(groupName string) *GroupConfig {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.Groups {
		if strings.EqualFold(c.Groups[i].Name, groupName) {
			return &c.Groups[i]
		}
	}
	return nil
}

func (c *Config) ToggleRepoInGroup(repoName, groupName string) (bool, error) {
	c.mu.Lock()
	groupIndex := -1
	for i, g := range c.Groups {
		if strings.EqualFold(g.Name, groupName) {
			groupIndex = i
			break
		}
	}

	if groupIndex == -1 {
		// Create new group
		c.Groups = append(c.Groups, GroupConfig{
			Name:        groupName,
			DisplayName: groupName,
			Repos:       []string{repoName},
		})
		c.mu.Unlock()
		return true, c.Save()
	}

	inGroup := false
	var updatedRepos []string
	for _, r := range c.Groups[groupIndex].Repos {
		if strings.EqualFold(r, repoName) {
			inGroup = true
		} else {
			updatedRepos = append(updatedRepos, r)
		}
	}

	if inGroup {
		c.Groups[groupIndex].Repos = updatedRepos
		c.mu.Unlock()
		return false, c.Save()
	}

	c.Groups[groupIndex].Repos = append(c.Groups[groupIndex].Repos, repoName)
	c.mu.Unlock()
	return true, c.Save()
}

// Squad Helpers

func (c *Config) GetActiveSquad() *SquadConfig {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.Squads) == 0 {
		return nil
	}
	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, c.ActiveSquad) {
			return &c.Squads[i]
		}
	}
	// Fallback to first squad if active_squad is unset
	return &c.Squads[0]
}

func (c *Config) SetActiveSquad(squadName string) error {
	c.mu.Lock()
	c.ActiveSquad = squadName
	c.mu.Unlock()
	return c.Save()
}

func (c *Config) AddMemberToSquad(squadName, member string) error {
	c.mu.Lock()
	cleanMember := strings.TrimPrefix(strings.TrimSpace(member), "@")
	if cleanMember == "" {
		c.mu.Unlock()
		return fmt.Errorf("invalid member name")
	}

	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, squadName) {
			for _, m := range c.Squads[i].Members {
				if strings.EqualFold(m, cleanMember) {
					c.mu.Unlock()
					return nil // Already member
				}
			}
			c.Squads[i].Members = append(c.Squads[i].Members, cleanMember)
			c.mu.Unlock()
			return c.Save()
		}
	}

	// Create squad if not found
	c.Squads = append(c.Squads, SquadConfig{
		Name:        squadName,
		DisplayName: squadName,
		Members:     []string{cleanMember},
	})
	if c.ActiveSquad == "" {
		c.ActiveSquad = squadName
	}
	c.mu.Unlock()
	return c.Save()
}

func (c *Config) RemoveMemberFromSquad(squadName, member string) error {
	c.mu.Lock()
	cleanMember := strings.TrimPrefix(strings.TrimSpace(member), "@")

	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, squadName) {
			var updated []string
			for _, m := range c.Squads[i].Members {
				if !strings.EqualFold(m, cleanMember) {
					updated = append(updated, m)
				}
			}
			c.Squads[i].Members = updated
			c.mu.Unlock()
			return c.Save()
		}
	}
	c.mu.Unlock()
	return nil
}

func (c *Config) CreateSquad(name, displayName string, members []string) error {
	c.mu.Lock()
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		c.mu.Unlock()
		return fmt.Errorf("squad name cannot be empty")
	}
	if displayName == "" {
		displayName = cleanName
	}
	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, cleanName) {
			c.Squads[i].DisplayName = displayName
			c.Squads[i].Members = members
			c.mu.Unlock()
			return c.Save()
		}
	}
	c.Squads = append(c.Squads, SquadConfig{
		Name:        cleanName,
		DisplayName: displayName,
		Members:     members,
	})
	if c.ActiveSquad == "" {
		c.ActiveSquad = cleanName
	}
	c.mu.Unlock()
	return c.Save()
}

func (c *Config) RenameSquad(oldName, newName string) error {
	c.mu.Lock()

	cleanNew := strings.TrimSpace(newName)
	if cleanNew == "" {
		c.mu.Unlock()
		return fmt.Errorf("new squad name cannot be empty")
	}

	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, oldName) {
			c.Squads[i].Name = cleanNew
			c.Squads[i].DisplayName = cleanNew
			if strings.EqualFold(c.ActiveSquad, oldName) {
				c.ActiveSquad = cleanNew
			}
			c.mu.Unlock()
			return c.Save()
		}
	}
	c.mu.Unlock()
	return fmt.Errorf("squad '%s' not found", oldName)
}

func (c *Config) DeleteSquad(squadName string) error {
	c.mu.Lock()

	foundIndex := -1
	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, squadName) {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		c.mu.Unlock()
		return fmt.Errorf("squad '%s' not found", squadName)
	}

	c.Squads = append(c.Squads[:foundIndex], c.Squads[foundIndex+1:]...)

	if strings.EqualFold(c.ActiveSquad, squadName) {
		if len(c.Squads) > 0 {
			c.ActiveSquad = c.Squads[0].Name
		} else {
			c.ActiveSquad = ""
		}
	}

	c.mu.Unlock()
	return c.Save()
}

func (c *Config) SetSquadMembers(squadName string, members []string) error {
	c.mu.Lock()

	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, squadName) {
			c.Squads[i].Members = members
			c.mu.Unlock()
			return c.Save()
		}
	}
	c.mu.Unlock()
	return fmt.Errorf("squad '%s' not found", squadName)
}

// Squad Repo Ownership Helpers

func (c *Config) IsRepoOwnedBySquad(squadName, repoName string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	cleanRepo := strings.ToLower(strings.TrimSpace(repoName))
	for _, s := range c.Squads {
		if strings.EqualFold(s.Name, squadName) {
			for _, r := range s.Repos {
				if strings.ToLower(strings.TrimSpace(r)) == cleanRepo {
					return true
				}
			}
			return false
		}
	}
	return false
}

func (c *Config) ToggleRepoOwnership(squadName, repoName string) (bool, error) {
	if c == nil {
		return false, nil
	}
	c.mu.Lock()

	cleanRepo := strings.ToLower(strings.TrimSpace(repoName))
	if cleanRepo == "" {
		c.mu.Unlock()
		return false, fmt.Errorf("invalid repository name")
	}

	squadIdx := -1
	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, squadName) {
			squadIdx = i
			break
		}
	}

	if squadIdx == -1 {
		// Auto-create squad if it doesn't exist
		c.Squads = append(c.Squads, SquadConfig{
			Name:        squadName,
			DisplayName: squadName,
			Repos:       []string{cleanRepo},
		})
		c.mu.Unlock()
		return true, c.Save()
	}

	inSquad := false
	var updatedRepos []string
	for _, r := range c.Squads[squadIdx].Repos {
		if strings.ToLower(strings.TrimSpace(r)) == cleanRepo {
			inSquad = true
		} else {
			updatedRepos = append(updatedRepos, r)
		}
	}

	if inSquad {
		c.Squads[squadIdx].Repos = updatedRepos
		c.mu.Unlock()
		return false, c.Save()
	}

	c.Squads[squadIdx].Repos = append(c.Squads[squadIdx].Repos, cleanRepo)
	c.mu.Unlock()
	return true, c.Save()
}

func (c *Config) GetReposForSquad(squadName string) []string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, s := range c.Squads {
		if strings.EqualFold(s.Name, squadName) {
			res := make([]string, len(s.Repos))
			copy(res, s.Repos)
			return res
		}
	}
	return nil
}

func (c *Config) GetSquadsForRepo(repoName string) []string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	cleanRepo := strings.ToLower(strings.TrimSpace(repoName))
	var squads []string
	for _, s := range c.Squads {
		for _, r := range s.Repos {
			if strings.ToLower(strings.TrimSpace(r)) == cleanRepo {
				squads = append(squads, s.Name)
				break
			}
		}
	}
	return squads
}

func (c *Config) SetSquadRepos(squadName string, repos []string) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()

	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, squadName) {
			c.Squads[i].Repos = repos
			c.mu.Unlock()
			return c.Save()
		}
	}
	c.mu.Unlock()
	return fmt.Errorf("squad '%s' not found", squadName)
}

// Repo Interaction Helpers

// RecordRepoInteraction stores the current timestamp for a repository action and persists the config.
func (c *Config) RecordRepoInteraction(repoName string) error {
	c.mu.Lock()
	if c.RepoInteractions == nil {
		c.RepoInteractions = make(map[string]time.Time)
	}
	c.RepoInteractions[strings.ToLower(strings.TrimSpace(repoName))] = time.Now()
	c.mu.Unlock()
	return c.Save()
}

// GetRepoLastInteracted returns the recorded last interaction time for a repository.
func (c *Config) GetRepoLastInteracted(repoName string) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.RepoInteractions == nil {
		return time.Time{}
	}
	return c.RepoInteractions[strings.ToLower(strings.TrimSpace(repoName))]
}

// Hidden Repositories Helpers

// IsRepoHidden returns true if repoName is marked hidden in the config.
func (c *Config) IsRepoHidden(repoName string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	clean := strings.ToLower(strings.TrimSpace(repoName))
	for _, r := range c.HiddenRepos {
		if strings.ToLower(strings.TrimSpace(r)) == clean {
			return true
		}
	}
	return false
}

// SetRepoHidden marks or unmarks a repository as hidden and persists the config.
func (c *Config) SetRepoHidden(repoName string, hidden bool) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	clean := strings.ToLower(strings.TrimSpace(repoName))
	var updated []string
	for _, r := range c.HiddenRepos {
		if strings.ToLower(strings.TrimSpace(r)) != clean {
			updated = append(updated, r)
		}
	}
	if hidden {
		updated = append(updated, repoName)
	}
	c.HiddenRepos = updated
	c.mu.Unlock()
	return c.Save()
}

// ToggleRepoHidden toggles the hidden state of a repository and persists the config.
func (c *Config) ToggleRepoHidden(repoName string) (bool, error) {
	if c == nil {
		return false, nil
	}
	nowHidden := !c.IsRepoHidden(repoName)
	err := c.SetRepoHidden(repoName, nowHidden)
	return nowHidden, err
}

// Central Tasks Repository Helpers

// GetCentralTasksRepo resolves the owner and repo name for the central tasks repository.
// If central_tasks_repo is configured as "owner/repo", it returns (owner, repo).
// If configured as "repo", it returns (defaultOrg or c.DefaultOrg, repo).
// If unset, it defaults to (defaultOrg or c.DefaultOrg, "grubby-tasks").
func (c *Config) GetCentralTasksRepo(defaultOrg string) (owner, repo string) {
	if c == nil {
		if defaultOrg != "" {
			return defaultOrg, "grubby-tasks"
		}
		return "", "grubby-tasks"
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	tasksRepo := strings.TrimSpace(c.CentralTasksRepo)
	if tasksRepo == "" {
		org := defaultOrg
		if org == "" {
			org = c.DefaultOrg
		}
		return org, "grubby-tasks"
	}

	parts := strings.Split(tasksRepo, "/")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}

	org := defaultOrg
	if org == "" {
		org = c.DefaultOrg
	}
	return org, strings.TrimSpace(parts[0])
}

// GetCentralTasksRepoFullName returns "owner/repo" or "repo" if owner is empty.
func (c *Config) GetCentralTasksRepoFullName(defaultOrg string) string {
	owner, repo := c.GetCentralTasksRepo(defaultOrg)
	if owner == "" {
		return repo
	}
	return owner + "/" + repo
}

// SetCentralTasksRepo configures the central tasks repository and persists the config.
func (c *Config) SetCentralTasksRepo(repoFullName string) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	c.CentralTasksRepo = strings.TrimSpace(repoFullName)
	c.mu.Unlock()
	return c.Save()
}



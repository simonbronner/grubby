package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type SquadConfig struct {
	Name        string   `yaml:"name"`
	DisplayName string   `yaml:"display_name"`
	GithubTeam  string   `yaml:"github_team,omitempty"`
	Members     []string `yaml:"members"`
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
	Version      int            `yaml:"version"`
	DefaultOrg   string         `yaml:"default_org"`
	WorkspaceDir string         `yaml:"workspace_dir"`
	Editor       string         `yaml:"editor"`
	ActiveSquad  string         `yaml:"active_squad"`
	Squads       []SquadConfig  `yaml:"squads"`
	Groups       []GroupConfig  `yaml:"groups"`
	Search       SearchConfig   `yaml:"search"`

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
	for i := range c.Squads {
		if strings.EqualFold(c.Squads[i].Name, name) {
			c.Squads[i].DisplayName = displayName
			c.Squads[i].Members = members
			c.mu.Unlock()
			return c.Save()
		}
	}
	c.Squads = append(c.Squads, SquadConfig{
		Name:        name,
		DisplayName: displayName,
		Members:     members,
	})
	if c.ActiveSquad == "" {
		c.ActiveSquad = name
	}
	c.mu.Unlock()
	return c.Save()
}

package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"grubby/internal/config"
)

type WorkspaceManager struct {
	workspaceDir string
	mu           sync.RWMutex
	statusCache  map[string]RepoGitStatus
}

func NewWorkspaceManager(workspaceDir string) *WorkspaceManager {
	expanded := config.ExpandHome(workspaceDir)
	return &WorkspaceManager{
		workspaceDir: expanded,
		statusCache:  make(map[string]RepoGitStatus),
	}
}

func (w *WorkspaceManager) WorkspaceDir() string {
	return w.workspaceDir
}

// GetRepoPath returns the expected local directory for a repo name
func (w *WorkspaceManager) GetRepoPath(repoName string) string {
	return filepath.Join(w.workspaceDir, repoName)
}

// ScanWorkspaces inspects all local directories in the workspace folder
func (w *WorkspaceManager) ScanWorkspaces(ctx context.Context, knownRepos []string) map[string]RepoGitStatus {
	w.mu.Lock()
	defer w.mu.Unlock()

	results := make(map[string]RepoGitStatus)
	entries, err := os.ReadDir(w.workspaceDir)
	if err != nil {
		return results
	}

	clonedDirs := make(map[string]string) // repoName (lowercase) -> fullPath
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(w.workspaceDir, entry.Name())
		gitDir := filepath.Join(dirPath, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			clonedDirs[strings.ToLower(entry.Name())] = dirPath
		}
	}

	for _, repo := range knownRepos {
		lower := strings.ToLower(repo)
		if path, found := clonedDirs[lower]; found {
			results[repo] = InspectRepo(ctx, repo, path)
		} else {
			results[repo] = RepoGitStatus{
				RepoName:  repo,
				LocalPath: filepath.Join(w.workspaceDir, repo),
				IsCloned:  false,
				State:     StateNotCloned,
			}
		}
	}

	w.statusCache = results
	return results
}

// GetStatus returns the cached or freshly inspected status for a repo
func (w *WorkspaceManager) GetStatus(ctx context.Context, repoName string) RepoGitStatus {
	w.mu.RLock()
	st, ok := w.statusCache[repoName]
	w.mu.RUnlock()
	if ok {
		return st
	}

	path := w.GetRepoPath(repoName)
	return InspectRepo(ctx, repoName, path)
}

// ListClonedPaths returns all existing cloned directory paths in the workspace
func (w *WorkspaceManager) ListClonedPaths() []string {
	var paths []string
	entries, err := os.ReadDir(w.workspaceDir)
	if err != nil {
		return paths
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(w.workspaceDir, entry.Name())
		gitDir := filepath.Join(dirPath, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			paths = append(paths, dirPath)
		}
	}
	return paths
}

package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type GitState string

const (
	StateNotCloned  GitState = "NOT_CLONED"
	StateSynced     GitState = "SYNCED"
	StateBehind     GitState = "BEHIND"
	StateAhead      GitState = "AHEAD"
	StateDiverged   GitState = "DIVERGED"
	StateDirty      GitState = "DIRTY"
	StateNoUpstream GitState = "NO_UPSTREAM"
)

type RepoGitStatus struct {
	RepoName  string
	LocalPath string
	IsCloned  bool
	Branch    string
	IsDirty   bool
	Ahead     int
	Behind    int
	State     GitState
}

func (s RepoGitStatus) Badge() string {
	if !s.IsCloned {
		return "[Not Cloned]"
	}
	if s.IsDirty {
		return "[* dirty]"
	}
	switch s.State {
	case StateBehind:
		return fmt.Sprintf("[↓ %d behind]", s.Behind)
	case StateAhead:
		return fmt.Sprintf("[↑ %d ahead]", s.Ahead)
	case StateDiverged:
		return fmt.Sprintf("[⇡%d ⇣%d diverged]", s.Ahead, s.Behind)
	case StateSynced:
		return "[✓ synced]"
	case StateNoUpstream:
		return "[no upstream]"
	default:
		return "[local]"
	}
}

// InspectRepo returns the git status of a local directory
func InspectRepo(ctx context.Context, repoName, dir string) RepoGitStatus {
	st := RepoGitStatus{
		RepoName:  repoName,
		LocalPath: dir,
		IsCloned:  false,
		State:     StateNotCloned,
	}

	gitDir := filepath.Join(dir, ".git")
	if info, err := os.Stat(gitDir); err != nil || !info.IsDir() {
		return st
	}
	st.IsCloned = true

	// Check branch
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	if out, err := cmd.Output(); err == nil {
		st.Branch = strings.TrimSpace(string(out))
	} else {
		st.Branch = "main"
	}

	// Check dirty status
	cmd = exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain")
	if out, err := cmd.Output(); err == nil {
		if len(strings.TrimSpace(string(out))) > 0 {
			st.IsDirty = true
		}
	}

	// Check ahead/behind tracking branch
	cmd = exec.CommandContext(ctx, "git", "-C", dir, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	if out, err := cmd.Output(); err == nil {
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) == 2 {
			st.Ahead, _ = strconv.Atoi(parts[0])
			st.Behind, _ = strconv.Atoi(parts[1])
		}
	} else {
		// Could mean no upstream branch configured
		st.State = StateNoUpstream
		if st.IsDirty {
			st.State = StateDirty
		}
		return st
	}

	// Compute state
	if st.IsDirty {
		st.State = StateDirty
	} else if st.Ahead > 0 && st.Behind > 0 {
		st.State = StateDiverged
	} else if st.Behind > 0 {
		st.State = StateBehind
	} else if st.Ahead > 0 {
		st.State = StateAhead
	} else {
		st.State = StateSynced
	}

	return st
}

// FastForward safely fast-forwards a repository
func FastForward(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "merge", "--ff-only")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

// BackgroundFetch fetches latest objects non-destructively
func BackgroundFetch(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "fetch", "--all", "--prune")
	return cmd.Run()
}

package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"grubby/internal/git"
	"grubby/internal/github"
)

func (m Model) handleReposKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.RepoCursor > 0 {
			m.RepoCursor--
		}
	case "down", "j":
		if m.RepoCursor < len(m.Repos)-1 {
			m.RepoCursor++
		}
	case "g":
		// Open Groups modal for current repo
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			m.ActiveModal = ModalManageGroups
			m.ModalCursor = 0
			m.SelectedRepoGroups = make(map[string]bool)
			for _, grp := range m.Config.GetGroupsForRepo(repo.Name) {
				m.SelectedRepoGroups[grp] = true
			}
		}
	case "o":
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			_ = OpenBrowser(repo.URL)
			return m, func() tea.Msg { return StatusToastMsg("Opened " + repo.Name + " in browser") }
		}
	case "e":
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			st := m.GitStatuses[repo.Name]
			if st.IsCloned {
				_ = OpenInEditor(m.Config.Editor, st.LocalPath, 1)
				return m, func() tea.Msg { return StatusToastMsg("Opened " + repo.Name + " in editor") }
			} else {
				return m, func() tea.Msg { return StatusToastMsg("Repo is not cloned yet! Press 'c' to clone.") }
			}
		}
	case "c":
		// Clone repo
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			st := m.GitStatuses[repo.Name]
			if st.IsCloned {
				return m, func() tea.Msg { return StatusToastMsg(repo.Name + " is already cloned!") }
			}
			targetDir := m.GitMgr.GetRepoPath(repo.Name)
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				err := github.CloneRepo(ctx, repo.FullName, targetDir)
				if err != nil {
					return StatusToastMsg("Clone failed: " + err.Error())
				}
				return StatusToastMsg("Successfully cloned " + repo.Name + " to " + targetDir)
			}
		}
	case "u":
		// Fast-forward repo
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			st := m.GitStatuses[repo.Name]
			if !st.IsCloned {
				return m, func() tea.Msg { return StatusToastMsg("Repo is not cloned yet.") }
			}
			if st.IsDirty {
				return m, func() tea.Msg { return StatusToastMsg("Cannot fast-forward: working tree has uncommitted changes!") }
			}
			if st.Behind == 0 {
				return m, func() tea.Msg { return StatusToastMsg("Repo is already up to date.") }
			}
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				err := git.FastForward(ctx, st.LocalPath)
				if err != nil {
					return StatusToastMsg("Fast-forward error: " + err.Error())
				}
				return StatusToastMsg("Successfully fast-forwarded " + repo.Name)
			}
		}
	}
	return m, nil
}

func (m Model) renderReposView() string {
	if m.Loading {
		return "\n  Fetching repositories from GitHub...\n"
	}
	if len(m.Repos) == 0 {
		return "\n  No repositories found for " + m.CurrentOrg + ".\n"
	}

	header := fmt.Sprintf("  %-25s %-22s %-16s %-12s %-18s %s",
		"REPOSITORY", "GROUPS", "GIT STATUS", "BRANCH", "LAST PUSHED", "LOCAL PATH")
	renderedHeader := TableHeaderStyle.Render(header)

	var rows []string
	rows = append(rows, renderedHeader)

	visibleCount := m.Height - 8
	if visibleCount < 5 {
		visibleCount = 10
	}

	start := 0
	if m.RepoCursor >= visibleCount {
		start = m.RepoCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(m.Repos) {
		end = len(m.Repos)
	}

	for i := start; i < end; i++ {
		r := m.Repos[i]
		st := m.GitStatuses[r.Name]

		// Format Groups
		groups := m.Config.GetGroupsForRepo(r.Name)
		groupText := "-"
		if len(groups) > 0 {
			groupText = strings.Join(groups, ", ")
		}
		if len(groupText) > 20 {
			groupText = groupText[:17] + "..."
		}

		// Git badge
		gitBadge := st.Badge()
		var styledBadge string
		switch st.State {
		case git.StateSynced:
			styledBadge = SyncedBadge.Render(gitBadge)
		case git.StateBehind:
			styledBadge = BehindBadge.Render(gitBadge)
		case git.StateDirty:
			styledBadge = DirtyBadge.Render(gitBadge)
		case git.StateNotCloned:
			styledBadge = NotClonedBadge.Render(gitBadge)
		default:
			styledBadge = gitBadge
		}

		branch := st.Branch
		if branch == "" {
			branch = r.DefaultBranch
		}
		if len(branch) > 10 {
			branch = branch[:9] + "…"
		}

		lastPushed := formatTimeAgo(r.PushedAt)

		localPath := "-"
		if st.IsCloned {
			localPath = st.LocalPath
			if len(localPath) > 35 {
				localPath = "…" + localPath[len(localPath)-34:]
			}
		}

		prefix := "  "
		if i == m.RepoCursor {
			prefix = "▶ "
		}

		rowContent := fmt.Sprintf("%s%-23s %-22s %-26s %-12s %-18s %s",
			prefix, r.Name, groupText, styledBadge, branch, lastPushed, localPath)

		if i == m.RepoCursor {
			rows = append(rows, SelectedRowStyle.Render(rowContent))
		} else {
			rows = append(rows, NormalRowStyle.Render(rowContent))
		}
	}

	return strings.Join(rows, "\n")
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d.Hours() < 1 {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d.Hours() < 24 {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "yesterday"
	}
	if days < 30 {
		return fmt.Sprintf("%dd ago", days)
	}
	return t.Format("02 Jan 2006")
}

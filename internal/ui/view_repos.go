package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/config"
	"grubby/internal/git"
	"grubby/internal/github"
)

var (
	repoColCursor = lipgloss.NewStyle().Width(3)
	repoColName   = lipgloss.NewStyle().Width(24)
	repoColGroup  = lipgloss.NewStyle().Width(18)
	repoColGit    = lipgloss.NewStyle().Width(18)
	repoColBranch = lipgloss.NewStyle().Width(14)
	repoColLast   = lipgloss.NewStyle().Width(16)
	repoColPath   = lipgloss.NewStyle().Width(34)
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
	case "t":
		// Open repo in a new shell (iTerm2 on macOS)
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			st := m.GitStatuses[repo.Name]
			if st.IsCloned {
				err := OpenInShell(st.LocalPath)
				if err != nil {
					return m, func() tea.Msg { return StatusToastMsg("Failed to open shell: " + err.Error()) }
				}
				collapsed := config.CollapseHome(st.LocalPath)
				return m, func() tea.Msg { return StatusToastMsg("Opened iTerm at " + collapsed) }
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
				collapsed := config.CollapseHome(targetDir)
				return StatusToastMsg("Successfully cloned " + repo.Name + " to " + collapsed)
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

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		repoColCursor.Render(""),
		repoColName.Render("REPOSITORY"),
		repoColGroup.Render("GROUPS"),
		repoColGit.Render("GIT STATUS"),
		repoColBranch.Render("BRANCH"),
		repoColLast.Render("LAST PUSHED"),
		repoColPath.Render("LOCAL PATH"),
	)
	renderedHeader := TableHeaderStyle.Render(headerRow)

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
		if len(groupText) > 16 {
			groupText = groupText[:13] + "..."
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
		if len(branch) > 12 {
			branch = branch[:11] + "…"
		}

		lastPushed := formatTimeAgo(r.PushedAt)

		localPath := "-"
		if st.IsCloned {
			localPath = config.CollapseHome(st.LocalPath)
			if len(localPath) > 32 {
				localPath = "…" + localPath[len(localPath)-31:]
			}
		}

		prefix := "  "
		if i == m.RepoCursor {
			prefix = "▶ "
		}

		name := r.Name
		if len(name) > 22 {
			name = name[:20] + "…"
		}

		rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
			repoColCursor.Render(prefix),
			repoColName.Render(name),
			repoColGroup.Render(groupText),
			repoColGit.Render(styledBadge),
			repoColBranch.Render(branch),
			repoColLast.Render(lastPushed),
			repoColPath.Render(localPath),
		)

		if i == m.RepoCursor {
			rows = append(rows, SelectedRowStyle.Render(rowCells))
		} else {
			rows = append(rows, NormalRowStyle.Render(rowCells))
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

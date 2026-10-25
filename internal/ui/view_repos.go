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
	case "enter":
		// Enter key opens repository root in dedicated browser
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			_ = OpenBrowser(repo.URL)
			SendNativeNotification("Grubby • Browser", "Opened Repository", repo.FullName)
			return m, func() tea.Msg { return StatusToastMsg("Opened " + repo.Name + " in dedicated browser") }
		}
	case "o":
		// 'o' opens the repo tabs submenu (issues, pull requests, actions, security, etc.)
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			m.ActiveModal = ModalRepoTabs
			m.ModalCursor = 0
			return m, nil
		}
	case "S":
		// 'S' opens repository Security (Dependabot) directly in dedicated browser
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			url := fmt.Sprintf("https://github.com/%s/security/dependabot", repo.FullName)
			_ = OpenBrowser(url)
			SendNativeNotification("Grubby • Browser", "Security Dependabot", repo.FullName)
			return m, func() tea.Msg { return StatusToastMsg("Opened Security tab for " + repo.Name) }
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
	case "e":
		if len(m.Repos) > 0 && m.RepoCursor < len(m.Repos) {
			repo := m.Repos[m.RepoCursor]
			st := m.GitStatuses[repo.Name]
			if st.IsCloned {
				err := OpenInITermNvim(st.LocalPath, "", 0)
				if err != nil {
					return m, func() tea.Msg { return StatusToastMsg("Failed to open nvim: " + err.Error()) }
				}
				collapsed := config.CollapseHome(st.LocalPath)
				SendNativeNotification("Grubby • Editor", "Opened Neovim", collapsed)
				return m, func() tea.Msg { return StatusToastMsg("Opened nvim in " + collapsed) }
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
				SendNativeNotification("Grubby • Terminal", "Opened iTerm2", collapsed)
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
				SendNativeNotification("Grubby • Git Clone", "Cloned successfully", repo.FullName)
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
				SendNativeNotification("Grubby • Fast-Forward", "Branch updated", repo.Name)
				return StatusToastMsg("Successfully fast-forwarded " + repo.Name)
			}
		}
	}
	return m, nil
}

func (m Model) renderReposView() string {
	if m.Loading && len(m.Repos) == 0 {
		return fmt.Sprintf("\n  %s Fetching repositories from GitHub...\n", m.Spinner.View())
	}
	if len(m.Repos) == 0 {
		return "\n  No repositories found for " + m.CurrentOrg + ".\n"
	}

	cols := computeRepoColumns(m.Width)

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		cols.Cursor.Render(""),
		cols.Name.Render("REPOSITORY"),
		cols.Sec.Render("SECURITY"),
		cols.Group.Render("GROUPS"),
		cols.Git.Render("GIT STATUS"),
		cols.Branch.Render("BRANCH"),
		cols.Last.Render("LAST PUSHED"),
		cols.Path.Render("LOCAL PATH"),
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

		// 1. Calculate open PR count for this repository
		prCount := 0
		for _, pr := range m.PRs {
			if strings.EqualFold(pr.Repo, r.Name) {
				prCount++
			}
		}

		maxRepoLen := cols.NameWidth - 7
		if maxRepoLen < 6 {
			maxRepoLen = 6
		}
		repoDisplayName := r.Name
		if len(repoDisplayName) > maxRepoLen {
			repoDisplayName = repoDisplayName[:maxRepoLen-1] + "…"
		}

		var nameColText string
		if m.Loading {
			nameColText = fmt.Sprintf("(%s) %s", m.Spinner.View(), repoDisplayName)
		} else {
			var prBadge string
			if prCount > 0 {
				prBadge = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("(%d)", prCount))
			} else {
				prBadge = lipgloss.NewStyle().Foreground(MutedColor).Render("(0)")
			}
			nameColText = fmt.Sprintf("%s %s", prBadge, repoDisplayName)
		}

		// 2. Format Security Issues column: visual summary (crit) (high) (medium + the rest)
		var secColText string
		if m.Loading {
			secColText = m.Spinner.View()
		} else {
			sum := m.SecuritySummaries[r.Name]
			if sum.Total == 0 {
				secColText = lipgloss.NewStyle().Foreground(SuccessColor).Render("0 [✓ clean]")
			} else {
				var parts []string
				if sum.Critical > 0 {
					parts = append(parts, lipgloss.NewStyle().Foreground(DangerColor).Bold(true).Render(fmt.Sprintf("(%dc)", sum.Critical)))
				}
				if sum.High > 0 {
					parts = append(parts, lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render(fmt.Sprintf("(%dh)", sum.High)))
				}
				medRest := sum.Medium + sum.Low
				if medRest > 0 {
					parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Render(fmt.Sprintf("(%dm+)", medRest)))
				}
				if len(parts) == 0 {
					parts = append(parts, lipgloss.NewStyle().Foreground(WarningColor).Render(fmt.Sprintf("(%d)", sum.Total)))
				}
				secColText = strings.Join(parts, " ")
			}
		}

		// 3. Format Groups
		groups := m.Config.GetGroupsForRepo(r.Name)
		groupText := "-"
		if len(groups) > 0 {
			groupText = strings.Join(groups, ", ")
		}
		if len(groupText) > cols.GroupWidth-1 && cols.GroupWidth > 3 {
			groupText = groupText[:cols.GroupWidth-4] + "..."
		}

		// 4. Git badge
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
		if len(branch) > cols.BranchWidth-2 && cols.BranchWidth > 3 {
			branch = branch[:cols.BranchWidth-3] + "…"
		}

		lastPushed := formatTimeAgo(r.PushedAt)

		localPath := "-"
		if st.IsCloned {
			localPath = config.CollapseHome(st.LocalPath)
			if len(localPath) > cols.PathWidth-2 && cols.PathWidth > 3 {
				localPath = "…" + localPath[len(localPath)-(cols.PathWidth-3):]
			}
		}

		prefix := "  "
		if i == m.RepoCursor {
			prefix = "▶ "
		}

		rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
			cols.Cursor.Render(prefix),
			cols.Name.Render(nameColText),
			cols.Sec.Render(secColText),
			cols.Group.Render(groupText),
			cols.Git.Render(styledBadge),
			cols.Branch.Render(branch),
			cols.Last.Render(lastPushed),
			cols.Path.Render(localPath),
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

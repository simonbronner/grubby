package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"grubby/internal/config"
	"grubby/internal/git"
	"grubby/internal/github"
	"grubby/internal/storage"
)

func (m Model) getDisplayRepos() []github.Repository {
	var result []github.Repository

	filter := strings.ToLower(strings.TrimSpace(m.RepoFilter))

	for _, r := range m.Repos {
		if filter != "" {
			// Check if repo name or full name contains filter keyword
			matchesName := strings.Contains(strings.ToLower(r.Name), filter) ||
				strings.Contains(strings.ToLower(r.FullName), filter)

			// Check if any group assigned to this repo matches filter keyword
			matchesGroup := false
			if m.Config != nil {
				for _, grp := range m.Config.GetGroupsForRepo(r.Name) {
					if strings.Contains(strings.ToLower(grp), filter) {
						matchesGroup = true
						break
					}
				}
			}

			if !matchesName && !matchesGroup {
				continue
			}
		}

		result = append(result, r)
	}

	if m.RepoSortVulnerabilities {
		sort.SliceStable(result, func(i, j int) bool {
			secA := m.SecuritySummaries[result[i].Name]
			secB := m.SecuritySummaries[result[j].Name]

			if secA.Total != secB.Total {
				return secA.Total > secB.Total
			}
			if secA.Critical != secB.Critical {
				return secA.Critical > secB.Critical
			}
			if secA.High != secB.High {
				return secA.High > secB.High
			}
			if secA.Medium != secB.Medium {
				return secA.Medium > secB.Medium
			}
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
	} else {
		tmuxSessions := m.TmuxSessions
		if tmuxSessions == nil {
			tmuxSessions = GetTmuxSessions()
		}

		sort.SliceStable(result, func(i, j int) bool {
			// 0. Group by Tier priority (Hot=0, Active=1, Dormant=2)
			tierA := storage.TierPriority(m.getRepoTier(result[i].Name))
			tierB := storage.TierPriority(m.getRepoTier(result[j].Name))
			if tierA != tierB {
				return tierA < tierB
			}

			actA := m.getRepoLastInteractedTime(result[i].Name, tmuxSessions)
			actB := m.getRepoLastInteractedTime(result[j].Name, tmuxSessions)

			hasActA := !actA.IsZero()
			hasActB := !actB.IsZero()

			// 1. Repositories opened / interacted with come first, sorted by last interaction descending
			if hasActA && hasActB {
				if !actA.Equal(actB) {
					return actA.After(actB)
				}
			} else if hasActA != hasActB {
				return hasActA
			}

			// 2. Then by last updated / pushed descending
			updA := result[i].LastUpdated()
			updB := result[j].LastUpdated()
			if !updA.Equal(updB) {
				return updA.After(updB)
			}

			// 3. Alphabetical tiebreaker
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		})
	}

	return result
}

func (m Model) handleReposKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	repos := m.getDisplayRepos()

	switch msg.String() {
	case "up", "k":
		if m.RepoCursor > 0 {
			m.RepoCursor--
		}
	case "down", "j":
		if m.RepoCursor < len(repos)-1 {
			m.RepoCursor++
		}
	case "pgdown", "ctrl+d":
		visibleCount := m.Height - 8
		if visibleCount < 5 {
			visibleCount = 10
		}
		m.RepoCursor += visibleCount
		if m.RepoCursor >= len(repos) {
			m.RepoCursor = len(repos) - 1
		}
	case "pgup", "ctrl+u":
		visibleCount := m.Height - 8
		if visibleCount < 5 {
			visibleCount = 10
		}
		m.RepoCursor -= visibleCount
		if m.RepoCursor < 0 {
			m.RepoCursor = 0
		}
	case "home":
		m.RepoCursor = 0
	case "end", "G":
		if len(repos) > 0 {
			m.RepoCursor = len(repos) - 1
		}
	case "/":
		// '/' filter: filter repositories by name or group
		m.ActiveModal = ModalFilterRepoName
		m.RepoFilterInput.SetValue(m.RepoFilter)
		m.RepoFilterInput.Focus()
		return m, textinput.Blink
	case "v":
		// 'v' sort by vulnerabilities: toggle sorting from most to least vulnerabilities
		m.RepoSortVulnerabilities = !m.RepoSortVulnerabilities
		m.RepoCursor = 0
		if m.RepoSortVulnerabilities {
			return m, func() tea.Msg { return StatusToastMsg("Sorted repositories by vulnerabilities (most to least)") }
		}
		return m, func() tea.Msg { return StatusToastMsg("Reset repository sort order to default") }
	case "p":
		// 'p' pull requests: open Pull Requests tab filtered by the selected repository
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m = m.recordRepoInteraction(repo.Name)
			m.ActiveTab = TabPRs
			m.PRFilter = repo.Name
			m.PRCursor = 0
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Filtered pull requests for %s", repo.Name))
			}
		}
	case "s":
		// 's' session capability: switch to repository tmux session in Grubby window
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m = m.recordRepoInteraction(repo.Name)
			m.RepoCursor = 0
			st := m.GitStatuses[repo.Name]
			if st.IsCloned {
				msg, err := GoToRepoSession(repo.Name, st.LocalPath)
				if err != nil {
					return m, func() tea.Msg { return StatusToastMsg("Failed to switch session: " + err.Error()) }
				}
				return m, tea.Batch(
					func() tea.Msg { return StatusToastMsg(msg) },
					m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
				)
			}
			return m.startCloneWithAction(repo, RepoActionSession)
		}
	case "e", "E":
		// 'e' editor: open editor in a new window in the tmux session for that repo
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m = m.recordRepoInteraction(repo.Name)
			m.RepoCursor = 0
			st := m.GitStatuses[repo.Name]
			if st.IsCloned {
				msg, err := OpenRepoEditor(repo.Name, st.LocalPath)
				if err != nil {
					return m, func() tea.Msg { return StatusToastMsg("Failed to open editor: " + err.Error()) }
				}
				return m, tea.Batch(
					func() tea.Msg { return StatusToastMsg(msg) },
					m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
				)
			}
			return m.startCloneWithAction(repo, RepoActionEditor)
		}
	case "t":
		// 't' open repo shell (creates new window if session exists, or creates session)
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m = m.recordRepoInteraction(repo.Name)
			m.RepoCursor = 0
			st := m.GitStatuses[repo.Name]
			if st.IsCloned {
				msg, err := OpenRepoShell(repo.Name, st.LocalPath)
				if err != nil {
					return m, func() tea.Msg { return StatusToastMsg("Failed to open shell: " + err.Error()) }
				}
				return m, tea.Batch(
					func() tea.Msg { return StatusToastMsg(msg) },
					m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
				)
			}
			return m.startCloneWithAction(repo, RepoActionShell)
		}
	case "c":
		// 'c' clone repo
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m = m.recordRepoInteraction(repo.Name)
			m.RepoCursor = 0
			st := m.GitStatuses[repo.Name]
			if st.IsCloned {
				return m, func() tea.Msg { return StatusToastMsg(repo.Name + " is already cloned!") }
			}
			return m.startCloneWithAction(repo, RepoActionNone)
		}
	case "esc":
		// 'esc' clears active filters and sorting
		if m.RepoFilter != "" || m.RepoGroupFilter != "" || m.RepoSortVulnerabilities {
			m.RepoFilter = ""
			m.RepoGroupFilter = ""
			m.RepoSortVulnerabilities = false
			m.RepoCursor = 0
			return m, func() tea.Msg { return StatusToastMsg("Cleared all repository filters and sorting") }
		}
	case "enter":
		// Enter key opens repository root in dedicated browser
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m = m.recordRepoInteraction(repo.Name)
			m.RepoCursor = 0
			_ = OpenBrowser(repo.URL)
			return m, tea.Batch(
				func() tea.Msg { return StatusToastMsg("Opened " + repo.Name + " in dedicated browser") },
				m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
			)
		}
	case "o":
		// 'o' opens the repo tabs submenu (issues, pull requests, actions, security, etc.)
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m = m.recordRepoInteraction(repo.Name)
			m.ActiveModal = ModalRepoTabs
			m.ModalCursor = 0
			return m, m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name)
		}
	case "S":
		// 'S' opens repository Security (Dependabot) directly in dedicated browser
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m = m.recordRepoInteraction(repo.Name)
			m.RepoCursor = 0
			url := fmt.Sprintf("https://github.com/%s/security/dependabot", repo.FullName)
			_ = OpenBrowser(url)
			return m, tea.Batch(
				func() tea.Msg { return StatusToastMsg("Opened Security tab for " + repo.Name) },
				m.fetchSingleRepoCmd(m.CurrentOrg, repo.Name),
			)
		}
	case "g":
		// Open Groups modal for current repo
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m.ActiveModal = ModalManageGroups
			m.ModalCursor = 0
			m.SelectedRepoGroups = make(map[string]bool)
			for _, grp := range m.Config.GetGroupsForRepo(repo.Name) {
				m.SelectedRepoGroups[grp] = true
			}
		}
	case "u":
		// Fast-forward repo
		if len(repos) > 0 && m.RepoCursor < len(repos) {
			repo := repos[m.RepoCursor]
			m = m.recordRepoInteraction(repo.Name)
			m.RepoCursor = 0
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

func (m Model) startCloneWithAction(repo github.Repository, postAction RepoActionType) (tea.Model, tea.Cmd) {
	m = m.recordRepoInteraction(repo.Name)
	m.RepoCursor = 0
	var targetDir string
	if m.GitMgr != nil {
		targetDir = m.GitMgr.GetRepoPath(repo.Name)
	} else {
		targetDir = filepath.Join(os.Getenv("HOME"), "workspaces", repo.Name)
	}
	actionDesc := "session"
	if postAction == RepoActionEditor {
		actionDesc = "editor"
	} else if postAction == RepoActionShell {
		actionDesc = "shell"
	}

	if postAction != RepoActionNone {
		m.StatusMsg = fmt.Sprintf("Cloning %s to open %s...", repo.FullName, actionDesc)
	} else {
		m.StatusMsg = "Cloning " + repo.FullName + "..."
	}
	m.StatusMsgTimer = time.Now().Add(3 * time.Minute)

	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		err := github.CloneRepo(ctx, repo.FullName, targetDir)
		if err != nil {
			return RepoClonedMsg{RepoName: repo.Name, TargetDir: targetDir, Err: err, PostAction: postAction}
		}
		newStatus := git.InspectRepo(ctx, repo.Name, targetDir)
		return RepoClonedMsg{RepoName: repo.Name, TargetDir: targetDir, Status: newStatus, Err: nil, PostAction: postAction}
	}
}

func (m Model) renderReposView() string {
	repos := m.getDisplayRepos()

	if m.Loading && len(m.Repos) == 0 {
		return fmt.Sprintf("\n  %s Fetching repositories from GitHub...\n", m.Spinner.View())
	}
	if len(repos) == 0 {
		if m.RepoFilter != "" || m.RepoGroupFilter != "" {
			return "\n  No repositories match the current filter(s).\n  Press [Esc] to clear filters.\n"
		}
		return "\n  No repositories found for " + m.CurrentOrg + ".\n"
	}

	if m.RepoCursor >= len(repos) {
		m.RepoCursor = len(repos) - 1
	}

	cols := computeRepoColumns(m.Width)
	tmuxSessions := m.TmuxSessions
	if tmuxSessions == nil {
		tmuxSessions = GetTmuxSessions()
	}

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		cols.Cursor.Render(""),
		cols.Name.Render("REPOSITORY"),
		cols.Sec.Render("SECURITY"),
		cols.Group.Render("GROUPS"),
		cols.Git.Render("GIT STATUS"),
		cols.Session.Render("TMUX"),
		cols.Last.Render("LAST PUSHED"),
		cols.Path.Render("LOCAL PATH"),
	)
	renderedHeader := TableHeaderStyle.Render(headerRow)

	var rows []string

	// Active filter indicator if filtering or sorting is on
	var filterBadges []string
	if m.RepoGroupFilter != "" {
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Render("Group: "+m.RepoGroupFilter))
	}
	if m.RepoFilter != "" {
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("Name: %q", m.RepoFilter)))
	}
	if m.RepoSortVulnerabilities {
		filterBadges = append(filterBadges, lipgloss.NewStyle().Foreground(WarningColor).Bold(true).Render("Sort: 🛡️ Most Vulns ↓"))
	}

	hotCount, warmCount, coldCount, deadCount := m.getRepoTierCounts()
	tierSummary := fmt.Sprintf("🔥 %d Hot  ⚡ %d Warm  ❄️ %d Cold  💀 %d Dead", hotCount, warmCount, coldCount, deadCount)

	if len(filterBadges) > 0 {
		filterSummary := MetaBarStyle.Render("  🔍 ") + strings.Join(filterBadges, "  ") +
			MetaBarStyle.Render(fmt.Sprintf("  (%d of %d repos) • %s • [Esc] Clear", len(repos), len(m.Repos), tierSummary))
		rows = append(rows, filterSummary)
	} else if len(m.Repos) > 0 {
		tierBar := MetaBarStyle.Render(fmt.Sprintf("  %s  (periodic polling focuses on Hot & Warm repos)", tierSummary))
		rows = append(rows, tierBar)
	}

	rows = append(rows, renderedHeader)

	visibleCount := m.Height - 9
	if visibleCount < 5 {
		visibleCount = 10
	}

	start := 0
	if m.RepoCursor >= visibleCount {
		start = m.RepoCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(repos) {
		end = len(repos)
	}

	for i := start; i < end; i++ {
		r := repos[i]
		st := m.GitStatuses[r.Name]

		// 1. Calculate open PR count for this repository
		prCount := 0
		for _, pr := range m.PRs {
			if strings.EqualFold(pr.Repo, r.Name) {
				prCount++
			}
		}

		tier := m.getRepoTier(r.Name)
		tierIcon := storage.TierIcon(tier)

		maxRepoLen := cols.NameWidth - 10
		if maxRepoLen < 6 {
			maxRepoLen = 6
		}
		repoDisplayName := r.Name
		if len(repoDisplayName) > maxRepoLen {
			repoDisplayName = repoDisplayName[:maxRepoLen-1] + "…"
		}

		var nameColText string
		if m.Loading {
			nameColText = fmt.Sprintf("%s (%s) %s", tierIcon, m.Spinner.View(), repoDisplayName)
		} else {
			var prBadge string
			if prCount > 0 {
				prBadge = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("(%d)", prCount))
			} else {
				prBadge = lipgloss.NewStyle().Foreground(MutedColor).Render("(0)")
			}
			nameColText = fmt.Sprintf("%s %s %s", tierIcon, prBadge, repoDisplayName)
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
		var groups []string
		if m.Config != nil {
			groups = m.Config.GetGroupsForRepo(r.Name)
		}
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

		var sessionColText string
		if sess, ok := FindTmuxSession(r.Name, tmuxSessions); ok && sess.Windows > 0 {
			plural := "win"
			if sess.Windows > 1 {
				plural = "wins"
			}
			sessionColText = lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("● %d %s", sess.Windows, plural))
		} else {
			sessionColText = lipgloss.NewStyle().Foreground(lipgloss.Color("#505565")).Render("○ none")
		}

		lastPushed := formatTimeAgo(r.LastUpdated())

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
			cols.Session.Render(sessionColText),
			cols.Last.Render(lastPushed),
			cols.Path.Render(localPath),
		)

		if i == m.RepoCursor {
			rows = append(rows, SelectedRowStyle.MaxHeight(1).Render(rowCells))
		} else {
			rows = append(rows, NormalRowStyle.MaxHeight(1).Render(rowCells))
		}
	}

	return strings.Join(rows, "\n")
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
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

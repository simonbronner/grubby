package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"grubby/internal/github"
)

func (m Model) handleModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.ActiveModal {
	case ModalManageGroups:
		return m.handleGroupsModalKeys(msg)
	case ModalSelectSquad:
		return m.handleSelectSquadModalKeys(msg)
	case ModalAddMember:
		return m.handleAddMemberModalKeys(msg)
	case ModalCreateSquad:
		return m.handleCreateSquadModalKeys(msg)
	case ModalRepoTabs:
		return m.handleRepoTabsModalKeys(msg)
	case ModalFilterGroup:
		return m.handleFilterGroupModalKeys(msg)
	case ModalFilterRepoName:
		return m.handleFilterNameModalKeys(msg)
	case ModalFilterPR:
		return m.handleFilterPRModalKeys(msg)
	case ModalManageSquad:
		return m.handleManageSquadModalKeys(msg)
	case ModalCreateTask:
		return m.handleCreateTaskModalKeys(msg)
	case ModalEditTask:
		return m.handleEditTaskModalKeys(msg)
	case ModalTaskNotes:
		return m.handleTaskNotesModalKeys(msg)
	case ModalCompletedTasks:
		return m.handleCompletedTasksModalKeys(msg)
	case ModalCreatePRTask:
		return m.handleCreatePRTaskModalKeys(msg)
	case ModalAssignSquadTask:
		return m.handleAssignSquadTaskModalKeys(msg)
	case ModalHelp:
		if msg.String() == "esc" || msg.String() == "q" || msg.String() == "?" {
			m.ActiveModal = ModalNone
		}
		return m, nil
	}
	return m, nil
}

func (m Model) renderModalOverlay(baseView string) string {
	var modalContent string
	switch m.ActiveModal {
	case ModalManageGroups:
		modalContent = m.renderManageGroupsModal()
	case ModalSelectSquad:
		modalContent = m.renderSelectSquadModal()
	case ModalAddMember:
		modalContent = m.renderAddMemberModal()
	case ModalCreateSquad:
		modalContent = m.renderCreateSquadModal()
	case ModalRepoTabs:
		modalContent = m.renderRepoTabsModal()
	case ModalFilterGroup:
		modalContent = m.renderFilterGroupModal()
	case ModalFilterRepoName:
		modalContent = m.renderFilterNameModal()
	case ModalFilterPR:
		modalContent = m.renderFilterPRModal()
	case ModalManageSquad:
		modalContent = m.renderManageSquadModal()
	case ModalCreateTask:
		modalContent = m.renderCreateTaskModal()
	case ModalEditTask:
		modalContent = m.renderEditTaskModal()
	case ModalTaskNotes:
		modalContent = m.renderTaskNotesModal()
	case ModalCompletedTasks:
		modalContent = m.renderCompletedTasksModal()
	case ModalCreatePRTask:
		modalContent = m.renderCreatePRTaskModal()
	case ModalAssignSquadTask:
		modalContent = m.renderAssignSquadTaskModal()
	case ModalHelp:
		modalContent = m.renderHelpModal()
	}

	if modalContent == "" {
		return baseView
	}

	return lipgloss.Place(
		m.Width,
		m.Height,
		lipgloss.Center,
		lipgloss.Center,
		modalContent,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#000000")),
	)
}

func (m Model) renderSelectSquadModal() string {
	title := ModalTitleStyle.Render("Switch Watched Squad")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Use [↑/↓] or [j/k] to choose, [Enter] to select, [m] to manage squad, [+] or [n] to create new squad, [Esc] to exit.\n")

	squads := m.Config.Squads
	if len(squads) == 0 {
		lines = append(lines, "  No squads defined yet. Press [+] or [n] to create one!")
	}

	for i, s := range squads {
		cursor := "  "
		if i == m.ModalCursor {
			cursor = "▶ "
		}

		watched := ""
		if strings.EqualFold(s.Name, m.Config.ActiveSquad) {
			watched = " (Active)"
		}

		line := fmt.Sprintf("%s%s%s - %d members", cursor, s.DisplayName, watched, len(s.Members))
		if i == m.ModalCursor {
			lines = append(lines, SelectedRowStyle.Render(line))
		} else {
			lines = append(lines, NormalRowStyle.Render(line))
		}
	}

	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleSelectSquadModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	squads := m.Config.Squads
	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		return m, nil
	case "+", "n":
		m.ActiveModal = ModalCreateSquad
		m.ModalInput.Reset()
		m.ModalInput.Placeholder = "New squad name (e.g. backend-squad)"
		m.ModalInput.Focus()
		return m, textinput.Blink
	case "m":
		if len(squads) > 0 && m.ModalCursor < len(squads) {
			m.ManagedSquadName = squads[m.ModalCursor].Name
			m.SquadManageMode = SquadModeOverview
			m.ActiveModal = ModalManageSquad
			return m, nil
		}
	case "up", "k":
		if m.ModalCursor > 0 {
			m.ModalCursor--
		}
	case "down", "j":
		if m.ModalCursor < len(squads)-1 {
			m.ModalCursor++
		}
	case "enter":
		if len(squads) > 0 && m.ModalCursor < len(squads) {
			selected := squads[m.ModalCursor]
			_ = m.Config.SetActiveSquad(selected.Name)
			m.ActiveModal = ModalNone

			// Trigger refresh of squad activity
			return m, tea.Batch(
				func() tea.Msg {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					events, _ := m.GHClient.FetchSquadActivity(ctx, selected.Members, nil)
					return DataLoadedMsg{
						Repos:       m.Repos,
						PRs:         m.PRs,
						SquadEvents: events,
						OrgMembers:  m.OrgMembers,
						Security:    m.SecuritySummaries,
						RateLimit:   m.GHClient.RateLimit(),
					}
				},
				m.fetchSquadAvatarsCmd(selected.Members),
			)
		}
	}
	return m, nil
}

func (m Model) renderCreateSquadModal() string {
	title := ModalTitleStyle.Render("Create New Squad")

	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Enter a name for the squad, then choose members from the organization.\n")
	lines = append(lines, "Squad Name: "+m.ModalInput.View()+"\n")
	lines = append(lines, MetaBarStyle.Render("Press [Enter] to create squad and choose members, [Esc] to cancel."))

	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleCreateSquadModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		m.ModalInput.Reset()
		m.ModalInput.Blur()
		return m, nil
	case "enter":
		squadName := strings.TrimSpace(m.ModalInput.Value())
		if squadName != "" {
			_ = m.Config.CreateSquad(squadName, squadName, nil)
			_ = m.Config.SetActiveSquad(squadName)
			// Transition directly to Member Selector modal
			m.ActiveModal = ModalAddMember
			m.ModalCursor = 0
			m.ModalInput.Reset()
			m.ModalInput.Placeholder = "Type to search members or enter username..."
			m.ModalInput.Focus()
			return m, func() tea.Msg {
				return StatusToastMsg(fmt.Sprintf("Created squad '%s'. Select members below!", squadName))
			}
		}
	default:
		var cmd tea.Cmd
		m.ModalInput, cmd = m.ModalInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) getFilteredOrgMembers() []github.OrgMember {
	filter := strings.ToLower(strings.TrimSpace(m.ModalInput.Value()))
	if filter == "" {
		return m.OrgMembers
	}
	var res []github.OrgMember
	for _, om := range m.OrgMembers {
		if strings.Contains(strings.ToLower(om.Login), filter) || strings.Contains(strings.ToLower(om.Name), filter) {
			res = append(res, om)
		}
	}
	return res
}

func (m Model) renderAddMemberModal() string {
	activeSquad := m.Config.GetActiveSquad()
	squadName := "Squad"
	if activeSquad != nil {
		squadName = activeSquad.DisplayName
	}

	title := ModalTitleStyle.Render(fmt.Sprintf("Add Member to '%s'", squadName))
	var lines []string
	lines = append(lines, title)
	lines = append(lines, fmt.Sprintf("Select an organization member from '%s' or enter a username.\n", m.CurrentOrg))
	lines = append(lines, "Search / Username: "+m.ModalInput.View()+"\n")

	filtered := m.getFilteredOrgMembers()
	if len(filtered) == 0 {
		typed := strings.TrimSpace(m.ModalInput.Value())
		if typed != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(SecondaryColor).Bold(true).Render(fmt.Sprintf("  ▶ Add '%s' directly (Press [Enter])", typed)))
		} else {
			lines = append(lines, "  No organization members found. Type a GitHub username and press [Enter].")
		}
	} else {
		visibleLimit := 8
		if m.ModalCursor >= len(filtered) {
			m.ModalCursor = len(filtered) - 1
		}
		if m.ModalCursor < 0 {
			m.ModalCursor = 0
		}

		start := 0
		if m.ModalCursor >= visibleLimit {
			start = m.ModalCursor - visibleLimit + 1
		}
		end := start + visibleLimit
		if end > len(filtered) {
			end = len(filtered)
		}

		squadMembersMap := make(map[string]bool)
		if activeSquad != nil {
			for _, mem := range activeSquad.Members {
				squadMembersMap[strings.ToLower(mem)] = true
			}
		}

		for i := start; i < end; i++ {
			om := filtered[i]
			cursor := "  "
			if i == m.ModalCursor {
				cursor = "▶ "
			}

			inSquad := squadMembersMap[strings.ToLower(om.Login)]
			tag := "[ ]"
			suffix := ""
			if inSquad {
				tag = "[✓]"
				suffix = lipgloss.NewStyle().Foreground(MutedColor).Render(" (In Squad)")
			}

			disp := om.Login
			if om.Name != "" && om.Name != om.Login {
				disp = fmt.Sprintf("%-16s (%s)", om.Login, om.Name)
			}
			line := fmt.Sprintf("%s%s %s%s", cursor, tag, disp, suffix)
			if i == m.ModalCursor {
				lines = append(lines, SelectedRowStyle.Render(line))
			} else {
				lines = append(lines, NormalRowStyle.Render(line))
			}
		}

		if len(filtered) > visibleLimit {
			lines = append(lines, MetaBarStyle.Render(fmt.Sprintf("  (Showing %d-%d of %d members)", start+1, end, len(filtered))))
		}
	}

	lines = append(lines, "\n"+MetaBarStyle.Render("[↑/↓] Select  [Enter/Space] Add Member  [Esc] Done"))
	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleAddMemberModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.getFilteredOrgMembers()

	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		m.ModalInput.Reset()
		m.ModalInput.Blur()
		return m, nil
	case "up", "k":
		if m.ModalCursor > 0 {
			m.ModalCursor--
		}
		return m, nil
	case "down", "j":
		if m.ModalCursor < len(filtered)-1 {
			m.ModalCursor++
		}
		return m, nil
	case "enter", " ":
		var username string
		if len(filtered) > 0 && m.ModalCursor < len(filtered) {
			username = filtered[m.ModalCursor].Login
		} else {
			username = strings.TrimSpace(m.ModalInput.Value())
		}

		if username != "" {
			squadName := "default-squad"
			if s := m.Config.GetActiveSquad(); s != nil {
				squadName = s.Name
			}
			_ = m.Config.AddMemberToSquad(squadName, username)
			m.ActiveModal = ModalNone
			m.ModalInput.Reset()
			m.ModalInput.Blur()

			// Refresh squad activity
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				squad := m.Config.GetActiveSquad()
				var members []string
				if squad != nil {
					members = squad.Members
				}
				events, _ := m.GHClient.FetchSquadActivity(ctx, members, nil)
				return DataLoadedMsg{
					Repos:       m.Repos,
					PRs:         m.PRs,
					SquadEvents: events,
					Security:    m.SecuritySummaries,
					RateLimit:   m.GHClient.RateLimit(),
				}
			}
		}
	default:
		prevVal := m.ModalInput.Value()
		var cmd tea.Cmd
		m.ModalInput, cmd = m.ModalInput.Update(msg)
		if m.ModalInput.Value() != prevVal {
			m.ModalCursor = 0
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) renderHelpModal() string {
	title := ModalTitleStyle.Render("Grubby Keyboard Shortcuts")

	helpText := `
Global Navigation:
  [1-6] / [Tab]       Switch between primary tabs
  [r]                 Refresh data from GitHub & GitHub Status
  [?]                 Toggle this help dialog
  [q] / [Ctrl+C]      Quit

Repositories Tab (1):
  [j/k] or [↑/↓]      Select repository
  [/]                 Filter repositories by name or group
  [v]                 Sort by vulnerabilities (toggle most to least)
  [s]                 Switch to / open tmux session in Grubby window (auto-clones if needed)
  [e]                 Open editor in new window in tmux session (auto-clones if needed)
  [t]                 Open shell / new window in tmux session (auto-clones if needed)
  [Esc]               Clear active filter and sorting
  [Enter]             Open repo in dedicated browser
  [o]                 Open GitHub Web Tabs Submenu (Issues, PRs, Actions, Security...)
  [S]                 Open Security (Dependabot) tab directly in browser
  [g]                 Manage Groups (assign / create groups)
  [c]                 Clone repo to ~/workspaces/
  [u]                 Fast-forward clean behind repo

Pull Requests Tab (2):
  [j/k] or [↑/↓]      Select pull request
  [t]                 Create PR review task in squad tasks
  [f]                 Filter PRs by title, repo, or author
  [o]                 Open PR in browser
  [p]                 Trigger GitHub Copilot PR review
  [c]                 Checkout PR branch in local workspace

Tasks Tab:
  [j/k] or [↑/↓]      Select task
  [a]                 Add new task inline
  [Enter]             Edit task title (or Open PR directly if PR review task)
  [o]                 Open in browser (PR if review task, issue otherwise)
  [i]                 Always open underlying GitHub issue in browser
  [Space]             Toggle completed / active
  [A]                 Assign task to multiple squad members (squad tasks)
  [n]                 View / edit notes
  [p]                 Cycle priority (High / Medium / Low)
  [c]                 Cycle assignee
  [d]                 Show all completed tasks modal

Squad Activity Tab (3):
  [m]                 Manage squad (add/delete multiple members, rename, delete squad)
  [s]                 Switch watched squad
  [n]                 Create new squad
  [a]                 Add members from organization or username
  [d]                 Remove members from squad
  [o]                 Open event target in browser

Code Search Tab (4):
  [/]                 Focus search query input
  [j/k] or [↑/↓]      Navigate results tree
  [h/l] or [Space]    Expand / Collapse repo or file branch
  [Tab]               Toggle focus between Tree and Code Preview
  [Enter]             Open match in $EDITOR at exact line number

Notifications Tab (5):
  [j/k] or [↑/↓]      Select notification
  [Enter] / [o]       Open in browser and mark as read
  [m]                 Mark selected notification as read
  [M]                 Mark all notifications as read

GitHub Status Tab (6):
  [j/k] or [↑/↓]      Select service component
  [o] / [Enter]       Open githubstatus.com in browser
  [r]                 Refresh GitHub status now

Press [Esc] or [q] to close this help window.
`

	return ModalBoxStyle.Render(title + "\n" + helpText)
}

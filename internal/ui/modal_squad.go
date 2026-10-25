package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) handleModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.ActiveModal {
	case ModalManageGroups:
		return m.handleGroupsModalKeys(msg)
	case ModalSelectSquad:
		return m.handleSelectSquadModalKeys(msg)
	case ModalAddMember:
		return m.handleAddMemberModalKeys(msg)
	case ModalRepoTabs:
		return m.handleRepoTabsModalKeys(msg)
	case ModalFilterGroup:
		return m.handleFilterGroupModalKeys(msg)
	case ModalFilterRepoName:
		return m.handleFilterNameModalKeys(msg)
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
	case ModalRepoTabs:
		modalContent = m.renderRepoTabsModal()
	case ModalFilterGroup:
		modalContent = m.renderFilterGroupModal()
	case ModalFilterRepoName:
		modalContent = m.renderFilterNameModal()
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
	lines = append(lines, "Use [↑/↓] or [j/k] to choose, [Enter] to select, [Esc] to cancel.\n")

	squads := m.Config.Squads
	if len(squads) == 0 {
		lines = append(lines, "  No squads defined yet. Press [Esc] and then [a] to create one!")
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
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				events, _ := m.GHClient.FetchSquadActivity(ctx, selected.Members, nil)
				return DataLoadedMsg{
					Repos:       m.Repos,
					PRs:         m.PRs,
					SquadEvents: events,
					Security:    m.SecuritySummaries,
					RateLimit:   m.GHClient.RateLimit(),
				}
			}
		}
	}
	return m, nil
}

func (m Model) renderAddMemberModal() string {
	activeSquad := m.Config.GetActiveSquad()
	squadName := "New Squad"
	if activeSquad != nil {
		squadName = activeSquad.DisplayName
	}

	title := ModalTitleStyle.Render(fmt.Sprintf("Add Member to '%s'", squadName))
	var lines []string
	lines = append(lines, title)
	lines = append(lines, "Enter GitHub handle, press [Enter] to add, [Esc] to cancel.\n")
	lines = append(lines, m.ModalInput.View())

	return ModalBoxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) handleAddMemberModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.ActiveModal = ModalNone
		m.ModalInput.Reset()
		m.ModalInput.Blur()
		return m, nil
	case "enter":
		username := strings.TrimSpace(m.ModalInput.Value())
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
	}

	var cmd tea.Cmd
	m.ModalInput, cmd = m.ModalInput.Update(msg)
	return m, cmd
}

func (m Model) renderHelpModal() string {
	title := ModalTitleStyle.Render("Grubby Keyboard Shortcuts")

	helpText := `
Global Navigation:
  [1-5] / [Tab]       Switch between primary tabs
  [r]                 Refresh data from GitHub
  [?]                 Toggle this help dialog
  [q] / [Ctrl+C]      Quit

Repositories Tab (1):
  [j/k] or [↑/↓]      Select repository
  [v]                 View by Group (filter repositories by group)
  [f]                 Filter repositories by name
  [s]                 Sort by vulnerabilities (highest to lowest toggle)
  [Esc]               Clear active group, name filters, and sorting
  [Enter]             Open repo in dedicated browser
  [o]                 Open GitHub Web Tabs Submenu (Issues, PRs, Actions, Security...)
  [S]                 Open Security (Dependabot) tab directly in browser
  [g]                 Manage Groups (assign / create groups)
  [c]                 Clone repo to ~/workspaces/
  [e]                 Open in $EDITOR (Neovim/VS Code)
  [t]                 Open in shell (iTerm2 on macOS)
  [u]                 Fast-forward clean behind repo

Pull Requests Tab (2):
  [j/k] or [↑/↓]      Select pull request
  [o]                 Open PR in browser
  [p]                 Trigger GitHub Copilot PR review
  [c]                 Checkout PR branch in local workspace

Squad Activity Tab (3):
  [s]                 Switch watched squad
  [a]                 Add GitHub user to watched squad
  [d]                 Remove user from watched squad
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

Press [Esc] or [q] to close this help window.
`

	return ModalBoxStyle.Render(title + "\n" + helpText)
}

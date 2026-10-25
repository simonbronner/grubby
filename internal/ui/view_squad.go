package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	squadColCursor  = lipgloss.NewStyle().Width(3)
	squadColTime    = lipgloss.NewStyle().Width(12)
	squadColActor   = lipgloss.NewStyle().Width(16)
	squadColAction  = lipgloss.NewStyle().Width(18)
	squadColTarget  = lipgloss.NewStyle().Width(28)
	squadColDetails = lipgloss.NewStyle().Width(45)
)

func (m Model) handleSquadKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.SquadCursor > 0 {
			m.SquadCursor--
		}
	case "down", "j":
		if m.SquadCursor < len(m.SquadEvents)-1 {
			m.SquadCursor++
		}
	case "s":
		// Open Squad switch modal
		m.ActiveModal = ModalSelectSquad
		m.ModalCursor = 0
	case "a":
		// Open Add member modal
		m.ActiveModal = ModalAddMember
		m.ModalInput.Reset()
		m.ModalInput.Placeholder = "GitHub username (e.g. alice)"
		m.ModalInput.Focus()
	case "d":
		// Remove selected member if on squad list
		activeSquad := m.Config.GetActiveSquad()
		if activeSquad != nil && len(activeSquad.Members) > 0 {
			lastMember := activeSquad.Members[len(activeSquad.Members)-1]
			_ = m.Config.RemoveMemberFromSquad(activeSquad.Name, lastMember)
			return m, func() tea.Msg { return StatusToastMsg("Removed " + lastMember + " from " + activeSquad.Name) }
		}
	case "o":
		if len(m.SquadEvents) > 0 && m.SquadCursor < len(m.SquadEvents) {
			ev := m.SquadEvents[m.SquadCursor]
			url := fmt.Sprintf("https://github.com/%s", ev.Repo)
			_ = OpenBrowser(url)
			return m, func() tea.Msg { return StatusToastMsg("Opened " + ev.Repo + " in browser") }
		}
	}
	return m, nil
}

func (m Model) renderSquadView() string {
	activeSquad := m.Config.GetActiveSquad()
	squadName := "None Configured"
	membersList := "None"

	if activeSquad != nil {
		squadName = activeSquad.DisplayName
		if len(activeSquad.Members) > 0 {
			membersList = strings.Join(activeSquad.Members, ", ")
		}
	}

	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(SecondaryColor).
		Render(fmt.Sprintf("  Watched Squad: %s", squadName)) +
		MetaBarStyle.Render(fmt.Sprintf("  (Members: %s)", membersList)) + "\n"

	if len(m.SquadEvents) == 0 {
		return banner + "\n  No recent activity recorded for squad members.\n  Press 'a' to add members to this squad, or 's' to switch squads.\n"
	}

	headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
		squadColCursor.Render(""),
		squadColTime.Render("TIME"),
		squadColActor.Render("ACTOR"),
		squadColAction.Render("ACTION"),
		squadColTarget.Render("TARGET"),
		squadColDetails.Render("DETAILS"),
	)
	renderedHeader := TableHeaderStyle.Render(headerRow)

	var rows []string
	rows = append(rows, banner, renderedHeader)

	visibleCount := m.Height - 10
	if visibleCount < 5 {
		visibleCount = 10
	}

	start := 0
	if m.SquadCursor >= visibleCount {
		start = m.SquadCursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(m.SquadEvents) {
		end = len(m.SquadEvents)
	}

	for i := start; i < end; i++ {
		ev := m.SquadEvents[i]
		timeAgo := formatTimeAgo(ev.Timestamp)

		prefix := "  "
		if i == m.SquadCursor {
			prefix = "▶ "
		}

		target := ev.Target
		if len(target) > 26 {
			target = target[:24] + "…"
		}

		details := ev.Details
		if len(details) > 42 {
			details = details[:39] + "..."
		}

		rowCells := lipgloss.JoinHorizontal(lipgloss.Top,
			squadColCursor.Render(prefix),
			squadColTime.Render(timeAgo),
			squadColActor.Render(ev.Actor),
			squadColAction.Render(ev.Action),
			squadColTarget.Render(target),
			squadColDetails.Render(details),
		)

		if i == m.SquadCursor {
			rows = append(rows, SelectedRowStyle.Render(rowCells))
		} else {
			rows = append(rows, NormalRowStyle.Render(rowCells))
		}
	}

	return strings.Join(rows, "\n")
}
